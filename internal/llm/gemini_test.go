package llm

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

func TestNewGeminiProvider_ShouldCreateProvider(t *testing.T) {
	p := NewGeminiProvider("key", "gemini-pro")
	if p.apiKey != "key" || p.model != "gemini-pro" {
		t.Errorf("expected key=key model=gemini-pro, got key=%q model=%q", p.apiKey, p.model)
	}
}

func TestGeminiProvider_Generate_WhenContextCanceled_ShouldReturnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewGeminiProvider("key", "gemini-pro")

	_, err := p.Generate(ctx, "hi")
	if err == nil {
		t.Error("expected error when context canceled")
	}
}

func TestGeminiProvider_Generate_IsCalled(t *testing.T) {
	p := NewGeminiProvider("invalid-key", "gemini-pro")
	// This will fail with network error, but tests that the code path is reached
	_, err := p.Generate(context.Background(), "hi")
	if err == nil {
		t.Error("expected error with invalid key")
	}
}

func TestGeminiProvider_Generate_WhenAPISuccess_ShouldReturnResponse(t *testing.T) {
	// Mock successful response
	mockResp := `{
		"candidates": [{
			"content": {
				"parts": [{"text": "Hello from Gemini"}]
			}
		}]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", "gemini-pro")
	p.baseURL = server.URL
	p.client = server.Client()

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Hello from Gemini" {
		t.Errorf("expected 'Hello from Gemini', got %q", result)
	}
}

func TestGeminiProvider_Generate_WhenAPIError_ShouldReturnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	p := NewGeminiProvider("key", "gemini-pro")
	p.baseURL = server.URL
	p.client = server.Client()

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("500")) {
		t.Errorf("expected error containing 500, got %v", err)
	}
}

func TestGeminiProvider_Generate_WhenAPIEmptyContent_ShouldReturnEmptyString(t *testing.T) {
	// Mock response with content but no text parts
	mockResp := `{"candidates": [{"content": {"parts": []}}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewGeminiProvider("key", "gemini-pro")
	p.baseURL = server.URL
	p.client = server.Client()

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestGeminiProvider_Generate_WhenAPIInvalidJSON_ShouldReturnError(t *testing.T) {
	// Mock response with invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	p := NewGeminiProvider("key", "gemini-pro")
	p.baseURL = server.URL
	p.client = server.Client()

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "gemini decode") {
		t.Errorf("expected decode error, got %v", err)
	}
}

func TestGeminiProvider_Generate_WhenAPIMultipleTextParts_ShouldConcatenateText(t *testing.T) {
	// Mock response with multiple text parts
	mockResp := `{
		"candidates": [{
			"content": {
				"parts": [
					{"text": "Hello"},
					{"text": " from"},
					{"text": " Gemini"}
				]
			}
		}]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewGeminiProvider("key", "gemini-pro")
	p.baseURL = server.URL
	p.client = server.Client()

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Hello from Gemini" {
		t.Errorf("expected 'Hello from Gemini', got %q", result)
	}
}

func TestGeminiProvider_Generate_WhenMarshalFails_ShouldReturnError(t *testing.T) {
	p := NewGeminiProvider("key", "gemini-pro")
	p.marshalFunc = failingMarshalFunc

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "gemini marshal") {
		t.Errorf("expected marshal error, got %v", err)
	}
}

func TestGeminiProvider_Generate_WhenHTTPDoFails_ShouldReturnError(t *testing.T) {
	p := NewGeminiProvider("key", "gemini-pro")
	// Set a client with a transport that always fails
	p.client = &http.Client{
		Transport: &failingTransport{},
	}

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "gemini do") {
		t.Errorf("expected do error, got %v", err)
	}
}

func TestGeminiProvider_Generate_WhenInvalidURL_ShouldReturnError(t *testing.T) {
	// Use an invalid URL to trigger request creation error
	p := NewGeminiProvider("key", "gemini-pro")
	p.baseURL = "http://invalid\x00url" // Invalid URL with null byte

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "gemini request") {
		t.Errorf("expected request creation error, got %v", err)
	}
}

func TestGeminiProvider_Generate_WhenAPINoCandidates_ShouldReturnError(t *testing.T) {
	// Mock response with no candidates
	mockResp := `{"candidates": []}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewGeminiProvider("key", "gemini-pro")
	p.baseURL = server.URL
	p.client = server.Client()

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "no candidates") {
		t.Errorf("expected no candidates error, got %v", err)
	}
}

var _ domain.LLMProvider = (*GeminiProvider)(nil)