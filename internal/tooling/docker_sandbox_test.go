package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockContainerRuntime is a configurable test double for ContainerRuntime.
type mockContainerRuntime struct {
	// Configurable return values
	ensureImageErr     error
	createContainerID  string
	createContainerErr error
	startErr           error
	waitExitCode       int64
	waitErr            error
	logs               string
	logsErr            error
	removeErr          error

	// Spy tracking: records every call for assertion
	ensureImageCalls []string
	createCalls      []SandboxContainerConfig
	startCalls       []string
	waitCalls        []string
	logsCalls        []string
	removeCalls      []string
}

func (m *mockContainerRuntime) EnsureImage(_ context.Context, image string) error {
	m.ensureImageCalls = append(m.ensureImageCalls, image)
	return m.ensureImageErr
}

func (m *mockContainerRuntime) CreateContainer(_ context.Context, cfg SandboxContainerConfig) (string, error) {
	m.createCalls = append(m.createCalls, cfg)
	return m.createContainerID, m.createContainerErr
}

func (m *mockContainerRuntime) StartContainer(_ context.Context, containerID string) error {
	m.startCalls = append(m.startCalls, containerID)
	return m.startErr
}

func (m *mockContainerRuntime) WaitContainer(_ context.Context, containerID string) (int64, error) {
	m.waitCalls = append(m.waitCalls, containerID)
	return m.waitExitCode, m.waitErr
}

func (m *mockContainerRuntime) GetLogs(_ context.Context, containerID string) (string, error) {
	m.logsCalls = append(m.logsCalls, containerID)
	return m.logs, m.logsErr
}

func (m *mockContainerRuntime) RemoveContainer(_ context.Context, containerID string) error {
	m.removeCalls = append(m.removeCalls, containerID)
	return m.removeErr
}

// happyRuntime returns a mock that succeeds at every step.
func happyRuntime() *mockContainerRuntime {
	return &mockContainerRuntime{
		createContainerID: "container-abc123",
		logs:              "hello world\n",
	}
}

// =============================================================================
// DockerSandboxTool — Name, Description, Definition
// =============================================================================

func TestDockerSandboxTool_Name_ShouldReturnDockerSandbox(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	if tool.Name() != "docker_sandbox" {
		t.Errorf("Expected name 'docker_sandbox', got '%s'", tool.Name())
	}
}

func TestDockerSandboxTool_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
	if !strings.Contains(strings.ToLower(desc), "docker") && !strings.Contains(strings.ToLower(desc), "sandbox") {
		t.Errorf("Expected description to mention docker or sandbox, got: %s", desc)
	}
}

func TestDockerSandboxTool_Definition_ShouldContainLanguageAndCodeProperties(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
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
	if _, exists := props["language"]; !exists {
		t.Error("Expected 'language' property in schema")
	}
	if _, exists := props["code"]; !exists {
		t.Error("Expected 'code' property in schema")
	}
}

func TestDockerSandboxTool_Definition_ShouldRequireLanguageAndCodeFields(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("Expected 'required' array in schema")
	}
	foundLanguage, foundCode := false, false
	for _, r := range required {
		if r == "language" {
			foundLanguage = true
		}
		if r == "code" {
			foundCode = true
		}
	}
	if !foundLanguage {
		t.Error("Expected 'language' in required fields")
	}
	if !foundCode {
		t.Error("Expected 'code' in required fields")
	}
}

func TestDockerSandboxTool_Definition_ShouldContainTimeoutProperty(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	props := parsed["properties"].(map[string]interface{})
	if _, exists := props["timeout"]; !exists {
		t.Error("Expected 'timeout' property in schema")
	}
}

// =============================================================================
// DockerSandboxTool.Call — Input Validation
// =============================================================================

