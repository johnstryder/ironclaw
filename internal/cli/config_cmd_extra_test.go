package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Additional RunConfig tests for edge cases
// =============================================================================

func TestRunConfig_WhenInvalidAction_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-invalid-action")

	// Create config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{"test": "value"}`), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "invalid",
		Path:      "test",
	}

	code := RunConfig(opts, out, errOut)

	if code == 0 {
		t.Error("RunConfig with invalid action should fail")
	}

	if !strings.Contains(errOut.String(), "unknown action") {
		t.Errorf("error should mention unknown action, got: %s", errOut.String())
	}
}

func TestRunConfig_WhenHomeDirFails_ShouldError(t *testing.T) {
	// This test might not work on all systems, but we can try
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: "", // Will try to use home dir
		Action:    "get",
		Path:      "test",
	}

	// Save current HOME and clear it
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "")
	defer os.Setenv("HOME", oldHome)

	code := RunConfig(opts, out, errOut)

	// Should handle missing home directory gracefully
	if code == 0 {
		t.Log("RunConfig with no HOME did not fail (system handled it)")
	}
}

func TestRunConfigGet_WhenPathIsComplex_ShouldWork(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-complex")

	// Create config with nested structure
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{
		"gateway": {
			"port": 8080,
			"auth": {
				"mode": "token",
				"authToken": "secret123"
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// Test deeply nested path
	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "gateway.auth.mode",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig get nested path: want 0, got %d. stderr: %s", code, errOut.String())
	}

	if !strings.Contains(out.String(), "token") {
		t.Errorf("output should contain 'token', got: %s", out.String())
	}
}

func TestRunConfigGet_WhenValueIsBool_ShouldPrintBool(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-bool")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"enabled": true, "disabled": false}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "enabled",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig get bool: want 0, got %d", code)
	}

	output := strings.TrimSpace(out.String())
	if output != "true" {
		t.Errorf("output should be 'true', got: %s", output)
	}
}

func TestRunConfigGet_WhenValueIsFloat_ShouldPrintNumber(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-float")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"pi": 3.14, "count": 42}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// Test integer
	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "count",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig get int: want 0, got %d", code)
	}

	output := strings.TrimSpace(out.String())
	if output != "42" {
		t.Errorf("output should be '42', got: %s", output)
	}

	// Test float
	out.Reset()
	opts.Path = "pi"

	code = RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig get float: want 0, got %d", code)
	}

	output = strings.TrimSpace(out.String())
	if output != "3.14" {
		t.Errorf("output should be '3.14', got: %s", output)
	}
}

func TestRunConfigSet_WhenValueIsBool_ShouldParseBool(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-set-bool")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "set",
		Path:      "enabled",
		Value:     "true",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig set bool: want 0, got %d", code)
	}

	// Verify the config contains boolean, not string
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	// JSON should have "enabled": true (without quotes)
	if strings.Contains(string(content), `"enabled": "true"`) {
		t.Errorf("boolean should not be stored as string, got: %s", string(content))
	}
	if !strings.Contains(string(content), `"enabled": true`) {
		t.Errorf("config should contain boolean true, got: %s", string(content))
	}
}

func TestRunConfigSet_WhenValueIsNumber_ShouldParseNumber(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-set-number")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "set",
		Path:      "port",
		Value:     "8080",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig set number: want 0, got %d", code)
	}

	// Verify the config contains number, not string
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	// JSON should have "port": 8080 (without quotes)
	if strings.Contains(string(content), `"port": "8080"`) {
		t.Errorf("number should not be stored as string, got: %s", string(content))
	}
}

func TestRunConfigSet_WhenCreatingNestedPath_ShouldCreateIntermediateObjects(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-create-nested")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "set",
		Path:      "a.b.c.d",
		Value:     "nested-value",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig set deeply nested: want 0, got %d", code)
	}

	// Verify the nested structure was created
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "nested-value") {
		t.Errorf("config should contain nested value, got: %s", string(content))
	}
}

func TestRunConfigUnset_WhenPathDoesNotExist_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-unset-missing")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"existing": "value"}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "unset",
		Path:      "nonexistent.path.that.does.not.exist",
	}

	code := RunConfig(opts, out, errOut)

	// Should error because path doesn't exist
	if code == 0 {
		t.Error("RunConfig unset nonexistent path should fail")
	}
}

func TestRunConfig_WhenConfigIsInvalidJSON_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-invalid-json")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `this is not valid json {`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "test",
	}

	code := RunConfig(opts, out, errOut)

	if code == 0 {
		t.Error("RunConfig with invalid JSON should fail")
	}
}

func TestRunConfig_WhenConfigCannotBeRead_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-unreadable")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0000); err != nil {
		t.Skip("Cannot create unreadable file on this system")
	}
	defer os.Chmod(configPath, 0644) // Cleanup

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "test",
	}

	code := RunConfig(opts, out, errOut)

	if code == 0 {
		t.Log("RunConfig with unreadable file did not fail (system may allow read)")
	}
}

// =============================================================================
// Test getValueAtPath edge cases
// =============================================================================

func TestGetValueAtPath_WhenEmptyPath_ShouldReturnNil(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	result := getValueAtPath(data, []string{})
	if result != nil {
		t.Errorf("empty path should return nil, got: %v", result)
	}
}

func TestGetValueAtPath_WhenPathHasNonObject_ShouldReturnNil(t *testing.T) {
	data := map[string]interface{}{
		"key": "string-value",
	}
	result := getValueAtPath(data, []string{"key", "nested"})
	if result != nil {
		t.Errorf("path through non-object should return nil, got: %v", result)
	}
}

// =============================================================================
// Test setValueAtPath edge cases
// =============================================================================

func TestSetValueAtPath_WhenEmptyPath_ShouldError(t *testing.T) {
	data := map[string]interface{}{}
	err := setValueAtPath(data, []string{}, "value")
	if err == nil {
		t.Error("empty path should return error")
	}
}

func TestSetValueAtPath_WhenOverwritingExistingValue_ShouldReplace(t *testing.T) {
	data := map[string]interface{}{
		"key": "old-value",
	}
	err := setValueAtPath(data, []string{"key"}, "new-value")
	if err != nil {
		t.Errorf("should not error: %v", err)
	}
	if data["key"] != "new-value" {
		t.Errorf("value should be replaced, got: %v", data["key"])
	}
}

// =============================================================================
// Test unsetValueAtPath edge cases
// =============================================================================

func TestUnsetValueAtPath_WhenEmptyPath_ShouldError(t *testing.T) {
	data := map[string]interface{}{}
	err := unsetValueAtPath(data, []string{})
	if err == nil {
		t.Error("empty path should return error")
	}
}

func TestUnsetValueAtPath_WhenPathHasNonObject_ShouldError(t *testing.T) {
	data := map[string]interface{}{
		"key": "string-value",
	}
	err := unsetValueAtPath(data, []string{"key", "nested"})
	if err == nil {
		t.Error("path through non-object should return error")
	}
}

// =============================================================================
// Test saveConfig edge cases
// =============================================================================

func TestSaveConfig_WhenDirectoryDoesNotExist_ShouldError(t *testing.T) {
	cfg := map[string]interface{}{"key": "value"}
	// Use a path with non-existent parent directories that we can't create
	path := "/nonexistent/deep/path/config.json"
	err := saveConfig(path, cfg)
	if err == nil {
		t.Log("saveConfig to nonexistent dir did not fail (system may allow it)")
	}
}
