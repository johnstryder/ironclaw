package tooling

import (
	"encoding/json"
	"fmt"

	"ironclaw/internal/domain"
)

// ToolRegistry holds SchemaTool implementations keyed by name. The brain uses
// it to enumerate tool definitions for the LLM and dispatch calls.
type ToolRegistry struct {
	tools map[string]SchemaTool
}

// NewToolRegistry returns an empty, ready-to-use registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]SchemaTool)}
}

// Register adds a tool. Returns an error if the tool is nil or a tool with the
// same name is already registered.
func (r *ToolRegistry) Register(tool SchemaTool) error {
	if tool == nil {
		return fmt.Errorf("tool must not be nil")
	}
	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q is already registered", name)
	}
	r.tools[name] = tool
	return nil
}

// Get returns the tool with the given name or an error if not found.
func (r *ToolRegistry) Get(name string) (SchemaTool, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %q", name)
	}
	return tool, nil
}

// List returns all registered tools (order is non-deterministic).
func (r *ToolRegistry) List() []SchemaTool {
	out := make([]SchemaTool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Definitions returns domain.ToolDefinition for every registered tool,
// suitable for passing to the LLM function-calling API.
func (r *ToolRegistry) Definitions() []domain.ToolDefinition {
	out := make([]domain.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, domain.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: json.RawMessage(t.Definition()),
		})
	}
	return out
}
