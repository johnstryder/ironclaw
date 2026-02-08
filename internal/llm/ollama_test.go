package llm

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

func TestOllamaProvider_Generate_ShouldReturnSuccessfulResponse(t *testing.T) {
	// Given: Mock Ollama server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected /api/generate, got %s", r.URL.Path)
		}

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		expectedModel := "llama3"
		if req["model"] != expectedModel {
			t.Errorf("expected model %q, got %q", expectedModel, req["model"])
		}

		expectedPrompt := "Hello, world!"
		if req["prompt"] != expectedPrompt {
			t.Errorf("expected prompt %q, got %q", expectedPrompt, req["prompt"])
		}

		if req["stream"] != false {
			t.Errorf("expected stream=false, got %v", req["stream"])
		}

		response := map[string]string{
			"response": "Hello from Ollama!",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// When: Create provider and generate
	provider := NewOllamaProvider("llama3")
	provider.baseURL = server.URL + "/api" // Override for test

	got, err := provider.Generate(context.Background(), "Hello, world!")

	// Then: Should return successful response
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	expected := "Hello from Ollama!"
	if got != expected {
		t.Errorf("want %q, got %q", expected, got)
	}
}

func TestOllamaProvider_Generate_WhenContextCanceled_ShouldReturnError(t *testing.T) {
	// Given: Provider and canceled context
	provider := NewOllamaProvider("llama3")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When: Generate with canceled context
	_, err := provider.Generate(ctx, "test")

	// Then: Should return error
	if err == nil {
		t.Error("expected error when context canceled")
	}
}

func TestOllamaProvider_Generate_WhenServerReturnsError_ShouldReturnError(t *testing.T) {
	// Given: Mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// When: Create provider and generate
	provider := NewOllamaProvider("llama3")
	provider.baseURL = server.URL + "/api"

	_, err := provider.Generate(context.Background(), "test")

	// Then: Should return error
	if err == nil {
		t.Error("expected error when server returns 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status code, got %q", err.Error())
	}
}

func TestOllamaProvider_Generate_WhenServerUnreachable_ShouldReturnError(t *testing.T) {
	// Given: Provider with unreachable URL
	provider := NewOllamaProvider("llama3")
	provider.baseURL = "http://nonexistent-server:12345/api"

	// When: Generate
	_, err := provider.Generate(context.Background(), "test")

	// Then: Should return error
	if err == nil {
		t.Error("expected error when server unreachable")
	}
}

func TestOllamaProvider_Generate_WhenInvalidJSONResponse_ShouldReturnError(t *testing.T) {
	// Given: Mock server with invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	// When: Create provider and generate
	provider := NewOllamaProvider("llama3")
	provider.baseURL = server.URL + "/api"

	_, err := provider.Generate(context.Background(), "test")

	// Then: Should return error
	if err == nil {
		t.Error("expected error when invalid JSON response")
	}
}

func TestOllamaProvider_Generate_WhenResponseMissingResponseField_ShouldReturnError(t *testing.T) {
	// Given: Mock server with response missing 'response' field
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]string{
			"other_field": "value",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// When: Create provider and generate
	provider := NewOllamaProvider("llama3")
	provider.baseURL = server.URL + "/api"

	_, err := provider.Generate(context.Background(), "test")

	// Then: Should return error
	if err == nil {
		t.Error("expected error when response missing 'response' field")
	}
}

func TestOllamaProvider_Generate_WhenInvalidURL_ShouldReturnError(t *testing.T) {
	// Given: Provider with invalid base URL
	provider := NewOllamaProvider("llama3")
	provider.baseURL = "http://invalid\x00url/api" // Invalid URL with null byte

	// When: Generate
	_, err := provider.Generate(context.Background(), "test")

	// Then: Should return error
	if err == nil {
		t.Error("expected error when URL is invalid")
	}
}

type failingMarshaler struct{}

func (f failingMarshaler) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("intentional marshal error")
}

// failingMarshaller always returns an error for testing
type failingMarshaller struct{}

func (m *failingMarshaller) Marshal(v interface{}) ([]byte, error) {
	return nil, fmt.Errorf("intentional marshal failure for testing")
}

func TestOllamaProvider_Generate_WhenMarshalFails_ShouldReturnError(t *testing.T) {
	// Given: Provider with a marshaller that fails
	provider := &OllamaProvider{
		model:      "llama3",
		client:     &http.Client{},
		baseURL:    "http://localhost:11434/api",
		marshaller: &failingMarshaller{},
	}

	// When: Generate
	_, err := provider.Generate(context.Background(), "test")

	// Then: Should return error
	if err == nil {
		t.Error("expected error when marshalling fails")
	}
	if !strings.Contains(err.Error(), "ollama marshal") {
		t.Errorf("expected error to contain 'ollama marshal', got %q", err.Error())
	}
}

func TestNewOllamaProvider_ShouldReturnProviderWithCorrectModel(t *testing.T) {
	// Given: Model name
	model := "llama3.2"

	// When: Create provider
	provider := NewOllamaProvider(model)

	// Then: Should have correct model
	if provider.model != model {
		t.Errorf("want model %q, got %q", model, provider.model)
	}

	// And: Should have default base URL
	expectedURL := "http://localhost:11434/api"
	if provider.baseURL != expectedURL {
		t.Errorf("want baseURL %q, got %q", expectedURL, provider.baseURL)
	}

	// And: Should implement LLMProvider interface
	var _ domain.LLMProvider = provider
}