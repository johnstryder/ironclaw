package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"ironclaw/internal/domain"
)


func TestLoad_WhenFileDoesNotExist_ShouldReturnError(t *testing.T) {
	_, err := Load("/nonexistent/ironclaw.json")
	if err == nil {
		t.Fatal("expected error when config file does not exist")
	}
}

func TestLoad_WhenFileIsInvalidJSON_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ironclaw.json")
	if err := os.WriteFile(path, []byte(`{ invalid }`), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when config is invalid JSON")
	}
}

func TestLoad_WhenFileIsValid_ShouldReturnConfigWithCleanedPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ironclaw.json")
	cfg := `{
		"gateway": { "port": 8080, "auth": { "mode": "none" }, "allowedHosts": [] },
		"agents": {
			"defaultModel": "gpt-4",
			"modelAliases": {},
			"paths": { "root": "agents/../agents", "memory": "memory/./logs" }
		},
		"infra": { "logFormat": "json", "logLevel": "info" }
	}`
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil config")
	}
	// Paths must be cleaned (no .. or .)
	if got.Agents.Paths.Root != "agents" {
		t.Errorf("expected cleaned root path 'agents', got %q", got.Agents.Paths.Root)
	}
	if got.Agents.Paths.Memory != filepath.Join("memory", "logs") {
		t.Errorf("expected cleaned memory path 'memory/logs', got %q", got.Agents.Paths.Memory)
	}
}

func TestLoad_WhenFileIsValid_ShouldPopulateAllSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ironclaw.json")
	cfg := `{
		"gateway": {
			"port": 3000,
			"auth": {
				"mode": "password",
				"authToken": "secret-gateway-token",
				"promptPin": "1234",
				"requirePinForExternal": true,
				"externalChannels": ["telegram", "whatsapp"],
				"rateLimitMaxAttempts": 5
			},
			"allowedHosts": ["localhost"]
		},
		"agents": {
			"defaultModel": "gpt-4o",
			"modelAliases": { "default": "gpt-4o-mini" },
			"paths": { "root": "/app/agents", "memory": "/app/memory" }
		},
		"infra": { "logFormat": "text", "logLevel": "debug" }
	}`
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Gateway.Port != 3000 {
		t.Errorf("gateway.port: want 3000, got %d", got.Gateway.Port)
	}
	if got.Gateway.Auth.Mode != "password" {
		t.Errorf("gateway.auth.mode: want password, got %q", got.Gateway.Auth.Mode)
	}
	if got.Gateway.Auth.AuthToken != "secret-gateway-token" {
		t.Errorf("gateway.auth.authToken: want secret-gateway-token, got %q", got.Gateway.Auth.AuthToken)
	}
	if got.Gateway.Auth.PromptPIN != "1234" {
		t.Errorf("gateway.auth.promptPin: want 1234, got %q", got.Gateway.Auth.PromptPIN)
	}
	if !got.Gateway.Auth.RequirePINForExternal {
		t.Error("gateway.auth.requirePinForExternal: want true")
	}
	if got.Agents.DefaultModel != "gpt-4o" {
		t.Errorf("agents.defaultModel: want gpt-4o, got %q", got.Agents.DefaultModel)
	}
	if got.Agents.Paths.Root != "/app/agents" {
		t.Errorf("agents.paths.root: want /app/agents, got %q", got.Agents.Paths.Root)
	}
	if got.Infra.LogLevel != "debug" {
		t.Errorf("infra.logLevel: want debug, got %q", got.Infra.LogLevel)
	}
}

func TestCleanPaths_WhenConfigIsNil_ShouldNotPanic(t *testing.T) {
	CleanPaths(nil)
}

func TestWriteDefault_ShouldCreateValidConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ironclaw.json")
	if err := WriteDefault(path); err != nil {
		t.Fatalf("WriteDefault: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load after WriteDefault: %v", err)
	}
	if cfg.Gateway.Port != 8080 || cfg.Gateway.Auth.Mode != "none" {
		t.Errorf("unexpected default: port=%d auth=%s", cfg.Gateway.Port, cfg.Gateway.Auth.Mode)
	}
	if cfg.Agents.Paths.Root != "agents" || cfg.Agents.Paths.Memory != "memory" {
		t.Errorf("unexpected paths: root=%q memory=%q", cfg.Agents.Paths.Root, cfg.Agents.Paths.Memory)
	}
}

func TestCleanPaths_WhenGivenPathWithTraversal_ShouldReturnCleanedPath(t *testing.T) {
	c := &domain.Config{
		Agents: domain.AgentsConfig{
			Paths: domain.AgentPaths{
				Root:   filepath.Join("foo", "..", "bar"),
				Memory: filepath.Join("mem", ".", "day"),
			},
		},
	}
	CleanPaths(c)
	if c.Agents.Paths.Root != "bar" {
		t.Errorf("root: expected cleaned 'bar', got %q", c.Agents.Paths.Root)
	}
	if c.Agents.Paths.Memory != filepath.Join("mem", "day") {
		t.Errorf("memory: expected cleaned 'mem/day', got %q", c.Agents.Paths.Memory)
	}
}

