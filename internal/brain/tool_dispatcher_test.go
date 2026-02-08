package brain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"ironclaw/internal/domain"
	"ironclaw/internal/tooling"
)

// =============================================================================
// fakeSchemaTool — test double for dispatcher tests
// =============================================================================

type fakeSchemaTool struct {
	name       string
	desc       string
	schema     string
	callResult *domain.ToolResult
	callErr    error
}

func (f *fakeSchemaTool) Name() string                                          { return f.name }
func (f *fakeSchemaTool) Description() string                                   { return f.desc }
func (f *fakeSchemaTool) Definition() string                                    { return f.schema }
func (f *fakeSchemaTool) Call(args json.RawMessage) (*domain.ToolResult, error) { return f.callResult, f.callErr }

func validSchema() string {
	return `{"type":"object","properties":{"x":{"type":"number"}},"required":["x"]}`
}

func newFake(name string) *fakeSchemaTool {
	return &fakeSchemaTool{
		name:       name,
		desc:       name + " description",
		schema:     validSchema(),
		callResult: &domain.ToolResult{Data: name + "-result"},
	}
}

// =============================================================================
// NewToolDispatcher
// =============================================================================

func TestNewToolDispatcher_ShouldReturnDispatcherWithRegistry(t *testing.T) {
	reg := tooling.NewToolRegistry()
	d := NewToolDispatcher(reg)
	if d == nil {
		t.Fatal("Expected non-nil dispatcher")
	}
}

func TestNewToolDispatcher_ShouldPanicWhenRegistryIsNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when registry is nil")
		}
	}()
	NewToolDispatcher(nil)
}

// =============================================================================
// FormatToolsForLLM
// =============================================================================

func TestToolDispatcher_FormatToolsForLLM_ShouldReturnToolDefinitions(t *testing.T) {
	reg := tooling.NewToolRegistry()
	_ = reg.Register(newFake("echo"))
	_ = reg.Register(newFake("calc"))
	d := NewToolDispatcher(reg)

	defs := d.FormatToolsForLLM()
	if len(defs) != 2 {
		t.Fatalf("Expected 2 definitions, got %d", len(defs))
	}

	names := map[string]bool{}
	for _, def := range defs {
		names[def.Name] = true
		if def.Description == "" {
			t.Errorf("Tool '%s' should have a description", def.Name)
		}
		if len(def.InputSchema) == 0 {
			t.Errorf("Tool '%s' should have an InputSchema", def.Name)
		}
	}
	for _, expected := range []string{"echo", "calc"} {
		if !names[expected] {
			t.Errorf("Expected tool '%s' in definitions", expected)
		}
	}
}

func TestToolDispatcher_FormatToolsForLLM_ShouldReturnEmptyWhenNoTools(t *testing.T) {
	reg := tooling.NewToolRegistry()
	d := NewToolDispatcher(reg)

	defs := d.FormatToolsForLLM()
	if len(defs) != 0 {
		t.Errorf("Expected 0 definitions, got %d", len(defs))
	}
}

// =============================================================================
// HandleToolCall — happy path
// =============================================================================

