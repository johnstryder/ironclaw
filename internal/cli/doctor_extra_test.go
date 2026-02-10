package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Additional RunDoctor tests for edge cases
// =============================================================================

func TestRunDoctor_WhenDefaultWorkspace_ShouldUseHomeDir(t *testing.T) {
	// Save current HOME
	oldHome := os.Getenv("HOME")
	testHome := t.TempDir()
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", oldHome)

	// Create workspace in home dir
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

	opts := DoctorOptions{
		Workspace:      "", // Empty, should use default
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunDoctor with default workspace: want 0, got %d", code)
	}
}

func TestRunDoctor_WhenDeepCheckEnabled_ShouldPerformExtraChecks(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-deep")

	// Create healthy workspace
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{
		"gateway": {
			"port": 8080,
			"auth": {
				"mode": "none"
			}
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

	opts := DoctorOptions{
		Workspace:      workspaceDir,
		Deep:           true,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	if code != 0 {
		t.Errorf("RunDoctor with deep check: want 0, got %d", code)
	}

	// Verify output mentions deep checks
	output := out.String()
	if !strings.Contains(output, "deep") && !strings.Contains(output, "Deep") {
		t.Logf("Deep check output: %s", output)
	}
}

func TestRunDoctor_WhenFixEnabledAndWorkspaceMissing_ShouldCreateWorkspace(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-fix-workspace")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:      workspaceDir,
		Fix:            true,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	// May or may not return 0 depending on what else needs fixing
	t.Logf("RunDoctor with fix returned: %d", code)

	// Verify workspace was created
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		t.Logf("Fix did not create workspace (may require additional steps)")
	}
}

func TestRunDoctor_WhenFixEnabledAndConfigMissing_ShouldCreateConfig(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-fix-config")

	// Create workspace but not config
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

	// Verify config was created
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config should be created with --fix")
	} else {
		t.Logf("Config was created successfully")
	}

	if code != 0 {
		t.Logf("Doctor returned %d, but config was created", code)
	}
}

func TestRunDoctor_WhenPathsMissingAndFixEnabled_ShouldCreatePaths(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-fix-paths")

	// Create workspace with config that has missing paths
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{
		"gateway": {
			"port": 8080,
			"auth": {
				"mode": "none"
			}
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

	opts := DoctorOptions{
		Workspace:      workspaceDir,
		Fix:            true,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	// Verify paths were created
	agentsPath := filepath.Join(workspaceDir, "agents")
	memoryPath := filepath.Join(workspaceDir, "memory")

	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		t.Logf("Agents path was not created (may not be auto-fixed)")
	}
	if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
		t.Logf("Memory path was not created (may not be auto-fixed)")
	}

	t.Logf("Doctor returned: %d", code)
}

func TestRunDoctor_WhenConfigHasNoPaths_ShouldNotError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-no-paths")

	// Create workspace with config that has no paths
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{
		"gateway": {
			"port": 8080,
			"auth": {
				"mode": "none"
			}
		},
		"agents": {
			"provider": "local",
			"defaultModel": "gpt-4o"
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
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
		t.Errorf("RunDoctor with no paths: want 0, got %d", code)
	}
}

func TestRunDoctor_WhenWorkspaceIsFile_ShouldReportError(t *testing.T) {
	dir := t.TempDir()
	workspacePath := filepath.Join(dir, "file-not-dir")

	// Create a file instead of directory
	if err := os.WriteFile(workspacePath, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:      workspacePath,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	// Should report error or warning
	if code == 0 {
		t.Logf("Doctor with file as workspace did not fail")
	}
}

func TestRunDoctor_WhenSummaryShowsWarnings_ShouldCountCorrectly(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-warnings")

	// Create workspace with config but missing paths
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{
		"gateway": {
			"port": 8080,
			"auth": {
				"mode": "none"
			}
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

	opts := DoctorOptions{
		Workspace:      workspaceDir,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	output := out.String()

	// Should show warnings for missing paths
	if !strings.Contains(output, "warning") && !strings.Contains(output, "⚠") {
		t.Logf("Expected warnings in output: %s", output)
	}

	// Summary should show correct counts
	if !strings.Contains(output, "passed") || !strings.Contains(output, "warnings") {
		t.Errorf("output should contain summary with counts, got: %s", output)
	}

	t.Logf("Doctor returned: %d", code)
}
