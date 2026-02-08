package tooling

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// =============================================================================
// mockSubAgentRunner — test double implementing domain.SubAgentRunner
// =============================================================================

type mockSubAgentRunner struct {
	response    string
	err         error
	gotSystem   string // last system prompt passed
	gotTask     string // last task passed
}

func (m *mockSubAgentRunner) RunSubAgent(ctx context.Context, systemPrompt string, task string) (string, error) {
	m.gotSystem = systemPrompt
	m.gotTask = task
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return m.response, m.err
}

// =============================================================================
// SpawnAgentTool — Construction
// =============================================================================

func TestNewSpawnAgentTool_ShouldReturnNonNilTool(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	if tool == nil {
		t.Fatal("expected non-nil SpawnAgentTool")
	}
}

func TestNewSpawnAgentTool_WhenRunnerIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSpawnAgentTool(nil) should panic")
		}
	}()
	NewSpawnAgentTool(nil)
}

// =============================================================================
// SpawnAgentTool — SchemaTool interface
// =============================================================================

func TestSpawnAgentTool_Name_ShouldReturnSpawnAgent(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	if tool.Name() != "spawn_agent" {
		t.Errorf("expected name 'spawn_agent', got %q", tool.Name())
	}
}

func TestSpawnAgentTool_Description_ShouldReturnNonEmptyDescription(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
	if !strings.Contains(strings.ToLower(desc), "sub-agent") && !strings.Contains(strings.ToLower(desc), "subagent") && !strings.Contains(strings.ToLower(desc), "specialist") {
		t.Errorf("description should mention sub-agent/specialist delegation, got %q", desc)
	}
}

func TestSpawnAgentTool_Definition_ShouldReturnValidJSONSchema(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Definition() should return valid JSON, got error: %v", err)
	}

	// Should be an object schema with "role" and "task" properties
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema should have 'properties' field")
	}
	if _, ok := props["role"]; !ok {
		t.Error("schema should have 'role' property")
	}
	if _, ok := props["task"]; !ok {
		t.Error("schema should have 'task' property")
	}
}

func TestSpawnAgentTool_Definition_ShouldRequireRoleAndTask(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Definition() should return valid JSON, got error: %v", err)
	}

	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("schema should have 'required' array")
	}
	requiredFields := make(map[string]bool)
	for _, r := range required {
		requiredFields[r.(string)] = true
	}
	if !requiredFields["role"] {
		t.Error("'role' should be required")
	}
	if !requiredFields["task"] {
		t.Error("'task' should be required")
	}
}

// =============================================================================
// SpawnAgentTool — Call (happy path)
// =============================================================================

func TestSpawnAgentTool_Call_ShouldReturnSubAgentResponse(t *testing.T) {
	runner := &mockSubAgentRunner{response: "Python uses duck typing"}
	tool := NewSpawnAgentTool(runner)

	args := json.RawMessage(`{"role": "You are a Python Expert", "task": "Explain Python typing"}`)
	result, err := tool.Call(args)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Data != "Python uses duck typing" {
		t.Errorf("expected %q, got %q", "Python uses duck typing", result.Data)
	}
}

func TestSpawnAgentTool_Call_ShouldPassRoleAsSystemPromptToRunner(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)

	args := json.RawMessage(`{"role": "You are a Summarizer", "task": "Summarize this text"}`)
	_, err := tool.Call(args)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if runner.gotSystem != "You are a Summarizer" {
		t.Errorf("expected system prompt %q, got %q", "You are a Summarizer", runner.gotSystem)
	}
}

func TestSpawnAgentTool_Call_ShouldPassTaskToRunner(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)

	args := json.RawMessage(`{"role": "Expert", "task": "Summarize this text"}`)
	_, err := tool.Call(args)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if runner.gotTask != "Summarize this text" {
		t.Errorf("expected task %q, got %q", "Summarize this text", runner.gotTask)
	}
}

func TestSpawnAgentTool_Call_ShouldIncludeRoleInMetadata(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)

	args := json.RawMessage(`{"role": "Python Expert", "task": "some task"}`)
	result, err := tool.Call(args)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result.Metadata == nil {
		t.Fatal("expected non-nil metadata")
	}
	if result.Metadata["role"] != "Python Expert" {
		t.Errorf("expected metadata role 'Python Expert', got %q", result.Metadata["role"])
	}
}

// =============================================================================
// SpawnAgentTool — Call (error cases)
// =============================================================================

func TestSpawnAgentTool_Call_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)

	_, err := tool.Call(json.RawMessage(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSpawnAgentTool_Call_WhenMissingRole_ShouldReturnError(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)

	_, err := tool.Call(json.RawMessage(`{"task": "do something"}`))
	if err == nil {
		t.Error("expected error when role is missing")
	}
}

func TestSpawnAgentTool_Call_WhenMissingTask_ShouldReturnError(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)

	_, err := tool.Call(json.RawMessage(`{"role": "Expert"}`))
	if err == nil {
		t.Error("expected error when task is missing")
	}
}

func TestSpawnAgentTool_Call_WhenRunnerFails_ShouldReturnError(t *testing.T) {
	runner := &mockSubAgentRunner{err: errors.New("LLM unavailable")}
	tool := NewSpawnAgentTool(runner)

	args := json.RawMessage(`{"role": "Expert", "task": "do something"}`)
	_, err := tool.Call(args)
	if err == nil {
		t.Error("expected error when runner fails")
	}
	if !strings.Contains(err.Error(), "LLM unavailable") {
		t.Errorf("expected error to contain 'LLM unavailable', got %q", err.Error())
	}
}

