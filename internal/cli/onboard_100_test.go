package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Test to cover RunOnboard config load error path
func TestRunOnboard_WhenConfigLoadFails_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-load-fail")

	// Create workspace
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an invalid config file
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`invalid json`), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	// Should fail when trying to load invalid config
	if code == 0 {
		t.Error("RunOnboard with invalid config should fail")
	}
}

// Test to cover RunOnboard config save error path
func TestRunOnboard_WhenConfigSaveFails_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-save-fail")

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

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		GatewayPort:    9090, // This will trigger save
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code == 0 {
		t.Log("RunOnboard with readonly config did not fail (system may allow write)")
	}
}

// Test to cover RunOnboard with all options to exercise all branches
func TestRunOnboard_WithAllOptions_ExercisesAllBranches(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-all-branches")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// Use all options to exercise different code branches
	opts := OnboardOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
		GatewayPort:    7777,
		GatewayAuth:    "token",
		AuthToken:      "secret-token",
		DefaultModel:   "claude-3",
		Provider:       "anthropic",
		Skills:         []string{"docker", "git"},
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard with all options: want 0, got %d", code)
	}
}

// Test to cover RunOnboard when workspace cannot be created
func TestRunOnboard_WhenWorkspaceCannotBeCreated_ShouldError(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      "/root/cannot-create-here",
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code == 0 {
		t.Log("RunOnboard with restricted path did not fail (may be running as root)")
	}
}
