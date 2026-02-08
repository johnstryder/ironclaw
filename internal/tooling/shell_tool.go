package tooling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"ironclaw/internal/config"
	"ironclaw/internal/domain"
)

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	Run(command string) (stdout string, stderr string, err error)
}

// ExitCoder is satisfied by errors that carry a process exit code
// (e.g., *exec.ExitError).
type ExitCoder interface {
	ExitCode() int
}

// ShellInput represents the input structure for shell command execution.
type ShellInput struct {
	Command string `json:"command" jsonschema:"minLength=1"`
}

// shellUnmarshalFunc is the JSON unmarshaler used by Call. Package-level so
// tests can inject a failing unmarshaler to cover the defense-in-depth error path.
var shellUnmarshalFunc = json.Unmarshal

// ShellTool executes shell commands with allowlist validation.
// Supports both batched execution (Call) and real-time streaming (CallStreaming).
type ShellTool struct {
	cfg          *domain.Config
	runner       CommandRunner
	streamRunner StreamingCommandRunner
}

// NewShellTool creates a ShellTool with the given config and command runner.
func NewShellTool(cfg *domain.Config, runner CommandRunner) *ShellTool {
	return &ShellTool{cfg: cfg, runner: runner}
}

// Name returns the tool name used in function-calling.
func (s *ShellTool) Name() string { return "shell" }

// Description returns a human-readable description for the LLM.
func (s *ShellTool) Description() string {
	return "Executes shell commands on the host system and returns stdout/stderr output"
}

// Definition returns the JSON Schema for shell input.
func (s *ShellTool) Definition() string {
	return GenerateSchema(ShellInput{})
}

// Call validates the command against the allowlist and executes it.
func (s *ShellTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	// 1. Validate input against JSON schema
	schema := s.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 2. Unmarshal input
	var input ShellInput
	if err := shellUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// 3. Validate command against allowlist (BEFORE execution)
	if err := config.ValidateCommand(s.cfg, input.Command); err != nil {
		return nil, err
	}

	// 4. Execute command
	stdout, stderr, err := s.runner.Run(input.Command)

	// 5. Handle execution errors
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(ExitCoder); ok {
			// Non-zero exit: capture exit code, still return output
			exitCode = exitErr.ExitCode()
		} else {
			// Command failed to start entirely
			return nil, fmt.Errorf("failed to execute command: %w", err)
		}
	}

	// 6. Format output
	output := stdout
	if stderr != "" {
		if output != "" {
			output += "\n--- stderr ---\n" + stderr
		} else {
			output = "--- stderr ---\n" + stderr
		}
	}

	return &domain.ToolResult{
		Data: output,
		Metadata: map[string]string{
			"command":   input.Command,
			"exit_code": fmt.Sprintf("%d", exitCode),
		},
	}, nil
}

// ExecCommandRunner executes commands using os/exec via "sh -c".
type ExecCommandRunner struct{}

// Run executes the command string in a shell and returns stdout, stderr, and any error.
func (e *ExecCommandRunner) Run(command string) (string, string, error) {
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