func TestDockerSandboxTool_Call_ShouldRejectInvalidJSON(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	_, err := tool.Call(json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestDockerSandboxTool_Call_ShouldRejectMissingLanguageField(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	_, err := tool.Call(json.RawMessage(`{"code":"print('hi')"}`))
	if err == nil {
		t.Fatal("Expected error for missing language field")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestDockerSandboxTool_Call_ShouldRejectMissingCodeField(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	_, err := tool.Call(json.RawMessage(`{"language":"python"}`))
	if err == nil {
		t.Fatal("Expected error for missing code field")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestDockerSandboxTool_Call_ShouldRejectEmptyCodeString(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":""}`))
	if err == nil {
		t.Fatal("Expected error for empty code string")
	}
}

func TestDockerSandboxTool_Call_ShouldRejectUnsupportedLanguage(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	_, err := tool.Call(json.RawMessage(`{"language":"cobol","code":"DISPLAY 'HI'"}`))
	if err == nil {
		t.Fatal("Expected error for unsupported language")
	}
}

func TestDockerSandboxTool_Call_ShouldRejectWrongTypeForLanguage(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	_, err := tool.Call(json.RawMessage(`{"language":123,"code":"x"}`))
	if err == nil {
		t.Fatal("Expected error for wrong type in language field")
	}
}

func TestDockerSandboxTool_Call_ShouldRejectWrongTypeForCode(t *testing.T) {
	tool := NewDockerSandboxTool(happyRuntime())
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":123}`))
	if err == nil {
		t.Fatal("Expected error for wrong type in code field")
	}
}

// =============================================================================
// DockerSandboxTool.Call — Container Lifecycle
// =============================================================================

func TestDockerSandboxTool_Call_ShouldEnsureImageBeforeCreatingContainer(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print('hi')"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.ensureImageCalls) == 0 {
		t.Fatal("Expected EnsureImage to be called")
	}
	if rt.ensureImageCalls[0] != "python:3-slim" {
		t.Errorf("Expected image 'python:3-slim', got '%s'", rt.ensureImageCalls[0])
	}
}

func TestDockerSandboxTool_Call_ShouldCreateContainerWithCorrectImage(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"bash","code":"echo hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.createCalls) == 0 {
		t.Fatal("Expected CreateContainer to be called")
	}
	if rt.createCalls[0].Image != "alpine:latest" {
		t.Errorf("Expected image 'alpine:latest', got '%s'", rt.createCalls[0].Image)
	}
}

func TestDockerSandboxTool_Call_ShouldStartContainerWithCorrectID(t *testing.T) {
	rt := happyRuntime()
	rt.createContainerID = "test-container-xyz"
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.startCalls) == 0 {
		t.Fatal("Expected StartContainer to be called")
	}
	if rt.startCalls[0] != "test-container-xyz" {
		t.Errorf("Expected container ID 'test-container-xyz', got '%s'", rt.startCalls[0])
	}
}

func TestDockerSandboxTool_Call_ShouldWaitForContainerCompletion(t *testing.T) {
	rt := happyRuntime()
	rt.createContainerID = "wait-test"
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.waitCalls) == 0 {
		t.Fatal("Expected WaitContainer to be called")
	}
	if rt.waitCalls[0] != "wait-test" {
		t.Errorf("Expected WaitContainer called with 'wait-test', got '%s'", rt.waitCalls[0])
	}
}

func TestDockerSandboxTool_Call_ShouldReturnContainerLogs(t *testing.T) {
	rt := happyRuntime()
	rt.logs = "computation result: 42\n"
	tool := NewDockerSandboxTool(rt)
	result, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(42)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Data, "computation result: 42") {
		t.Errorf("Expected logs in output, got: '%s'", result.Data)
	}
}

func TestDockerSandboxTool_Call_ShouldReturnExitCodeInMetadata(t *testing.T) {
	rt := happyRuntime()
	rt.waitExitCode = 0
	tool := NewDockerSandboxTool(rt)
	result, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metadata["exit_code"] != "0" {
		t.Errorf("Expected exit_code=0, got '%s'", result.Metadata["exit_code"])
	}
}

func TestDockerSandboxTool_Call_ShouldReturnNonZeroExitCodeInMetadata(t *testing.T) {
	rt := happyRuntime()
	rt.waitExitCode = 1
	rt.logs = "Traceback: error\n"
	tool := NewDockerSandboxTool(rt)
	result, err := tool.Call(json.RawMessage(`{"language":"python","code":"raise Exception()"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metadata["exit_code"] != "1" {
		t.Errorf("Expected exit_code=1, got '%s'", result.Metadata["exit_code"])
	}
}

func TestDockerSandboxTool_Call_ShouldReturnLanguageInMetadata(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	result, err := tool.Call(json.RawMessage(`{"language":"javascript","code":"console.log(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metadata["language"] != "javascript" {
		t.Errorf("Expected language='javascript', got '%s'", result.Metadata["language"])
	}
}

func TestDockerSandboxTool_Call_ShouldReturnImageInMetadata(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	result, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metadata["image"] != "python:3-slim" {
		t.Errorf("Expected image='python:3-slim', got '%s'", result.Metadata["image"])
	}
}

func TestDockerSandboxTool_Call_ShouldRemoveContainerAfterExecution(t *testing.T) {
	rt := happyRuntime()
	rt.createContainerID = "to-remove"
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.removeCalls) == 0 {
		t.Fatal("Expected RemoveContainer to be called")
	}
	if rt.removeCalls[0] != "to-remove" {
		t.Errorf("Expected removal of 'to-remove', got '%s'", rt.removeCalls[0])
	}
}

func TestDockerSandboxTool_Call_ShouldRemoveContainerEvenWhenStartFails(t *testing.T) {
	rt := happyRuntime()
	rt.createContainerID = "cleanup-on-fail"
	rt.startErr = fmt.Errorf("start failed")
	tool := NewDockerSandboxTool(rt)
	_, _ = tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if len(rt.removeCalls) == 0 {
		t.Fatal("Expected RemoveContainer to be called even after start failure")
	}
	if rt.removeCalls[0] != "cleanup-on-fail" {
		t.Errorf("Expected removal of 'cleanup-on-fail', got '%s'", rt.removeCalls[0])
	}
}

func TestDockerSandboxTool_Call_ShouldRemoveContainerEvenWhenWaitFails(t *testing.T) {
	rt := happyRuntime()
	rt.createContainerID = "cleanup-on-wait-fail"
	rt.waitErr = fmt.Errorf("wait failed")
	tool := NewDockerSandboxTool(rt)
	_, _ = tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if len(rt.removeCalls) == 0 {
		t.Fatal("Expected RemoveContainer to be called even after wait failure")
	}
	if rt.removeCalls[0] != "cleanup-on-wait-fail" {
		t.Errorf("Expected removal of 'cleanup-on-wait-fail', got '%s'", rt.removeCalls[0])
	}
}

func TestDockerSandboxTool_Call_ShouldNotRemoveWhenCreateFails(t *testing.T) {
	rt := happyRuntime()
	rt.createContainerErr = fmt.Errorf("create failed")
	tool := NewDockerSandboxTool(rt)
	_, _ = tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if len(rt.removeCalls) != 0 {
		t.Error("RemoveContainer should NOT be called when CreateContainer fails (no container to remove)")
	}
}

// =============================================================================
// DockerSandboxTool.Call — Security Constraints
// =============================================================================

func TestDockerSandboxTool_Call_ShouldDisableNetworkAccess(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.createCalls) == 0 {
		t.Fatal("Expected CreateContainer to be called")
	}
	if !rt.createCalls[0].NetworkDisabled {
		t.Error("Expected NetworkDisabled=true for security isolation")
	}
}

func TestDockerSandboxTool_Call_ShouldSetMemoryLimit(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.createCalls) == 0 {
		t.Fatal("Expected CreateContainer to be called")
	}
	if rt.createCalls[0].MemoryLimit != defaultMemoryLimit {
		t.Errorf("Expected MemoryLimit=%d, got %d", defaultMemoryLimit, rt.createCalls[0].MemoryLimit)
	}
}