func TestToolDispatcher_HandleToolCall_ShouldCallToolWithValidArgs(t *testing.T) {
	reg := tooling.NewToolRegistry()
	_ = reg.Register(newFake("echo"))
	d := NewToolDispatcher(reg)

	result, err := d.HandleToolCall("echo", json.RawMessage(`{"x": 42}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Data != "echo-result" {
		t.Errorf("Expected data 'echo-result', got '%s'", result.Data)
	}
}

// =============================================================================
// HandleToolCall — unknown tool
// =============================================================================

func TestToolDispatcher_HandleToolCall_ShouldReturnErrorForUnknownTool(t *testing.T) {
	reg := tooling.NewToolRegistry()
	d := NewToolDispatcher(reg)

	_, err := d.HandleToolCall("nonexistent", json.RawMessage(`{"x": 1}`))
	if err == nil {
		t.Error("Expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("Expected 'unknown tool' in error, got: %v", err)
	}
}

// =============================================================================
// HandleToolCall — schema validation
// =============================================================================

func TestToolDispatcher_HandleToolCall_ShouldRejectInvalidArgsWithSchemaError(t *testing.T) {
	reg := tooling.NewToolRegistry()
	_ = reg.Register(newFake("echo"))
	d := NewToolDispatcher(reg)

	// "x" should be a number, not a string
	_, err := d.HandleToolCall("echo", json.RawMessage(`{"x": "not-a-number"}`))
	if err == nil {
		t.Error("Expected schema validation error for invalid args")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("Expected 'validation' in error, got: %v", err)
	}
}

func TestToolDispatcher_HandleToolCall_ShouldRejectMissingRequiredField(t *testing.T) {
	reg := tooling.NewToolRegistry()
	_ = reg.Register(newFake("echo"))
	d := NewToolDispatcher(reg)

	_, err := d.HandleToolCall("echo", json.RawMessage(`{}`))
	if err == nil {
		t.Error("Expected error for missing required field 'x'")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("Expected 'validation' in error, got: %v", err)
	}
}

func TestToolDispatcher_HandleToolCall_ShouldRejectInvalidJSON(t *testing.T) {
	reg := tooling.NewToolRegistry()
	_ = reg.Register(newFake("echo"))
	d := NewToolDispatcher(reg)

	_, err := d.HandleToolCall("echo", json.RawMessage(`{invalid`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestToolDispatcher_HandleToolCall_ShouldNotCallToolWhenValidationFails(t *testing.T) {
	// Ensure the tool's Call() is never reached on bad input
	called := false
	fake := &fakeSchemaTool{
		name:   "guard",
		desc:   "guarded tool",
		schema: validSchema(),
		callResult: &domain.ToolResult{Data: "should-not-see"},
	}
	// Override Call to detect invocation
	type callTracker struct {
		*fakeSchemaTool
	}
	tracker := &callTrackingSchemaTool{
		inner:  fake,
		called: &called,
	}

	reg := tooling.NewToolRegistry()
	_ = reg.Register(tracker)
	d := NewToolDispatcher(reg)

	_, _ = d.HandleToolCall("guard", json.RawMessage(`{"x": "bad-type"}`))
	if called {
		t.Error("Tool.Call() should not be invoked when schema validation fails")
	}
}

// callTrackingSchemaTool wraps a SchemaTool and tracks if Call was invoked.
type callTrackingSchemaTool struct {
	inner  *fakeSchemaTool
	called *bool
}

func (c *callTrackingSchemaTool) Name() string        { return c.inner.Name() }
func (c *callTrackingSchemaTool) Description() string  { return c.inner.Description() }
func (c *callTrackingSchemaTool) Definition() string   { return c.inner.Definition() }
func (c *callTrackingSchemaTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	*c.called = true
	return c.inner.Call(args)
}

// =============================================================================
// HandleToolCall — tool returns error
// =============================================================================

func TestToolDispatcher_HandleToolCall_ShouldPropagateToolError(t *testing.T) {
	reg := tooling.NewToolRegistry()
	fake := newFake("failme")
	fake.callErr = &json.SyntaxError{}
	_ = reg.Register(fake)
	d := NewToolDispatcher(reg)

	_, err := d.HandleToolCall("failme", json.RawMessage(`{"x": 1}`))
	if err == nil {
		t.Error("Expected propagated error from tool")
	}
}

// =============================================================================
// Integration: SpawnAgentTool through dispatcher
// =============================================================================

func TestToolDispatcher_Integration_SpawnAgentToolThroughDispatcher(t *testing.T) {
	// Create a mock SubAgentRunner
	runner := &fakeSubAgentRunner{response: "The capital of France is Paris."}

	reg := tooling.NewToolRegistry()
	_ = reg.Register(tooling.NewSpawnAgentTool(runner))
	d := NewToolDispatcher(reg)

	// Valid call
	result, err := d.HandleToolCall("spawn_agent", json.RawMessage(`{"role":"You are a Geography Expert","task":"What is the capital of France?"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "The capital of France is Paris." {
		t.Errorf("Expected 'The capital of France is Paris.', got '%s'", result.Data)
	}
	if result.Metadata["role"] != "You are a Geography Expert" {
		t.Errorf("Expected role metadata 'You are a Geography Expert', got '%s'", result.Metadata["role"])
	}

	// Verify role and task were passed correctly
	if runner.gotSystem != "You are a Geography Expert" {
		t.Errorf("Expected system prompt 'You are a Geography Expert', got '%s'", runner.gotSystem)
	}
	if runner.gotTask != "What is the capital of France?" {
		t.Errorf("Expected task 'What is the capital of France?', got '%s'", runner.gotTask)
	}

	// Invalid call — missing required field
	_, err = d.HandleToolCall("spawn_agent", json.RawMessage(`{"role":"Expert"}`))
	if err == nil {
		t.Error("Expected validation error for missing 'task'")
	}

	// Invalid call — wrong type
	_, err = d.HandleToolCall("spawn_agent", json.RawMessage(`{"role":123,"task":"something"}`))
	if err == nil {
		t.Error("Expected validation error for wrong type")
	}
}

func TestToolDispatcher_Integration_SpawnAgentToolWhenRunnerFails(t *testing.T) {
	runner := &fakeSubAgentRunner{err: errors.New("sub-agent timeout")}

	reg := tooling.NewToolRegistry()
	_ = reg.Register(tooling.NewSpawnAgentTool(runner))
	d := NewToolDispatcher(reg)

	_, err := d.HandleToolCall("spawn_agent", json.RawMessage(`{"role":"Expert","task":"do something"}`))
	if err == nil {
		t.Error("Expected error when runner fails")
	}
	if !strings.Contains(err.Error(), "sub-agent timeout") {
		t.Errorf("Expected error to contain 'sub-agent timeout', got: %v", err)
	}
}

// fakeSubAgentRunner is a test double for domain.SubAgentRunner.
type fakeSubAgentRunner struct {
	response  string
	err       error
	gotSystem string
	gotTask   string
}

func (f *fakeSubAgentRunner) RunSubAgent(ctx context.Context, systemPrompt string, task string) (string, error) {
	f.gotSystem = systemPrompt
	f.gotTask = task
	return f.response, f.err
}

// =============================================================================
// Integration: CalculatorTool through dispatcher
// =============================================================================

func TestToolDispatcher_Integration_CalculatorToolThroughDispatcher(t *testing.T) {
	reg := tooling.NewToolRegistry()
	_ = reg.Register(&tooling.CalculatorTool{})
	d := NewToolDispatcher(reg)

	// Valid call
	result, err := d.HandleToolCall("calculator", json.RawMessage(`{"operation":"add","a":10,"b":5}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "15.00" {
		t.Errorf("Expected '15.00', got '%s'", result.Data)
	}

	// Invalid call — bad operation
	_, err = d.HandleToolCall("calculator", json.RawMessage(`{"operation":"power","a":2,"b":3}`))
	if err == nil {
		t.Error("Expected validation error for unsupported operation 'power'")
	}

	// Invalid call — wrong type
	_, err = d.HandleToolCall("calculator", json.RawMessage(`{"operation":"add","a":"text","b":3}`))
	if err == nil {
		t.Error("Expected validation error for wrong type")
	}
}
