package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"ironclaw/internal/domain"
)

// JSONMarshaller interface for testing
type JSONMarshaller interface {
	Marshal(v interface{}) ([]byte, error)
}

// OllamaProvider calls the local Ollama API.
type OllamaProvider struct {
	model      string
	client     *http.Client
	baseURL    string
	marshaller JSONMarshaller
}

// defaultMarshaller uses json.Marshal
type defaultMarshaller struct{}

func (m *defaultMarshaller) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// NewOllamaProvider returns an Ollama-backed LLMProvider.
func NewOllamaProvider(model string) *OllamaProvider {
	return &OllamaProvider{
		model:      model,
		client:     &http.Client{},
		baseURL:    "http://localhost:11434/api",
		marshaller: &defaultMarshaller{},
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

// Generate implements domain.LLMProvider.
func (p *OllamaProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	body := ollamaRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: false, // Disable streaming for simpler handling
	}

	raw, err := p.marshaller.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/generate", bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama api: %s", resp.Status)
	}

	var out ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("ollama decode: %w", err)
	}

	if out.Response == "" {
		return "", fmt.Errorf("ollama: empty response")
	}

	return out.Response, nil
}

// Ensure OllamaProvider implements domain.LLMProvider at compile time.
var _ domain.LLMProvider = (*OllamaProvider)(nil)