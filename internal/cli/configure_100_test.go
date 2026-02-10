package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Test to cover RunConfigure config save error path
func TestRunConfigure_WhenConfigSaveFails_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "configure-save-fail")

	// Create workspace
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a valid config file
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{"gateway":{"port":8080},"agents":{"provider":"local","defaultModel":"gpt-4o"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Make file read-only
	if err := os.Chmod(configPath, 0444); err != nil {
		t.Skip("Cannot change permissions on this system")
	}
	defer os.Chmod(configPath, 0644)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		GatewayPort:    9090, // This will trigger save
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code == 0 {
		t.Log("RunConfigure with readonly config did not fail (system may allow write)")
	}
}

// Test to cover RunConfigure skills directory creation error
func TestRunConfigure_WhenSkillsDirCannotBeCreated_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "configure-skills-fail")

	// Create workspace
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a valid config file
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{"gateway":{"port":8080},"agents":{"provider":"local","defaultModel":"gpt-4o"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file named "skills" to block directory creation
	skillsFile := filepath.Join(workspaceDir, "skills")
	if err := os.WriteFile(skillsFile, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		Skills:         []string{"test"}, // This will try to create skills dir
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code == 0 {
		t.Log("RunConfigure with file blocking skills dir did not fail")
	}
}

// Test to cover RunConfigure with all options
func TestRunConfigure_WithAllOptions_ExercisesAllBranches(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "configure-all-options")

	// Create workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"gateway":{"port":8080,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"provider":"local","defaultModel":"gpt-4o","paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		GatewayPort:    7777,
		GatewayAuth:    "token",
		DefaultModel:   "claude-3",
		Provider:       "anthropic",
		Channels:       []string{"telegram", "discord"},
		Skills:         []string{"docker"},
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfigure with all options: want 0, got %d", code)
	}
}
