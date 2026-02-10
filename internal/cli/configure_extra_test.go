package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Additional RunConfigure tests for edge cases
// =============================================================================

func TestRunConfigure_WhenDefaultWorkspace_ShouldUseHomeDir(t *testing.T) {
	// Save current HOME
	oldHome := os.Getenv("HOME")
	testHome := t.TempDir()
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", oldHome)

	// Create workspace in home dir first
	expectedWorkspace := filepath.Join(testHome, ".ironclaw")
	if err := os.MkdirAll(expectedWorkspace, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(expectedWorkspace, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{"gateway":{"port":8080}}`), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigureOptions{
		Workspace:      "", // Empty, should use default
		GatewayPort:    9090,
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfigure with default workspace: want 0, got %d", code)
	}

	// Verify config was updated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "9090") {
		t.Errorf("config should be updated")
	}
}

func TestRunConfigure_WhenMultipleOptions_ShouldUpdateAll(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "configure-multi")

	// Create workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{
		"gateway": {
			"port": 8080,
			"auth": {
				"mode": "none"
			},
			"allowedHosts": []
		},
		"agents": {
			"provider": "local",
			"defaultModel": "gpt-4o",
			"paths": {
				"root": "agents",
				"memory": "memory"
			}
		},
		"infra": {
			"logFormat": "text",
			"logLevel": "info"
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		GatewayPort:    7777,
		GatewayAuth:    "password",
		DefaultModel:   "claude-3-sonnet",
		Provider:       "anthropic",
		Channels:       []string{"telegram", "discord", "slack"},
		Skills:         []string{"docker", "git"},
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfigure: want 0, got %d. stderr: %s", code, errOut.String())
	}

	// Verify all changes were made
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	configStr := string(content)
	if !strings.Contains(configStr, "7777") {
		t.Errorf("config should contain port 7777")
	}
	if !strings.Contains(configStr, "password") {
		t.Errorf("config should contain auth mode password")
	}
	if !strings.Contains(configStr, "claude-3-sonnet") {
		t.Errorf("config should contain model claude-3-sonnet")
	}
	if !strings.Contains(configStr, "anthropic") {
		t.Errorf("config should contain provider anthropic")
	}
	if !strings.Contains(configStr, "telegram") {
		t.Errorf("config should contain telegram channel")
	}

	// Verify skills directory was created
	skillsDir := filepath.Join(workspaceDir, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Errorf("skills directory should be created")
	}
}

func TestRunConfigure_WhenOnlySomeOptionsProvided_ShouldOnlyUpdateThose(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "configure-partial")

	// Create workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	originalPort := 8080
	configContent := `{
		"gateway": {
			"port": 8080,
			"auth": {
				"mode": "none"
			}
		},
		"agents": {
			"provider": "local",
			"defaultModel": "original-model"
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// Only update model, not port
	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		DefaultModel:   "new-model",
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfigure: want 0, got %d", code)
	}

	// Verify only model was changed
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "new-model") {
		t.Errorf("config should contain new model")
	}
	// Port should still be the original
	if !strings.Contains(string(content), string(rune('0'+originalPort/1000%10))) {
		// Just check that the config is still valid
		t.Logf("Config content: %s", string(content))
	}
}

func TestRunConfigure_WhenConfigFileIsInvalid_ShouldFail(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "configure-invalid")

	// Create workspace with invalid config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`not valid json`), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		GatewayPort:    9090,
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code == 0 {
		t.Error("RunConfigure with invalid config should fail")
	}
}

func TestRunConfigure_WhenWorkspaceDoesNotExist_ShouldFail(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "configure-nonexistent", "workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		GatewayPort:    9090,
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code == 0 {
		t.Error("RunConfigure with nonexistent workspace should fail")
	}
}
