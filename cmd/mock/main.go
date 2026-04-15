package main

import (
	"fmt"
	"time"

	"github.com/kokinedo/pipestream/internal/tui"
	"github.com/kokinedo/pipestream/pkg/models"
)

func main() {
	now := time.Now()

	events := []models.ClassifiedEvent{
		{
			ID:              "1",
			EventType:       "PushEvent",
			ActorLogin:      "k8s-ci-robot",
			RepoName:        "kubernetes/kubernetes",
			Category:        models.CategoryNotableRelease,
			Interestingness: 9,
			Summary:         "Major v1.32 release with new pod scheduling improvements",
			CreatedAt:       now.Add(-2 * time.Minute),
			ClassifiedAt:    now.Add(-1 * time.Minute),
		},
		{
			ID:              "2",
			EventType:       "CreateEvent",
			ActorLogin:      "openai-bot",
			RepoName:        "openai/tiktoken",
			Category:        models.CategoryInteresting,
			Interestingness: 8,
			Summary:         "New fast BPE tokenizer with multi-language support",
			CreatedAt:       now.Add(-3 * time.Minute),
			ClassifiedAt:    now.Add(-2 * time.Minute),
		},
		{
			ID:              "3",
			EventType:       "ReleaseEvent",
			ActorLogin:      "rust-lang-bot",
			RepoName:        "rust-lang/rust",
			Category:        models.CategoryNotableRelease,
			Interestingness: 9,
			Summary:         "Rust 1.82 released with async trait stabilization",
			CreatedAt:       now.Add(-5 * time.Minute),
			ClassifiedAt:    now.Add(-4 * time.Minute),
		},
		{
			ID:              "4",
			EventType:       "IssuesEvent",
			ActorLogin:      "security-reporter",
			RepoName:        "lodash/lodash",
			Category:        models.CategorySecurityConcern,
			Interestingness: 7,
			Summary:         "Critical prototype pollution vulnerability reported",
			CreatedAt:       now.Add(-6 * time.Minute),
			ClassifiedAt:    now.Add(-5 * time.Minute),
		},
		{
			ID:              "5",
			EventType:       "PushEvent",
			ActorLogin:      "timneutkens",
			RepoName:        "vercel/next.js",
			Category:        models.CategoryTrending,
			Interestingness: 8,
			Summary:         "Server Actions performance optimization merged",
			CreatedAt:       now.Add(-8 * time.Minute),
			ClassifiedAt:    now.Add(-7 * time.Minute),
		},
		{
			ID:              "6",
			EventType:       "PushEvent",
			ActorLogin:      "torvalds",
			RepoName:        "torvalds/linux",
			Category:        models.CategoryRoutine,
			Interestingness: 3,
			Summary:         "Minor driver cleanup for Intel WiFi",
			CreatedAt:       now.Add(-10 * time.Minute),
			ClassifiedAt:    now.Add(-9 * time.Minute),
		},
		{
			ID:              "7",
			EventType:       "PushEvent",
			ActorLogin:      "acdlite",
			RepoName:        "facebook/react",
			Category:        models.CategoryTrending,
			Interestingness: 7,
			Summary:         "React Compiler beta improvements",
			CreatedAt:       now.Add(-12 * time.Minute),
			ClassifiedAt:    now.Add(-11 * time.Minute),
		},
		{
			ID:              "8",
			EventType:       "IssuesEvent",
			ActorLogin:      "rsc",
			RepoName:        "golang/go",
			Category:        models.CategoryInteresting,
			Interestingness: 6,
			Summary:         "Proposal for range-over-function iterators",
			CreatedAt:       now.Add(-15 * time.Minute),
			ClassifiedAt:    now.Add(-14 * time.Minute),
		},
		{
			ID:              "9",
			EventType:       "PushEvent",
			ActorLogin:      "RyanCavanaugh",
			RepoName:        "microsoft/TypeScript",
			Category:        models.CategoryRoutine,
			Interestingness: 4,
			Summary:         "Documentation update for utility types",
			CreatedAt:       now.Add(-18 * time.Minute),
			ClassifiedAt:    now.Add(-17 * time.Minute),
		},
		{
			ID:              "10",
			EventType:       "CreateEvent",
			ActorLogin:      "anthropic-eng",
			RepoName:        "anthropics/claude-code",
			Category:        models.CategoryInteresting,
			Interestingness: 8,
			Summary:         "New CLI features for AI-assisted development",
			CreatedAt:       now.Add(-20 * time.Minute),
			ClassifiedAt:    now.Add(-19 * time.Minute),
		},
	}

	stats := models.PipelineStats{
		EventsIngested:   1247,
		EventsClassified: 1183,
		EventsPerSecond:  3.2,
		UptimeSeconds:    547,
	}

	app := tui.NewMockApp(events, stats, 120, 35, 0)
	fmt.Print(app.View())
}
