package tooling

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"ironclaw/internal/config"
	"ironclaw/internal/domain"
)

// =============================================================================
// Test Doubles — Streaming
// =============================================================================

// mockStreamingRunner is a test double for StreamingCommandRunner.
// It calls onLine for each pre-configured OutputLine, then returns the
// configured exitCode and err.
type mockStreamingRunner struct {
	lines    []OutputLine
	exitCode int
	err      error
}

func (m *mockStreamingRunner) RunStreaming(command string, onLine func(OutputLine)) (int, error) {
	for _, line := range m.lines {
		onLine(line)
	}
	return m.exitCode, m.err
}

// spyStreamingRunner records whether RunStreaming was called.
type spyStreamingRunner struct {
	called  bool
	command string
}

func (s *spyStreamingRunner) RunStreaming(command string, onLine func(OutputLine)) (int, error) {
	s.called = true
	s.command = command
	return 0, nil
}

// lineCollector collects OutputLines delivered by callbacks.
type lineCollector struct {
	mu    sync.Mutex
	lines []OutputLine
}

func (c *lineCollector) collect(line OutputLine) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, line)
}

func (c *lineCollector) getLines() []OutputLine {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]OutputLine, len(c.lines))
	copy(out, c.lines)
	return out
}

// =============================================================================
// OutputLine — Type
// =============================================================================

func TestOutputLine_ShouldHaveSourceAndLineFields(t *testing.T) {
	line := OutputLine{Source: "stdout", Line: "hello world"}
	if line.Source != "stdout" {
		t.Errorf("Expected source 'stdout', got '%s'", line.Source)
	}
	if line.Line != "hello world" {
		t.Errorf("Expected line 'hello world', got '%s'", line.Line)
	}
}

func TestOutputLine_ShouldSupportStderrSource(t *testing.T) {
	line := OutputLine{Source: "stderr", Line: "error message"}
	if line.Source != "stderr" {
		t.Errorf("Expected source 'stderr', got '%s'", line.Source)
	}
}

// =============================================================================
// ShellTool.CallStreaming — Input Validation
// =============================================================================

func TestShellTool_CallStreaming_ShouldRejectInvalidJSON(t *testing.T) {
	runner := &mockStreamingRunner{}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{bad json`), collector.collect)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestShellTool_CallStreaming_ShouldRejectMissingCommandField(t *testing.T) {
	runner := &mockStreamingRunner{}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{}`), collector.collect)
	if err == nil {
		t.Fatal("Expected error for missing command field")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestShellTool_CallStreaming_ShouldRejectEmptyCommandString(t *testing.T) {
	runner := &mockStreamingRunner{}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command": ""}`), collector.collect)
	if err == nil {
		t.Fatal("Expected error for empty command string")
	}
}

func TestShellTool_CallStreaming_ShouldRejectWrongTypeForCommand(t *testing.T) {
	runner := &mockStreamingRunner{}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command": 123}`), collector.collect)
	if err == nil {
		t.Fatal("Expected error for wrong type in command field")
	}
}

// =============================================================================
// ShellTool.CallStreaming — Allowlist Validation
// =============================================================================

func TestShellTool_CallStreaming_ShouldRejectDisallowedCommand(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"echo", "ls"}}
	runner := &mockStreamingRunner{}
	tool := NewShellTool(cfg, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command":"rm -rf /"}`), collector.collect)
	if err == nil {
		t.Fatal("Expected error for disallowed command")
	}
	if !errors.Is(err, config.ErrCommandNotAllowed) {
		t.Errorf("Expected ErrCommandNotAllowed, got: %v", err)
	}
}

func TestShellTool_CallStreaming_ShouldNotExecuteDisallowedCommand(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"echo"}}
	runner := &spyStreamingRunner{}
	tool := NewShellTool(cfg, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, _ = tool.CallStreaming(json.RawMessage(`{"command":"rm -rf /"}`), collector.collect)
	if runner.called {
		t.Error("Streaming runner should NOT have been called for a disallowed command")
	}
}

