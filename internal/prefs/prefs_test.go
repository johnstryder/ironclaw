package prefs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestManager_Load_WhenFileDoesNotExist_ShouldReturnDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	m := NewManager(path)

	err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	c := m.Config()
	if c == nil {
		t.Fatal("Config() should not be nil")
	}
	if c.Theme != "" {
		t.Errorf("default Theme should be empty, got %q", c.Theme)
	}
	if c.DefaultModel != "" {
		t.Errorf("default DefaultModel should be empty, got %q", c.DefaultModel)
	}
}

func TestManager_Load_WhenFileExistsWithValidJSON_ShouldReturnParsedConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{"theme":"dark","defaultModel":"gpt-4o"}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(path)
	err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	c := m.Config()
	if c.Theme != "dark" {
		t.Errorf("Theme: want dark, got %q", c.Theme)
	}
	if c.DefaultModel != "gpt-4o" {
		t.Errorf("DefaultModel: want gpt-4o, got %q", c.DefaultModel)
	}
}

func TestManager_Load_WhenFileIsInvalidJSON_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{invalid`), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(path)
	err := m.Load()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestManager_SetPreference_WhenKeyTheme_ShouldUpdateAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	m := NewManager(path)
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}

	err := m.SetPreference("theme", "dark")
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	if m.Config().Theme != "dark" {
		t.Errorf("Theme: want dark, got %q", m.Config().Theme)
	}

	// Reload from disk and verify persisted
	m2 := NewManager(path)
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	if m2.Config().Theme != "dark" {
		t.Errorf("after reload Theme: want dark, got %q", m2.Config().Theme)
	}
}

func TestManager_SetPreference_WhenKeyDefaultModel_ShouldUpdateAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	m := NewManager(path)
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}

	err := m.SetPreference("defaultModel", "claude-3")
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	if m.Config().DefaultModel != "claude-3" {
		t.Errorf("DefaultModel: want claude-3, got %q", m.Config().DefaultModel)
	}

	m2 := NewManager(path)
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	if m2.Config().DefaultModel != "claude-3" {
		t.Errorf("after reload DefaultModel: want claude-3, got %q", m2.Config().DefaultModel)
	}
}

func TestManager_SetPreference_WhenKeyUnknown_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	m := NewManager(path)
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}

	err := m.SetPreference("unknownKey", "value")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !errors.Is(err, ErrUnknownKey) {
		t.Errorf("expected ErrUnknownKey, got %v", err)
	}
}

func TestManager_Config_WhenCalledBeforeLoad_ShouldReturnNonNilDefault(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "config.json"))
	// Config() without Load: config is nil internally, Config() returns &Config{}
	c := m.Config()
	if c == nil {
		t.Fatal("Config() before Load should return non-nil")
	}
	if c.Theme != "" || c.DefaultModel != "" {
		t.Errorf("default config should be zero values, got theme=%q defaultModel=%q", c.Theme, c.DefaultModel)
	}
}

func TestManager_Config_WhenConfigIsNil_ShouldAllocateAndReturn(t *testing.T) {
	// Manager with nil config (e.g. struct literal without config set) triggers the nil branch in Config().
	m := &Manager{path: filepath.Join(t.TempDir(), "config.json")}
	if m.Config() == nil {
		t.Fatal("Config() when config is nil should return non-nil")
	}
	if m.Config() != m.Config() {
		t.Error("Config() should return same instance after first call")
	}
}

func TestManager_save_WhenParentIsFile_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	// config.json's parent should be a dir; make "ironclaw" a file so MkdirAll fails
	ironclawPath := filepath.Join(dir, "ironclaw")
	if err := os.WriteFile(ironclawPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ironclawPath, "config.json")
	m := NewManager(path)
	m.Load()
	m.Config().Theme = "dark"
	err := m.SetPreference("theme", "dark")
	if err == nil {
		t.Fatal("save when parent is file: expected error")
	}
}

func TestConfigPath_ShouldReturnDirUnderUserConfigAndCreateDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}

	wantDir := filepath.Join(dir, "ironclaw")
	if filepath.Dir(path) != wantDir {
		t.Errorf("config dir: want %q, got %q", wantDir, filepath.Dir(path))
	}
	if filepath.Base(path) != "config.json" {
		t.Errorf("config file name: want config.json, got %q", filepath.Base(path))
	}

	// ConfigPath should ensure directory exists (so we can write later)
	info, err := os.Stat(wantDir)
	if err != nil {
		t.Fatalf("config dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("config path should be a directory")
	}
}

func TestManager_save_WhenMarshalIndentFails_ShouldReturnError(t *testing.T) {
	prev := prefsMarshalIndent
	defer func() { prefsMarshalIndent = prev }()
	prefsMarshalIndent = func(interface{}, string, string) ([]byte, error) {
		return nil, fmt.Errorf("injected marshal error")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	m := NewManager(path)
	m.Load()
	err := m.SetPreference("theme", "dark")
	if err == nil {
		t.Fatal("save when marshal fails: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("marshal")) {
		t.Errorf("error should mention marshal: %v", err)
	}
}

func TestConfigPath_WhenUserConfigDirFails_ShouldReturnError(t *testing.T) {
	prev := userConfigDir
	defer func() { userConfigDir = prev }()
	userConfigDir = func() (string, error) {
		return "", fmt.Errorf("injected UserConfigDir error")
	}
	_, err := ConfigPath()
	if err == nil {
		t.Fatal("ConfigPath when UserConfigDir fails: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("config path")) {
		t.Errorf("error should mention config path: %v", err)
	}
}

func TestConfigPath_WhenBaseIsFile_MkdirAllFailsAndReturnsError(t *testing.T) {
	dir := t.TempDir()
	// Set XDG_CONFIG_HOME to a path that is a file so "ironclaw" subdir can't be created
	fileAsConfigHome := filepath.Join(dir, "config_home_file")
	if err := os.WriteFile(fileAsConfigHome, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", fileAsConfigHome)
	_, err := ConfigPath()
	if err == nil {
		t.Fatal("ConfigPath when base is file: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("mkdir")) && !bytes.Contains([]byte(err.Error()), []byte("config path")) {
		t.Errorf("error should mention mkdir or config path: %v", err)
	}
}

func TestManager_save_WhenWriteFileFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0555); err != nil {
		t.Skip("chmod 0555 not supported")
	}
	defer os.Chmod(sub, 0755)
	path := filepath.Join(sub, "config.json")
	m := NewManager(path)
	m.Load()
	m.Config().Theme = "dark"
	err := m.SetPreference("theme", "dark")
	if err == nil {
		t.Fatal("save when dir read-only: expected error")
	}
}
