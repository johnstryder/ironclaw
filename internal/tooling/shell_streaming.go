package tooling

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"ironclaw/internal/config"
	"ironclaw/internal/domain"
)

// OutputLine represents a single line of streaming output from a command.
type OutputLine struct {
	Source string // "stdout" or "stderr"
	Line   string // The content of the line
}

// StreamingCommandRunner abstracts streaming command execution for testability.
// It runs a command and calls onLine for each line of output as it is produced.
// Returns the process exit code and any error (nil error + non-zero exit code
// means the command ran but exited with a failure code).
type StreamingCommandRunner interface {
	RunStreaming(command string, onLine func(OutputLine)) (exitCode int, err error)
}

// CallStreaming validates the command against the allowlist and executes it via
// the streaming runner. Each line of output is delivered to onLine in real time.
// The final ToolResult contains the combined output and exit code metadata.
func (s *ShellTool) CallStreaming(args json.RawMessage, onLine func(OutputLine)) (*domain.ToolResult, error) {
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

	// 4. Check streaming runner is configured
	if s.streamRunner == nil {
		return nil, fmt.Errorf("streaming runner not configured")
	}

	// 5. Collect output while streaming
	var stdoutLines []string
	var stderrLines []string

	exitCode, err := s.streamRunner.RunStreaming(input.Command, func(line OutputLine) {
		// Deliver to caller's callback
		onLine(line)
		// Also collect for final result
		switch line.Source {
		case "stdout":
			stdoutLines = append(stdoutLines, line.Line)
		case "stderr":
			stderrLines = append(stderrLines, line.Line)
		}
	})

	// 6. Handle execution errors
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	// 7. Format combined output (same style as Call)
	stdout := strings.Join(stdoutLines, "\n")
	stderr := strings.Join(stderrLines, "\n")

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
			"mode":      "streaming",
		},
	}, nil
}

// NewShellToolWithStreaming creates a ShellTool with both a batched runner
// and a streaming runner pre-configured.
func NewShellToolWithStreaming(cfg *domain.Config, runner CommandRunner, streamRunner StreamingCommandRunner) *ShellTool {
	return &ShellTool{cfg: cfg, runner: runner, streamRunner: streamRunner}
}

// ExecStreamingCommandRunner executes commands using os/exec via "sh -c" and
// streams stdout/stderr line-by-line through the onLine callback using pipes.
type ExecStreamingCommandRunner struct{}

// execStreamCommand is the function used to create exec.Cmd; tests may replace
// it to force pipe/start errors.
var execStreamCommand = func(command string) *exec.Cmd {
	return exec.Command("sh", "-c", command)
}

// execStreamWait is the function used to wait for the command to finish; tests
// may replace it to force non-ExitError wait failures.
var execStreamWait = func(cmd *exec.Cmd) error {
	return cmd.Wait()
}

// RunStreaming starts the command, attaches to stdout and stderr pipes, and
// delivers each line to onLine as it is produced. Returns the process exit code
// (0 on success) and any error that prevented the command from starting.
func (e *ExecStreamingCommandRunner) RunStreaming(command string, onLine func(OutputLine)) (int, error) {
	cmd := execStreamCommand(command)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start command: %w", err)
	}

	// Use a mutex to serialize onLine calls from the two goroutines
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			onLine(OutputLine{Source: "stdout", Line: line})
			mu.Unlock()
		}
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			onLine(OutputLine{Source: "stderr", Line: line})
			mu.Unlock()
		}
	}()

	// Wait for both scanners to complete before calling Wait
	wg.Wait()

	// Wait for the process to exit
	exitCode := 0
	if err := execStreamWait(cmd); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return 0, fmt.Errorf("failed waiting for command: %w", err)
		}
	}

	return exitCode, nil
}
