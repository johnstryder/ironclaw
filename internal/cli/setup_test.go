package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupCommand_WhenNoFlags_ShouldInitializeDefaultWorkspace(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunSetup: want exit code 0, got %d", code)
	}

	// Check that workspace directory was created
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		t.Errorf("workspace directory should be created at %s", workspaceDir)
	}

	// Check that config file was created
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config file should be created at %s", configPath)
	}

	// Verify output contains success message
	output := out.String()
	if !strings.Contains(output, "initialized") && !strings.Contains(output, "workspace") {
		t.Errorf("output should indicate successful initialization, got: %s", output)
	}
}

func TestSetupCommand_WhenWorkspaceFlagProvided_ShouldUseCustomPath(t *testing.T) {
	dir := t.TempDir()
	customWorkspace := filepath.Join(dir, "my-custom-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      customWorkspace,
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunSetup: want exit code 0, got %d", code)
	}

	// Check that custom workspace directory was created
	if _, err := os.Stat(customWorkspace); os.IsNotExist(err) {
		t.Errorf("custom workspace directory should be created at %s", customWorkspace)
	}
}

func TestSetupCommand_WhenWorkspaceAlreadyExists_ShouldNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "existing-workspace")

	// Create the workspace directory and a config file
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	existingConfig := filepath.Join(workspaceDir, "ironclaw.json")
	customContent := `{"custom": "content"}`
	if err := os.WriteFile(existingConfig, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunSetup: want exit code 0, got %d", code)
	}

	// Verify existing config was NOT overwritten
	content, err := os.ReadFile(existingConfig)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != customContent {
		t.Errorf("existing config should not be overwritten, want %q, got %q", customContent, string(content))
	}
}

func TestSetupCommand_WhenWizardMode_ShouldRunInteractive(t *testing.T) {
	// This test verifies the wizard flag is accepted
	// Full interactive testing would require mocking stdin
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "wizard-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      workspaceDir,
		Wizard:         true,
		NonInteractive: true, // Still non-interactive for test, but Wizard flag should be set
	}

	code := RunSetup(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunSetup with wizard: want exit code 0, got %d", code)
	}

	// Verify workspace was still created
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		t.Errorf("workspace should be created even in wizard mode")
	}
}

func TestSetupCommand_WhenModeFlagProvided_ShouldSetMode(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "mode-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      workspaceDir,
		Mode:           "server",
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunSetup: want exit code 0, got %d", code)
	}

	// Verify config was created with mode setting
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "server") {
		t.Errorf("config should contain mode setting, got: %s", string(content))
	}
}

func TestSetupCommand_WhenRemoteFlagsProvided_ShouldConfigureRemote(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "remote-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      workspaceDir,
		RemoteURL:      "https://api.example.com",
		RemoteToken:    "secret-token-123",
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunSetup: want exit code 0, got %d", code)
	}

	// Verify config was created with remote settings
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	// Remote URL should be in config
	if !strings.Contains(string(content), "example.com") {
		t.Errorf("config should contain remote URL, got: %s", string(content))
	}
}
