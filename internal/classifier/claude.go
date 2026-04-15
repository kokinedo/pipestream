package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/kokinedo/pipestream/pkg/models"
)

// Config holds classifier configuration.
type Config struct {
	APIKey       string
	Model        string
	BatchSize    int
	BatchTimeout time.Duration
	DryRun       bool
}

// Classifier reads raw events, classifies them via Claude, and sends results downstream.
type Classifier struct {
	cfg   Config
	in    <-chan []models.RawGitHubEvent
	out   chan<- []models.ClassifiedEvent
	count int64
}

// NewClassifier creates a Classifier.
func NewClassifier(cfg Config, in <-chan []models.RawGitHubEvent, out chan<- []models.ClassifiedEvent) *Classifier {
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 5
	}
	if cfg.BatchTimeout <= 0 {
		cfg.BatchTimeout = 3 * time.Second
	}
	return &Classifier{cfg: cfg, in: in, out: out}
}

// ClassifiedCount returns the number of events classified so far.
func (c *Classifier) ClassifiedCount() int64 {
	return atomic.LoadInt64(&c.count)
}

// Start reads from in, batches events, classifies them, and writes to out.
func (c *Classifier) Start(ctx context.Context) error {
	var batch []models.RawGitHubEvent
	timer := time.NewTimer(c.cfg.BatchTimeout)
	defer timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		toClassify := batch
		batch = nil

		classified, err := c.classifyBatch(ctx, toClassify)
		if err != nil {
			log.Printf("[classifier] error: %v", err)
			return
		}
		atomic.AddInt64(&c.count, int64(len(classified)))

		select {
		case c.out <- classified:
		case <-ctx.Done():
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case events, ok := <-c.in:
			if !ok {
				flush()
				return nil
			}
			batch = append(batch, events...)
			if len(batch) >= c.cfg.BatchSize {
				flush()
				timer.Reset(c.cfg.BatchTimeout)
			}
		case <-timer.C:
			flush()
			timer.Reset(c.cfg.BatchTimeout)
		}
	}
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

type classificationResult struct {
	ID              string `json:"id"`
	Category        string `json:"category"`
	Interestingness int    `json:"interestingness"`
	Summary         string `json:"summary"`
}

const systemPrompt = `You are a GitHub event classifier. For each event, assign:
1. A category: one of "notable_release", "interesting_project", "security_concern", "trending", "routine"
2. An interestingness score from 1 (boring) to 10 (fascinating)
3. A short one-sentence summary

Respond ONLY with a JSON array of objects with fields: id, category, interestingness, summary.
No markdown, no explanation, just the JSON array.`

func (c *Classifier) classifyBatch(ctx context.Context, events []models.RawGitHubEvent) ([]models.ClassifiedEvent, error) {
	now := time.Now()

	if c.cfg.DryRun {
		return c.dryRunClassify(events, now), nil
	}

	// Build event summaries for the prompt.
	type eventSummary struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Actor string `json:"actor"`
		Repo  string `json:"repo"`
	}
	summaries := make([]eventSummary, len(events))
	for i, e := range events {
		summaries[i] = eventSummary{ID: e.ID, Type: e.Type, Actor: e.Actor.Login, Repo: e.Repo.Name}
	}
	userContent, _ := json.Marshal(summaries)

	reqBody := claudeRequest{
		Model:     c.cfg.Model,
		MaxTokens: 2048,
		System:    systemPrompt,
		Messages: []claudeMessage{
			{Role: "user", Content: string(userContent)},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.fallbackClassify(events, now), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[classifier] Claude API error %d: %s", resp.StatusCode, string(body))
		return c.fallbackClassify(events, now), nil
	}

	var cresp claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&cresp); err != nil {
		return c.fallbackClassify(events, now), nil
	}
	if len(cresp.Content) == 0 {
		return c.fallbackClassify(events, now), nil
	}

	var results []classificationResult
	if err := json.Unmarshal([]byte(cresp.Content[0].Text), &results); err != nil {
		log.Printf("[classifier] failed to parse response: %v", err)
		return c.fallbackClassify(events, now), nil
	}

	// Build a map of results by ID for easy lookup.
	rm := make(map[string]classificationResult, len(results))
	for _, r := range results {
		rm[r.ID] = r
	}

	classified := make([]models.ClassifiedEvent, len(events))
	for i, e := range events {
		r, ok := rm[e.ID]
		if !ok {
			r = classificationResult{ID: e.ID, Category: "routine", Interestingness: 1, Summary: e.Type + " event"}
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
	return classified, nil
}

func (c *Classifier) fallbackClassify(events []models.RawGitHubEvent, now time.Time) []models.ClassifiedEvent {
	classified := make([]models.ClassifiedEvent, len(events))
	for i, e := range events {
		payload, _ := e.Payload.MarshalJSON()
		classified[i] = models.ClassifiedEvent{
			ID:              e.ID,
			EventType:       e.Type,
			ActorLogin:      e.Actor.Login,
			RepoName:        e.Repo.Name,
			Category:        models.CategoryRoutine,
			Interestingness: 1,
			Summary:         e.Type + " event in " + e.Repo.Name,
			RawPayload:      string(payload),
			CreatedAt:       e.CreatedAt,
			ClassifiedAt:    now,
		}
	}
	return classified
}

func (c *Classifier) dryRunClassify(events []models.RawGitHubEvent, now time.Time) []models.ClassifiedEvent {
	cats := models.AllCategories()
	classified := make([]models.ClassifiedEvent, len(events))
	for i, e := range events {
		payload, _ := e.Payload.MarshalJSON()
		cat := cats[rand.Intn(len(cats))]
		score := rand.Intn(10) + 1
		classified[i] = models.ClassifiedEvent{
			ID:              e.ID,
			EventType:       e.Type,
			ActorLogin:      e.Actor.Login,
			RepoName:        e.Repo.Name,
			Category:        cat,
			Interestingness: score,
			Summary:         fmt.Sprintf("[dry-run] %s by %s on %s", e.Type, e.Actor.Login, e.Repo.Name),
			RawPayload:      string(payload),
			CreatedAt:       e.CreatedAt,
			ClassifiedAt:    now,
		}
	}
	return classified
}
