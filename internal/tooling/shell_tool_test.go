package tooling

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"ironclaw/internal/config"
	"ironclaw/internal/domain"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockCommandRunner is a test double for CommandRunner.
type mockCommandRunner struct {
	stdout string
	stderr string
	err    error
}

func (m *mockCommandRunner) Run(command string) (string, string, error) {
	return m.stdout, m.stderr, m.err
}

// spyCommandRunner records whether Run was called.
type spyCommandRunner struct {
	called bool
}

func (s *spyCommandRunner) Run(command string) (string, string, error) {
	s.called = true
	return "", "", nil
}

// mockExitError is a test double for process exit errors with non-zero codes.
type mockExitError struct {
	code int
}

func (m *mockExitError) Error() string { return fmt.Sprintf("exit status %d", m.code) }
func (m *mockExitError) ExitCode() int { return m.code }

// =============================================================================
// ShellTool — Name, Description, Definition
// =============================================================================

func TestShellTool_Name_ShouldReturnShell(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
	if tool.Name() != "shell" {
		t.Errorf("Expected name 'shell', got '%s'", tool.Name())
	}
}

func TestShellTool_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestShellTool_Definition_ShouldContainCommandProperty(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
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
	if _, exists := props["command"]; !exists {
		t.Error("Expected 'command' property in schema")
	}
}

func TestShellTool_Definition_ShouldRequireCommandField(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("Expected 'required' array in schema")
	}
	found := false
	for _, r := range required {
		if r == "command" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'command' in required fields")
	}
}

// =============================================================================
// ShellTool.Call — Input Validation
// =============================================================================

func TestShellTool_Call_ShouldRejectInvalidJSON(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
	_, err := tool.Call(json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestShellTool_Call_ShouldRejectMissingCommandField(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
	_, err := tool.Call(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("Expected error for missing command field")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestShellTool_Call_ShouldRejectWrongTypeForCommand(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
	_, err := tool.Call(json.RawMessage(`{"command": 123}`))
	if err == nil {
		t.Fatal("Expected error for wrong type in command field")
	}
}

func TestShellTool_Call_ShouldRejectEmptyCommandString(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
	_, err := tool.Call(json.RawMessage(`{"command": ""}`))
	if err == nil {
		t.Fatal("Expected error for empty command string")
	}
}

// =============================================================================
// ShellTool.Call — Allowlist Validation
// =============================================================================

func TestShellTool_Call_ShouldRejectDisallowedCommand(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"echo", "ls"}}
	tool := NewShellTool(cfg, &mockCommandRunner{stdout: "should not reach"})
	_, err := tool.Call(json.RawMessage(`{"command":"rm -rf /"}`))
	if err == nil {
		t.Fatal("Expected error for disallowed command")
	}
	if !errors.Is(err, config.ErrCommandNotAllowed) {
		t.Errorf("Expected ErrCommandNotAllowed, got: %v", err)
	}
}

func TestShellTool_Call_ShouldNotExecuteDisallowedCommand(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"echo"}}
	runner := &spyCommandRunner{}
	tool := NewShellTool(cfg, runner)
	_, _ = tool.Call(json.RawMessage(`{"command":"rm -rf /"}`))
	if runner.called {
		t.Error("Runner should NOT have been called for a disallowed command")
	}
}

func TestShellTool_Call_ShouldAllowCommandWhenAllowlistIsEmpty(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: nil}
	tool := NewShellTool(cfg, &mockCommandRunner{stdout: "hello"})
	result, err := tool.Call(json.RawMessage(`{"command":"echo hello"}`))
	if err != nil {
		t.Fatalf("Expected success when allowlist empty, got: %v", err)
	}
	if result == nil || !strings.Contains(result.Data, "hello") {
		t.Errorf("Expected output containing 'hello', got: %v", result)
	}
}

func TestShellTool_Call_ShouldAllowCommandWhenConfigIsNil(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{stdout: "output"})
	result, err := tool.Call(json.RawMessage(`{"command":"anything"}`))
	if err != nil {
		t.Fatalf("Expected success when config nil, got: %v", err)
	}
	if result == nil || !strings.Contains(result.Data, "output") {
		t.Errorf("Expected output containing 'output', got: %v", result)
	}
}

