package main

import (
	"os"
	"strings"
	"testing"
)

// =============================================================================
// main — Happy path with custom script
// =============================================================================

func TestMain_ShouldStreamOutputWithCustomScript(t *testing.T) {
	origArgs := os.Args
	origExit := osExit
	os.Args = []string{"streamtest", "echo hello && echo warn >&2"}
	osExit = func(_ int) {}
	defer func() {
		os.Args = origArgs
		osExit = origExit
	}()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	w.Close()
	os.Stdout = old

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "Real-time streaming output") {
		t.Errorf("Expected streaming header, got: %s", output)
	}
	if !strings.Contains(output, "stdout") {
		t.Errorf("Expected stdout label, got: %s", output)
	}
	if !strings.Contains(output, "stderr") {
		t.Errorf("Expected stderr label, got: %s", output)
	}
	if !strings.Contains(output, "Summary") {
		t.Errorf("Expected summary section, got: %s", output)
	}
	if !strings.Contains(output, "Lines streamed") {
		t.Errorf("Expected lines streamed count, got: %s", output)
	}
	if !strings.Contains(output, "Final ToolResult.Data") {
		t.Errorf("Expected final data section, got: %s", output)
	}
}

// =============================================================================
// main — Error path (empty command fails schema validation)
// =============================================================================

func TestMain_ShouldExitWithErrorForInvalidCommand(t *testing.T) {
	origArgs := os.Args
	origExit := osExit
	os.Args = []string{"streamtest", ""}
	var exitCode int
	osExit = func(code int) { exitCode = code }
	defer func() {
		os.Args = origArgs
		osExit = origExit
	}()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	w.Close()
	os.Stdout = old

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(output, "ERROR") {
		t.Errorf("Expected ERROR in output, got: %s", output)
	}
}

// =============================================================================
// main — Default script (no extra args)
// =============================================================================

func TestMain_ShouldUseDefaultScriptWhenNoArgs(t *testing.T) {
	origArgs := os.Args
	origExit := osExit
	os.Args = []string{"streamtest"}
	osExit = func(_ int) {}
	defer func() {
		os.Args = origArgs
		osExit = origExit
	}()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	w.Close()
	os.Stdout = old

	var buf [16384]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "Deploy Script") {
		t.Errorf("Expected default deploy script output, got: %s", output)
	}
	if !strings.Contains(output, "Deploy complete") {
		t.Errorf("Expected deploy complete message, got: %s", output)
	}
	if !strings.Contains(output, "Summary") {
		t.Errorf("Expected summary section, got: %s", output)
	}
}
