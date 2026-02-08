package brain

import (
	"encoding/json"
	"fmt"

	"ironclaw/internal/domain"
	"ironclaw/internal/tooling"
)

// ToolDispatcher connects the brain to SchemaTool implementations.
// It formats tool definitions for the LLM function-calling API and validates
// returned JSON arguments against each tool's schema before execution.
type ToolDispatcher struct {
	registry *tooling.ToolRegistry
}

// NewToolDispatcher creates a dispatcher backed by the given registry.
// Panics if registry is nil.
func NewToolDispatcher(registry *tooling.ToolRegistry) *ToolDispatcher {
	if registry == nil {
		panic("tool_dispatcher: registry must not be nil")
	}
	return &ToolDispatcher{registry: registry}
}

// FormatToolsForLLM returns domain.ToolDefinition slices ready to be serialised
// into the LLM function-calling request (e.g. Anthropic tools array).
func (d *ToolDispatcher) FormatToolsForLLM() []domain.ToolDefinition {
	return d.registry.Definitions()
}

// HandleToolCall looks up the tool by name, validates the raw JSON arguments
// against the tool's JSON Schema, and only then calls the tool. If the tool is
// unknown or validation fails, a descriptive error is returned and the tool is
// never invoked.
func (d *ToolDispatcher) HandleToolCall(name string, args json.RawMessage) (*domain.ToolResult, error) {
	tool, err := d.registry.Get(name)
	if err != nil {
		return nil, err // "unknown tool: ..."
	}

	// Validate args against the tool's JSON Schema before execution.
	schema := tool.Definition()
	if err := tooling.ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("schema validation failed for tool %q: %w", name, err)
	}

	return tool.Call(args)
}
