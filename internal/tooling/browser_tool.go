package tooling

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"ironclaw/internal/domain"
)

// Browser abstracts headless browser operations for testability.
type Browser interface {
	Navigate(url string) (string, error)
	GetText(selector string) (string, error)
	Screenshot() ([]byte, error)
	Close() error
}

// BrowserInput represents the input structure for browser operations.
type BrowserInput struct {
	Operation string `json:"operation" jsonschema:"enum=navigate,enum=get_text,enum=screenshot"`
	URL       string `json:"url,omitempty"`
	Selector  string `json:"selector,omitempty"`
}

// BrowserTool provides headless browser automation via the Browser interface.
// Supports navigating to URLs, extracting text by CSS selector, and screenshots.
type BrowserTool struct {
	browser Browser
}

// NewBrowserTool creates a BrowserTool with the given Browser implementation.
func NewBrowserTool(browser Browser) *BrowserTool {
	return &BrowserTool{browser: browser}
}

// Name returns the tool name used in function-calling.
func (b *BrowserTool) Name() string { return "browser" }

// Description returns a human-readable description for the LLM.
func (b *BrowserTool) Description() string {
	return "Headless browser automation: navigate to URLs, extract text by CSS selector, and take screenshots"
}

// Definition returns the JSON Schema for browser input.
func (b *BrowserTool) Definition() string {
	return GenerateSchema(BrowserInput{})
}

// browserUnmarshalFunc is the JSON unmarshaler used by Call. Package-level so
// tests can inject a failing unmarshaler to cover the defense-in-depth error path.
var browserUnmarshalFunc = json.Unmarshal

// Call validates the input and dispatches to the appropriate browser operation.
func (b *BrowserTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	// 1. Validate input against JSON schema
	schema := b.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 2. Unmarshal input
	var input BrowserInput
	if err := browserUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// 3. Dispatch to the appropriate operation
	switch input.Operation {
	case "navigate":
		return b.navigate(input.URL)
	case "get_text":
		return b.getText(input.Selector)
	case "screenshot":
		return b.screenshot()
	default:
		return nil, fmt.Errorf("unknown operation: %s", input.Operation)
	}
}

func (b *BrowserTool) navigate(url string) (*domain.ToolResult, error) {
	title, err := b.browser.Navigate(url)
	if err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	return &domain.ToolResult{
		Data: title,
		Metadata: map[string]string{
			"operation": "navigate",
			"url":       url,
		},
	}, nil
}

func (b *BrowserTool) getText(selector string) (*domain.ToolResult, error) {
	text, err := b.browser.GetText(selector)
	if err != nil {
		return nil, fmt.Errorf("failed to get text: %w", err)
	}

	return &domain.ToolResult{
		Data: text,
		Metadata: map[string]string{
			"operation": "get_text",
			"selector":  selector,
		},
	}, nil
}

func (b *BrowserTool) screenshot() (*domain.ToolResult, error) {
	data, err := b.browser.Screenshot()
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	return &domain.ToolResult{
		Data: encoded,
		Metadata: map[string]string{
			"operation": "screenshot",
			"encoding":  "base64",
		},
	}, nil
}
