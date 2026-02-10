package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// Test helpers: save/restore function variables
// =============================================================================

func withSetValueAtPath(fn func(map[string]interface{}, []string, interface{}) error) func() {
	orig := setValueAtPathFn
	setValueAtPathFn = fn
	return func() { setValueAtPathFn = orig }
}

func withUserHomeDir(fn func() (string, error)) func() {
	orig := osUserHomeDir
	osUserHomeDir = fn
	return func() { osUserHomeDir = orig }
}

func withConfigLoad(fn func(string) (*domain.Config, error)) func() {
	orig := configLoad
	configLoad = fn
	return func() { configLoad = orig }
}

func withConfigSave(fn func(string, *domain.Config) error) func() {
	orig := configSave
	configSave = fn
	return func() { configSave = orig }
}

func withConfigWriteDefault(fn func(string) error) func() {
	orig := configWriteDefault
	configWriteDefault = fn
	return func() { configWriteDefault = orig }
}

func withMkdirAll(fn func(string, os.FileMode) error) func() {
	orig := osMkdirAll
	osMkdirAll = fn
	return func() { osMkdirAll = orig }
}

// =============================================================================
// config_cmd.go — runConfigSet setValueAtPath error (lines 115-118)
// =============================================================================

func TestRunConfigSet_WhenSetValueAtPathFails_ShouldReturnError(t *testing.T) {
	restore := withSetValueAtPath(func(_ map[string]interface{}, _ []string, _ interface{}) error {
		return fmt.Errorf("injected setValueAtPath error")
	})
	defer restore()

	cfg := map[string]interface{}{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	configPath := filepath.Join(t.TempDir(), "test.json")

	code := runConfigSet(&cfg, "key", "value", configPath, out, errOut)

	if code != 1 {
		t.Errorf("should return exit code 1 when setValueAtPath fails, got %d", code)
	}
	if !strings.Contains(errOut.String(), "injected setValueAtPath error") {
		t.Errorf("stderr should contain error message, got: %s", errOut.String())
	}
}

// =============================================================================
// configure.go — RunConfigure UserHomeDir error (lines 30-33)
// =============================================================================

func TestRunConfigure_WhenUserHomeDirFails_ShouldReturnError(t *testing.T) {
	restore := withUserHomeDir(func() (string, error) {
		return "", fmt.Errorf("injected home dir error")
	})
	defer restore()

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := ConfigureOptions{
		Workspace: "", // Empty triggers osUserHomeDir call
	}

	code := RunConfigure(opts, out, errOut)

	if code != 1 {
		t.Errorf("should return exit code 1 when UserHomeDir fails, got %d", code)
	}
	if !strings.Contains(errOut.String(), "could not determine home directory") {
		t.Errorf("stderr should contain home directory error, got: %s", errOut.String())
	}
}

// =============================================================================
// doctor.go — RunDoctor MkdirAll failures during fix (lines 126-128, 157-159)
// =============================================================================

func TestRunDoctor_WhenFixMkdirAllFails_ShouldReportErrors(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-fix-perm")

	// Create workspace and write config with agents/memory paths defined
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{
		"gateway":{"port":8080,"auth":{"mode":"none","externalChannels":[],"rateLimitMaxAttempts":5},"allowedHosts":[]},
		"agents":{"provider":"local","defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},
		"infra":{"logFormat":"text","logLevel":"info"},
		"retry":{"maxRetries":3,"initialBackoff":500,"maxBackoff":30000,"multiplier":2},
		"allowedCommands":[]
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Do NOT create agents/ or memory/ directories

	// Mock osMkdirAll to fail for agents and memory paths
	agentsPath := filepath.Join(workspaceDir, "agents")
	memoryPath := filepath.Join(workspaceDir, "memory")
	restore := withMkdirAll(func(path string, perm os.FileMode) error {
		if path == agentsPath || path == memoryPath {
			return fmt.Errorf("injected mkdir error for %s", path)
		}
		return os.MkdirAll(path, perm)
	})
	defer restore()

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace: workspaceDir,
		Fix:       true,
	}

	RunDoctor(opts, out, errOut)

	stderr := errOut.String()
	if !strings.Contains(stderr, "Failed to create agents directory") {
		t.Errorf("stderr should report failed agents dir creation, got: %s", stderr)
	}
	if !strings.Contains(stderr, "Failed to create memory directory") {
		t.Errorf("stderr should report failed memory dir creation, got: %s", stderr)
	}
}

// =============================================================================
// doctor.go — RunDoctor WriteDefault error during config fix (lines 88-90)
// =============================================================================

func TestRunDoctor_WhenFixWriteDefaultFails_ShouldReportError(t *testing.T) {
	restore := withConfigWriteDefault(func(_ string) error {
		return fmt.Errorf("injected write default error")
	})
	defer restore()

	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-wd-fail")

	// Create workspace but do NOT create config file
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := DoctorOptions{
		Workspace: workspaceDir,
		Fix:       true,
	}

	RunDoctor(opts, out, errOut)

	stderr := errOut.String()
	if !strings.Contains(stderr, "Failed to write default config") {
		t.Errorf("stderr should report failed write default, got: %s", stderr)
	}
}

// =============================================================================
// onboard.go — RunOnboard WriteDefault error for new config (lines 69-72)
// =============================================================================

func TestRunOnboard_WhenWriteDefaultFailsForNewConfig_ShouldReturnError(t *testing.T) {
	restore := withConfigWriteDefault(func(_ string) error {
		return fmt.Errorf("injected write default error")
	})
	defer restore()

	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-wd-fail")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// No existing config → enters else branch → calls configWriteDefault → fails
	opts := OnboardOptions{
		Workspace: workspaceDir,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 1 {
		t.Errorf("should return exit code 1 when WriteDefault fails, got %d", code)
	}
	if !strings.Contains(errOut.String(), "failed to write default config") {
		t.Errorf("stderr should contain write error, got: %s", errOut.String())
	}
}

// =============================================================================
// onboard.go — RunOnboard Load error after WriteDefault (lines 74-77)
// =============================================================================

func TestRunOnboard_WhenLoadFailsAfterWriteDefault_ShouldReturnError(t *testing.T) {
	restore := withConfigLoad(func(_ string) (*domain.Config, error) {
		return nil, fmt.Errorf("injected config load error")
	})
	defer restore()

	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-load-fail")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// No existing config → else branch → WriteDefault succeeds (real) → configLoad fails (mocked)
	opts := OnboardOptions{
		Workspace: workspaceDir,
	}

	code := RunOnboard(opts, out, errOut)

	if code != 1 {
		t.Errorf("should return exit code 1 when Load fails after WriteDefault, got %d", code)
	}
	if !strings.Contains(errOut.String(), "failed to load new config") {
		t.Errorf("stderr should contain load error, got: %s", errOut.String())
	}
}

// =============================================================================
// setup.go — RunSetup Load error after WriteDefault (lines 58-61)
// =============================================================================

func TestRunSetup_WhenConfigLoadFails_ShouldReturnError(t *testing.T) {
	restore := withConfigLoad(func(_ string) (*domain.Config, error) {
		return nil, fmt.Errorf("injected config load error")
	})
	defer restore()

	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "setup-load-fail")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// No existing config → WriteDefault succeeds (real) → configLoad fails (mocked)
	opts := SetupOptions{
		Workspace: workspaceDir,
	}

	code := RunSetup(opts, out, errOut)

	if code != 1 {
		t.Errorf("should return exit code 1 when Load fails, got %d", code)
	}
	if !strings.Contains(errOut.String(), "failed to load config for updates") {
		t.Errorf("stderr should contain load error, got: %s", errOut.String())
	}
}

// =============================================================================
// setup.go — RunSetup Save error (lines 79-82)
// =============================================================================

func TestRunSetup_WhenConfigSaveFails_ShouldReturnError(t *testing.T) {
	restore := withConfigSave(func(_ string, _ *domain.Config) error {
		return fmt.Errorf("injected config save error")
	})
	defer restore()

	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "setup-save-fail")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	// No existing config → WriteDefault+Load succeed (real) → configSave fails (mocked)
	opts := SetupOptions{
		Workspace: workspaceDir,
	}

	code := RunSetup(opts, out, errOut)

	if code != 1 {
		t.Errorf("should return exit code 1 when Save fails, got %d", code)
	}
	if !strings.Contains(errOut.String(), "failed to save config") {
		t.Errorf("stderr should contain save error, got: %s", errOut.String())
	}
}

// =============================================================================
// setup.go — RunSetup wizard branch (lines 84-87)
// =============================================================================

func TestRunSetup_WhenWizardMode_ShouldPrintWizardMessage(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "setup-wizard")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := SetupOptions{
		Workspace:      workspaceDir,
		Wizard:         true,
		NonInteractive: false, // Both conditions needed for the wizard branch
	}

	code := RunSetup(opts, out, errOut)

	if code != 0 {
		t.Errorf("should return exit code 0 for wizard mode, got %d; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Running interactive setup wizard") {
		t.Errorf("stdout should contain wizard message, got: %s", out.String())
	}
}
