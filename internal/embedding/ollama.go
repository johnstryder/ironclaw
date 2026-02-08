package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"ironclaw/internal/domain"
)

// JSONMarshaller interface for testing (matches llm package pattern).
type JSONMarshaller interface {
	Marshal(v interface{}) ([]byte, error)
}

// defaultMarshaller uses json.Marshal.
type defaultMarshaller struct{}

func (m *defaultMarshaller) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// OllamaEmbedder generates vector embeddings using the Ollama /api/embed endpoint.
type OllamaEmbedder struct {
	model      string
	client     *http.Client
	baseURL    string
	marshaller JSONMarshaller
}

// NewOllamaEmbedder returns an Embedder backed by a local Ollama instance.
func NewOllamaEmbedder(model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		model:      model,
		client:     &http.Client{},
		baseURL:    "http://localhost:11434",
		marshaller: &defaultMarshaller{},
	}
}

// embedRequest is the request body for Ollama /api/embed.
type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// embedResponse is the response body from Ollama /api/embed.
type embedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

// Embed implements domain.Embedder. It sends text to Ollama and returns the embedding.
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	if text == "" {
		return nil, fmt.Errorf("text must not be empty")
	}

	body := embedRequest{
		Model: e.model,
		Input: text,
	}

	raw, err := e.marshaller.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("embed marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed api: %s", resp.Status)
	}

	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("embed decode: %w", err)
	}

	if len(out.Embeddings) == 0 {
		return nil, fmt.Errorf("embed: no embeddings returned")
	}
	if len(out.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("embed: empty embedding vector")
	}

	return out.Embeddings[0], nil
}

// Ensure OllamaEmbedder implements domain.Embedder at compile time.
var _ domain.Embedder = (*OllamaEmbedder)(nil)