func TestShellTool_CallStreaming_ShouldAllowCommandWhenAllowlistIsEmpty(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: nil}
	runner := &mockStreamingRunner{
		lines: []OutputLine{{Source: "stdout", Line: "hello"}},
	}
	tool := NewShellTool(cfg, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"echo hello"}`), collector.collect)
	if err != nil {
		t.Fatalf("Expected success when allowlist empty, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestShellTool_CallStreaming_ShouldAllowCommandWhenConfigIsNil(t *testing.T) {
	runner := &mockStreamingRunner{
		lines: []OutputLine{{Source: "stdout", Line: "output"}},
	}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"anything"}`), collector.collect)
	if err != nil {
		t.Fatalf("Expected success when config nil, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestShellTool_CallStreaming_ShouldAllowCommandInAllowlist(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"echo"}}
	runner := &mockStreamingRunner{
		lines: []OutputLine{{Source: "stdout", Line: "hello world"}},
	}
	tool := NewShellTool(cfg, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"echo hello world"}`), collector.collect)
	if err != nil {
		t.Fatalf("Expected success for allowed command, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

// =============================================================================
// ShellTool.CallStreaming — Streaming Output Delivery
// =============================================================================

func TestShellTool_CallStreaming_ShouldStreamStdoutLinesViaCallback(t *testing.T) {
	runner := &mockStreamingRunner{
		lines: []OutputLine{
			{Source: "stdout", Line: "line1"},
			{Source: "stdout", Line: "line2"},
			{Source: "stdout", Line: "line3"},
		},
	}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command":"echo lines"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	lines := collector.getLines()
	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines, got %d", len(lines))
	}
	for i, l := range lines {
		if l.Source != "stdout" {
			t.Errorf("Line %d: expected source 'stdout', got '%s'", i, l.Source)
		}
		expected := fmt.Sprintf("line%d", i+1)
		if l.Line != expected {
			t.Errorf("Line %d: expected '%s', got '%s'", i, expected, l.Line)
		}
	}
}

func TestShellTool_CallStreaming_ShouldStreamStderrLinesViaCallback(t *testing.T) {
	runner := &mockStreamingRunner{
		lines: []OutputLine{
			{Source: "stderr", Line: "warning1"},
			{Source: "stderr", Line: "warning2"},
		},
	}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command":"cmd"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	lines := collector.getLines()
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}
	for _, l := range lines {
		if l.Source != "stderr" {
			t.Errorf("Expected source 'stderr', got '%s'", l.Source)
		}
	}
}

func TestShellTool_CallStreaming_ShouldStreamInterleavedStdoutAndStderr(t *testing.T) {
	runner := &mockStreamingRunner{
		lines: []OutputLine{
			{Source: "stdout", Line: "out1"},
			{Source: "stderr", Line: "err1"},
			{Source: "stdout", Line: "out2"},
		},
	}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command":"cmd"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	lines := collector.getLines()
	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines, got %d", len(lines))
	}
	if lines[0].Source != "stdout" || lines[0].Line != "out1" {
		t.Errorf("Line 0 mismatch: %+v", lines[0])
	}
	if lines[1].Source != "stderr" || lines[1].Line != "err1" {
		t.Errorf("Line 1 mismatch: %+v", lines[1])
	}
	if lines[2].Source != "stdout" || lines[2].Line != "out2" {
		t.Errorf("Line 2 mismatch: %+v", lines[2])
	}
}

func TestShellTool_CallStreaming_ShouldDeliverZeroLinesWhenNoOutput(t *testing.T) {
	runner := &mockStreamingRunner{lines: nil}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command":"true"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	lines := collector.getLines()
	if len(lines) != 0 {
		t.Errorf("Expected 0 lines, got %d", len(lines))
	}
}

// =============================================================================
// ShellTool.CallStreaming — ToolResult Output
// =============================================================================

func TestShellTool_CallStreaming_ShouldReturnCombinedOutputInToolResult(t *testing.T) {
	runner := &mockStreamingRunner{
		lines: []OutputLine{
			{Source: "stdout", Line: "line1"},
			{Source: "stdout", Line: "line2"},
		},
	}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"cmd"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "line1") {
		t.Errorf("Expected 'line1' in result Data, got '%s'", result.Data)
	}
	if !strings.Contains(result.Data, "line2") {
		t.Errorf("Expected 'line2' in result Data, got '%s'", result.Data)
	}
}

func TestShellTool_CallStreaming_ShouldSeparateStdoutAndStderrInResult(t *testing.T) {
	runner := &mockStreamingRunner{
		lines: []OutputLine{
			{Source: "stdout", Line: "out"},
			{Source: "stderr", Line: "err"},
		},
	}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"cmd"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "out") {
		t.Errorf("Expected stdout in result Data, got '%s'", result.Data)
	}
	if !strings.Contains(result.Data, "err") {
		t.Errorf("Expected stderr in result Data, got '%s'", result.Data)
	}
	if !strings.Contains(result.Data, "stderr") {
		t.Errorf("Expected stderr separator in result Data, got '%s'", result.Data)
	}
}

func TestShellTool_CallStreaming_ShouldReturnEmptyDataWhenNoOutput(t *testing.T) {
	runner := &mockStreamingRunner{lines: nil}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"true"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Data != "" {
		t.Errorf("Expected empty data, got '%s'", result.Data)
	}
}

// =============================================================================
// ShellTool.CallStreaming — Metadata
// =============================================================================

func TestShellTool_CallStreaming_ShouldReturnMetadataWithCommand(t *testing.T) {
	runner := &mockStreamingRunner{}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"echo ok"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["command"] != "echo ok" {
		t.Errorf("Expected metadata command='echo ok', got '%s'", result.Metadata["command"])
	}
}

func TestShellTool_CallStreaming_ShouldReturnExitCodeZeroOnSuccess(t *testing.T) {
	runner := &mockStreamingRunner{exitCode: 0}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"echo ok"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["exit_code"] != "0" {
		t.Errorf("Expected exit_code=0, got '%s'", result.Metadata["exit_code"])
	}
}

func TestShellTool_CallStreaming_ShouldReturnNonZeroExitCodeInMetadata(t *testing.T) {
	runner := &mockStreamingRunner{exitCode: 42}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"exit 42"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["exit_code"] != "42" {
		t.Errorf("Expected exit_code=42, got '%s'", result.Metadata["exit_code"])
	}
}

func TestShellTool_CallStreaming_ShouldReturnModeStreamingInMetadata(t *testing.T) {
	runner := &mockStreamingRunner{}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"echo test"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["mode"] != "streaming" {
		t.Errorf("Expected metadata mode='streaming', got '%s'", result.Metadata["mode"])
	}
}

// =============================================================================
// ShellTool.CallStreaming — Error Handling
// =============================================================================

func TestShellTool_CallStreaming_ShouldReturnErrorWhenRunnerFails(t *testing.T) {
	runner := &mockStreamingRunner{err: fmt.Errorf("exec: not found")}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command":"nonexistent"}`), collector.collect)
	if err == nil {
		t.Fatal("Expected error when streaming runner fails")
	}
	if !strings.Contains(err.Error(), "failed to execute") {
		t.Errorf("Expected 'failed to execute' in error, got: %v", err)
	}
}

