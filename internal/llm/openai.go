package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"ironclaw/internal/domain"
)

// OpenAIProvider calls the OpenAI Chat Completions API.
type OpenAIProvider struct {
	apiKey      string
	model       string
	client      *http.Client
	baseURL     string
	marshalFunc func(v interface{}) ([]byte, error) // for testing
}

// NewOpenAIProvider returns an OpenAI-backed LLMProvider.
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:      apiKey,
		model:       model,
		client:      &http.Client{},
		baseURL:     "https://api.openai.com/v1/chat/completions",
		marshalFunc: json.Marshal,
	}
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

// Generate implements domain.LLMProvider.
func (p *OpenAIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	body := openAIRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{Role: "user", Content: prompt},
		},
	}
	raw, err := p.marshalFunc(body)
	if err != nil {
		return "", fmt.Errorf("openai marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai api: %s", resp.Status)
	}
	var out openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("openai decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices in response")
	}
	return out.Choices[0].Message.Content, nil
}

var _ domain.LLMProvider = (*OpenAIProvider)(nil)
