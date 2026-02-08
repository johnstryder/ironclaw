package tooling

import (
	"encoding/json"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// stubSchemaTool — minimal SchemaTool for registry tests
// =============================================================================

type stubSchemaTool struct {
	name string
	desc string
	def  string
}

func (s *stubSchemaTool) Name() string        { return s.name }
func (s *stubSchemaTool) Description() string  { return s.desc }
func (s *stubSchemaTool) Definition() string   { return s.def }
func (s *stubSchemaTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	return &domain.ToolResult{Data: "stub-ok"}, nil
}

func newStub(name, desc string) *stubSchemaTool {
	return &stubSchemaTool{
		name: name,
		desc: desc,
		def:  `{"type":"object","properties":{"x":{"type":"number"}},"required":["x"]}`,
	}
}

// =============================================================================
// ToolRegistry Tests
// =============================================================================

func TestNewToolRegistry_ShouldReturnEmptyRegistry(t *testing.T) {
	reg := NewToolRegistry()
	if reg == nil {
		t.Fatal("Expected non-nil registry")
	}
	tools := reg.List()
	if len(tools) != 0 {
		t.Errorf("Expected empty tool list, got %d", len(tools))
	}
}

func TestToolRegistry_Register_ShouldAddTool(t *testing.T) {
	reg := NewToolRegistry()
	stub := newStub("echo", "Echo tool")

	err := reg.Register(stub)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}
}

func TestToolRegistry_Register_ShouldRejectDuplicateName(t *testing.T) {
	reg := NewToolRegistry()
	stub1 := newStub("echo", "Echo v1")
	stub2 := newStub("echo", "Echo v2")

	if err := reg.Register(stub1); err != nil {
		t.Fatalf("First register should succeed: %v", err)
	}
	err := reg.Register(stub2)
	if err == nil {
		t.Error("Expected error when registering duplicate tool name")
	}
}

func TestToolRegistry_Register_ShouldRejectNilTool(t *testing.T) {
	reg := NewToolRegistry()
	err := reg.Register(nil)
	if err == nil {
		t.Error("Expected error when registering nil tool")
	}
}

func TestToolRegistry_Get_ShouldReturnRegisteredTool(t *testing.T) {
	reg := NewToolRegistry()
	stub := newStub("echo", "Echo tool")
	_ = reg.Register(stub)

	tool, err := reg.Get("echo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if tool.Name() != "echo" {
		t.Errorf("Expected tool name 'echo', got '%s'", tool.Name())
	}
}

func TestToolRegistry_Get_ShouldReturnErrorForUnknownTool(t *testing.T) {
	reg := NewToolRegistry()

	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for unknown tool")
	}
}

func TestToolRegistry_List_ShouldReturnAllRegisteredTools(t *testing.T) {
	reg := NewToolRegistry()
	_ = reg.Register(newStub("tool_a", "Tool A"))
	_ = reg.Register(newStub("tool_b", "Tool B"))
	_ = reg.Register(newStub("tool_c", "Tool C"))

	tools := reg.List()
	if len(tools) != 3 {
		t.Fatalf("Expected 3 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	for _, expected := range []string{"tool_a", "tool_b", "tool_c"} {
		if !names[expected] {
			t.Errorf("Expected tool '%s' in list", expected)
		}
	}
}

func TestToolRegistry_Definitions_ShouldReturnLLMCompatibleSchemas(t *testing.T) {
	reg := NewToolRegistry()
	_ = reg.Register(newStub("echo", "Echo tool"))

	defs := reg.Definitions()
	if len(defs) != 1 {
		t.Fatalf("Expected 1 definition, got %d", len(defs))
	}

	def := defs[0]
	if def.Name != "echo" {
		t.Errorf("Expected name 'echo', got '%s'", def.Name)
	}
	if def.Description != "Echo tool" {
		t.Errorf("Expected description 'Echo tool', got '%s'", def.Description)
	}
	if len(def.InputSchema) == 0 {
		t.Error("Expected non-empty InputSchema")
	}

	// InputSchema should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(def.InputSchema, &parsed); err != nil {
		t.Errorf("InputSchema should be valid JSON: %v", err)
	}
}

func TestToolRegistry_Definitions_ShouldReturnEmptyForEmptyRegistry(t *testing.T) {
	reg := NewToolRegistry()
	defs := reg.Definitions()
	if len(defs) != 0 {
		t.Errorf("Expected 0 definitions, got %d", len(defs))
	}
}