func TestSpawnAgentTool_Call_WhenRunnerFails_ShouldReturnNilResult(t *testing.T) {
	runner := &mockSubAgentRunner{err: errors.New("fail")}
	tool := NewSpawnAgentTool(runner)

	args := json.RawMessage(`{"role": "Expert", "task": "do something"}`)
	result, _ := tool.Call(args)
	if result != nil {
		t.Error("expected nil result on error")
	}
}

// =============================================================================
// SpawnAgentTool — Implements SchemaTool interface
// =============================================================================

func TestSpawnAgentTool_ShouldImplementSchemaToolInterface(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	var _ SchemaTool = NewSpawnAgentTool(runner)
}

// =============================================================================
// SpawnAgentTool — Registration in ToolRegistry
// =============================================================================

func TestSpawnAgentTool_ShouldBeRegistrableInToolRegistry(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)

	reg := NewToolRegistry()
	err := reg.Register(tool)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := reg.Get("spawn_agent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name() != "spawn_agent" {
		t.Errorf("expected name 'spawn_agent', got %q", got.Name())
	}
}

// =============================================================================
// SpawnAgentTool — Schema validation via ValidateAgainstSchema
// =============================================================================

func TestSpawnAgentTool_SchemaValidation_ShouldAcceptValidArgs(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	schema := tool.Definition()

	err := ValidateAgainstSchema(
		json.RawMessage(`{"role": "Expert", "task": "do something"}`),
		schema,
	)
	if err != nil {
		t.Fatalf("expected valid args to pass schema validation, got: %v", err)
	}
}

func TestSpawnAgentTool_SchemaValidation_ShouldRejectMissingRole(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	schema := tool.Definition()

	err := ValidateAgainstSchema(
		json.RawMessage(`{"task": "do something"}`),
		schema,
	)
	if err == nil {
		t.Error("expected schema validation to reject missing 'role'")
	}
}

func TestSpawnAgentTool_SchemaValidation_ShouldRejectMissingTask(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	schema := tool.Definition()

	err := ValidateAgainstSchema(
		json.RawMessage(`{"role": "Expert"}`),
		schema,
	)
	if err == nil {
		t.Error("expected schema validation to reject missing 'task'")
	}
}

func TestSpawnAgentTool_SchemaValidation_ShouldRejectWrongTypeForRole(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	schema := tool.Definition()

	err := ValidateAgainstSchema(
		json.RawMessage(`{"role": 123, "task": "do something"}`),
		schema,
	)
	if err == nil {
		t.Error("expected schema validation to reject numeric 'role'")
	}
}

func TestSpawnAgentTool_SchemaValidation_ShouldRejectWrongTypeForTask(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)
	schema := tool.Definition()

	err := ValidateAgainstSchema(
		json.RawMessage(`{"role": "Expert", "task": true}`),
		schema,
	)
	if err == nil {
		t.Error("expected schema validation to reject boolean 'task'")
	}
}

// =============================================================================
// SpawnAgentTool — Definitions for LLM
// =============================================================================

func TestSpawnAgentTool_Definitions_ShouldAppearInRegistryDefinitions(t *testing.T) {
	runner := &mockSubAgentRunner{response: "ok"}
	tool := NewSpawnAgentTool(runner)

	reg := NewToolRegistry()
	_ = reg.Register(tool)

	defs := reg.Definitions()
	found := false
	for _, d := range defs {
		if d.Name == "spawn_agent" {
			found = true
			if d.Description == "" {
				t.Error("expected non-empty description in ToolDefinition")
			}
			if len(d.InputSchema) == 0 {
				t.Error("expected non-empty InputSchema in ToolDefinition")
			}
			// Verify schema is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal(d.InputSchema, &parsed); err != nil {
				t.Fatalf("InputSchema is not valid JSON: %v", err)
			}
		}
	}
	if !found {
		t.Error("expected spawn_agent in registry definitions")
	}
}

// =============================================================================
// SpawnAgentTool — Runner error propagation
// =============================================================================

func TestSpawnAgentTool_Call_WhenRunnerReturnsContextCanceled_ShouldReturnError(t *testing.T) {
	runner := &mockSubAgentRunner{err: context.Canceled}
	tool := NewSpawnAgentTool(runner)

	args := json.RawMessage(`{"role": "Expert", "task": "do something"}`)
	_, err := tool.Call(args)
	if err == nil {
		t.Error("expected error when runner returns context.Canceled")
	}
}

// =============================================================================
// SpawnAgentTool — Defense-in-depth unmarshal error path
// =============================================================================

func TestSpawnAgentTool_Call_WhenUnmarshalFails_ShouldReturnError(t *testing.T) {
	// Inject a failing unmarshaler to cover the defense-in-depth path.
	// Schema validation passes but json.Unmarshal fails (e.g. due to a Go struct issue).
	original := spawnUnmarshalFunc
	spawnUnmarshalFunc = func(data []byte, v interface{}) error {
		return errors.New("injected unmarshal failure")
	}
	defer func() { spawnUnmarshalFunc = original }()

	runner := &mockSubAgentRunner{response: "should not reach"}
	tool := NewSpawnAgentTool(runner)

	args := json.RawMessage(`{"role": "Expert", "task": "do something"}`)
	result, err := tool.Call(args)
	if err == nil {
		t.Error("expected error when unmarshal fails")
	}
	if result != nil {
		t.Error("expected nil result when unmarshal fails")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("expected error to contain 'failed to parse input', got %q", err.Error())
	}
}
