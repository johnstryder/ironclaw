package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"ironclaw/internal/domain"
)

const anthropicAPIBase = "https://api.anthropic.com/v1/messages"

// AnthropicProvider calls the Anthropic Messages API.
type AnthropicProvider struct {
	apiKey     string
	model      string
	client     *http.Client
	version    string
	baseURL    string
	marshalFunc func(v interface{}) ([]byte, error) // for testing
}

// NewAnthropicProvider returns an Anthropic-backed LLMProvider.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:      apiKey,
		model:       model,
		client:      &http.Client{},
		version:     "2023-06-01",
		baseURL:     anthropicAPIBase,
		marshalFunc: json.Marshal,
	}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage  `json:"messages"`
}



// Anthropic accepts content as string or array of blocks; we send a single text block.
type anthropicMessage struct {
	Role    string                   `json:"role"`
	Content []anthropicContentBlock  `json:"content"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// Generate implements domain.LLMProvider.
func (p *AnthropicProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	body := anthropicRequest{
		Model:     p.model,
		MaxTokens: 1024,
		Messages: []anthropicMessage{
			{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: prompt}}},
		},
	}
	raw, err := p.marshalFunc(body)
	if err != nil {
		return "", fmt.Errorf("anthropic marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", p.version)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic api: %s", resp.Status)
	}
	var out anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("anthropic decode: %w", err)
	}
	var text string
	for _, c := range out.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	return text, nil
}

var _ domain.LLMProvider = (*AnthropicProvider)(nil)
