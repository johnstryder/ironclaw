package tooling

import (
	"encoding/json"

	"ironclaw/internal/domain"
)

// SchemaTool is a tool whose input is described by a JSON Schema generated from
// a Go struct via invopop/jsonschema. The brain passes Definition() to the LLM
// (function-calling API) and validates returned arguments before calling Call().
type SchemaTool interface {
	// Name returns the unique tool name used in function-calling (e.g. "calculator").
	Name() string
	// Description returns a human-readable description for the LLM.
	Description() string
	// Definition returns the JSON Schema string for the tool's input struct.
	Definition() string
	// Call executes the tool with the given JSON arguments.
	// Implementations must validate args against the schema before execution.
	Call(args json.RawMessage) (*domain.ToolResult, error)
}
