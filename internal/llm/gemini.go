package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"ironclaw/internal/domain"
)

const geminiAPIBase = "https://generativelanguage.googleapis.com/v1/models"

// GeminiProvider calls the Google Gemini API.
type GeminiProvider struct {
	apiKey      string
	model       string
	client      *http.Client
	baseURL     string
	marshalFunc func(v interface{}) ([]byte, error) // for testing
}

// NewGeminiProvider returns a Gemini-backed LLMProvider.
func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	return &GeminiProvider{
		apiKey:      apiKey,
		model:       model,
		client:      &http.Client{},
		baseURL:     geminiAPIBase,
		marshalFunc: json.Marshal,
	}
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// Generate implements domain.LLMProvider.
func (p *GeminiProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	body := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
	}
	raw, err := p.marshalFunc(body)
	if err != nil {
		return "", fmt.Errorf("gemini marshal: %w", err)
	}
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", p.baseURL, p.model, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini api: %s", resp.Status)
	}
	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("gemini decode: %w", err)
	}
	if len(out.Candidates) == 0 {
		return "", fmt.Errorf("gemini: no candidates in response")
	}
	var text string
	for _, part := range out.Candidates[0].Content.Parts {
		text += part.Text
	}
	return text, nil
}

var _ domain.LLMProvider = (*GeminiProvider)(nil)