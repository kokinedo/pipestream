package tui

import "github.com/kokinedo/pipestream/pkg/models"

// NewMockApp creates an App with mock data for screenshots.
func NewMockApp(events []models.ClassifiedEvent, stats models.PipelineStats, width, height, selectedIdx int) *App {
	return &App{
		events:      events,
		stats:       stats,
		width:       width,
		height:      height,
		selectedIdx: selectedIdx,
		ready:       true,
	}
}
