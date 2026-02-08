package llm

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

func TestNewAnthropicProvider_ShouldCreateProvider(t *testing.T) {
	p := NewAnthropicProvider("key", "claude-3")
	if p.apiKey != "key" || p.model != "claude-3" {
		t.Errorf("expected key=key model=claude-3, got key=%q model=%q", p.apiKey, p.model)
	}
}

func TestAnthropicProvider_Generate_WhenContextCanceled_ShouldReturnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewAnthropicProvider("key", "claude-3")

	_, err := p.Generate(ctx, "hi")
	if err == nil {
		t.Error("expected error when context canceled")
	}
}

func TestAnthropicProvider_Generate_IsCalled(t *testing.T) {
	p := NewAnthropicProvider("invalid-key", "claude-3")
	// This will fail with network error, but tests that the code path is reached
	_, err := p.Generate(context.Background(), "hi")
	if err == nil {
		t.Error("expected error with invalid key")
	}
}

func TestAnthropicProvider_Generate_WhenAPISuccess_ShouldReturnResponse(t *testing.T) {
	// Mock successful response
	mockResp := `{
		"content": [{"type": "text", "text": "Hello from Anthropic"}]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-3")
	p.baseURL = server.URL
	p.client = server.Client()

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Hello from Anthropic" {
		t.Errorf("expected 'Hello from Anthropic', got %q", result)
	}
}

func TestAnthropicProvider_Generate_WhenAPIError_ShouldReturnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	p := NewAnthropicProvider("key", "claude-3")
	p.baseURL = server.URL
	p.client = server.Client()

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("500")) {
		t.Errorf("expected error containing 500, got %v", err)
	}
}

func TestAnthropicProvider_Generate_WhenAPIEmptyContent_ShouldReturnEmptyString(t *testing.T) {
	// Mock response with content but no text type
	mockResp := `{"content": [{"type": "image", "data": "someimage"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewAnthropicProvider("key", "claude-3")
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

func TestAnthropicProvider_Generate_WhenAPIInvalidJSON_ShouldReturnError(t *testing.T) {
	// Mock response with invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	p := NewAnthropicProvider("key", "claude-3")
	p.baseURL = server.URL
	p.client = server.Client()

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "anthropic decode") {
		t.Errorf("expected decode error, got %v", err)
	}
}

func TestAnthropicProvider_Generate_WhenAPIMultipleTextBlocks_ShouldConcatenateText(t *testing.T) {
	// Mock response with multiple text content blocks
	mockResp := `{
		"content": [
			{"type": "text", "text": "Hello"},
			{"type": "text", "text": " from"},
			{"type": "text", "text": " Anthropic"}
		]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewAnthropicProvider("key", "claude-3")
	p.baseURL = server.URL
	p.client = server.Client()

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Hello from Anthropic" {
		t.Errorf("expected 'Hello from Anthropic', got %q", result)
	}
}

func TestAnthropicProvider_Generate_WhenAPIMixedContentTypes_ShouldOnlyIncludeText(t *testing.T) {
	// Mock response with mixed content types
	mockResp := `{
		"content": [
			{"type": "text", "text": "Hello"},
			{"type": "image", "data": "ignored"},
			{"type": "text", "text": " World"}
		]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewAnthropicProvider("key", "claude-3")
	p.baseURL = server.URL
	p.client = server.Client()

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", result)
	}
}

func TestAnthropicProvider_Generate_WhenMarshalFails_ShouldReturnError(t *testing.T) {
	p := NewAnthropicProvider("key", "claude-3")
	p.marshalFunc = failingMarshalFunc

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "anthropic marshal") {
		t.Errorf("expected marshal error, got %v", err)
	}
}

func TestAnthropicProvider_Generate_WhenHTTPDoFails_ShouldReturnError(t *testing.T) {
	p := NewAnthropicProvider("key", "claude-3")
	// Set a client with a transport that always fails
	p.client = &http.Client{
		Transport: &failingTransport{},
	}

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "anthropic do") {
		t.Errorf("expected do error, got %v", err)
	}
}

// failingTransport always fails for testing HTTP client errors
type failingTransport struct{}

func (f *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("intentional HTTP failure for testing")
}

func TestAnthropicProvider_Generate_WhenInvalidURL_ShouldReturnError(t *testing.T) {
	// Use an invalid URL to trigger request creation error
	p := NewAnthropicProvider("key", "claude-3")
	p.baseURL = "http://invalid\x00url" // Invalid URL with null byte

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "anthropic request") {
		t.Errorf("expected request creation error, got %v", err)
	}
}







// Test that covers the JSON marshaling error path by using a mock that causes marshal to fail
// This is difficult to test directly, but we can test the overall error handling

// anthropicMockTransport returns a fixed response for testing.
type anthropicMockTransport struct {
	response string
	status   int
}

func (m *anthropicMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	status := "200 OK"
	if m.status != 200 {
		status = "500 Internal Server Error"
	}
	return &http.Response{
		StatusCode: m.status,
		Status:     status,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

// failingMarshalFunc always fails to marshal for testing JSON marshaling error paths
func failingMarshalFunc(v interface{}) ([]byte, error) {
	return nil, fmt.Errorf("intentional marshal failure for testing")
}


var _ domain.LLMProvider = (*AnthropicProvider)(nil)