package provider

import (
	"context"
	"time"

	"github.com/kokinedo/pipestream/pkg/models"
)

// ClassificationResult holds the AI classification for a single event.
type ClassificationResult struct {
	ID              string `json:"id"`
	Category        string `json:"category"`
	Interestingness int    `json:"interestingness"`
	Summary         string `json:"summary"`
}

// Provider is the interface for AI classification providers.
type Provider interface {
	Name() string
	DefaultModel() string
	Classify(ctx context.Context, events []models.RawGitHubEvent, model string) ([]ClassificationResult, error)
}

// BuildClassifiedEvents converts raw events + classification results into ClassifiedEvent structs.
func BuildClassifiedEvents(events []models.RawGitHubEvent, results []ClassificationResult, now time.Time) []models.ClassifiedEvent {
	rm := make(map[string]ClassificationResult, len(results))
	for _, r := range results {
		rm[r.ID] = r
	}

	classified := make([]models.ClassifiedEvent, len(events))
	for i, e := range events {
		r, ok := rm[e.ID]
		if !ok {
			r = ClassificationResult{ID: e.ID, Category: "routine", Interestingness: 1, Summary: e.Type + " event"}
		}
		payload, _ := e.Payload.MarshalJSON()
		classified[i] = models.ClassifiedEvent{
			ID:              e.ID,
			EventType:       e.Type,
			ActorLogin:      e.Actor.Login,
			RepoName:        e.Repo.Name,
			Category:        models.Category(r.Category),
			Interestingness: r.Interestingness,
			Summary:         r.Summary,
			RawPayload:      string(payload),
			CreatedAt:       e.CreatedAt,
			ClassifiedAt:    now,
		}
	}
	return classified
}
