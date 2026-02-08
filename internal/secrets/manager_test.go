package secrets

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultManager_WhenPassphraseSet_ShouldReturnWorkingManager(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "test-pass")
	m, err := DefaultManager()
	if err != nil {
		t.Fatalf("DefaultManager: %v", err)
	}
	if m == nil {
		t.Fatal("DefaultManager: expected non-nil manager")
	}
	err = m.Set("testkey", "testvalue")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := m.Get("testkey")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "testvalue" {
		t.Errorf("Get: want testvalue, got %q", got)
	}
}

func TestNewFileManager_WhenPassphraseSet_ShouldReturnWorkingManager(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "test-pass")
	m, err := NewFileManager(path)
	if err != nil {
		t.Fatalf("NewFileManager: %v", err)
	}
	if m == nil {
		t.Fatal("NewFileManager: expected non-nil manager")
	}
	err = m.Set("openai", "sk-xyz")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := m.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "sk-xyz" {
		t.Errorf("Get: want sk-xyz, got %q", got)
	}
}

func TestDefaultManager_WhenSecretsPathFails_ShouldReturnError(t *testing.T) {
	// DefaultManager calls DefaultSecretsPath() then NewFileManager. If DefaultSecretsPath fails, we get error.
	dir := t.TempDir()
	fileAsConfig := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsConfig, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", fileAsConfig)
	_, err := DefaultManager()
	if err == nil {
		t.Fatal("DefaultManager when DefaultSecretsPath fails: expected error")
	}
}

func TestNewFileManager_WhenKeySourceFails_ShouldReturnError(t *testing.T) {
	os.Unsetenv("IRONCLAW_SECRETS_PASSPHRASE")
	defer t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "")
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/etc/machine-id"); err == nil {
			t.Skip("machine-id present; cannot test KeySource failure")
		}
	}
	_, err := NewFileManager(filepath.Join(t.TempDir(), ".secrets"))
	if err == nil {
		t.Fatal("NewFileManager when DefaultKeySource fails: expected error")
	}
}
