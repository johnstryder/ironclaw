package config

import (
	"os"
	"path/filepath"
	"testing"

	"ironclaw/internal/domain"
)

func TestSave_WhenConfigWritten_ShouldPersistAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ironclaw.json")
	cfg := &domain.Config{
		Gateway:         domain.GatewayConfig{Port: 8080, Auth: domain.AuthConfig{Mode: "none"}, AllowedHosts: []string{}},
		Agents:          domain.AgentsConfig{DefaultModel: "gpt-4o", ModelAliases: map[string]string{}, Paths: domain.AgentPaths{Root: "agents", Memory: "memory"}},
		Infra:           domain.InfraConfig{LogFormat: "text", LogLevel: "info"},
		AllowedCommands: []string{"ls", "cat"},
	}

	err := Save(path, cfg)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(loaded.AllowedCommands) != 2 || loaded.AllowedCommands[0] != "ls" || loaded.AllowedCommands[1] != "cat" {
		t.Errorf("reloaded AllowedCommands: want [ls cat], got %v", loaded.AllowedCommands)
	}
}

func TestAddAllowedCommand_WhenNotPresent_ShouldAppend(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls"}}
	AddAllowedCommand(cfg, "cat")
	if len(cfg.AllowedCommands) != 2 || cfg.AllowedCommands[1] != "cat" {
		t.Errorf("want [ls cat], got %v", cfg.AllowedCommands)
	}
}

func TestAddAllowedCommand_WhenAlreadyPresent_ShouldNotDuplicate(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls", "cat"}}
	AddAllowedCommand(cfg, "ls")
	if len(cfg.AllowedCommands) != 2 {
		t.Errorf("should not duplicate, got %v", cfg.AllowedCommands)
	}
}

func TestAddAllowedCommand_WhenSliceNil_ShouldInitializeAndAdd(t *testing.T) {
	cfg := &domain.Config{}
	AddAllowedCommand(cfg, "echo")
	if len(cfg.AllowedCommands) != 1 || cfg.AllowedCommands[0] != "echo" {
		t.Errorf("want [echo], got %v", cfg.AllowedCommands)
	}
}

func TestRemoveAllowedCommand_WhenPresent_ShouldRemove(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls", "cat", "echo"}}
	RemoveAllowedCommand(cfg, "cat")
	if len(cfg.AllowedCommands) != 2 || cfg.AllowedCommands[0] != "ls" || cfg.AllowedCommands[1] != "echo" {
		t.Errorf("want [ls echo], got %v", cfg.AllowedCommands)
	}
}

func TestRemoveAllowedCommand_WhenNotPresent_ShouldNoOp(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls"}}
	RemoveAllowedCommand(cfg, "rm")
	if len(cfg.AllowedCommands) != 1 || cfg.AllowedCommands[0] != "ls" {
		t.Errorf("want [ls], got %v", cfg.AllowedCommands)
	}
}

func TestAgentEditAllowlist_LoadAddSaveReload_ShouldPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ironclaw.json")
	if err := WriteDefault(path); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	AddAllowedCommand(cfg, "cat")
	AddAllowedCommand(cfg, "echo")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	cfg2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.AllowedCommands) != 2 {
		t.Errorf("after agent edit: want 2 commands, got %v", cfg2.AllowedCommands)
	}
	// Default has [] then we added cat, echo
	if cfg2.AllowedCommands[0] != "cat" || cfg2.AllowedCommands[1] != "echo" {
		t.Errorf("want [cat echo], got %v", cfg2.AllowedCommands)
	}
}

func TestSave_WhenPathDirMissing_ShouldCreateDirOrReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "ironclaw.json")
	cfg := &domain.Config{
		Gateway:         domain.GatewayConfig{Port: 8080, Auth: domain.AuthConfig{Mode: "none"}, AllowedHosts: []string{}},
		Agents:          domain.AgentsConfig{Paths: domain.AgentPaths{Root: "agents", Memory: "memory"}},
		Infra:           domain.InfraConfig{LogFormat: "text", LogLevel: "info"},
		AllowedCommands: []string{},
	}
	err := Save(path, cfg)
	if err != nil {
		// Creating parent dir might not be in scope; at least writing to existing dir works
		t.Logf("Save to new subdir: %v (acceptable if we don't create parents)", err)
		return
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist after Save: %v", err)
	}
}
