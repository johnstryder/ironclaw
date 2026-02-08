package embedding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// Interface compliance
// =============================================================================

func TestOllamaEmbedder_ShouldImplementEmbedderInterface(t *testing.T) {
	var _ domain.Embedder = &OllamaEmbedder{}
}

// =============================================================================
// Embed success tests
// =============================================================================

func TestOllamaEmbedder_Embed_ShouldReturnEmbeddingFromAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{{0.1, 0.2, 0.3}},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("nomic-embed-text")
	e.baseURL = server.URL

	result, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3-dim embedding, got %d", len(result))
	}
	if result[0] != 0.1 || result[1] != 0.2 || result[2] != 0.3 {
		t.Errorf("unexpected embedding values: %v", result)
	}
}

func TestOllamaEmbedder_Embed_ShouldPassModelNameToAPI(t *testing.T) {
	var receivedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedModel = req.Model
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{{0.1}},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("nomic-embed-text")
	e.baseURL = server.URL
	e.Embed(context.Background(), "test")

	if receivedModel != "nomic-embed-text" {
		t.Errorf("expected model 'nomic-embed-text', got %q", receivedModel)
	}
}

func TestOllamaEmbedder_Embed_ShouldPassInputTextToAPI(t *testing.T) {
	var receivedInput string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedInput = req.Input
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{{0.1}},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("test-model")
	e.baseURL = server.URL
	e.Embed(context.Background(), "meeting on Tuesday")

	if receivedInput != "meeting on Tuesday" {
		t.Errorf("expected input 'meeting on Tuesday', got %q", receivedInput)
	}
}

func TestOllamaEmbedder_Embed_ShouldSendPOSTRequest(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{{0.1}},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("test-model")
	e.baseURL = server.URL
	e.Embed(context.Background(), "test")

	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
}

func TestOllamaEmbedder_Embed_ShouldSetContentTypeJSON(t *testing.T) {
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{{0.1}},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("test-model")
	e.baseURL = server.URL
	e.Embed(context.Background(), "test")

	if receivedContentType != "application/json" {
		t.Errorf("expected 'application/json', got %q", receivedContentType)
	}
}

func TestOllamaEmbedder_Embed_ShouldCallEmbedEndpoint(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{{0.1}},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("test-model")
	e.baseURL = server.URL
	e.Embed(context.Background(), "test")

	if receivedPath != "/api/embed" {
		t.Errorf("expected '/api/embed', got %q", receivedPath)
	}
}

// =============================================================================
// Embed error tests
// =============================================================================

func TestOllamaEmbedder_Embed_ShouldReturnErrorForEmptyText(t *testing.T) {
	e := NewOllamaEmbedder("test-model")
	_, err := e.Embed(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestOllamaEmbedder_Embed_ShouldReturnErrorForAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	e := NewOllamaEmbedder("test-model")
	e.baseURL = server.URL

	_, err := e.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestOllamaEmbedder_Embed_ShouldReturnErrorForBadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	e := NewOllamaEmbedder("test-model")
	e.baseURL = server.URL

	_, err := e.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestOllamaEmbedder_Embed_ShouldReturnErrorForEmptyEmbeddings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("test-model")
	e.baseURL = server.URL

	_, err := e.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for empty embeddings array")
	}
}

func TestOllamaEmbedder_Embed_ShouldReturnErrorForEmptyFirstEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{{}},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("test-model")
	e.baseURL = server.URL

	_, err := e.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for empty first embedding vector")
	}
}

func TestOllamaEmbedder_Embed_ShouldReturnErrorForNetworkFailure(t *testing.T) {
	e := NewOllamaEmbedder("test-model")
	e.baseURL = "http://127.0.0.1:1" // Closed port

	_, err := e.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestOllamaEmbedder_Embed_ShouldRespectCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{{0.1}},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("test-model")
	e.baseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := e.Embed(ctx, "test")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestOllamaEmbedder_Embed_ShouldReturnErrorWhenMarshalFails(t *testing.T) {
	e := NewOllamaEmbedder("test-model")
	e.marshaller = &failingMarshaller{}

	_, err := e.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error when marshal fails")
	}
	if !strings.Contains(err.Error(), "marshal") {
		t.Errorf("error should mention 'marshal', got: %v", err)
	}
}

func TestOllamaEmbedder_Embed_ShouldReturnErrorForBadURL(t *testing.T) {
	e := NewOllamaEmbedder("test-model")
	e.baseURL = "://bad-url"

	_, err := e.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for bad URL")
	}
}

// failingMarshaller always returns an error.
type failingMarshaller struct{}

func (m *failingMarshaller) Marshal(v interface{}) ([]byte, error) {
	return nil, fmt.Errorf("injected marshal error")
}

// =============================================================================
// Constructor tests
// =============================================================================

func TestNewOllamaEmbedder_ShouldSetModelName(t *testing.T) {
	e := NewOllamaEmbedder("nomic-embed-text")
	if e.model != "nomic-embed-text" {
		t.Errorf("expected model 'nomic-embed-text', got %q", e.model)
	}
}

func TestNewOllamaEmbedder_ShouldSetDefaultBaseURL(t *testing.T) {
	e := NewOllamaEmbedder("test-model")
	if e.baseURL != "http://localhost:11434" {
		t.Errorf("expected default base URL, got %q", e.baseURL)
	}
}

func TestNewOllamaEmbedder_ShouldSetHTTPClient(t *testing.T) {
	e := NewOllamaEmbedder("test-model")
	if e.client == nil {
		t.Error("expected non-nil http client")
	}
}

// =============================================================================
// E2E with test server (simulates Ollama)
// =============================================================================

func TestOllamaEmbedder_E2E_ShouldHandleRealisticEmbeddingResponse(t *testing.T) {
	// Simulate a 768-dim embedding like nomic-embed-text
	embedding := make([]float64, 768)
	for i := range embedding {
		embedding[i] = float64(i) * 0.001
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float64{embedding},
		})
	}))
	defer server.Close()

	e := NewOllamaEmbedder("nomic-embed-text")
	e.baseURL = server.URL

	result, err := e.Embed(context.Background(), "What did we decide about the meeting?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 768 {
		t.Errorf("expected 768-dim embedding, got %d", len(result))
	}
}
