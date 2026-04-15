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

// GeminiProvider classifies events using the Google Gemini API.
type GeminiProvider struct {
	apiKey string
}

// NewGeminiProvider creates a Gemini provider.
func NewGeminiProvider(apiKey string) *GeminiProvider {
	return &GeminiProvider{apiKey: apiKey}
}

func (p *GeminiProvider) Name() string         { return "gemini" }
func (p *GeminiProvider) DefaultModel() string { return "gemini-2.0-flash" }

func (p *GeminiProvider) Classify(ctx context.Context, events []models.RawGitHubEvent, model string) ([]ClassificationResult, error) {
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
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": string(userContent)}}},
		},
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": classifySystemPrompt}},
		},
		"generationConfig": map[string]string{
			"responseMimeType": "application/json",
		},
	}
	bodyBytes, _ := json.Marshal(body)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:generateContent?key=%s", model, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini API error %d: %s", resp.StatusCode, string(b))
	}

	var gresp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gresp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(gresp.Candidates) == 0 || len(gresp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	var results []ClassificationResult
	if err := json.Unmarshal([]byte(gresp.Candidates[0].Content.Parts[0].Text), &results); err != nil {
		return nil, fmt.Errorf("parse classification JSON: %w", err)
	}
	return results, nil
}
