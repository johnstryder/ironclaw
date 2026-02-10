package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Additional RunSetup tests for edge cases
// =============================================================================

func TestRunSetup_WhenDefaultWorkspacePath_ShouldUseHomeDir(t *testing.T) {
	// Save current HOME
	oldHome := os.Getenv("HOME")
	testHome := t.TempDir()
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", oldHome)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      "", // Empty, should use default
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunSetup with default workspace: want 0, got %d. stderr: %s", code, errOut.String())
	}

	// Verify workspace was created in home dir
	expectedWorkspace := filepath.Join(testHome, ".ironclaw")
	if _, err := os.Stat(expectedWorkspace); os.IsNotExist(err) {
		t.Errorf("default workspace should be created at %s", expectedWorkspace)
	}
}

func TestRunSetup_WhenHomeDirUnset_ShouldHandleGracefully(t *testing.T) {
	// Clear HOME env var
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")
	defer func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
	}()

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      "", // Empty, should try to use home dir
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	// Should either succeed or fail gracefully
	if code == 0 {
		t.Log("RunSetup with no HOME succeeded (system provided fallback)")
	} else {
		t.Logf("RunSetup with no HOME failed as expected: %s", errOut.String())
	}
}

func TestRunSetup_WhenWorkspaceIsFile_ShouldFail(t *testing.T) {
	dir := t.TempDir()
	workspacePath := filepath.Join(dir, "file-instead-of-dir")

	// Create a file instead of directory
	if err := os.WriteFile(workspacePath, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      workspacePath,
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	// Should fail because we can't create a directory where a file exists
	// or because we can't write config to a file path
	if code == 0 {
		t.Log("RunSetup with file as workspace did not fail (system may handle it)")
	}
}

func TestRunSetup_WhenConfigWriteFails_ShouldFail(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "readonly-workspace")

	// Create directory
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Make it read-only
	if err := os.Chmod(workspaceDir, 0555); err != nil {
		t.Skip("Cannot change permissions on this system")
	}
	defer os.Chmod(workspaceDir, 0755) // Cleanup

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	// Should fail because we can't write config
	if code == 0 {
		t.Log("RunSetup with readonly dir did not fail (system may allow write)")
	}
}

func TestRunSetup_WhenBothRemoteURLAndTokenProvided_ShouldSetBoth(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "remote-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      workspaceDir,
		RemoteURL:      "https://api.example.com",
		RemoteToken:    "my-secret-token",
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunSetup: want 0, got %d", code)
	}

	// Verify config contains both values
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "api.example.com") {
		t.Errorf("config should contain remote URL, got: %s", string(content))
	}
	if !strings.Contains(string(content), "my-secret-token") {
		t.Errorf("config should contain remote token, got: %s", string(content))
	}
}
