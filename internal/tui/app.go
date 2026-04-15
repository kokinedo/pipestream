package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kokinedo/pipestream/pkg/models"
)

const maxEvents = 500

// App is the main bubbletea model for the TUI dashboard.
type App struct {
	events      []models.ClassifiedEvent
	selectedIdx int
	focusPanel  int // 0=list, 1=detail
	width       int
	height      int
	stats       models.PipelineStats
	eventsChan  <-chan *models.ClassifiedEvent
	ready       bool
}

// NewApp creates a new TUI application.
func NewApp(eventsChan <-chan *models.ClassifiedEvent) *App {
	return &App{
		eventsChan: eventsChan,
	}
}

// Init returns initial commands.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		waitForEvent(a.eventsChan),
		tickCmd(),
	)
}

// Update handles messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "j", "down":
			if a.focusPanel == 0 && a.selectedIdx < len(a.events)-1 {
				a.selectedIdx++
			}
		case "k", "up":
			if a.focusPanel == 0 && a.selectedIdx > 0 {
				a.selectedIdx--
			}
		case "tab":
			a.focusPanel = (a.focusPanel + 1) % 2
		case "enter":
			// Select current item (focus switches to detail).
			a.focusPanel = 1
		}
		return a, nil

	case NewEventsMsg:
		for i := len(msg) - 1; i >= 0; i-- {
			a.events = append([]models.ClassifiedEvent{msg[i]}, a.events...)
		}
		if len(a.events) > maxEvents {
			a.events = a.events[:maxEvents]
		}
		return a, waitForEvent(a.eventsChan)

	case StatsUpdateMsg:
		a.stats = models.PipelineStats(msg)
		return a, nil

	case TickMsg:
		return a, tickCmd()
	}

	return a, nil
}

// View renders the entire TUI.
func (a *App) View() string {
	if !a.ready {
		return "Initializing pipestream..."
	}

	w := a.width
	h := a.height

	// Stats bar at top (1 line + padding).
	statsBar := renderStatsBar(a.stats, w)
	statsH := lipgloss.Height(statsBar)

	// Help bar at bottom (1 line).
	helpBar := renderHelpBar(w)
	helpH := lipgloss.Height(helpBar)

	// Available height for main content.
	contentH := h - statsH - helpH - 1
	if contentH < 5 {
		contentH = 5
	}

	// Split width: 55% event list, 45% detail.
	listW := w*55/100 - 2
	detailW := w - listW - 4

	// Render event list.
	var listItems []string
	visibleCount := contentH - 2 // account for border
	if visibleCount < 1 {
		visibleCount = 1
	}
	start := 0
	if a.selectedIdx >= visibleCount {
		start = a.selectedIdx - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(a.events) {
		end = len(a.events)
	}
	for i := start; i < end; i++ {
		listItems = append(listItems, renderEventItem(a.events[i], i == a.selectedIdx, listW))
	}
	if len(listItems) == 0 {
		listItems = append(listItems, lipgloss.NewStyle().
			Foreground(colorDim).
			Width(listW).
			Align(lipgloss.Center).
			Render("Waiting for events..."))
	}
	listContent := lipgloss.JoinVertical(lipgloss.Left, listItems...)
	listPanel := EventFeedStyle.Width(listW).Height(contentH).Render(listContent)

	// Render detail panel.
	var selectedEvent models.ClassifiedEvent
	if len(a.events) > 0 && a.selectedIdx < len(a.events) {
		selectedEvent = a.events[a.selectedIdx]
	}
	detailPanel := renderEventDetail(selectedEvent, detailW, contentH)

	// Compose main content.
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, detailPanel)

	return lipgloss.JoinVertical(lipgloss.Left, statsBar, mainContent, helpBar)
}

// waitForEvent is a tea.Cmd that blocks until an event arrives on the channel.
func waitForEvent(ch <-chan *models.ClassifiedEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return nil
		}
		return NewEventsMsg{*event}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// UpdateStats allows the main loop to push stats updates into the TUI.
func (a *App) UpdateStats(stats models.PipelineStats) tea.Cmd {
	return func() tea.Msg {
		return StatsUpdateMsg(stats)
	}
}