func TestDockerSandboxTool_Call_ShouldSetCPULimit(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.createCalls) == 0 {
		t.Fatal("Expected CreateContainer to be called")
	}
	if rt.createCalls[0].CPULimit != defaultCPULimit {
		t.Errorf("Expected CPULimit=%d, got %d", defaultCPULimit, rt.createCalls[0].CPULimit)
	}
}

func TestDockerSandboxTool_Call_ShouldSetPidsLimit(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.createCalls) == 0 {
		t.Fatal("Expected CreateContainer to be called")
	}
	if rt.createCalls[0].PidsLimit != defaultPidsLimit {
		t.Errorf("Expected PidsLimit=%d, got %d", defaultPidsLimit, rt.createCalls[0].PidsLimit)
	}
}

func TestDockerSandboxTool_Call_ShouldPassCommandToContainer(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.createCalls) == 0 {
		t.Fatal("Expected CreateContainer to be called")
	}
	cmd := rt.createCalls[0].Cmd
	if len(cmd) == 0 {
		t.Fatal("Expected non-empty command in container config")
	}
	// The command should contain "python3" (the interpreter for python)
	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "python3") {
		t.Errorf("Expected command to contain 'python3', got: %v", cmd)
	}
}

// =============================================================================
// DockerSandboxTool.Call — Error Handling
// =============================================================================

