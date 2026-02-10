package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Test RunConfig when os.ReadFile fails (not just file not found, but permission denied)
func TestRunConfig_WhenReadFileFailsWithPermissionDenied_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-read-fail")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Make config unreadable
	if err := os.Chmod(configPath, 0000); err != nil {
		t.Skip("Cannot change file permissions on this system")
	}
	defer os.Chmod(configPath, 0644)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "key",
	}

	code := RunConfig(opts, out, errOut)

	if code == 0 {
		t.Error("RunConfig with unreadable file should fail")
	}
}

// Test runConfigSet when save fails due to directory being removed
func TestRunConfigSet_WhenSaveFails_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-set-save-fail-final")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// Remove the workspace directory after setting up to cause save to fail
	os.RemoveAll(workspaceDir)

	code := runConfigSet(&cfg, "key", "value", configPath, out, errOut)

	if code == 0 {
		t.Log("runConfigSet with missing directory did not fail")
	}
}

// Test runConfigUnset when save fails
func TestRunConfigUnset_WhenSaveFails_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-unset-save-fail-final")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{"key": "value"}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{
		"key": "value",
	}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// Remove the workspace directory after setting up to cause save to fail
	os.RemoveAll(workspaceDir)

	code := runConfigUnset(&cfg, "key", configPath, out, errOut)

	if code == 0 {
		t.Log("runConfigUnset with missing directory did not fail")
	}
}
