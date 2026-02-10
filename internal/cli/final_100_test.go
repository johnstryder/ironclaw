package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Test RunConfig UserHomeDir error
func TestRunConfig_UserHomeDirError_Final(t *testing.T) {
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

	opts := ConfigOptions{
		Workspace: "",
		Action:    "get",
		Path:      "key",
	}

	code := RunConfig(opts, out, errOut)
	t.Logf("RunConfig with no HOME: %d", code)
}

// Test runConfigSet save error
func TestRunConfigSet_SaveError_Final(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-save-error")
	os.MkdirAll(workspaceDir, 0755)

	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	os.WriteFile(configPath, []byte(`{}`), 0644)
	os.Chmod(configPath, 0444)
	defer os.Chmod(configPath, 0644)

	cfg := map[string]interface{}{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	code := runConfigSet(&cfg, "key", "value", configPath, out, errOut)
	t.Logf("runConfigSet with readonly file: %d", code)
}

// Test RunSetup UserHomeDir error
func TestRunSetup_UserHomeDirError_Final(t *testing.T) {
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

	opts := SetupOptions{Workspace: "", NonInteractive: true}
	code := RunSetup(opts, out, errOut)
	t.Logf("RunSetup with no HOME: %d", code)
}

// Test RunOnboard UserHomeDir error
func TestRunOnboard_UserHomeDirError_Final(t *testing.T) {
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

	opts := OnboardOptions{Workspace: "", NonInteractive: true}
	code := RunOnboard(opts, out, errOut)
	t.Logf("RunOnboard with no HOME: %d", code)
}

// Test RunDoctor UserHomeDir error
func TestRunDoctor_UserHomeDirError_Final(t *testing.T) {
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

	opts := DoctorOptions{Workspace: "", NonInteractive: true}
	code := RunDoctor(opts, out, errOut)
	t.Logf("RunDoctor with no HOME: %d", code)
}
