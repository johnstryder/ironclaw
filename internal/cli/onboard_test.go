package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnboardCommand_WhenNonInteractive_ShouldSetupBasicWorkspace(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard: want exit code 0, got %d. stderr: %s", code, errOut.String())
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
	if !strings.Contains(output, "onboard") && !strings.Contains(output, "complete") {
		t.Errorf("output should indicate successful onboarding, got: %s", output)
	}
}

func TestOnboardCommand_WhenGatewayOptionsProvided_ShouldConfigureGateway(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "gateway-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		GatewayPort:    9090,
		GatewayAuth:    "token",
		AuthToken:      "test-token-123",
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard: want exit code 0, got %d", code)
	}

	// Verify config contains gateway settings
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configStr := string(content)

	if !strings.Contains(configStr, "9090") {
		t.Errorf("config should contain gateway port 9090, got: %s", configStr)
	}
	if !strings.Contains(configStr, "token") {
		t.Errorf("config should contain auth mode 'token', got: %s", configStr)
	}
}

func TestOnboardCommand_WhenModelOptionsProvided_ShouldConfigureModels(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "model-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		DefaultModel:   "gpt-4",
		Provider:       "openai",
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard: want exit code 0, got %d", code)
	}

	// Verify config contains model settings
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configStr := string(content)

	if !strings.Contains(configStr, "gpt-4") {
		t.Errorf("config should contain default model 'gpt-4', got: %s", configStr)
	}
	if !strings.Contains(configStr, "openai") {
		t.Errorf("config should contain provider 'openai', got: %s", configStr)
	}
}

func TestOnboardCommand_WhenSkillsOptionProvided_ShouldSetupSkills(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "skills-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		Skills:         []string{"filesystem", "shell", "web"},
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard: want exit code 0, got %d", code)
	}

	// Check that skills directory was created
	skillsDir := filepath.Join(workspaceDir, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Errorf("skills directory should be created at %s", skillsDir)
	}

	// Verify output mentions skills
	output := out.String()
	if !strings.Contains(output, "skill") {
		t.Errorf("output should mention skills setup, got: %s", output)
	}
}

func TestOnboardCommand_WhenExistingWorkspace_ShouldNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "existing-workspace")

	// Pre-create workspace with custom config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	customContent := `{"agents":{"defaultModel":"custom-model"}}`
	if err := os.WriteFile(configPath, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard: want exit code 0, got %d", code)
	}

	// Verify custom config was preserved
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "custom-model") {
		t.Errorf("existing config should be preserved, got: %s", string(content))
	}
}

func TestOnboardCommand_WhenOutputShouldShowSummary(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "summary-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		DefaultModel:   "claude-3",
		Provider:       "anthropic",
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunOnboard: want exit code 0, got %d", code)
	}

	// Verify output contains a summary of configured options
	output := out.String()
	if !strings.Contains(output, "claude-3") {
		t.Errorf("output should mention the configured model, got: %s", output)
	}
	if !strings.Contains(output, "anthropic") {
		t.Errorf("output should mention the configured provider, got: %s", output)
	}
}
