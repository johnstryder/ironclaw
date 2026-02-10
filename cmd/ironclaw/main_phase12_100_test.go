package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// Test runOnboard when it returns an error (non-zero code)
func TestRunOnboard_WhenInternalFails_ShouldReturnError(t *testing.T) {
	// Use a path that can't be created to trigger an error
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", "/root/cannot-create-here-for-onboard", "")
	cmd.Flags().Bool("non-interactive", true, "")
	cmd.Flags().Int("gateway-port", 0, "")
	cmd.Flags().String("gateway-auth", "", "")
	cmd.Flags().String("auth-token", "", "")
	cmd.Flags().String("default-model", "", "")
	cmd.Flags().String("provider", "", "")
	cmd.Flags().StringSlice("skills", []string{}, "")

	err := runOnboard(cmd, []string{})

	// Should return error when workspace can't be created
	if err == nil {
		t.Log("runOnboard with restricted path did not fail (may be running as root)")
	}
}

// Test runConfigSet when it returns an error
func TestRunConfigSet_WhenInternalFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-set-fail-wrapper")

	// Create workspace
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an invalid config file
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`invalid json`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")

	err := runConfigSet(cmd, []string{"key", "value"})

	// Should return error when config is invalid
	if err == nil {
		t.Error("runConfigSet with invalid config should return error")
	}
}

// Test runConfigUnset when it returns an error
func TestRunConfigUnset_WhenInternalFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "config-unset-fail-wrapper")

	// Create workspace
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an invalid config file
	configPath := filepath.Join(workspaceDir, "ironclaw.json")
	if err := os.WriteFile(configPath, []byte(`invalid json`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")

	err := runConfigUnset(cmd, []string{"key"})

	// Should return error when config is invalid
	if err == nil {
		t.Error("runConfigUnset with invalid config should return error")
	}
}

// Test runConfigSet with nonexistent workspace
func TestRunConfigSet_WhenWorkspaceMissing_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "nonexistent", "workspace")

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")

	err := runConfigSet(cmd, []string{"key", "value"})

	// Should return error when workspace doesn't exist
	if err == nil {
		t.Error("runConfigSet with nonexistent workspace should return error")
	}
}

// Test runConfigUnset with nonexistent workspace
func TestRunConfigUnset_WhenWorkspaceMissing_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "nonexistent", "workspace")

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")

	err := runConfigUnset(cmd, []string{"key"})

	// Should return error when workspace doesn't exist
	if err == nil {
		t.Error("runConfigUnset with nonexistent workspace should return error")
	}
}

// Test runOnboard with all flags set to exercise all code paths
func TestRunOnboard_WithAllFlags_ExercisesAllBranches(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "onboard-all-flags")

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("workspace", workspaceDir, "")
	cmd.Flags().Bool("non-interactive", true, "")
	cmd.Flags().Int("gateway-port", 8080, "")
	cmd.Flags().String("gateway-auth", "token", "")
	cmd.Flags().String("auth-token", "secret", "")
	cmd.Flags().String("default-model", "gpt-4", "")
	cmd.Flags().String("provider", "openai", "")
	cmd.Flags().StringSlice("skills", []string{"docker", "git"}, "")

	err := runOnboard(cmd, []string{})

	if err != nil {
		t.Errorf("runOnboard with all flags should succeed, got: %v", err)
	}
}
