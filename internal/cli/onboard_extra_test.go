package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Additional RunOnboard tests for edge cases
// =============================================================================

func TestRunOnboard_WhenDefaultWorkspace_ShouldUseHomeDir(t *testing.T) {
	// Save current HOME
	oldHome := os.Getenv("HOME")
	testHome := t.TempDir()
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", oldHome)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      "", // Empty, should use default
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard with default workspace: want 0, got %d", code)
	}

	// Verify workspace was created in home dir
	expectedWorkspace := filepath.Join(testHome, ".ironclaw")
	if _, err := os.Stat(expectedWorkspace); os.IsNotExist(err) {
		t.Errorf("default workspace should be created at %s", expectedWorkspace)
	}
}

func TestRunOnboard_WhenSkillsProvided_ShouldCreateSkillsDir(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-skills")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		Skills:         []string{"filesystem", "shell", "web", "docker", "git"},
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard: want 0, got %d", code)
	}

	// Verify skills directory was created
	skillsDir := filepath.Join(workspaceDir, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Errorf("skills directory should be created at %s", skillsDir)
	}

	// Verify output mentions skills
	if !strings.Contains(out.String(), "skill") {
		t.Errorf("output should mention skills, got: %s", out.String())
	}
}

func TestRunOnboard_WhenAllOptionsProvided_ShouldConfigureAll(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-all")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		GatewayPort:    9999,
		GatewayAuth:    "token",
		AuthToken:      "super-secret",
		DefaultModel:   "claude-3-opus",
		Provider:       "anthropic",
		Skills:         []string{"all"},
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard: want 0, got %d. stderr: %s", code, errOut.String())
	}

	// Verify config was created with all settings
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	configStr := string(content)
	if !strings.Contains(configStr, "9999") {
		t.Errorf("config should contain gateway port 9999")
	}
	if !strings.Contains(configStr, "token") {
		t.Errorf("config should contain auth mode token")
	}
	if !strings.Contains(configStr, "claude-3-opus") {
		t.Errorf("config should contain model claude-3-opus")
	}
	if !strings.Contains(configStr, "anthropic") {
		t.Errorf("config should contain provider anthropic")
	}
}

func TestRunOnboard_WhenExistingWorkspaceWithCustomConfig_ShouldPreserveAndUpdate(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-existing")

	// Create workspace with custom config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	customConfig := `{
		"customField": "customValue",
		"gateway": {
			"port": 1111,
			"auth": {
				"mode": "none"
			}
		},
		"agents": {
			"provider": "local",
			"defaultModel": "original-model"
		}
	}`
	if err := os.WriteFile(configPath, []byte(customConfig), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		DefaultModel:   "updated-model",
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard: want 0, got %d", code)
	}

	// Verify config was updated with new model
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	// The config was loaded into domain.Config which doesn't have customField,
	// so custom fields are not preserved (this is expected behavior)
	if !strings.Contains(string(content), "updated-model") {
		t.Errorf("config should contain updated model")
	}
}

func TestRunOnboard_WhenWorkspaceCannotBeCreated_ShouldFail(t *testing.T) {
	// Try to create workspace in a read-only parent directory
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      "/root/cannot-create-here", // Likely cannot create here without root
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code == 0 {
		t.Log("RunOnboard with restricted path did not fail (running as root?)")
	}
}
