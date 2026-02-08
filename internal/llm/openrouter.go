package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"ironclaw/internal/domain"
)

// OpenRouterProvider calls the OpenRouter Chat Completions API.
type OpenRouterProvider struct {
	apiKey      string
	model       string
	client      *http.Client
	baseURL     string
	marshalFunc func(v interface{}) ([]byte, error) // for testing
}

// NewOpenRouterProvider returns an OpenRouter-backed LLMProvider.
func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	return &OpenRouterProvider{
		apiKey:      apiKey,
		model:       model,
		client:      &http.Client{},
		baseURL:     "https://openrouter.ai/api/v1/chat/completions",
		marshalFunc: json.Marshal,
	}
}

type openRouterRequest struct {
	Model    string                `json:"model"`
	Messages []openRouterMessage   `json:"messages"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	Choices []struct {
		Message openRouterMessage `json:"message"`
	} `json:"choices"`
}

// Generate implements domain.LLMProvider.
func (p *OpenRouterProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	body := openRouterRequest{
		Model: p.model,
		Messages: []openRouterMessage{
			{Role: "user", Content: prompt},
		},
	}
	raw, err := p.marshalFunc(body)
	if err != nil {
		return "", fmt.Errorf("openrouter marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("openrouter request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter api: %s", resp.Status)
	}
	var out openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("openrouter decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openrouter: no choices in response")
	}
	return out.Choices[0].Message.Content, nil
}

var _ domain.LLMProvider = (*OpenRouterProvider)(nil)