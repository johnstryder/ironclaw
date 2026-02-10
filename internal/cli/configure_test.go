package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ironclaw/internal/config"
)

func TestConfigureCommand_WhenNonInteractiveAndNoExistingConfig_ShouldError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "nonexistent-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	// Should fail because no existing config to configure
	if code == 0 {
		t.Error("RunConfigure without existing config: expected error")
	}
}

func TestConfigureCommand_WhenNonInteractiveWithExistingConfig_ShouldUpdateSettings(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "existing-workspace")

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

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		GatewayPort:    7070,
		GatewayAuth:    "password",
		DefaultModel:   "claude-3-opus",
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfigure: want exit code 0, got %d. stderr: %s", code, errOut.String())
	}

	// Verify config was updated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configStr := string(content)

	if !strings.Contains(configStr, "7070") {
		t.Errorf("config should contain updated port 7070, got: %s", configStr)
	}
	if !strings.Contains(configStr, "password") {
		t.Errorf("config should contain auth mode 'password', got: %s", configStr)
	}
	if !strings.Contains(configStr, "claude-3-opus") {
		t.Errorf("config should contain updated model 'claude-3-opus', got: %s", configStr)
	}
}

func TestConfigureCommand_WhenModelsOption_ShouldConfigureModels(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "models-workspace")

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

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		Provider:       "openai",
		DefaultModel:   "gpt-4-turbo",
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfigure: want exit code 0, got %d", code)
	}

	// Verify model settings were updated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "gpt-4-turbo") {
		t.Errorf("config should contain 'gpt-4-turbo', got: %s", string(content))
	}
}

func TestConfigureCommand_WhenChannelsOption_ShouldConfigureChannels(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "channels-workspace")

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

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		Channels:       []string{"telegram", "discord"},
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfigure: want exit code 0, got %d", code)
	}

	// Verify channels were configured
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configStr := string(content)
	if !strings.Contains(configStr, "telegram") {
		t.Errorf("config should contain 'telegram' in channels, got: %s", configStr)
	}
}

func TestConfigureCommand_WhenSkillsOption_ShouldConfigureSkills(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "skills-config-workspace")

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

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		Skills:         []string{"git", "docker"},
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfigure: want exit code 0, got %d", code)
	}

	// Verify skills directory was created/ensured
	skillsDir := filepath.Join(workspaceDir, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Errorf("skills directory should exist after configure")
	}
}

func TestConfigureCommand_ShouldOutputConfirmation(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "output-workspace")

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

	opts := ConfigureOptions{
		Workspace:      workspaceDir,
		GatewayPort:    8888,
		NonInteractive: true,
	}

	code := RunConfigure(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunConfigure: want exit code 0, got %d", code)
	}

	// Verify output confirms changes
	output := out.String()
	if !strings.Contains(output, "configured") && !strings.Contains(output, "updated") {
		t.Errorf("output should confirm configuration was updated, got: %s", output)
	}
}