func TestDockerSandboxTool_Call_ShouldReturnErrorWhenImagePullFails(t *testing.T) {
	rt := happyRuntime()
	rt.ensureImageErr = fmt.Errorf("pull failed: network error")
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err == nil {
		t.Fatal("Expected error when image pull fails")
	}
	if !strings.Contains(err.Error(), "image") {
		t.Errorf("Expected error to mention 'image', got: %v", err)
	}
}

func TestDockerSandboxTool_Call_ShouldReturnErrorWhenContainerCreateFails(t *testing.T) {
	rt := happyRuntime()
	rt.createContainerErr = fmt.Errorf("create failed: no space")
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err == nil {
		t.Fatal("Expected error when container create fails")
	}
	if !strings.Contains(err.Error(), "container") {
		t.Errorf("Expected error to mention 'container', got: %v", err)
	}
}

func TestDockerSandboxTool_Call_ShouldReturnErrorWhenStartFails(t *testing.T) {
	rt := happyRuntime()
	rt.startErr = fmt.Errorf("start failed")
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err == nil {
		t.Fatal("Expected error when container start fails")
	}
	if !strings.Contains(err.Error(), "start") {
		t.Errorf("Expected error to mention 'start', got: %v", err)
	}
}

func TestDockerSandboxTool_Call_ShouldReturnErrorWhenWaitFails(t *testing.T) {
	rt := happyRuntime()
	rt.waitErr = fmt.Errorf("wait timed out")
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err == nil {
		t.Fatal("Expected error when container wait fails")
	}
	if !strings.Contains(err.Error(), "wait") {
		t.Errorf("Expected error to mention 'wait', got: %v", err)
	}
}

func TestDockerSandboxTool_Call_ShouldReturnErrorWhenLogsRetrievalFails(t *testing.T) {
	rt := happyRuntime()
	rt.logsErr = fmt.Errorf("logs unavailable")
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err == nil {
		t.Fatal("Expected error when logs retrieval fails")
	}
	if !strings.Contains(err.Error(), "logs") {
		t.Errorf("Expected error to mention 'logs', got: %v", err)
	}
}

