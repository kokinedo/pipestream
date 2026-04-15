package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kokinedo/pipestream/pkg/models"
)

// ClaudeProvider classifies events using the Anthropic Claude API.
type ClaudeProvider struct {
	apiKey string
}

// NewClaudeProvider creates a Claude provider.
func NewClaudeProvider(apiKey string) *ClaudeProvider {
	return &ClaudeProvider{apiKey: apiKey}
}

func (p *ClaudeProvider) Name() string         { return "claude" }
func (p *ClaudeProvider) DefaultModel() string { return "claude-sonnet-4-20250514" }

func (p *ClaudeProvider) Classify(ctx context.Context, events []models.RawGitHubEvent, model string) ([]ClassificationResult, error) {
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

	body := map[string]any{
		"model":      model,
		"max_tokens": 2048,
		"system":     classifySystemPrompt,
		"messages":   []map[string]string{{"role": "user", "content": string(userContent)}},
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(b))
	}

	var cresp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cresp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(cresp.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}

	var results []ClassificationResult
	if err := json.Unmarshal([]byte(cresp.Content[0].Text), &results); err != nil {
		return nil, fmt.Errorf("parse classification JSON: %w", err)
	}
	return results, nil
}

const classifySystemPrompt = `You are a GitHub event classifier. For each event, assign:
1. A category: one of "notable_release", "interesting_project", "security_concern", "trending", "routine"
2. An interestingness score from 1 (boring) to 10 (fascinating)
3. A short one-sentence summary

Respond ONLY with a JSON array of objects with fields: id, category, interestingness, summary.
No markdown, no explanation, just the JSON array.`
