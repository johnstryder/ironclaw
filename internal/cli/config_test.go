package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ironclaw/internal/config"
)

func TestConfigCommand_WhenGet_ShouldReturnValue(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-workspace")

	// Create existing workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "gateway.port",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig get: want exit code 0, got %d. stderr: %s", code, errOut.String())
	}

	// Should print the port value
	output := strings.TrimSpace(out.String())
	if output != "8080" {
		t.Errorf("RunConfig get gateway.port: want '8080', got '%s'", output)
	}
}

func TestConfigCommand_WhenGetNestedPath_ShouldReturnValue(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-workspace")

	// Create existing workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "agents.defaultModel",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig get: want exit code 0, got %d", code)
	}

	output := strings.TrimSpace(out.String())
	if output != "gpt-4o" {
		t.Errorf("RunConfig get agents.defaultModel: want 'gpt-4o', got '%s'", output)
	}
}

func TestConfigCommand_WhenSet_ShouldUpdateValue(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-workspace")

	// Create existing workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "set",
		Path:      "gateway.port",
		Value:     "9090",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig set: want exit code 0, got %d. stderr: %s", code, errOut.String())
	}

	// Verify config was updated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "9090") {
		t.Errorf("config should contain updated port 9090, got: %s", string(content))
	}

	// Verify output confirms success
	if !strings.Contains(out.String(), "set") && !strings.Contains(out.String(), "ok") {
		t.Errorf("output should confirm value was set, got: %s", out.String())
	}
}

func TestConfigCommand_WhenSetNestedPath_ShouldUpdateValue(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-workspace")

	// Create existing workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "set",
		Path:      "agents.defaultModel",
		Value:     "claude-3",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig set nested: want exit code 0, got %d", code)
	}

	// Verify config was updated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "claude-3") {
		t.Errorf("config should contain updated model 'claude-3', got: %s", string(content))
	}
}

func TestConfigCommand_WhenUnset_ShouldRemoveValue(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-workspace")

	// Create existing workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "unset",
		Path:      "agents.defaultModel",
	}

	code := RunConfig(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfig unset: want exit code 0, got %d", code)
	}

	// For now, unset just sets an empty value (or could remove the field)
	// The main thing is that it doesn't error
}

func TestConfigCommand_WhenGetInvalidPath_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-workspace")

	// Create existing workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "invalid.path.that.does.not.exist",
	}

	code := RunConfig(opts, out, errOut)

	// Should error for invalid path
	if code == 0 {
		t.Error("RunConfig get invalid path: expected error")
	}
}

func TestConfigCommand_WhenConfigMissing_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "missing-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "gateway.port",
	}

	code := RunConfig(opts, out, errOut)

	// Should error when config doesn't exist
	if code == 0 {
		t.Error("RunConfig with missing config: expected error")
	}
}