func TestShellTool_Call_ShouldAllowCommandInAllowlist(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"echo"}}
	tool := NewShellTool(cfg, &mockCommandRunner{stdout: "hello world"})
	result, err := tool.Call(json.RawMessage(`{"command":"echo hello world"}`))
	if err != nil {
		t.Fatalf("Expected success for allowed command, got: %v", err)
	}
	if result.Data != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", result.Data)
	}
}

func TestShellTool_Call_ShouldAllowCommandByPathWhenBaseNameInAllowlist(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"echo"}}
	tool := NewShellTool(cfg, &mockCommandRunner{stdout: "from path"})
	result, err := tool.Call(json.RawMessage(`{"command":"/usr/bin/echo hello"}`))
	if err != nil {
		t.Fatalf("Expected success for allowed command via path, got: %v", err)
	}
	if result.Data != "from path" {
		t.Errorf("Expected 'from path', got '%s'", result.Data)
	}
}

// =============================================================================
// ShellTool.Call — Command Execution & Output
// =============================================================================

func TestShellTool_Call_ShouldReturnStdoutAsData(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{stdout: "some output"})
	result, err := tool.Call(json.RawMessage(`{"command":"echo some output"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "some output") {
		t.Errorf("Expected 'some output' in Data, got '%s'", result.Data)
	}
}

func TestShellTool_Call_ShouldIncludeStderrInOutput(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{stdout: "out", stderr: "warn"})
	result, err := tool.Call(json.RawMessage(`{"command":"some cmd"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "out") {
		t.Errorf("Expected stdout in output, got: %s", result.Data)
	}
	if !strings.Contains(result.Data, "warn") {
		t.Errorf("Expected stderr in output, got: %s", result.Data)
	}
}

func TestShellTool_Call_ShouldSeparateStdoutAndStderrClearly(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{stdout: "out", stderr: "err"})
	result, err := tool.Call(json.RawMessage(`{"command":"cmd"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "stderr") {
		t.Errorf("Expected stderr separator label in output, got: %s", result.Data)
	}
}

func TestShellTool_Call_ShouldReturnOnlyStdoutWhenStderrIsEmpty(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{stdout: "clean output"})
	result, err := tool.Call(json.RawMessage(`{"command":"cmd"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Data != "clean output" {
		t.Errorf("Expected exactly 'clean output', got '%s'", result.Data)
	}
}

func TestShellTool_Call_ShouldReturnStderrWhenStdoutIsEmpty(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{stderr: "error only"})
	result, err := tool.Call(json.RawMessage(`{"command":"cmd"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "error only") {
		t.Errorf("Expected stderr in output, got: %s", result.Data)
	}
}

func TestShellTool_Call_ShouldReturnEmptyDataWhenNoOutput(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
	result, err := tool.Call(json.RawMessage(`{"command":"true"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Data != "" {
		t.Errorf("Expected empty data, got '%s'", result.Data)
	}
}

// =============================================================================
// ShellTool.Call — Metadata
// =============================================================================

func TestShellTool_Call_ShouldReturnMetadataWithCommand(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{stdout: "ok"})
	result, err := tool.Call(json.RawMessage(`{"command":"echo ok"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["command"] != "echo ok" {
		t.Errorf("Expected metadata command='echo ok', got '%s'", result.Metadata["command"])
	}
}

func TestShellTool_Call_ShouldReturnExitCodeZeroOnSuccess(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{stdout: "ok"})
	result, err := tool.Call(json.RawMessage(`{"command":"echo ok"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["exit_code"] != "0" {
		t.Errorf("Expected exit_code=0, got '%s'", result.Metadata["exit_code"])
	}
}

// =============================================================================
// ShellTool.Call — Error Handling
// =============================================================================

func TestShellTool_Call_ShouldReturnErrorWhenCommandFailsToStart(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{err: fmt.Errorf("exec: not found")})
	_, err := tool.Call(json.RawMessage(`{"command":"nonexistent"}`))
	if err == nil {
		t.Fatal("Expected error when command fails to start")
	}
	if !strings.Contains(err.Error(), "failed to execute") {
		t.Errorf("Expected 'failed to execute' in error, got: %v", err)
	}
}

func TestShellTool_Call_ShouldReturnOutputOnNonZeroExit(t *testing.T) {
	exitErr := &mockExitError{code: 1}
	tool := NewShellTool(nil, &mockCommandRunner{
		stdout: "partial output",
		stderr: "error info",
		err:    exitErr,
	})
	result, err := tool.Call(json.RawMessage(`{"command":"failing cmd"}`))
	if err != nil {
		t.Fatalf("Non-zero exit should not be a Go error, got: %v", err)
	}
	if !strings.Contains(result.Data, "partial output") {
		t.Errorf("Expected stdout in output, got: %s", result.Data)
	}
	if !strings.Contains(result.Data, "error info") {
		t.Errorf("Expected stderr in output, got: %s", result.Data)
	}
}

func TestShellTool_Call_ShouldReturnNonZeroExitCodeInMetadata(t *testing.T) {
	exitErr := &mockExitError{code: 42}
	tool := NewShellTool(nil, &mockCommandRunner{
		stdout: "output",
		err:    exitErr,
	})
	result, err := tool.Call(json.RawMessage(`{"command":"exit 42"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["exit_code"] != "42" {
		t.Errorf("Expected exit_code=42, got '%s'", result.Metadata["exit_code"])
	}
}

// =============================================================================
// ShellTool.Call — Unmarshal error path (defense-in-depth)
// =============================================================================

func TestShellTool_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := shellUnmarshalFunc
	shellUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { shellUnmarshalFunc = original }()

	tool := NewShellTool(nil, &mockCommandRunner{})
	_, err := tool.Call(json.RawMessage(`{"command":"echo hello"}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

// =============================================================================
// ExecCommandRunner — Integration Tests (real os/exec)
// =============================================================================

func TestExecCommandRunner_Run_ShouldExecuteRealCommand(t *testing.T) {
	runner := &ExecCommandRunner{}
	stdout, stderr, err := runner.Run("echo hello")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if strings.TrimSpace(stdout) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", stdout)
	}
	if stderr != "" {
		t.Errorf("Expected empty stderr, got '%s'", stderr)
	}
}

func TestExecCommandRunner_Run_ShouldCaptureStderr(t *testing.T) {
	runner := &ExecCommandRunner{}
	stdout, stderr, err := runner.Run("echo error >&2")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if stdout != "" {
		t.Errorf("Expected empty stdout, got '%s'", stdout)
	}
	if strings.TrimSpace(stderr) != "error" {
		t.Errorf("Expected 'error', got '%s'", stderr)
	}
}

func TestExecCommandRunner_Run_ShouldReturnErrorForNonZeroExit(t *testing.T) {
	runner := &ExecCommandRunner{}
	_, _, err := runner.Run("exit 1")
	if err == nil {
		t.Fatal("Expected error for non-zero exit")
	}
}

func TestExecCommandRunner_Run_ShouldReturnExitCodeForFailedCommand(t *testing.T) {
	runner := &ExecCommandRunner{}
	_, _, err := runner.Run("exit 42")
	if err == nil {
		t.Fatal("Expected error for non-zero exit")
	}
	exitErr, ok := err.(ExitCoder)
	if !ok {
		t.Fatalf("Expected ExitCoder interface, got: %T", err)
	}
	if exitErr.ExitCode() != 42 {
		t.Errorf("Expected exit code 42, got %d", exitErr.ExitCode())
	}
}

func TestExecCommandRunner_Run_ShouldCaptureBothStdoutAndStderr(t *testing.T) {
	runner := &ExecCommandRunner{}
	stdout, stderr, _ := runner.Run("echo out && echo err >&2")
	if strings.TrimSpace(stdout) != "out" {
		t.Errorf("Expected stdout 'out', got '%s'", stdout)
	}
	if strings.TrimSpace(stderr) != "err" {
		t.Errorf("Expected stderr 'err', got '%s'", stderr)
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*ShellTool)(nil)
var _ ExitCoder = (*mockExitError)(nil)
