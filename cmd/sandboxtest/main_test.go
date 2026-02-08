package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"ironclaw/internal/tooling"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockRuntime implements tooling.ContainerRuntime for testing.
type mockRuntime struct {
	ensureImageErr     error
	createContainerID  string
	createContainerErr error
	startErr           error
	waitExitCode       int64
	waitErr            error
	logs               string
	logsErr            error
	removeErr          error
}

func (m *mockRuntime) EnsureImage(_ context.Context, _ string) error { return m.ensureImageErr }
func (m *mockRuntime) CreateContainer(_ context.Context, _ tooling.SandboxContainerConfig) (string, error) {
	return m.createContainerID, m.createContainerErr
}
func (m *mockRuntime) StartContainer(_ context.Context, _ string) error { return m.startErr }
func (m *mockRuntime) WaitContainer(_ context.Context, _ string) (int64, error) {
	return m.waitExitCode, m.waitErr
}
func (m *mockRuntime) GetLogs(_ context.Context, _ string) (string, error) {
	return m.logs, m.logsErr
}
func (m *mockRuntime) RemoveContainer(_ context.Context, _ string) error { return m.removeErr }

// noopCloser satisfies io.Closer.
type noopCloser struct{}

func (n *noopCloser) Close() error { return nil }

// happyMockRuntime returns a mock that succeeds at every step.
func happyMockRuntime() *mockRuntime {
	return &mockRuntime{
		createContainerID: "test-container",
		logs:              "test output\n",
	}
}

// =============================================================================
// run — Happy Path
// =============================================================================

func TestRun_ShouldPrintAllTestHeaders(t *testing.T) {
	origFactory := runtimeFactory
	runtimeFactory = func() (tooling.ContainerRuntime, io.Closer, error) {
		return happyMockRuntime(), &noopCloser{}, nil
	}
	defer func() { runtimeFactory = origFactory }()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	output := stdout.String()
	expectedHeaders := []string{
		"Docker Sandbox Manual Integration Test",
		"[OK] Connected to Docker daemon",
		"Test 1: Python",
		"Test 2: Bash",
		"Test 3: JavaScript",
		"Test 4: Non-zero exit",
		"Test 5: Network isolation",
		"Test 6: Host isolation",
		"Test 7: Multiline Python",
		"Test 8: Custom timeout",
		"Test 9: Input validation",
		"Manual Integration Test Complete",
	}
	for _, h := range expectedHeaders {
		if !strings.Contains(output, h) {
			t.Errorf("Expected output to contain '%s'", h)
		}
	}
	if stderr.Len() > 0 {
		t.Errorf("Expected no stderr, got: %s", stderr.String())
	}
}

func TestRun_ShouldShowOutputAndMetadataForSuccessfulTests(t *testing.T) {
	origFactory := runtimeFactory
	runtimeFactory = func() (tooling.ContainerRuntime, io.Closer, error) {
		return happyMockRuntime(), &noopCloser{}, nil
	}
	defer func() { runtimeFactory = origFactory }()

	var stdout bytes.Buffer
	run(&stdout, &bytes.Buffer{})

	output := stdout.String()
	if !strings.Contains(output, "test output") {
		t.Error("Expected test output in stdout")
	}
	if !strings.Contains(output, "exit_code=0") {
		t.Error("Expected exit_code metadata in stdout")
	}
}

func TestRun_ShouldShowErrorForInvalidInput(t *testing.T) {
	origFactory := runtimeFactory
	runtimeFactory = func() (tooling.ContainerRuntime, io.Closer, error) {
		return happyMockRuntime(), &noopCloser{}, nil
	}
	defer func() { runtimeFactory = origFactory }()

	var stdout bytes.Buffer
	run(&stdout, &bytes.Buffer{})

	output := stdout.String()
	// Test 9 sends language "cobol" which is rejected by schema validation
	if !strings.Contains(output, "Error:") {
		t.Error("Expected 'Error:' in output for invalid input test")
	}
}

// =============================================================================
// run — Error Path (runtime creation failure)
// =============================================================================

func TestRun_ShouldReportErrorWhenRuntimeCreationFails(t *testing.T) {
	origFactory := runtimeFactory
	origExit := osExit
	runtimeFactory = func() (tooling.ContainerRuntime, io.Closer, error) {
		return nil, nil, fmt.Errorf("docker not running")
	}
	var exitCode int
	osExit = func(code int) { exitCode = code }
	defer func() {
		runtimeFactory = origFactory
		osExit = origExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "docker not running") {
		t.Errorf("Expected error message in stderr, got: %s", stderr.String())
	}
}

// =============================================================================
// run — Nil closer path
// =============================================================================