func TestDockerSandboxTool_Call_ShouldReturnOutputEvenOnNonZeroExit(t *testing.T) {
	rt := happyRuntime()
	rt.waitExitCode = 1
	rt.logs = "error output here\n"
	tool := NewDockerSandboxTool(rt)
	result, err := tool.Call(json.RawMessage(`{"language":"python","code":"import sys; sys.exit(1)"}`))
	if err != nil {
		t.Fatalf("Non-zero exit should not be a Go error, got: %v", err)
	}
	if !strings.Contains(result.Data, "error output here") {
		t.Errorf("Expected error output in Data, got: '%s'", result.Data)
	}
}

func TestDockerSandboxTool_Call_ShouldNotCallCreateWhenImagePullFails(t *testing.T) {
	rt := happyRuntime()
	rt.ensureImageErr = fmt.Errorf("pull failed")
	tool := NewDockerSandboxTool(rt)
	_, _ = tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if len(rt.createCalls) != 0 {
		t.Error("CreateContainer should NOT be called when EnsureImage fails")
	}
}

func TestDockerSandboxTool_Call_ShouldNotCallStartWhenCreateFails(t *testing.T) {
	rt := happyRuntime()
	rt.createContainerErr = fmt.Errorf("create failed")
	tool := NewDockerSandboxTool(rt)
	_, _ = tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if len(rt.startCalls) != 0 {
		t.Error("StartContainer should NOT be called when CreateContainer fails")
	}
}

// =============================================================================
// DockerSandboxTool.Call — Unmarshal error path (defense-in-depth)
// =============================================================================

func TestDockerSandboxTool_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := dockerSandboxUnmarshalFunc
	dockerSandboxUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { dockerSandboxUnmarshalFunc = original }()

	tool := NewDockerSandboxTool(happyRuntime())
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

// =============================================================================
// DockerSandboxTool.Call — Defense-in-depth: unsupported language past schema
// =============================================================================

func TestDockerSandboxTool_Call_ShouldReturnErrorForUnsupportedLanguagePastSchema(t *testing.T) {
	// Temporarily add "cobol" to the schema enum by overriding the unmarshal func
	// to inject a language that passes schema but isn't in languageImages.
	original := dockerSandboxUnmarshalFunc
	dockerSandboxUnmarshalFunc = func(data []byte, v interface{}) error {
		// Unmarshal normally first, then override language
		if err := json.Unmarshal(data, v); err != nil {
			return err
		}
		if inp, ok := v.(*DockerSandboxInput); ok {
			inp.Language = "cobol"
		}
		return nil
	}
	defer func() { dockerSandboxUnmarshalFunc = original }()

	tool := NewDockerSandboxTool(happyRuntime())
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err == nil {
		t.Fatal("Expected error for unsupported language past schema validation")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("Expected 'unsupported language' in error, got: %v", err)
	}
}

// =============================================================================
// DockerSandboxTool.Call — Language Routing
// =============================================================================

func TestDockerSandboxTool_Call_ShouldUsePythonImageForPython(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"python","code":"print(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.ensureImageCalls) == 0 {
		t.Fatal("Expected EnsureImage to be called")
	}
	if rt.ensureImageCalls[0] != "python:3-slim" {
		t.Errorf("Expected 'python:3-slim', got '%s'", rt.ensureImageCalls[0])
	}
}

func TestDockerSandboxTool_Call_ShouldUseAlpineImageForBash(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"bash","code":"echo hi"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.ensureImageCalls[0] != "alpine:latest" {
		t.Errorf("Expected 'alpine:latest', got '%s'", rt.ensureImageCalls[0])
	}
}

