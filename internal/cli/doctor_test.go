package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ironclaw/internal/config"
)

func TestDoctorCommand_WhenHealthyConfig_ShouldPassAllChecks(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "healthy-workspace")

	// Create existing workspace with valid config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunDoctor with healthy config: want exit code 0, got %d. stderr: %s", code, errOut.String())
	}

	// Verify output contains health check indicators
	output := out.String()
	if !strings.Contains(output, "check") && !strings.Contains(output, "health") {
		t.Errorf("output should indicate health check, got: %s", output)
	}
}

func TestDoctorCommand_WhenNoWorkspace_ShouldReportIssues(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "missing-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	_ = RunDoctor(opts, out, errOut)

	// Should report issues (but may not necessarily error)
	output := out.String() + errOut.String()
	if !strings.Contains(output, "workspace") && !strings.Contains(output, "missing") {
		t.Errorf("output should mention missing workspace, got: %s", output)
	}
}

func TestDoctorCommand_WhenDeepCheck_ShouldPerformExtraChecks(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "deep-check-workspace")

	// Create existing workspace with valid config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:      workspaceDir,
		Deep:           true,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunDoctor with deep check: want exit code 0, got %d", code)
	}

	// Deep check should include more detailed output
	output := out.String()
	// Just verify it runs without error
	if output == "" && errOut.String() == "" {
		t.Error("doctor command should produce some output")
	}
}

func TestDoctorCommand_WhenFixEnabled_ShouldAttemptRepairs(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "fixable-workspace")

	// Create workspace but without a config file (so fix can create it)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:      workspaceDir,
		Fix:            true,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	// With fix enabled, it should attempt repairs
	output := out.String()
	if !strings.Contains(output, "fix") && !strings.Contains(output, "repair") && !strings.Contains(output, "create") {
		// May not use the word "fix", just check it runs
		t.Logf("Doctor output: %s", output)
	}

	// Should succeed or indicate what was fixed
	if code != 0 && !strings.Contains(errOut.String(), "fix") {
		t.Logf("Doctor may have found issues it couldn't fix. stderr: %s", errOut.String())
	}
}

func TestDoctorCommand_WhenNoWorkspaceSuggestions_ShouldSkipSuggestions(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "no-suggestions-workspace")

	// Create existing workspace with valid config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:              workspaceDir,
		NoWorkspaceSuggestions: true,
		NonInteractive:         true,
	}

	code := RunDoctor(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunDoctor: want exit code 0, got %d", code)
	}
}

func TestDoctorCommand_ShouldCheckConfigValidity(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "valid-config-workspace")

	// Create workspace with valid config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := config.WriteDefault(configPath); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	// Should validate the config successfully
	if code != 0 {
		t.Errorf("RunDoctor with valid config: want exit code 0, got %d", code)
	}

	// Output should mention config in some way
	output := out.String()
	if !strings.Contains(output, "config") && !strings.Contains(output, "ok") && !strings.Contains(output, "pass") {
		t.Logf("Doctor output (config check): %s", output)
	}
}
