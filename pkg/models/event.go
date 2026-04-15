package models

import (
	"encoding/json"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RawGitHubEvent represents an event from the GitHub Events API.
type RawGitHubEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Actor     Actor           `json:"actor"`
	Repo      Repo            `json:"repo"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// Actor is the user who triggered the event.
type Actor struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
}

// Repo is the repository associated with the event.
type Repo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Category represents the AI-assigned classification of an event.
type Category string

const (
	CategoryNotableRelease  Category = "notable_release"
	CategoryInteresting     Category = "interesting_project"
	CategorySecurityConcern Category = "security_concern"
	CategoryTrending        Category = "trending"
	CategoryRoutine         Category = "routine"
)

// AllCategories returns all valid categories.
func AllCategories() []Category {
	return []Category{
		CategoryNotableRelease,
		CategoryInteresting,
		CategorySecurityConcern,
		CategoryTrending,
		CategoryRoutine,
	}
}

// ClassifiedEvent is a GitHub event after AI classification.
type ClassifiedEvent struct {
	ID              string    `json:"id" db:"id"`
	EventType       string    `json:"event_type" db:"event_type"`
	ActorLogin      string    `json:"actor_login" db:"actor_login"`
	RepoName        string    `json:"repo_name" db:"repo_name"`
	Category        Category  `json:"category" db:"category"`
	Interestingness int       `json:"interestingness" db:"interestingness"`
	Summary         string    `json:"summary" db:"summary"`
	RawPayload      string    `json:"-" db:"raw_payload"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	ClassifiedAt    time.Time `json:"classified_at" db:"classified_at"`
}

// PipelineStats holds runtime statistics for the pipeline.
type PipelineStats struct {
	EventsIngested   int64   `json:"events_ingested"`
	EventsClassified int64   `json:"events_classified"`
	EventsPerSecond  float64 `json:"events_per_second"`
	UptimeSeconds    float64 `json:"uptime_seconds"`
}

// CategoryIcon returns a terminal-friendly icon for the category.
func (c Category) CategoryIcon() string {
	switch c {
	case CategoryNotableRelease:
		return "[R]"
	case CategoryInteresting:
		return "[*]"
	case CategorySecurityConcern:
		return "[!]"
	case CategoryTrending:
		return "[^]"
	case CategoryRoutine:
		return "[-]"
	default:
		return "[?]"
	}
}

// CategoryColor returns a lipgloss.Color for the category.
func (c Category) CategoryColor() lipgloss.Color {
	switch c {
	case CategoryNotableRelease:
		return lipgloss.Color("#FFCC00") // yellow
	case CategoryInteresting:
		return lipgloss.Color("#00CC66") // green
	case CategorySecurityConcern:
		return lipgloss.Color("#FF3333") // red
	case CategoryTrending:
		return lipgloss.Color("#3399FF") // blue
	case CategoryRoutine:
		return lipgloss.Color("#888888") // gray
	default:
		return lipgloss.Color("#AAAAAA")
	}
}
