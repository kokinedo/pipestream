package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/kokinedo/pipestream/internal/auth"
	"github.com/kokinedo/pipestream/internal/classifier"
	"github.com/kokinedo/pipestream/internal/ingester"
	"github.com/kokinedo/pipestream/internal/server"
	"github.com/kokinedo/pipestream/internal/store"
	"github.com/kokinedo/pipestream/internal/tui"
	"github.com/kokinedo/pipestream/pkg/models"
)

var (
	headless     bool
	port         int
	dbPath       string
	dryRun       bool
	pollInterval time.Duration
	modelFlag    string
	providerFlag string
)

var rootCmd = &cobra.Command{
	Use:   "pipestream",
	Short: "Real-time GitHub event pipeline with AI classification and TUI dashboard",
	Long: `Pipestream ingests GitHub public events, classifies them using Claude AI,
and presents them in a beautiful terminal dashboard. Events are persisted to SQLite
and also available via a REST/WebSocket API.`,
	RunE: runPipeline,
}

func init() {
	rootCmd.Flags().BoolVar(&headless, "headless", false, "Run without TUI (API server only)")
	rootCmd.Flags().IntVar(&port, "port", 8080, "HTTP API server port")
	rootCmd.Flags().StringVar(&dbPath, "db", "pipestream.db", "SQLite database path")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Skip Claude API calls, assign random classifications")
	rootCmd.Flags().DurationVar(&pollInterval, "poll-interval", 30*time.Second, "GitHub API poll interval")
	rootCmd.Flags().StringVar(&modelFlag, "model", "", "AI model for classification (default depends on provider)")
	rootCmd.Flags().StringVar(&providerFlag, "provider", "", "AI provider: claude, openai, gemini (default from credentials)")

	loginCmd.Flags().String("provider", "claude", "Provider: claude, openai, gemini")
	logoutCmd.Flags().String("provider", "claude", "Provider: claude, openai, gemini")

	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with an AI provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := cmd.Flags().GetString("provider")
		return auth.Login(p)
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored credentials for a provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := cmd.Flags().GetString("provider")
		return auth.Logout(p)
	},
}

// Execute is the main entry point.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runPipeline(cmd *cobra.Command, args []string) error {
	// Resolve provider and API key.
	provider := providerFlag
	if provider == "" {
		provider = auth.GetDefaultProvider()
	}
	apiKey := auth.GetAPIKey(provider)
	if apiKey == "" && !dryRun {
		fmt.Fprintf(os.Stderr, "Error: No API key for provider '%s'.\n", provider)
		fmt.Fprintf(os.Stderr, "Run: pipestream login --provider %s\n", provider)
		fmt.Fprintln(os.Stderr, "Or run with --dry-run to skip AI classification.")
		os.Exit(1)
	}
	model := modelFlag
	if model == "" {
		switch provider {
		case "openai":
			model = "gpt-4o"
		case "gemini":
			model = "gemini-2.0-flash"
		default:
			model = "claude-sonnet-4-20250514"
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Initialize store.
	st, err := store.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	defer st.Close()

	// Create channels.
	rawCh := make(chan []models.RawGitHubEvent, 10)
	classifiedCh := make(chan []models.ClassifiedEvent, 10)

	// Create event bus.
	bus := server.NewEventBus()

	// Create ingester.
	ing := ingester.NewIngester(ingester.Config{
		PollInterval: pollInterval,
		GitHubToken:  os.Getenv("GITHUB_TOKEN"),
	}, rawCh)

	// Create classifier.
	cls := classifier.NewClassifier(classifier.Config{
		APIKey:       apiKey,
		Model:        model,
		DryRun:       dryRun,
		ProviderName: provider,
	}, rawCh, classifiedCh)

	// Create server.
	srv := server.NewServer(st, port, bus)

	startTime := time.Now()
	var totalIngested int64
	var totalClassified int64

	g, gctx := errgroup.WithContext(ctx)

	// Start ingester.
	g.Go(func() error {
		return ing.Start(gctx)
	})

	// Start classifier.
	g.Go(func() error {
		return cls.Start(gctx)
	})

	// Start HTTP server.
	g.Go(func() error {
		return srv.Start(gctx)
	})

	// Persist + broadcast goroutine.
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return gctx.Err()
			case events, ok := <-classifiedCh:
				if !ok {
					return nil
				}
				if err := st.SaveEvents(gctx, events); err != nil {
					log.Printf("[persist] save error: %v", err)
					continue
				}
				atomic.AddInt64(&totalClassified, int64(len(events)))
				for i := range events {
					e := events[i]
					bus.Publish(&e)
				}
			}
		}
	})

	// Stats updater goroutine (used by TUI).
	statsFn := func() models.PipelineStats {
		ingested := ing.IngestedCount()
		classified := atomic.LoadInt64(&totalClassified)
		atomic.StoreInt64(&totalIngested, ingested)
		uptime := time.Since(startTime).Seconds()
		eps := 0.0
		if uptime > 0 {
			eps = float64(classified) / uptime
		}
		return models.PipelineStats{
			EventsIngested:   ingested,
			EventsClassified: classified,
			EventsPerSecond:  eps,
			UptimeSeconds:    uptime,
		}
	}

	if !headless {
		// Subscribe TUI to the event bus.
		tuiCh, unsub := bus.Subscribe()
		defer unsub()

		app := tui.NewApp(tuiCh)
		p := tea.NewProgram(app, tea.WithAltScreen())

		// Stats ticker for TUI.
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-gctx.Done():
					return
				case <-ticker.C:
					p.Send(tui.StatsUpdateMsg(statsFn()))
				}
			}
		}()

		// Run TUI (blocks).
		g.Go(func() error {
			if _, err := p.Run(); err != nil {
				return err
			}
			cancel() // TUI quit -> shut down everything.
			return nil
		})
	} else {
		log.Println("[pipestream] running in headless mode")
		log.Printf("[pipestream] API server at http://localhost:%d", port)

		// Stats logger in headless mode.
		g.Go(func() error {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-gctx.Done():
					return gctx.Err()
				case <-ticker.C:
					s := statsFn()
					log.Printf("[stats] ingested=%d classified=%d eps=%.1f uptime=%.0fs",
						s.EventsIngested, s.EventsClassified, s.EventsPerSecond, s.UptimeSeconds)
				}
			}
		})
	}

	return g.Wait()
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Fetch and display pipeline stats from the API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		url := fmt.Sprintf("http://localhost:%d/api/stats", port)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to connect to pipestream server: %w", err)
		}
		defer resp.Body.Close()

		var data map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	},
}