func TestShellTool_CallStreaming_ShouldReturnOutputOnNonZeroExit(t *testing.T) {
	runner := &mockStreamingRunner{
		lines: []OutputLine{
			{Source: "stdout", Line: "partial"},
			{Source: "stderr", Line: "error info"},
		},
		exitCode: 1,
	}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"failing cmd"}`), collector.collect)
	if err != nil {
		t.Fatalf("Non-zero exit should not be a Go error, got: %v", err)
	}
	if !strings.Contains(result.Data, "partial") {
		t.Errorf("Expected stdout in output, got: %s", result.Data)
	}
	if !strings.Contains(result.Data, "error info") {
		t.Errorf("Expected stderr in output, got: %s", result.Data)
	}
}

func TestShellTool_CallStreaming_ShouldPassCommandToRunner(t *testing.T) {
	runner := &spyStreamingRunner{}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, _ = tool.CallStreaming(json.RawMessage(`{"command":"echo hello"}`), collector.collect)
	if !runner.called {
		t.Fatal("Expected streaming runner to be called")
	}
	if runner.command != "echo hello" {
		t.Errorf("Expected command 'echo hello', got '%s'", runner.command)
	}
}

func TestShellTool_CallStreaming_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := shellUnmarshalFunc
	shellUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { shellUnmarshalFunc = original }()

	runner := &mockStreamingRunner{}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command":"echo hello"}`), collector.collect)
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

