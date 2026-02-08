package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

// mockTransport returns a fixed response for testing.
type mockTransport struct {
	response string
	status   int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	status := "200 OK"
	if m.status != 200 {
		status = "500 Internal Server Error"
	}
	return &http.Response{
		StatusCode: m.status,
		Status:     status,
		Body:       http.NoBody,
	}, nil
}

func TestNewOpenAIProvider_ShouldCreateProvider(t *testing.T) {
	p := NewOpenAIProvider("key", "gpt-4")
	if p.apiKey != "key" || p.model != "gpt-4" {
		t.Errorf("expected key=key model=gpt-4, got key=%q model=%q", p.apiKey, p.model)
	}
}

func TestOpenAIProvider_Generate_WhenContextCanceled_ShouldReturnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewOpenAIProvider("key", "gpt-4")

	_, err := p.Generate(ctx, "hi")
	if err == nil {
		t.Error("expected error when context canceled")
	}
}

func TestOpenAIProvider_Generate_WhenAPIError_ShouldReturnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	p := NewOpenAIProvider("key", "gpt-4")
	p.baseURL = server.URL
	p.client = server.Client()

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error containing 500, got %v", err)
	}
}

func TestOpenAIProvider_Generate_WhenAPIInvalidJSON_ShouldReturnError(t *testing.T) {
	// Mock response with invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	p := NewOpenAIProvider("key", "gpt-4")
	p.baseURL = server.URL
	p.client = server.Client()

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "openai decode") {
		t.Errorf("expected decode error, got %v", err)
	}
}

func TestOpenAIProvider_Generate_WhenInvalidURL_ShouldReturnError(t *testing.T) {
	// Use an invalid URL to trigger request creation error
	p := NewOpenAIProvider("key", "gpt-4")
	p.baseURL = "http://invalid\x00url" // Invalid URL with null byte

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "openai request") {
		t.Errorf("expected request creation error, got %v", err)
	}
}








// For now, test that Generate is called without error (but we can't mock the HTTP without changing the code)
func TestOpenAIProvider_Generate_IsCalled(t *testing.T) {
	p := NewOpenAIProvider("invalid-key", "gpt-4")
	// This will fail with network error, but tests that the code path is reached
	_, err := p.Generate(context.Background(), "hi")
	if err == nil {
		t.Error("expected error with invalid key")
	}
	// We can't easily test success without a real API key, so coverage is limited
}

func TestOpenAIProvider_Generate_WhenAPIEmptyChoices_ShouldReturnError(t *testing.T) {
	// Mock response with no choices
	mockResp := `{"choices": []}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewOpenAIProvider("key", "gpt-4")
	p.baseURL = server.URL
	p.client = server.Client()

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Errorf("expected error about no choices, got %v", err)
	}
}

func TestOpenAIProvider_Generate_WhenAPISuccess_ShouldReturnResponse(t *testing.T) {
	// Mock successful response
	mockResp := `{
		"choices": [{"message": {"content": "Hello from OpenAI"}}]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewOpenAIProvider("key", "gpt-4")
	p.baseURL = server.URL
	p.client = server.Client()

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Hello from OpenAI" {
		t.Errorf("expected 'Hello from OpenAI', got %q", result)
	}
}

func TestOpenAIProvider_Generate_WhenAPIHasMultipleChoices_ShouldReturnFirstChoice(t *testing.T) {
	// Mock response with multiple choices
	mockResp := `{
		"choices": [
			{"message": {"content": "First choice"}},
			{"message": {"content": "Second choice"}}
		]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(mockResp))
	}))
	defer server.Close()

	p := NewOpenAIProvider("key", "gpt-4")
	p.baseURL = server.URL
	p.client = server.Client()

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "First choice" {
		t.Errorf("expected 'First choice', got %q", result)
	}
}

func TestOpenAIProvider_Generate_WhenMarshalFails_ShouldReturnError(t *testing.T) {
	p := NewOpenAIProvider("key", "gpt-4")
	p.marshalFunc = failingMarshalFunc

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "openai marshal") {
		t.Errorf("expected marshal error, got %v", err)
	}
}

func TestOpenAIProvider_Generate_WhenHTTPDoFails_ShouldReturnError(t *testing.T) {
	p := NewOpenAIProvider("key", "gpt-4")
	// Set a client with a transport that always fails
	p.client = &http.Client{
		Transport: &failingTransport{},
	}

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "openai do") {
		t.Errorf("expected do error, got %v", err)
	}
}

var _ domain.LLMProvider = (*OpenAIProvider)(nil)