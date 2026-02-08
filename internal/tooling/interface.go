package tooling

import (
	"encoding/json"

	"ironclaw/internal/domain"
)

// Tool is re-exported from domain for use in this package. Tool implementations
// must return only *domain.ToolResult (Data, Metadata, Artifacts)—no raw API dumps.
type Tool = domain.Tool

// ToolDefinition and ToolResult are re-exported.
type ToolDefinition = domain.ToolDefinition
type ToolResult = domain.ToolResult

// ToolContext is the execution context for tools (matches context.Context).
type ToolContext = domain.ToolContext

// ExecuteTool runs the tool with the given input and returns the result or error.
func ExecuteTool(t domain.Tool, ctx domain.ToolContext, input json.RawMessage) (*domain.ToolResult, error) {
	return t.Execute(ctx, input)
}
