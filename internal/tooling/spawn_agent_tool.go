package tooling

import (
	"context"
	"encoding/json"
	"fmt"

	"ironclaw/internal/domain"
)

// SpawnAgentInput is the JSON Schema input for the spawn_agent tool.
type SpawnAgentInput struct {
	Role string `json:"role" jsonschema:"description=The specialist role / system prompt for the sub-agent (e.g. 'You are a Python Expert')"`
	Task string `json:"task" jsonschema:"description=The task to delegate to the sub-agent"`
}

// spawnUnmarshalFunc is the JSON unmarshaler used by Call. Package-level so
// tests can inject a failing unmarshaler to cover the defense-in-depth path.
var spawnUnmarshalFunc = json.Unmarshal

// SpawnAgentTool allows the main agent to delegate a task to a specialist
// sub-agent that runs in isolation with a custom system prompt. The sub-agent
// uses the same LLM provider but has no access to the parent's memory, history,
// or context — guaranteeing a clean, focused execution.
type SpawnAgentTool struct {
	runner domain.SubAgentRunner
}

// NewSpawnAgentTool creates a SpawnAgentTool backed by the given SubAgentRunner.
// Panics if runner is nil.
func NewSpawnAgentTool(runner domain.SubAgentRunner) *SpawnAgentTool {
	if runner == nil {
		panic("spawn_agent_tool: runner must not be nil")
	}
	return &SpawnAgentTool{runner: runner}
}

// Name returns the tool name used in function-calling.
func (s *SpawnAgentTool) Name() string { return "spawn_agent" }

// Description returns a human-readable description for the LLM.
func (s *SpawnAgentTool) Description() string {
	return "Spawns a specialist sub-agent with a custom role to handle a delegated task in isolation and returns the result."
}

// Definition returns the JSON Schema string for the tool's input struct.
func (s *SpawnAgentTool) Definition() string {
	return GenerateSchema(SpawnAgentInput{})
}

// Call executes the spawn_agent tool: validates input, delegates to the
// SubAgentRunner with the given role as system prompt, and returns the result.
func (s *SpawnAgentTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	// Validate against schema first.
	schema := s.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("spawn_agent input validation failed: %w", err)
	}

	var input SpawnAgentInput
	if err := spawnUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("spawn_agent: failed to parse input: %w", err)
	}

	// Run the task via the sub-agent runner (isolated execution).
	result, err := s.runner.RunSubAgent(context.Background(), input.Role, input.Task)
	if err != nil {
		return nil, fmt.Errorf("spawn_agent: sub-agent failed: %w", err)
	}

	return &domain.ToolResult{
		Data: result,
		Metadata: map[string]string{
			"role": input.Role,
		},
	}, nil
}
