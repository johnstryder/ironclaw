package secrets

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultKeySource_WhenPassphraseSet_ShouldReturn32Bytes(t *testing.T) {
	t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "test-secret-pass")
	key, err := DefaultKeySource()
	if err != nil {
		t.Fatalf("DefaultKeySource: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length: want 32, got %d", len(key))
	}
	// Same passphrase should yield same key
	key2, _ := DefaultKeySource()
	if string(key) != string(key2) {
		t.Error("same passphrase should yield same key")
	}
}

func TestSecretsDir_ShouldReturnDirUnderUserConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got, err := SecretsDir()
	if err != nil {
		t.Fatalf("SecretsDir: %v", err)
	}
	wantDir := filepath.Join(dir, "ironclaw")
	if got != wantDir {
		t.Errorf("SecretsDir: want %q, got %q", wantDir, got)
	}
}

func TestDefaultSecretsPath_ShouldReturnPathToSecretsFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path, err := DefaultSecretsPath()
	if err != nil {
		t.Fatalf("DefaultSecretsPath: %v", err)
	}
	if filepath.Base(path) != ".secrets" {
		t.Errorf("DefaultSecretsPath base: want .secrets, got %q", filepath.Base(path))
	}
	if filepath.Dir(path) != filepath.Join(dir, "ironclaw") {
		t.Errorf("DefaultSecretsPath dir: unexpected %q", filepath.Dir(path))
	}
}

func TestDefaultKeySource_WhenMachineIDExists_ShouldReturn32Bytes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("machine-id only used on Linux")
	}
	if _, err := os.Stat("/etc/machine-id"); err != nil {
		t.Skip("/etc/machine-id not present")
	}
	os.Unsetenv("IRONCLAW_SECRETS_PASSPHRASE")
	defer t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "") // restore for other tests
	key, err := DefaultKeySource()
	if err != nil {
		t.Fatalf("DefaultKeySource (machine-id): %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length: want 32, got %d", len(key))
	}
}

func TestDefaultKeySource_WhenNoPassphraseAndNoMachineID_ShouldReturnError(t *testing.T) {
	os.Unsetenv("IRONCLAW_SECRETS_PASSPHRASE")
	defer t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "")
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/etc/machine-id"); err == nil {
			t.Skip("machine-id present on Linux; cannot test error path")
		}
	}
	_, err := DefaultKeySource()
	if err == nil {
		t.Fatal("DefaultKeySource with no passphrase and no machine-id: expected error")
	}
}

func TestSecretsDir_WhenBaseIsFile_MkdirAllFailsAndReturnsError(t *testing.T) {
	dir := t.TempDir()
	fileAsConfig := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsConfig, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", fileAsConfig)
	_, err := SecretsDir()
	if err == nil {
		t.Fatal("SecretsDir when base is file: expected error")
	}
}

func TestDefaultSecretsPath_WhenSecretsDirFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	fileAsConfig := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsConfig, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", fileAsConfig)
	_, err := DefaultSecretsPath()
	if err == nil {
		t.Fatal("DefaultSecretsPath when SecretsDir fails: expected error")
	}
}

func TestDefaultKeySource_WhenReadFileFails_ShouldReturnError(t *testing.T) {
	os.Unsetenv("IRONCLAW_SECRETS_PASSPHRASE")
	defer os.Unsetenv("IRONCLAW_SECRETS_PASSPHRASE")
	prev := keySourceReadFile
	defer func() { keySourceReadFile = prev }()
	keySourceReadFile = func(string) ([]byte, error) {
		return nil, fmt.Errorf("injected read error")
	}
	_, err := DefaultKeySource()
	if err == nil {
		t.Fatal("DefaultKeySource when read fails: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("machine-id")) {
		t.Errorf("error should mention machine-id: %v", err)
	}
}

func TestDefaultKeySource_WhenMachineIDEmpty_ShouldReturnError(t *testing.T) {
	os.Unsetenv("IRONCLAW_SECRETS_PASSPHRASE")
	defer os.Unsetenv("IRONCLAW_SECRETS_PASSPHRASE")
	prev := keySourceReadFile
	defer func() { keySourceReadFile = prev }()
	keySourceReadFile = func(string) ([]byte, error) {
		return []byte(""), nil
	}
	_, err := DefaultKeySource()
	if err == nil {
		t.Fatal("DefaultKeySource when machine-id empty: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("empty")) {
		t.Errorf("error should mention empty: %v", err)
	}
}

func TestSecretsDir_WhenUserConfigDirFails_ShouldReturnError(t *testing.T) {
	prev := keySourceUserConfigDir
	defer func() { keySourceUserConfigDir = prev }()
	keySourceUserConfigDir = func() (string, error) {
		return "", fmt.Errorf("injected UserConfigDir error")
	}
	_, err := SecretsDir()
	if err == nil {
		t.Fatal("SecretsDir when UserConfigDir fails: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("secrets dir")) {
		t.Errorf("error should mention secrets dir: %v", err)
	}
}