func TestRun_ShouldHandleNilCloser(t *testing.T) {
	origFactory := runtimeFactory
	runtimeFactory = func() (tooling.ContainerRuntime, io.Closer, error) {
		return happyMockRuntime(), nil, nil // nil closer
	}
	defer func() { runtimeFactory = origFactory }()

	var stdout bytes.Buffer
	run(&stdout, &bytes.Buffer{})

	if !strings.Contains(stdout.String(), "Manual Integration Test Complete") {
		t.Error("Expected completion message")
	}
}

// =============================================================================
// runTest — Success and Error Paths
// =============================================================================

func TestRunTest_ShouldPrintOutputOnSuccess(t *testing.T) {
	rt := happyMockRuntime()
	rt.logs = "hello sandbox\n"
	tool := tooling.NewDockerSandboxTool(rt)

	var buf bytes.Buffer
	runTest(&buf, tool, `{"language":"python","code":"print('hi')"}`)

	output := buf.String()
	if !strings.Contains(output, "hello sandbox") {
		t.Errorf("Expected 'hello sandbox' in output, got: %s", output)
	}
	if !strings.Contains(output, "language=python") {
		t.Errorf("Expected 'language=python' in output, got: %s", output)
	}
}

func TestRunTest_ShouldPrintErrorOnFailure(t *testing.T) {
	rt := happyMockRuntime()
	tool := tooling.NewDockerSandboxTool(rt)

	var buf bytes.Buffer
	// Invalid language triggers schema validation error
	runTest(&buf, tool, `{"language":"invalid","code":"x"}`)

	output := buf.String()
	if !strings.Contains(output, "Error:") {
		t.Errorf("Expected 'Error:' in output, got: %s", output)
	}
}

// =============================================================================
// jsonEscape
// =============================================================================

func TestJsonEscape_ShouldEscapeString(t *testing.T) {
	result := jsonEscape(`hello "world"`)
	if result != `"hello \"world\""` {
		t.Errorf("Expected escaped string, got: %s", result)
	}
}

func TestJsonEscape_ShouldHandleNewlines(t *testing.T) {
	result := jsonEscape("line1\nline2")
	if !strings.Contains(result, `\n`) {
		t.Errorf("Expected escaped newline, got: %s", result)
	}
}

func TestJsonEscape_ShouldHandleEmptyString(t *testing.T) {
	result := jsonEscape("")
	if result != `""` {
		t.Errorf("Expected empty JSON string, got: %s", result)
	}
}

// =============================================================================
// main — Smoke test
// =============================================================================

func TestMain_ShouldCallRun(t *testing.T) {
	origFactory := runtimeFactory
	origExit := osExit
	runtimeFactory = func() (tooling.ContainerRuntime, io.Closer, error) {
		return happyMockRuntime(), &noopCloser{}, nil
	}
	osExit = func(_ int) {}
	defer func() {
		runtimeFactory = origFactory
		osExit = origExit
	}()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	w.Close()
	os.Stdout = old

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "Docker Sandbox Manual Integration Test") {
		t.Errorf("Expected header in output, got: %s", output)
	}
}

// =============================================================================
// Default runtimeFactory — exercises the real Docker runtime creation
// =============================================================================

func TestDefaultRuntimeFactory_ShouldCreateRuntimeSuccessfully(t *testing.T) {
	// The default runtimeFactory creates a real DockerContainerRuntime.
	// Client creation succeeds even without a running Docker daemon because
	// it only configures an HTTP client (no connection is made).
	rt, closer, err := runtimeFactory()
	if err != nil {
		t.Fatalf("Expected runtime creation to succeed, got: %v", err)
	}
	if rt == nil {
		t.Fatal("Expected non-nil runtime")
	}
	if closer == nil {
		t.Fatal("Expected non-nil closer")
	}
	if err := closer.Close(); err != nil {
		t.Errorf("Expected Close to succeed, got: %v", err)
	}
}

func TestDefaultRuntimeFactory_ShouldReturnErrorWhenTLSConfigFails(t *testing.T) {
	// When DOCKER_TLS_VERIFY=1 but DOCKER_CERT_PATH points to a nonexistent
	// directory, the Docker SDK fails to load TLS certificates.
	t.Setenv("DOCKER_TLS_VERIFY", "1")
	t.Setenv("DOCKER_CERT_PATH", "/nonexistent/cert/path")

	rt, closer, err := runtimeFactory()
	if err == nil {
		if closer != nil {
			closer.Close()
		}
		t.Skip("Docker SDK did not fail with invalid TLS config in this environment")
	}
	if rt != nil {
		t.Error("Expected nil runtime on error")
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ tooling.ContainerRuntime = (*mockRuntime)(nil)
var _ io.Closer = (*noopCloser)(nil)
