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

// OpenAIProvider classifies events using the OpenAI API.
type OpenAIProvider struct {
	apiKey string
}

// NewOpenAIProvider creates an OpenAI provider.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{apiKey: apiKey}
}

func (p *OpenAIProvider) Name() string         { return "openai" }
func (p *OpenAIProvider) DefaultModel() string { return "gpt-4o" }

func (p *OpenAIProvider) Classify(ctx context.Context, events []models.RawGitHubEvent, model string) ([]ClassificationResult, error) {
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
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": classifySystemPrompt},
			{"role": "user", "content": string(userContent)},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai API error %d: %s", resp.StatusCode, string(b))
	}

	var cresp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cresp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(cresp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	var results []ClassificationResult
	if err := json.Unmarshal([]byte(cresp.Choices[0].Message.Content), &results); err != nil {
		// OpenAI might wrap in an object
		var wrapped struct {
			Results []ClassificationResult `json:"results"`
		}
		if err2 := json.Unmarshal([]byte(cresp.Choices[0].Message.Content), &wrapped); err2 != nil {
			return nil, fmt.Errorf("parse classification JSON: %w", err)
		}
		results = wrapped.Results
	}
	return results, nil
}
