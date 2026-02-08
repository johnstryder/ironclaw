package prefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds user preferences (theme, default model, etc.).
type Config struct {
	Theme        string `json:"theme"`
	DefaultModel string `json:"defaultModel"`
}

// Manager loads and saves user preferences from a JSON file.
type Manager struct {
	path   string
	config *Config
}

// NewManager returns a manager that reads/writes the given path.
func NewManager(path string) *Manager {
	return &Manager{path: path, config: &Config{}}
}

// Load reads config from the manager's path. If the file does not exist,
// config is set to default (zero) values and no error is returned.
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			m.config = &Config{}
			return nil
		}
		return fmt.Errorf("prefs load: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("prefs parse: %w", err)
	}
	m.config = &c
	return nil
}

// Config returns the current in-memory config (never nil after Load).
func (m *Manager) Config() *Config {
	if m.config == nil {
		m.config = &Config{}
	}
	return m.config
}

// SetPreference updates a preference by key and writes config to disk immediately.
// Supported keys: "theme", "defaultModel". Unknown keys return an error.
func (m *Manager) SetPreference(key, value string) error {
	c := m.Config()
	switch strings.ToLower(key) {
	case "theme":
		c.Theme = value
	case "defaultmodel":
		c.DefaultModel = value
	default:
		return fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	return m.save()
}

func (m *Manager) save() error {
	dir := filepath.Dir(m.path)
	if err := prefsMkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("prefs save mkdir: %w", err)
	}
	data, err := prefsMarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("prefs save marshal: %w", err)
	}
	if err := prefsWriteFile(m.path, data, 0644); err != nil {
		return fmt.Errorf("prefs save write: %w", err)
	}
	return nil
}

// ConfigPath returns the path to the user preferences file:
// UserConfigDir()/ironclaw/config.json. The ironclaw directory is created if missing.
func ConfigPath() (string, error) {
	base, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("prefs config path: %w", err)
	}
	dir := filepath.Join(base, "ironclaw")
	if err := prefsMkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("prefs config path mkdir: %w", err)
	}
	return filepath.Join(dir, "config.json"), nil
}

// ErrUnknownKey is returned when SetPreference is called with an unsupported key.
var ErrUnknownKey = errors.New("unknown preference key")

// Hooks for tests to force error paths.
var (
	prefsMarshalIndent = json.MarshalIndent
	prefsWriteFile     = os.WriteFile
	userConfigDir      = os.UserConfigDir
	prefsMkdirAll      = os.MkdirAll
)
