package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/kokinedo/pipestream/pkg/models"
)

var (
	// Color palette.
	colorGreen  = lipgloss.Color("#00CC66")
	colorRed    = lipgloss.Color("#FF3333")
	colorBlue   = lipgloss.Color("#3399FF")
	colorYellow = lipgloss.Color("#FFCC00")
	colorGray   = lipgloss.Color("#888888")
	colorWhite  = lipgloss.Color("#FFFFFF")
	colorDim    = lipgloss.Color("#555555")
	colorBg     = lipgloss.Color("#1a1a2e")
	colorPanel  = lipgloss.Color("#16213e")

	// HeaderStyle for the top bar.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(lipgloss.Color("#0f3460")).
			Padding(0, 2).
			Align(lipgloss.Center)

	// EventFeedStyle for the left event list panel.
	EventFeedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDim).
			Padding(0, 1)

	// HighlightPanelStyle for the right detail panel.
	HighlightPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBlue).
				Padding(1, 2)

	// StatusBarStyle for the bottom bar.
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 1)

	// SelectedStyle for the currently selected event.
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#2a2a4a")).
			Foreground(colorWhite)
)

// CategoryStyle returns a styled lipgloss.Style for a category with intensity
// based on the interestingness score.
func CategoryStyle(cat models.Category, score int) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(score >= 7)
	color := cat.CategoryColor()
	return base.Foreground(color)
}