func TestSave_WhenConfigNil_ShouldReturnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ironclaw.json")
	err := Save(path, nil)
	if err == nil {
		t.Fatal("Save(nil) should return error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("nil")) {
		t.Errorf("error should mention nil: %v", err)
	}
}

func TestSave_WhenDirReadOnly_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0555); err != nil {
		t.Skip("chmod 0555 not supported")
	}
	defer os.Chmod(sub, 0755)
	path := filepath.Join(sub, "cfg.json")
	cfg := &domain.Config{Gateway: domain.GatewayConfig{Port: 8080}}
	err := Save(path, cfg)
	if err == nil {
		t.Fatal("Save to read-only dir should fail")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("write")) && !bytes.Contains([]byte(err.Error()), []byte("permission")) {
		t.Errorf("error should mention write or permission: %v", err)
	}
}

func TestSave_WhenConfigValid_ShouldPersistAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ironclaw.json")
	cfg := &domain.Config{
		Gateway: domain.GatewayConfig{
			Port: 9000,
			Auth: domain.AuthConfig{Mode: "password"},
		},
		Agents: domain.AgentsConfig{
			DefaultModel: "gpt-4o",
			Paths:        domain.AgentPaths{Root: "agents", Memory: "memory"},
		},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if loaded.Gateway.Port != 9000 || loaded.Gateway.Auth.Mode != "password" {
		t.Errorf("loaded gateway: port=%d mode=%s", loaded.Gateway.Port, loaded.Gateway.Auth.Mode)
	}
	if loaded.Agents.DefaultModel != "gpt-4o" || loaded.Agents.Paths.Root != "agents" {
		t.Errorf("loaded agents: defaultModel=%s root=%s", loaded.Agents.DefaultModel, loaded.Agents.Paths.Root)
	}
}

func TestSave_WhenParentDirIsFile_ShouldReturnMkdirError(t *testing.T) {
	dir := t.TempDir()
	fileAsParent := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fileAsParent, "ironclaw.json")
	cfg := &domain.Config{Gateway: domain.GatewayConfig{Port: 8080}}
	err := Save(path, cfg)
	if err == nil {
		t.Fatal("Save when parent is file: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("mkdir")) {
		t.Errorf("error should mention mkdir: %v", err)
	}
}

func TestWriteDefault_WhenParentDirMissing_ShouldReturnWriteError(t *testing.T) {
	dir := t.TempDir()
	// WriteDefault does not create parent dirs
	path := filepath.Join(dir, "nonexistent", "ironclaw.json")
	err := WriteDefault(path)
	if err == nil {
		t.Fatal("WriteDefault to path with missing parent: expected error")
	}
}

func TestWriteDefault_WhenMarshalFails_ShouldReturnError(t *testing.T) {
	prev := marshalIndent
	defer func() { marshalIndent = prev }()
	marshalIndent = func(interface{}, string, string) ([]byte, error) {
		return nil, fmt.Errorf("injected marshal error")
	}
	path := filepath.Join(t.TempDir(), "ironclaw.json")
	err := WriteDefault(path)
	if err == nil {
		t.Fatal("WriteDefault when marshal fails: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("marshal")) {
		t.Errorf("error should mention marshal: %v", err)
	}
}

func TestSave_WhenMarshalFails_ShouldReturnError(t *testing.T) {
	prev := marshalIndent
	defer func() { marshalIndent = prev }()
	marshalIndent = func(interface{}, string, string) ([]byte, error) {
		return nil, fmt.Errorf("injected marshal error")
	}
	path := filepath.Join(t.TempDir(), "ironclaw.json")
	cfg := &domain.Config{Gateway: domain.GatewayConfig{Port: 8080}}
	err := Save(path, cfg)
	if err == nil {
		t.Fatal("Save when marshal fails: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("marshal")) {
		t.Errorf("error should mention marshal: %v", err)
	}
}

func TestSave_WhenWriteFileFails_ShouldReturnError(t *testing.T) {
	prev := writeFile
	defer func() { writeFile = prev }()
	writeFile = func(string, []byte, os.FileMode) error {
		return fmt.Errorf("injected write error")
	}
	path := filepath.Join(t.TempDir(), "ironclaw.json")
	cfg := &domain.Config{Gateway: domain.GatewayConfig{Port: 8080}}
	err := Save(path, cfg)
	if err == nil {
		t.Fatal("Save when write fails: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("write")) {
		t.Errorf("error should mention write: %v", err)
	}
}
