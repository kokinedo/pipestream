package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kokinedo/pipestream/pkg/models"
)

// NewEventsMsg carries newly classified events to the TUI.
type NewEventsMsg []models.ClassifiedEvent

// StatsUpdateMsg carries updated pipeline statistics.
type StatsUpdateMsg models.PipelineStats

// TickMsg fires on regular intervals for stats refresh.
type TickMsg time.Time

func renderStatsBar(stats models.PipelineStats, width int) string {
	uptimeMin := int(stats.UptimeSeconds / 60)
	uptimeSec := int(stats.UptimeSeconds) % 60

	left := lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render(
		fmt.Sprintf(" PIPESTREAM "),
	)
	mid := lipgloss.NewStyle().Foreground(colorWhite).Render(
		fmt.Sprintf("  Ingested: %d  |  Classified: %d  |  %.1f evt/s",
			stats.EventsIngested, stats.EventsClassified, stats.EventsPerSecond),
	)
	right := lipgloss.NewStyle().Foreground(colorDim).Render(
		fmt.Sprintf("  Uptime: %02d:%02d ", uptimeMin, uptimeSec),
	)

	content := left + mid + right
	return HeaderStyle.Width(width).Render(content)
}

func renderEventItem(event models.ClassifiedEvent, selected bool, width int) string {
	icon := event.Category.CategoryIcon()
	catStyle := CategoryStyle(event.Category, event.Interestingness)

	scoreBar := renderScoreBar(event.Interestingness)

	line := fmt.Sprintf("%s %s %s %s",
		catStyle.Render(icon),
		catStyle.Render(truncate(event.RepoName, 30)),
		lipgloss.NewStyle().Foreground(colorDim).Render(scoreBar),
		lipgloss.NewStyle().Foreground(colorDim).Render(truncate(event.Summary, width-50)),
	)

	if selected {
		return SelectedStyle.Width(width - 4).Render(line)
	}
	return lipgloss.NewStyle().Width(width - 4).Render(line)
}

func renderScoreBar(score int) string {
	filled := score
	empty := 10 - score
	return "[" + strings.Repeat("#", filled) + strings.Repeat(".", empty) + "]"
}

func renderEventDetail(event models.ClassifiedEvent, width, height int) string {
	if event.ID == "" {
		return lipgloss.NewStyle().
			Foreground(colorDim).
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Select an event to view details")
	}

	catStyle := CategoryStyle(event.Category, event.Interestingness)

	title := lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(event.RepoName)
	category := catStyle.Render(fmt.Sprintf("%s %s", event.Category.CategoryIcon(), string(event.Category)))
	score := lipgloss.NewStyle().Foreground(colorYellow).Render(
		fmt.Sprintf("Interestingness: %d/10 %s", event.Interestingness, renderScoreBar(event.Interestingness)),
	)

	details := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		lipgloss.NewStyle().Foreground(colorDim).Render("Type: ")+event.EventType,
		lipgloss.NewStyle().Foreground(colorDim).Render("Actor: ")+event.ActorLogin,
		lipgloss.NewStyle().Foreground(colorDim).Render("Category: ")+category,
		score,
		"",
		lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Render("Summary"),
		lipgloss.NewStyle().Foreground(colorWhite).Width(width-6).Render(event.Summary),
		"",
		lipgloss.NewStyle().Foreground(colorDim).Render(
			fmt.Sprintf("Created: %s  |  Classified: %s",
				event.CreatedAt.Format("15:04:05"),
				event.ClassifiedAt.Format("15:04:05"),
			),
		),
	)

	return HighlightPanelStyle.Width(width).Height(height).Render(details)
}

func renderHelpBar(width int) string {
	help := lipgloss.NewStyle().Foreground(colorDim).Render(
		" j/k: navigate  |  tab: switch panel  |  enter: select  |  q: quit",
	)
	return StatusBarStyle.Width(width).Render(help)
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
