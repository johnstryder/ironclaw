package tooling

import (
	"encoding/json"
	"fmt"
	"testing"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockBrowser is a test double for Browser.
type mockBrowser struct {
	navigateResult string
	navigateErr    error
	getTextResult  string
	getTextErr     error
	screenshotData []byte
	screenshotErr  error
	closeCalled    bool
}

func (m *mockBrowser) Navigate(url string) (string, error) {
	return m.navigateResult, m.navigateErr
}

func (m *mockBrowser) GetText(selector string) (string, error) {
	return m.getTextResult, m.getTextErr
}

func (m *mockBrowser) Screenshot() ([]byte, error) {
	return m.screenshotData, m.screenshotErr
}

func (m *mockBrowser) Close() error {
	m.closeCalled = true
	return nil
}

// =============================================================================
// BrowserTool — Name, Description, Definition
// =============================================================================

func TestBrowserTool_Name_ShouldReturnBrowser(t *testing.T) {
	tool := NewBrowserTool(&mockBrowser{})
	if tool.Name() != "browser" {
		t.Errorf("Expected name 'browser', got '%s'", tool.Name())
	}
}

func TestBrowserTool_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	tool := NewBrowserTool(&mockBrowser{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestBrowserTool_Definition_ShouldContainOperationProperty(t *testing.T) {
	tool := NewBrowserTool(&mockBrowser{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", parsed["type"])
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'properties' in schema")
	}
	if _, exists := props["operation"]; !exists {
		t.Error("Expected 'operation' property in schema")
	}
}

func TestBrowserTool_Definition_ShouldContainURLProperty(t *testing.T) {
	tool := NewBrowserTool(&mockBrowser{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'properties' in schema")
	}
	if _, exists := props["url"]; !exists {
		t.Error("Expected 'url' property in schema")
	}
}

func TestBrowserTool_Definition_ShouldContainSelectorProperty(t *testing.T) {
	tool := NewBrowserTool(&mockBrowser{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'properties' in schema")
	}
	if _, exists := props["selector"]; !exists {
		t.Error("Expected 'selector' property in schema")
	}
}

func TestBrowserTool_Definition_ShouldRequireOperation(t *testing.T) {
	tool := NewBrowserTool(&mockBrowser{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("Expected 'required' array in schema")
	}
	requiredFields := make(map[string]bool)
	for _, r := range required {
		requiredFields[r.(string)] = true
	}
	if !requiredFields["operation"] {
		t.Error("Expected 'operation' in required fields")
	}
}

// =============================================================================
// BrowserTool.Call — Input Validation
// =============================================================================

func TestBrowserTool_Call_ShouldRejectInvalidJSON(t *testing.T) {
	tool := NewBrowserTool(&mockBrowser{})
	_, err := tool.Call(json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if !contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestBrowserTool_Call_ShouldRejectMissingOperationField(t *testing.T) {
	tool := NewBrowserTool(&mockBrowser{})
	_, err := tool.Call(json.RawMessage(`{"url":"https://example.com"}`))
	if err == nil {
		t.Fatal("Expected error for missing operation field")
	}
	if !contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestBrowserTool_Call_ShouldRejectInvalidOperation(t *testing.T) {
	tool := NewBrowserTool(&mockBrowser{})
	_, err := tool.Call(json.RawMessage(`{"operation":"delete"}`))
	if err == nil {
		t.Fatal("Expected error for invalid operation")
	}
}

// contains is a test helper to check substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// BrowserTool.Call — Navigate Operation
// =============================================================================

func TestBrowserTool_Call_Navigate_ShouldReturnPageTitle(t *testing.T) {
	browser := &mockBrowser{
		navigateResult: "Example Domain",
	}
	tool := NewBrowserTool(browser)
	result, err := tool.Call(json.RawMessage(`{"operation":"navigate","url":"https://example.com"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "Example Domain" {
		t.Errorf("Expected 'Example Domain', got '%s'", result.Data)
	}
}

func TestBrowserTool_Call_Navigate_ShouldReturnMetadata(t *testing.T) {
	browser := &mockBrowser{
		navigateResult: "Example Domain",
	}
	tool := NewBrowserTool(browser)
	result, err := tool.Call(json.RawMessage(`{"operation":"navigate","url":"https://example.com"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["operation"] != "navigate" {
		t.Errorf("Expected metadata operation='navigate', got '%s'", result.Metadata["operation"])
	}
	if result.Metadata["url"] != "https://example.com" {
		t.Errorf("Expected metadata url='https://example.com', got '%s'", result.Metadata["url"])
	}
}

func TestBrowserTool_Call_Navigate_ShouldReturnErrorWhenBrowserFails(t *testing.T) {
	browser := &mockBrowser{
		navigateErr: fmt.Errorf("connection refused"),
	}
	tool := NewBrowserTool(browser)
	_, err := tool.Call(json.RawMessage(`{"operation":"navigate","url":"https://example.com"}`))
	if err == nil {
		t.Fatal("Expected error when Navigate fails")
	}
	if !contains(err.Error(), "failed to navigate") {
		t.Errorf("Expected 'failed to navigate' in error, got: %v", err)
	}
}

func TestBrowserTool_Call_Navigate_ShouldReturnEmptyTitleForBlankPage(t *testing.T) {
	browser := &mockBrowser{
		navigateResult: "",
	}
	tool := NewBrowserTool(browser)
	result, err := tool.Call(json.RawMessage(`{"operation":"navigate","url":"about:blank"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "" {
		t.Errorf("Expected empty data for blank page, got '%s'", result.Data)
	}
}

// =============================================================================
// BrowserTool.Call — GetText Operation
// =============================================================================

func TestBrowserTool_Call_GetText_ShouldReturnElementText(t *testing.T) {
	browser := &mockBrowser{
		getTextResult: "Hello World",
	}
	tool := NewBrowserTool(browser)
	result, err := tool.Call(json.RawMessage(`{"operation":"get_text","selector":"h1"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", result.Data)
	}
}

func TestBrowserTool_Call_GetText_ShouldReturnMetadata(t *testing.T) {
	browser := &mockBrowser{
		getTextResult: "content",
	}
	tool := NewBrowserTool(browser)
	result, err := tool.Call(json.RawMessage(`{"operation":"get_text","selector":"div.main"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["operation"] != "get_text" {
		t.Errorf("Expected metadata operation='get_text', got '%s'", result.Metadata["operation"])
	}
	if result.Metadata["selector"] != "div.main" {
		t.Errorf("Expected metadata selector='div.main', got '%s'", result.Metadata["selector"])
	}
}

func TestBrowserTool_Call_GetText_ShouldReturnErrorWhenElementNotFound(t *testing.T) {
	browser := &mockBrowser{
		getTextErr: fmt.Errorf("element not found"),
	}
	tool := NewBrowserTool(browser)
	_, err := tool.Call(json.RawMessage(`{"operation":"get_text","selector":"#nonexistent"}`))
	if err == nil {
		t.Fatal("Expected error when GetText fails")
	}
	if !contains(err.Error(), "failed to get text") {
		t.Errorf("Expected 'failed to get text' in error, got: %v", err)
	}
}

func TestBrowserTool_Call_GetText_ShouldReturnEmptyForEmptyElement(t *testing.T) {
	browser := &mockBrowser{
		getTextResult: "",
	}
	tool := NewBrowserTool(browser)
	result, err := tool.Call(json.RawMessage(`{"operation":"get_text","selector":"span.empty"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "" {
		t.Errorf("Expected empty data, got '%s'", result.Data)
	}
}

// =============================================================================
// BrowserTool.Call — Screenshot Operation
// =============================================================================

func TestBrowserTool_Call_Screenshot_ShouldReturnBase64Data(t *testing.T) {
	fakeImage := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes
	browser := &mockBrowser{
		screenshotData: fakeImage,
	}
	tool := NewBrowserTool(browser)
	result, err := tool.Call(json.RawMessage(`{"operation":"screenshot"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data == "" {
		t.Error("Expected non-empty base64 data")
	}
}

func TestBrowserTool_Call_Screenshot_ShouldReturnMetadata(t *testing.T) {
	fakeImage := []byte{0x89, 0x50, 0x4E, 0x47}
	browser := &mockBrowser{
		screenshotData: fakeImage,
	}
	tool := NewBrowserTool(browser)
	result, err := tool.Call(json.RawMessage(`{"operation":"screenshot"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["operation"] != "screenshot" {
		t.Errorf("Expected metadata operation='screenshot', got '%s'", result.Metadata["operation"])
	}
	if result.Metadata["encoding"] != "base64" {
		t.Errorf("Expected metadata encoding='base64', got '%s'", result.Metadata["encoding"])
	}
}

func TestBrowserTool_Call_Screenshot_ShouldReturnErrorWhenScreenshotFails(t *testing.T) {
	browser := &mockBrowser{
		screenshotErr: fmt.Errorf("page not loaded"),
	}
	tool := NewBrowserTool(browser)
	_, err := tool.Call(json.RawMessage(`{"operation":"screenshot"}`))
	if err == nil {
		t.Fatal("Expected error when Screenshot fails")
	}
	if !contains(err.Error(), "failed to take screenshot") {
		t.Errorf("Expected 'failed to take screenshot' in error, got: %v", err)
	}
}

// =============================================================================
// BrowserTool.Call — Unmarshal error path (defense-in-depth)
// =============================================================================

func TestBrowserTool_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := browserUnmarshalFunc
	browserUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { browserUnmarshalFunc = original }()

	tool := NewBrowserTool(&mockBrowser{})
	_, err := tool.Call(json.RawMessage(`{"operation":"navigate","url":"https://example.com"}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

func TestBrowserTool_Call_ShouldReturnErrorForUnknownOperationDefenseInDepth(t *testing.T) {
	original := browserUnmarshalFunc
	browserUnmarshalFunc = func(data []byte, v interface{}) error {
		input, ok := v.(*BrowserInput)
		if !ok {
			return fmt.Errorf("unexpected type")
		}
		input.Operation = "unknown_op"
		return nil
	}
	defer func() { browserUnmarshalFunc = original }()

	tool := NewBrowserTool(&mockBrowser{})
	_, err := tool.Call(json.RawMessage(`{"operation":"navigate","url":"https://example.com"}`))
	if err == nil {
		t.Fatal("Expected error for unknown operation (defense-in-depth)")
	}
	if !contains(err.Error(), "unknown operation") {
		t.Errorf("Expected 'unknown operation' in error, got: %v", err)
	}
}

// =============================================================================
// spyBrowser — verifies the correct arguments are passed to the Browser layer
// =============================================================================

type spyBrowser struct {
	calledURL      string
	calledSelector string
	navigateCalled bool
	getTextCalled  bool
	screenshotCalled bool
	closeCalled    bool
}

func (s *spyBrowser) Navigate(url string) (string, error) {
	s.navigateCalled = true
	s.calledURL = url
	return "spy-title", nil
}

func (s *spyBrowser) GetText(selector string) (string, error) {
	s.getTextCalled = true
	s.calledSelector = selector
	return "spy-text", nil
}

func (s *spyBrowser) Screenshot() ([]byte, error) {
	s.screenshotCalled = true
	return []byte("spy-screenshot"), nil
}

func (s *spyBrowser) Close() error {
	s.closeCalled = true
	return nil
}

func TestBrowserTool_Call_Navigate_ShouldPassURLToBrowser(t *testing.T) {
	spy := &spyBrowser{}
	tool := NewBrowserTool(spy)
	_, err := tool.Call(json.RawMessage(`{"operation":"navigate","url":"https://test.example.com"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !spy.navigateCalled {
		t.Error("Expected Navigate to be called")
	}
	if spy.calledURL != "https://test.example.com" {
		t.Errorf("Expected URL 'https://test.example.com', got '%s'", spy.calledURL)
	}
}

func TestBrowserTool_Call_GetText_ShouldPassSelectorToBrowser(t *testing.T) {
	spy := &spyBrowser{}
	tool := NewBrowserTool(spy)
	_, err := tool.Call(json.RawMessage(`{"operation":"get_text","selector":"div#content"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !spy.getTextCalled {
		t.Error("Expected GetText to be called")
	}
	if spy.calledSelector != "div#content" {
		t.Errorf("Expected selector 'div#content', got '%s'", spy.calledSelector)
	}
}

func TestBrowserTool_Call_Screenshot_ShouldCallBrowserScreenshot(t *testing.T) {
	spy := &spyBrowser{}
	tool := NewBrowserTool(spy)
	_, err := tool.Call(json.RawMessage(`{"operation":"screenshot"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !spy.screenshotCalled {
		t.Error("Expected Screenshot to be called")
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*BrowserTool)(nil)
var _ Browser = (*mockBrowser)(nil)
var _ Browser = (*spyBrowser)(nil)
