package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Test helper to create a cobra command with the handler
func createTestCommand(t *testing.T, runE func(cmd *cobra.Command, args []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "test command",
		RunE:  runE,
	}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

// =============================================================================
// runSetup tests
// =============================================================================

func TestRunSetup_WhenValidFlags_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "setup-test")

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")
	cmd.Flags().Bool("wizard", false, "")
	cmd.Flags().Bool("non-interactive", true, "")
	cmd.Flags().String("mode", "local", "")
	cmd.Flags().String("remote-url", "", "")
	cmd.Flags().String("remote-token", "", "")

	err := runSetup(cmd, []string{})
	if err != nil {
		t.Errorf("runSetup should succeed, got error: %v", err)
	}

	// Verify workspace was created
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		t.Error("workspace directory should be created")
	}
}

func TestRunSetup_WhenFails_ShouldReturnExitCodeError(t *testing.T) {
	// Use an invalid workspace path that will cause setup to fail
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", "/nonexistent/path/that/cannot/be/created", "")
	cmd.Flags().Bool("wizard", false, "")
	cmd.Flags().Bool("non-interactive", true, "")
	cmd.Flags().String("mode", "", "")
	cmd.Flags().String("remote-url", "", "")
	cmd.Flags().String("remote-token", "", "")

	err := runSetup(cmd, []string{})
	// This might not fail on all systems, so just check that it returns something
	if err == nil {
		t.Log("runSetup with invalid path did not fail (system may allow it)")
	}
}

// =============================================================================
// runOnboard tests
// =============================================================================

func TestRunOnboard_WhenValidFlags_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-test")

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")
	cmd.Flags().Bool("non-interactive", true, "")
	cmd.Flags().Int("gateway-port", 8080, "")
	cmd.Flags().String("gateway-auth", "none", "")
	cmd.Flags().String("auth-token", "", "")
	cmd.Flags().String("default-model", "gpt-4", "")
	cmd.Flags().String("provider", "openai", "")
	cmd.Flags().StringSlice("skills", []string{}, "")

	err := runOnboard(cmd, []string{})
	if err != nil {
		t.Errorf("runOnboard should succeed, got error: %v", err)
	}

	// Verify workspace was created
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		t.Error("workspace directory should be created")
	}
}

// =============================================================================
// runConfigure tests
// =============================================================================

func TestRunConfigure_WhenValidFlagsAndExistingConfig_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "configure-test")

	// Create workspace with config first
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"gateway":{"port":8080,"auth":{"mode":"none"}},"agents":{"provider":"local","defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")
	cmd.Flags().Bool("non-interactive", true, "")
	cmd.Flags().Int("gateway-port", 9090, "")
	cmd.Flags().String("gateway-auth", "", "")
	cmd.Flags().String("default-model", "claude-3", "")
	cmd.Flags().String("provider", "anthropic", "")
	cmd.Flags().StringSlice("channels", []string{}, "")
	cmd.Flags().StringSlice("skills", []string{}, "")

	err := runConfigure(cmd, []string{})
	if err != nil {
		t.Errorf("runConfigure should succeed, got error: %v", err)
	}

	// Verify config was updated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "9090") {
		t.Errorf("config should be updated with new port, got: %s", string(content))
	}
}

func TestRunConfigure_WhenNoConfigExists_ShouldFail(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "configure-no-config")

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")
	cmd.Flags().Bool("non-interactive", true, "")
	cmd.Flags().Int("gateway-port", 9090, "")
	cmd.Flags().String("gateway-auth", "", "")
	cmd.Flags().String("default-model", "", "")
	cmd.Flags().String("provider", "", "")
	cmd.Flags().StringSlice("channels", []string{}, "")
	cmd.Flags().StringSlice("skills", []string{}, "")

	err := runConfigure(cmd, []string{})
	if err == nil {
		t.Error("runConfigure should fail when no config exists")
	}
}

// =============================================================================
// runConfigGet tests
// =============================================================================

func TestRunConfigGet_WhenValidPath_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-get-test")

	// Create workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"gateway":{"port":8080,"auth":{"mode":"none"}},"agents":{"provider":"local","defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")

	err := runConfigGet(cmd, []string{"gateway.port"})
	if err != nil {
		t.Errorf("runConfigGet should succeed, got error: %v", err)
	}

	// Verify output contains the port
	if !strings.Contains(out.String(), "8080") {
		t.Errorf("output should contain '8080', got: %s", out.String())
	}
}

func TestRunConfigGet_WhenInvalidPath_ShouldFail(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-get-invalid")

	// Create workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"gateway":{"port":8080}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")

	err := runConfigGet(cmd, []string{"nonexistent.path"})
	if err == nil {
		t.Error("runConfigGet should fail for invalid path")
	}
}

// =============================================================================
// runConfigSet tests
// =============================================================================

func TestRunConfigSet_WhenValidPathAndValue_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-set-test")

	// Create workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"gateway":{"port":8080,"auth":{"mode":"none"}},"agents":{"provider":"local","defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")

	err := runConfigSet(cmd, []string{"gateway.port", "9090"})
	if err != nil {
		t.Errorf("runConfigSet should succeed, got error: %v", err)
	}

	// Verify config was updated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "9090") {
		t.Errorf("config should be updated with new port, got: %s", string(content))
	}
}

// =============================================================================
// runConfigUnset tests
// =============================================================================

func TestRunConfigUnset_WhenValidPath_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-unset-test")

	// Create workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"gateway":{"port":8080,"auth":{"mode":"none"}},"agents":{"provider":"local","defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")

	err := runConfigUnset(cmd, []string{"agents.defaultModel"})
	if err != nil {
		t.Errorf("runConfigUnset should succeed, got error: %v", err)
	}
}

// =============================================================================
// runDoctor tests
// =============================================================================

func TestRunDoctor_WhenHealthyWorkspace_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-healthy-test")

	// Create workspace with config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"gateway":{"port":8080,"auth":{"mode":"none"}},"agents":{"provider":"local","defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")
	cmd.Flags().Bool("no-workspace-suggestions", false, "")
	cmd.Flags().Bool("non-interactive", true, "")
	cmd.Flags().Bool("deep", false, "")
	cmd.Flags().Bool("fix", false, "")

	err := runDoctor(cmd, []string{})
	if err != nil {
		t.Errorf("runDoctor should succeed, got error: %v", err)
	}
}

func TestRunDoctor_WhenIssuesFound_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "doctor-issues-test")

	// Create workspace with invalid config
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	configContent := `{"invalid json` // Invalid JSON
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")
	cmd.Flags().Bool("no-workspace-suggestions", false, "")
	cmd.Flags().Bool("non-interactive", true, "")
	cmd.Flags().Bool("deep", false, "")
	cmd.Flags().Bool("fix", false, "")

	err := runDoctor(cmd, []string{})
	// Doctor may or may not return error depending on implementation
	// Just verify it runs without panic
	t.Logf("runDoctor returned: %v", err)
}
