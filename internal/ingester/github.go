package ingester

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/kokinedo/pipestream/pkg/models"
)

// Config holds ingester configuration.
type Config struct {
	PollInterval time.Duration
	UserAgent    string
	GitHubToken  string
}

// Ingester polls the GitHub public events API and sends new events downstream.
type Ingester struct {
	cfg   Config
	out   chan<- []models.RawGitHubEvent
	seen  map[string]struct{}
	ring  []string
	head  int
	count int64
}

const dedupeCapacity = 10000

// NewIngester creates an Ingester that writes batches of new events to out.
func NewIngester(cfg Config, out chan<- []models.RawGitHubEvent) *Ingester {
	if cfg.UserAgent == "" {
		cfg.UserAgent = "pipestream/1.0"
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 30 * time.Second
	}
	return &Ingester{
		cfg:  cfg,
		out:  out,
		seen: make(map[string]struct{}, dedupeCapacity),
		ring: make([]string, dedupeCapacity),
	}
}

// IngestedCount returns the total number of events ingested so far.
func (i *Ingester) IngestedCount() int64 {
	return atomic.LoadInt64(&i.count)
}

// Start begins polling and blocks until the context is cancelled.
func (i *Ingester) Start(ctx context.Context) error {
	ticker := time.NewTicker(i.cfg.PollInterval)
	defer ticker.Stop()

	// Do an initial poll immediately.
	i.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			i.poll(ctx)
		}
	}
}

func (i *Ingester) poll(ctx context.Context) {
	events, err := i.fetchEvents(ctx)
	if err != nil {
		log.Printf("[ingester] fetch error: %v", err)
		return
	}

	var newEvents []models.RawGitHubEvent
	for _, e := range events {
		if _, ok := i.seen[e.ID]; ok {
			continue
		}
		i.addToSeen(e.ID)
		newEvents = append(newEvents, e)
	}

	if len(newEvents) == 0 {
		return
	}

	atomic.AddInt64(&i.count, int64(len(newEvents)))

	select {
	case i.out <- newEvents:
	case <-ctx.Done():
	}
}

func (i *Ingester) addToSeen(id string) {
	// Evict the oldest entry from the ring buffer.
	old := i.ring[i.head]
	if old != "" {
		delete(i.seen, old)
	}
	i.ring[i.head] = id
	i.seen[id] = struct{}{}
	i.head = (i.head + 1) % dedupeCapacity
}

func (i *Ingester) fetchEvents(ctx context.Context) ([]models.RawGitHubEvent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/events?per_page=30", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", i.cfg.UserAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if i.cfg.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+i.cfg.GitHubToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check rate limit.
	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if n, _ := strconv.Atoi(remaining); n < 5 {
			resetStr := resp.Header.Get("X-RateLimit-Reset")
			resetUnix, _ := strconv.ParseInt(resetStr, 10, 64)
			sleepUntil := time.Unix(resetUnix, 0)
			wait := time.Until(sleepUntil)
			if wait > 0 && wait < 10*time.Minute {
				log.Printf("[ingester] rate limit low (%d remaining), backing off %v", n, wait)
				select {
				case <-time.After(wait):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	var events []models.RawGitHubEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("decode events: %w", err)
	}
	return events, nil
}