func TestDockerSandboxTool_Call_ShouldUseNodeImageForJavaScript(t *testing.T) {
	rt := happyRuntime()
	tool := NewDockerSandboxTool(rt)
	_, err := tool.Call(json.RawMessage(`{"language":"javascript","code":"console.log(1)"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.ensureImageCalls[0] != "node:20-slim" {
		t.Errorf("Expected 'node:20-slim', got '%s'", rt.ensureImageCalls[0])
	}
}

// =============================================================================
// buildContainerCmd — Language Command Construction
// =============================================================================

func TestBuildContainerCmd_ShouldReturnNonEmptySliceForPython(t *testing.T) {
	cmd := buildContainerCmd("python", "print('hello')")
	if len(cmd) == 0 {
		t.Fatal("Expected non-empty command slice")
	}
}

func TestBuildContainerCmd_ShouldContainShDashC(t *testing.T) {
	cmd := buildContainerCmd("python", "print('hello')")
	if len(cmd) < 2 || cmd[0] != "sh" || cmd[1] != "-c" {
		t.Errorf("Expected command starting with ['sh', '-c'], got: %v", cmd)
	}
}

func TestBuildContainerCmd_ShouldContainPython3ForPython(t *testing.T) {
	cmd := buildContainerCmd("python", "print('hello')")
	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "python3") {
		t.Errorf("Expected 'python3' in command, got: %s", cmdStr)
	}
}

func TestBuildContainerCmd_ShouldContainShForBash(t *testing.T) {
	cmd := buildContainerCmd("bash", "echo hi")
	if len(cmd) < 3 {
		t.Fatal("Expected at least 3 elements in command")
	}
	// The inner command should pipe to sh
	if !strings.Contains(cmd[2], "| sh") {
		t.Errorf("Expected '| sh' in command, got: %s", cmd[2])
	}
}

func TestBuildContainerCmd_ShouldContainNodeForJavaScript(t *testing.T) {
	cmd := buildContainerCmd("javascript", "console.log(1)")
	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "node") {
		t.Errorf("Expected 'node' in command, got: %s", cmdStr)
	}
}

func TestBuildContainerCmd_ShouldBase64EncodeCode(t *testing.T) {
	cmd := buildContainerCmd("python", "print('hello')")
	if len(cmd) < 3 {
		t.Fatal("Expected at least 3 elements")
	}
	// The command should contain base64 decoding step
	if !strings.Contains(cmd[2], "base64") {
		t.Errorf("Expected 'base64' in command for safe code transport, got: %s", cmd[2])
	}
}

func TestBuildContainerCmd_ShouldHandleCodeWithSpecialChars(t *testing.T) {
	code := `print("hello's \"world\" & goodbye")`
	cmd := buildContainerCmd("python", code)
	if len(cmd) < 3 {
		t.Fatal("Expected at least 3 elements")
	}
	// Should still contain base64 (no shell escaping issues)
	if !strings.Contains(cmd[2], "base64") {
		t.Errorf("Expected base64 encoding for special chars, got: %s", cmd[2])
	}
}

func TestBuildContainerCmd_ShouldHandleMultilineCode(t *testing.T) {
	code := "x = 1\ny = 2\nprint(x + y)"
	cmd := buildContainerCmd("python", code)
	if len(cmd) < 3 {
		t.Fatal("Expected at least 3 elements")
	}
	if !strings.Contains(cmd[2], "base64") {
		t.Errorf("Expected base64 encoding for multiline code, got: %s", cmd[2])
	}
}

// =============================================================================
// resolveTimeout — Timeout Resolution
// =============================================================================

func TestResolveTimeout_ShouldReturnDefaultWhenZero(t *testing.T) {
	d := resolveTimeout(0)
	if d != resolveTimeout(defaultTimeout) {
		t.Errorf("Expected default timeout, got %v", d)
	}
}

func TestResolveTimeout_ShouldReturnSpecifiedValue(t *testing.T) {
	d := resolveTimeout(5)
	if d.Seconds() != 5 {
		t.Errorf("Expected 5s, got %v", d)
	}
}

func TestResolveTimeout_ShouldReturnDefaultForNegativeValue(t *testing.T) {
	d := resolveTimeout(-1)
	if d != resolveTimeout(0) {
		t.Errorf("Expected default timeout for negative value, got %v", d)
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*DockerSandboxTool)(nil)
var _ ContainerRuntime = (*mockContainerRuntime)(nil)
var _ ContainerRuntime = (*DockerContainerRuntime)(nil)