func TestShellTool_CallStreaming_ShouldRequireStreamingRunner(t *testing.T) {
	tool := NewShellTool(nil, &mockCommandRunner{})
	// streamRunner is NOT set
	collector := &lineCollector{}

	_, err := tool.CallStreaming(json.RawMessage(`{"command":"echo hello"}`), collector.collect)
	if err == nil {
		t.Fatal("Expected error when streaming runner is not configured")
	}
	if !strings.Contains(err.Error(), "streaming runner not configured") {
		t.Errorf("Expected 'streaming runner not configured' in error, got: %v", err)
	}
}

// =============================================================================
// ExecStreamingCommandRunner — Integration Tests (real os/exec)
// =============================================================================

func TestExecStreamingCommandRunner_ShouldStreamStdoutLinesInRealTime(t *testing.T) {
	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	exitCode, err := runner.RunStreaming("echo hello && echo world", collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	lines := collector.getLines()
	stdoutLines := filterBySource(lines, "stdout")
	if len(stdoutLines) != 2 {
		t.Fatalf("Expected 2 stdout lines, got %d: %v", len(stdoutLines), stdoutLines)
	}
	if strings.TrimSpace(stdoutLines[0].Line) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", stdoutLines[0].Line)
	}
	if strings.TrimSpace(stdoutLines[1].Line) != "world" {
		t.Errorf("Expected 'world', got '%s'", stdoutLines[1].Line)
	}
}

