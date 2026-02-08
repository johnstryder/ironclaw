package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ironclaw/internal/domain"
)

// marshalIndent and writeFile are used by WriteDefault and Save; tests may replace to force errors.
var (
	marshalIndent = json.MarshalIndent
	writeFile     = os.WriteFile
)

// WriteDefault writes a default Config to path (e.g. ironclaw.json). Paths are not created.
func WriteDefault(path string) error {
	cfg := &domain.Config{
		Gateway: domain.GatewayConfig{
			Port: 8080,
			Auth: domain.AuthConfig{
				Mode:                  "none",
				RequirePINForExternal: false,
				ExternalChannels:      []string{},
				RateLimitMaxAttempts:  5,
			},
			AllowedHosts: []string{},
		},
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "gpt-4o",
			ModelAliases: map[string]string{},
			Paths:        domain.AgentPaths{Root: "agents", Memory: "memory"},
		},
		Infra: domain.InfraConfig{LogFormat: "text", LogLevel: "info"},
		Retry: domain.RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 500,
			MaxBackoff:     30000,
			Multiplier:     2,
		},
		AllowedCommands: []string{},
	}
	data, err := marshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, data, 0644)
}

// Load reads path (e.g. ironclaw.json), unmarshals into domain.Config, and cleans
// all path fields to mitigate path traversal. Returns error if file is missing or invalid JSON.
func Load(path string) (*domain.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}
	var c domain.Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config parse: %w", err)
	}
	CleanPaths(&c)
	return &c, nil
}

// CleanPaths applies filepath.Clean to all path fields in cfg to prevent path traversal.
func CleanPaths(cfg *domain.Config) {
	if cfg == nil {
		return
	}
	cfg.Agents.Paths.Root = filepath.Clean(cfg.Agents.Paths.Root)
	cfg.Agents.Paths.Memory = filepath.Clean(cfg.Agents.Paths.Memory)
}

// Save writes cfg to path as JSON (so the agent can persist allowlist edits).
func Save(path string, cfg *domain.Config) error {
	if cfg == nil {
		return fmt.Errorf("config save: nil config")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("config save mkdir: %w", err)
	}
	data, err := marshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config save marshal: %w", err)
	}
	if err = writeFile(path, data, 0644); err != nil {
		err = fmt.Errorf("config save write: %w", err)
	}
	if err != nil {
		return err
	}
	return nil
}
