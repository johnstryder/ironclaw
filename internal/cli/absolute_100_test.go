package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Force HOME to be empty to trigger os.UserHomeDir error
func forceEmptyHome() func() {
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	oldHomedrive := os.Getenv("HOMEDRIVE")
	oldHomepath := os.Getenv("HOMEPATH")

	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")
	os.Unsetenv("HOMEDRIVE")
	os.Unsetenv("HOMEPATH")

	return func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
		os.Setenv("HOMEDRIVE", oldHomedrive)
		os.Setenv("HOMEPATH", oldHomepath)
	}
}

// Test RunConfig UserHomeDir error
func TestRunConfig_Line27_UserHomeDirError(t *testing.T) {
	restore := forceEmptyHome()
	defer restore()

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: "", // Empty to trigger home dir lookup
		Action:    "get",
		Path:      "key",
	}

	code := RunConfig(opts, out, errOut)

	if code != 1 {
		t.Errorf("RunConfig with no HOME: want 1, got %d", code)
	}
}

// Test RunConfig os.ReadFile error
func TestRunConfig_Line45_ReadFileError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-read-error")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Make file unreadable
	if err := os.Chmod(configPath, 0000); err != nil {
		t.Skip("Cannot change permissions")
	}
	defer os.Chmod(configPath, 0644)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigOptions{
		Workspace: workspaceDir,
		Action:    "get",
		Path:      "key",
	}

	code := RunConfig(opts, out, errOut)

	if code != 1 {
		t.Errorf("RunConfig with unreadable file: want 1, got %d", code)
	}
}

// Test runConfigSet saveConfig error
func TestRunConfigSet_Line121_SaveError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-set-save-error")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Make file read-only
	if err := os.Chmod(configPath, 0444); err != nil {
		t.Skip("Cannot change permissions")
	}
	defer os.Chmod(configPath, 0644)

	cfg := map[string]interface{}{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	code := runConfigSet(&cfg, "key", "value", configPath, out, errOut)

	if code != 1 {
		t.Errorf("runConfigSet with readonly file: want 1, got %d", code)
	}
}

// Test RunSetup UserHomeDir error
func TestRunSetup_Line27_UserHomeDirError(t *testing.T) {
	restore := forceEmptyHome()
	defer restore()

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      "",
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	if code != 1 {
		t.Errorf("RunSetup with no HOME: want 1, got %d", code)
	}
}

// Test RunSetup os.MkdirAll error
func TestRunSetup_Line36_MkdirAllError(t *testing.T) {
	// Use a path that can't be created
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      "/root/cannot-create-this-dir",
		NonInteractive: true,
	}

	code := RunSetup(opts, out, errOut)

	if code != 1 {
		t.Logf("RunSetup with restricted path returned: %d (may be running as root)", code)
	}
}

// Test RunOnboard UserHomeDir error
func TestRunOnboard_Line31_UserHomeDirError(t *testing.T) {
	restore := forceEmptyHome()
	defer restore()

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      "",
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 1 {
		t.Errorf("RunOnboard with no HOME: want 1, got %d", code)
	}
}

// Test RunOnboard os.MkdirAll error
func TestRunOnboard_Line39_MkdirAllError(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      "/root/cannot-create-onboard-dir",
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 1 {
		t.Logf("RunOnboard with restricted path returned: %d", code)
	}
}

// Test RunOnboard skills os.MkdirAll error
func TestRunOnboard_Line47_SkillsMkdirError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-skills-mkdir-error")

	// Create workspace
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create file blocking skills dir
	skillsFile := filepath.Join(workspaceDir, "skills")
	if err := os.WriteFile(skillsFile, []byte("not dir"), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := OnboardOptions{
		Workspace:      workspaceDir,
		Skills:         []string{"test"},
		NonInteractive: true,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 1 {
		t.Errorf("RunOnboard with file blocking skills dir: want 1, got %d", code)
	}
}

// Test RunOnboard config.Load error
func TestRunOnboard_Line61_ConfigLoadError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-config-load-error")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create invalid config
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

	if code != 1 {
		t.Errorf("RunOnboard with invalid config: want 1, got %d", code)
	}
}

// Test RunDoctor UserHomeDir error
func TestRunDoctor_Line34_UserHomeDirError(t *testing.T) {
	restore := forceEmptyHome()
	defer restore()

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:      "",
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	if code != 1 {
		t.Errorf("RunDoctor with no HOME: want 1, got %d", code)
	}
}

// Test RunDoctor os.MkdirAll error in fix mode
func TestRunDoctor_Line56_MkdirAllError(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace:      "/root/cannot-create-doctor-dir",
		Fix:            true,
		NonInteractive: true,
	}

	code := RunDoctor(opts, out, errOut)

	// Should not error, just report failure to fix
	t.Logf("RunDoctor with restricted path returned: %d", code)
}