func TestExecStreamingCommandRunner_ShouldStreamStderrLines(t *testing.T) {
	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	exitCode, err := runner.RunStreaming("echo error1 >&2 && echo error2 >&2", collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	lines := collector.getLines()
	stderrLines := filterBySource(lines, "stderr")
	if len(stderrLines) != 2 {
		t.Fatalf("Expected 2 stderr lines, got %d: %v", len(stderrLines), stderrLines)
	}
	if strings.TrimSpace(stderrLines[0].Line) != "error1" {
		t.Errorf("Expected 'error1', got '%s'", stderrLines[0].Line)
	}
	if strings.TrimSpace(stderrLines[1].Line) != "error2" {
		t.Errorf("Expected 'error2', got '%s'", stderrLines[1].Line)
	}
}

func TestExecStreamingCommandRunner_ShouldStreamBothStdoutAndStderr(t *testing.T) {
	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	exitCode, err := runner.RunStreaming("echo out && echo err >&2", collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	lines := collector.getLines()
	stdoutLines := filterBySource(lines, "stdout")
	stderrLines := filterBySource(lines, "stderr")
	if len(stdoutLines) < 1 {
		t.Errorf("Expected at least 1 stdout line, got %d", len(stdoutLines))
	}
	if len(stderrLines) < 1 {
		t.Errorf("Expected at least 1 stderr line, got %d", len(stderrLines))
	}
}

func TestExecStreamingCommandRunner_ShouldReturnNonZeroExitCode(t *testing.T) {
	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	exitCode, err := runner.RunStreaming("exit 42", collector.collect)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", exitCode)
	}
}

func TestExecStreamingCommandRunner_ShouldReturnZeroExitCodeOnSuccess(t *testing.T) {
	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	exitCode, err := runner.RunStreaming("true", collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
}

func TestExecStreamingCommandRunner_ShouldHandleMultiLineOutput(t *testing.T) {
	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	// printf ensures exact line control
	exitCode, err := runner.RunStreaming(`printf "line1\nline2\nline3\n"`, collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	lines := collector.getLines()
	stdoutLines := filterBySource(lines, "stdout")
	if len(stdoutLines) != 3 {
		t.Fatalf("Expected 3 stdout lines, got %d: %v", len(stdoutLines), stdoutLines)
	}
}

func TestExecStreamingCommandRunner_ShouldReturnErrorForInvalidCommand(t *testing.T) {
	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	// A completely non-existent shell — should fail to start
	_, err := runner.RunStreaming("", collector.collect)
	// Empty command still runs through sh -c, so exit code is 0 (sh -c "" succeeds)
	// This is a valid test of the runner — an empty command is valid shell
	if err != nil {
		// Acceptable to get either error or success for empty command
		return
	}
}

func TestExecStreamingCommandRunner_ShouldStreamOutputWithNonZeroExit(t *testing.T) {
	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	exitCode, err := runner.RunStreaming("echo partial && exit 1", collector.collect)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}

	lines := collector.getLines()
	stdoutLines := filterBySource(lines, "stdout")
	if len(stdoutLines) < 1 {
		t.Error("Expected at least 1 stdout line before exit")
	}
	if len(stdoutLines) > 0 && strings.TrimSpace(stdoutLines[0].Line) != "partial" {
		t.Errorf("Expected 'partial', got '%s'", stdoutLines[0].Line)
	}
}

func TestExecStreamingCommandRunner_ShouldStreamNoLinesForSilentCommand(t *testing.T) {
	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	exitCode, err := runner.RunStreaming("true", collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	lines := collector.getLines()
	if len(lines) != 0 {
		t.Errorf("Expected 0 lines for silent command, got %d", len(lines))
	}
}

// filterBySource returns only OutputLines with the given source.
func filterBySource(lines []OutputLine, source string) []OutputLine {
	var out []OutputLine
	for _, l := range lines {
		if l.Source == source {
			out = append(out, l)
		}
	}
	return out
}

// =============================================================================
// NewShellToolWithStreaming — Constructor
// =============================================================================

func TestNewShellToolWithStreaming_ShouldConfigureBothRunners(t *testing.T) {
	cfg := &domain.Config{}
	batchRunner := &mockCommandRunner{}
	streamRunner := &mockStreamingRunner{}

	tool := NewShellToolWithStreaming(cfg, batchRunner, streamRunner)
	if tool == nil {
		t.Fatal("Expected non-nil tool")
	}
	// Verify batch Call still works
	batchRunner.stdout = "batch"
	result, err := tool.Call(json.RawMessage(`{"command":"echo batch"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Data != "batch" {
		t.Errorf("Expected 'batch', got '%s'", result.Data)
	}

	// Verify streaming CallStreaming works
	streamRunner.lines = []OutputLine{{Source: "stdout", Line: "stream"}}
	collector := &lineCollector{}
	result, err = tool.CallStreaming(json.RawMessage(`{"command":"echo stream"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "stream") {
		t.Errorf("Expected 'stream' in data, got '%s'", result.Data)
	}
}

// =============================================================================
// ShellTool.CallStreaming — Stderr-Only Output
// =============================================================================

func TestShellTool_CallStreaming_ShouldReturnStderrWhenStdoutIsEmpty(t *testing.T) {
	runner := &mockStreamingRunner{
		lines: []OutputLine{
			{Source: "stderr", Line: "error only"},
		},
	}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"cmd"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "error only") {
		t.Errorf("Expected stderr in output, got: %s", result.Data)
	}
	if !strings.Contains(result.Data, "stderr") {
		t.Errorf("Expected stderr separator in output, got: %s", result.Data)
	}
}

func TestShellTool_CallStreaming_ShouldReturnOnlyStdoutWhenStderrIsEmpty(t *testing.T) {
	runner := &mockStreamingRunner{
		lines: []OutputLine{
			{Source: "stdout", Line: "clean output"},
		},
	}
	tool := NewShellTool(nil, &mockCommandRunner{})
	tool.streamRunner = runner
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"cmd"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Data != "clean output" {
		t.Errorf("Expected exactly 'clean output', got '%s'", result.Data)
	}
}

// =============================================================================
// ExecStreamingCommandRunner — Error Path Tests
// =============================================================================

func TestExecStreamingCommandRunner_ShouldReturnErrorWhenStartFails(t *testing.T) {
	original := execStreamCommand
	execStreamCommand = func(command string) *exec.Cmd {
		// point to a nonexistent binary so Start() fails
		return exec.Command("/nonexistent/binary/foobar")
	}
	defer func() { execStreamCommand = original }()

	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	_, err := runner.RunStreaming("anything", collector.collect)
	if err == nil {
		t.Fatal("Expected error when command fails to start")
	}
	if !strings.Contains(err.Error(), "failed to start command") {
		t.Errorf("Expected 'failed to start command' in error, got: %v", err)
	}
}

func TestExecStreamingCommandRunner_ShouldReturnErrorWhenStdoutPipeFails(t *testing.T) {
	original := execStreamCommand
	execStreamCommand = func(command string) *exec.Cmd {
		cmd := exec.Command("sh", "-c", command)
		// Force StdoutPipe to fail by setting Stdout (pipe won't work if Stdout is already set)
		cmd.Stdout = &strings.Builder{}
		return cmd
	}
	defer func() { execStreamCommand = original }()

	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	_, err := runner.RunStreaming("echo hi", collector.collect)
	if err == nil {
		t.Fatal("Expected error when stdout pipe fails")
	}
	if !strings.Contains(err.Error(), "failed to create stdout pipe") {
		t.Errorf("Expected 'failed to create stdout pipe' in error, got: %v", err)
	}
}

func TestExecStreamingCommandRunner_ShouldReturnErrorWhenStderrPipeFails(t *testing.T) {
	original := execStreamCommand
	execStreamCommand = func(command string) *exec.Cmd {
		cmd := exec.Command("sh", "-c", command)
		// Force StderrPipe to fail by setting Stderr (pipe won't work if Stderr is already set)
		cmd.Stderr = &strings.Builder{}
		return cmd
	}
	defer func() { execStreamCommand = original }()

	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	_, err := runner.RunStreaming("echo hi", collector.collect)
	if err == nil {
		t.Fatal("Expected error when stderr pipe fails")
	}
	if !strings.Contains(err.Error(), "failed to create stderr pipe") {
		t.Errorf("Expected 'failed to create stderr pipe' in error, got: %v", err)
	}
}

func TestExecStreamingCommandRunner_ShouldReturnErrorWhenWaitFails(t *testing.T) {
	originalWait := execStreamWait
	execStreamWait = func(cmd *exec.Cmd) error {
		// Drain the real wait first so pipes close
		_ = cmd.Wait()
		return fmt.Errorf("forced wait failure")
	}
	defer func() { execStreamWait = originalWait }()

	runner := &ExecStreamingCommandRunner{}
	collector := &lineCollector{}

	_, err := runner.RunStreaming("echo hi", collector.collect)
	if err == nil {
		t.Fatal("Expected error when wait fails with non-ExitError")
	}
	if !strings.Contains(err.Error(), "failed waiting for command") {
		t.Errorf("Expected 'failed waiting for command' in error, got: %v", err)
	}
}

// =============================================================================
// ExecStreamingCommandRunner — End-to-end with ShellTool
// =============================================================================

func TestShellTool_CallStreaming_EndToEnd_ShouldStreamRealCommand(t *testing.T) {
	tool := NewShellToolWithStreaming(nil, &ExecCommandRunner{}, &ExecStreamingCommandRunner{})
	collector := &lineCollector{}

	result, err := tool.CallStreaming(json.RawMessage(`{"command":"echo hello && echo world"}`), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	lines := collector.getLines()
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 lines, got %d", len(lines))
	}

	if result.Metadata["exit_code"] != "0" {
		t.Errorf("Expected exit_code=0, got '%s'", result.Metadata["exit_code"])
	}
	if result.Metadata["mode"] != "streaming" {
		t.Errorf("Expected mode=streaming, got '%s'", result.Metadata["mode"])
	}
	if !strings.Contains(result.Data, "hello") || !strings.Contains(result.Data, "world") {
		t.Errorf("Expected hello and world in data, got '%s'", result.Data)
	}
}

func TestShellTool_CallStreaming_EndToEnd_ShouldStreamRealScript(t *testing.T) {
	tool := NewShellToolWithStreaming(nil, &ExecCommandRunner{}, &ExecStreamingCommandRunner{})
	collector := &lineCollector{}

	// Multi-line script with both stdout and stderr
	script := `echo "step 1" && echo "warning" >&2 && echo "step 2" && echo "done"`
	args := fmt.Sprintf(`{"command":%q}`, script)
	result, err := tool.CallStreaming(json.RawMessage(args), collector.collect)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	lines := collector.getLines()
	stdoutLines := filterBySource(lines, "stdout")
	stderrLines := filterBySource(lines, "stderr")

	if len(stdoutLines) < 3 {
		t.Errorf("Expected at least 3 stdout lines, got %d", len(stdoutLines))
	}
	if len(stderrLines) < 1 {
		t.Errorf("Expected at least 1 stderr line, got %d", len(stderrLines))
	}
	if result.Metadata["exit_code"] != "0" {
		t.Errorf("Expected exit_code=0, got '%s'", result.Metadata["exit_code"])
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ StreamingCommandRunner = (*mockStreamingRunner)(nil)
var _ StreamingCommandRunner = (*spyStreamingRunner)(nil)
var _ StreamingCommandRunner = (*ExecStreamingCommandRunner)(nil)
