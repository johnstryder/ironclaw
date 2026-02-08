package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

func TestNewOpenRouterProvider_ShouldCreateProvider(t *testing.T) {
	p := NewOpenRouterProvider("key", "openai/gpt-4")
	if p.apiKey != "key" || p.model != "openai/gpt-4" {
		t.Errorf("expected key=key model=openai/gpt-4, got key=%q model=%q", p.apiKey, p.model)
	}
}

func TestOpenRouterProvider_Generate_WhenContextCanceled_ShouldReturnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewOpenRouterProvider("key", "openai/gpt-4")

	_, err := p.Generate(ctx, "hi")
	if err == nil {
		t.Error("expected error when context canceled")
	}
}

func TestOpenRouterProvider_Generate_WhenAPIError_ShouldReturnError(t *testing.T) {
	p := NewOpenRouterProvider("key", "openai/gpt-4")
	p.client = &http.Client{
		Transport: &openRouterMockTransport{status: 500},
	}

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error containing 500, got %v", err)
	}
}

func TestOpenRouterProvider_Generate_WhenAPIInvalidJSON_ShouldReturnError(t *testing.T) {
	// Mock response with invalid JSON
	p := NewOpenRouterProvider("key", "openai/gpt-4")
	p.client = &http.Client{
		Transport: &openRouterMockTransport{response: "invalid json", status: 200},
	}

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "openrouter decode") {
		t.Errorf("expected decode error, got %v", err)
	}
}

func TestOpenRouterProvider_Generate_WhenInvalidURL_ShouldReturnError(t *testing.T) {
	// Use an invalid URL to trigger request creation error
	p := NewOpenRouterProvider("key", "openai/gpt-4")
	p.baseURL = "http://invalid\x00url" // Invalid URL with null byte

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "openrouter request") {
		t.Errorf("expected request creation error, got %v", err)
	}
}

func TestOpenRouterProvider_Generate_WhenAPIEmptyChoices_ShouldReturnError(t *testing.T) {
	// Mock response with no choices
	mockResp := `{"choices": []}`
	p := NewOpenRouterProvider("key", "openai/gpt-4")
	p.client = &http.Client{
		Transport: &openRouterMockTransport{response: mockResp, status: 200},
	}

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Errorf("expected error about no choices, got %v", err)
	}
}

func TestOpenRouterProvider_Generate_WhenAPISuccess_ShouldReturnResponse(t *testing.T) {
	// Mock successful response
	mockResp := `{
		"choices": [{"message": {"content": "Hello from OpenRouter"}}]
	}`
	p := NewOpenRouterProvider("key", "openai/gpt-4")
	p.client = &http.Client{
		Transport: &openRouterMockTransport{response: mockResp, status: 200},
	}

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Hello from OpenRouter" {
		t.Errorf("expected 'Hello from OpenRouter', got %q", result)
	}
}

func TestOpenRouterProvider_Generate_WhenAPIHasMultipleChoices_ShouldReturnFirstChoice(t *testing.T) {
	// Mock response with multiple choices
	mockResp := `{
		"choices": [
			{"message": {"content": "First choice"}},
			{"message": {"content": "Second choice"}}
		]
	}`
	p := NewOpenRouterProvider("key", "openai/gpt-4")
	p.client = &http.Client{
		Transport: &openRouterMockTransport{response: mockResp, status: 200},
	}

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "First choice" {
		t.Errorf("expected 'First choice', got %q", result)
	}
}

func TestOpenRouterProvider_Generate_WhenMarshalFails_ShouldReturnError(t *testing.T) {
	p := NewOpenRouterProvider("key", "openai/gpt-4")
	p.marshalFunc = failingMarshalFunc

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "openrouter marshal") {
		t.Errorf("expected marshal error, got %v", err)
	}
}

func TestOpenRouterProvider_Generate_WhenHTTPDoFails_ShouldReturnError(t *testing.T) {
	p := NewOpenRouterProvider("key", "openai/gpt-4")
	// Set a client with a transport that always fails
	p.client = &http.Client{
		Transport: &failingTransport{},
	}

	_, err := p.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "openrouter do") {
		t.Errorf("expected do error, got %v", err)
	}
}

func TestOpenRouterProvider_Generate_WhenRequestHeadersAreSetCorrectly(t *testing.T) {
	mockResp := `{"choices": [{"message": {"content": "test"}}]}`
	transport := &headerCheckingTransport{
		response: mockResp,
		status:   200,
		t:        t,
		expectedAuth: "Bearer test-key",
	}
	p := NewOpenRouterProvider("test-key", "openai/gpt-4")
	p.client = &http.Client{Transport: transport}

	_, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
}

// headerCheckingTransport checks request headers and returns a mock response
type headerCheckingTransport struct {
	response     string
	status       int
	t            *testing.T
	expectedAuth string
}

func (h *headerCheckingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check that the correct headers are set
	if req.Header.Get("Authorization") != h.expectedAuth {
		h.t.Errorf("expected Authorization header %q, got %q", h.expectedAuth, req.Header.Get("Authorization"))
	}
	if req.Header.Get("Content-Type") != "application/json" {
		h.t.Errorf("expected Content-Type header 'application/json', got %q", req.Header.Get("Content-Type"))
	}
	if req.Header.Get("HTTP-Referer") != "" {
		h.t.Errorf("expected no HTTP-Referer header by default, got %q", req.Header.Get("HTTP-Referer"))
	}
	return &http.Response{
		StatusCode: h.status,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(h.response)),
		Header:     make(http.Header),
	}, nil
}

func TestOpenRouterProvider_Generate_WhenModelIsSetCorrectly(t *testing.T) {
	mockResp := `{"choices": [{"message": {"content": "test"}}]}`
	transport := &bodyCheckingTransport{
		response:        mockResp,
		status:          200,
		t:               t,
		expectedModel:   "anthropic/claude-3",
	}
	p := NewOpenRouterProvider("key", "anthropic/claude-3")
	p.client = &http.Client{Transport: transport}

	_, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
}

// bodyCheckingTransport checks request body and returns a mock response
type bodyCheckingTransport struct {
	response      string
	status        int
	t             *testing.T
	expectedModel string
}

func (b *bodyCheckingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read the request body to check the model
	buf := make([]byte, 1024)
	n, _ := req.Body.Read(buf)
	body := string(buf[:n])

	if !strings.Contains(body, `"model":"`+b.expectedModel+`"`) {
		b.t.Errorf("expected request body to contain model %q, got %q", b.expectedModel, body)
	}
	return &http.Response{
		StatusCode: b.status,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(b.response)),
		Header:     make(http.Header),
	}, nil
}

// openRouterMockTransport returns a fixed response for testing.
type openRouterMockTransport struct {
	response string
	status   int
}

func (m *openRouterMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	status := "200 OK"
	if m.status != 200 {
		status = "500 Internal Server Error"
	}
	return &http.Response{
		StatusCode: m.status,
		Status:     status,
		Body:       io.NopCloser(strings.NewReader(m.response)),
		Header:     make(http.Header),
	}, nil
}


var _ domain.LLMProvider = (*OpenRouterProvider)(nil)