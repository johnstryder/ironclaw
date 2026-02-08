package config

import (
	"errors"
	"testing"

	"ironclaw/internal/domain"
)

func TestValidateCommand_WhenAllowlistEmpty_ShouldAllowAnyCommand(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: nil}
	if err := ValidateCommand(cfg, "rm"); err != nil {
		t.Errorf("expected nil when allowlist nil, got %v", err)
	}
	cfg.AllowedCommands = []string{}
	if err := ValidateCommand(cfg, "ls"); err != nil {
		t.Errorf("expected nil when allowlist empty, got %v", err)
	}
}

func TestValidateCommand_WhenCommandInAllowlist_ShouldReturnNil(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls", "cat"}}
	if err := ValidateCommand(cfg, "ls"); err != nil {
		t.Errorf("ls: expected nil, got %v", err)
	}
	if err := ValidateCommand(cfg, "cat"); err != nil {
		t.Errorf("cat: expected nil, got %v", err)
	}
}

func TestValidateCommand_WhenCommandIsPath_ShouldMatchByBinaryName(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls"}}
	if err := ValidateCommand(cfg, "/usr/bin/ls"); err != nil {
		t.Errorf("/usr/bin/ls: expected nil (match by base name), got %v", err)
	}
}

func TestValidateCommand_WhenCommandWithArgs_ShouldMatchFirstToken(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls"}}
	if err := ValidateCommand(cfg, "ls -la"); err != nil {
		t.Errorf("ls -la: expected nil, got %v", err)
	}
}

func TestValidateCommand_WhenCommandNotInAllowlist_ShouldReturnErrNotAllowed(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls", "cat"}}
	err := ValidateCommand(cfg, "rm")
	if err == nil {
		t.Fatal("expected error when command not allowed")
	}
	if !errors.Is(err, ErrCommandNotAllowed) {
		t.Errorf("expected ErrCommandNotAllowed, got %v", err)
	}
	if err.Error() == "" || len(err.Error()) < 5 {
		t.Errorf("error message should mention policy: %q", err.Error())
	}
}

func TestValidateCommand_WhenConfigNil_ShouldAllowCommand(t *testing.T) {
	if err := ValidateCommand(nil, "anything"); err != nil {
		t.Errorf("nil config: treat as empty allowlist, got %v", err)
	}
}

func TestValidateCommand_WhenCmdEmpty_ShouldReturnErrNotAllowed(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls"}}
	err := ValidateCommand(cfg, "   ")
	if err == nil {
		t.Fatal("empty cmd with non-empty allowlist: expected error")
	}
	if !errors.Is(err, ErrCommandNotAllowed) {
		t.Errorf("expected ErrCommandNotAllowed, got %v", err)
	}
}

func TestAddAllowedCommand_WhenConfigNil_ShouldNoop(t *testing.T) {
	AddAllowedCommand(nil, "ls")
}

func TestAddAllowedCommand_WhenCmdEmpty_ShouldNoop(t *testing.T) {
	cfg := &domain.Config{}
	AddAllowedCommand(cfg, "   ")
	if len(cfg.AllowedCommands) != 0 {
		t.Errorf("empty cmd should not add: %v", cfg.AllowedCommands)
	}
}

func TestAddAllowedCommand_WhenCmdHasSpace_ShouldUseFirstToken(t *testing.T) {
	cfg := &domain.Config{}
	AddAllowedCommand(cfg, "ls -la /tmp")
	if len(cfg.AllowedCommands) != 1 || cfg.AllowedCommands[0] != "ls" {
		t.Errorf("expected [ls], got %v", cfg.AllowedCommands)
	}
}

func TestAddAllowedCommand_WhenAlreadyPresent_ShouldNotDuplicateSecond(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls"}}
	AddAllowedCommand(cfg, "ls")
	if len(cfg.AllowedCommands) != 1 {
		t.Errorf("should not duplicate ls: %v", cfg.AllowedCommands)
	}
}

func TestRemoveAllowedCommand_WhenConfigNil_ShouldNoop(t *testing.T) {
	RemoveAllowedCommand(nil, "ls")
}

func TestRemoveAllowedCommand_WhenListEmpty_ShouldNoop(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{}}
	RemoveAllowedCommand(cfg, "ls")
	if len(cfg.AllowedCommands) != 0 {
		t.Errorf("expected empty, got %v", cfg.AllowedCommands)
	}
}

func TestRemoveAllowedCommand_WhenPresent_ShouldRemoveOne(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls", "cat"}}
	RemoveAllowedCommand(cfg, "ls")
	if len(cfg.AllowedCommands) != 1 || cfg.AllowedCommands[0] != "cat" {
		t.Errorf("expected [cat], got %v", cfg.AllowedCommands)
	}
}

func TestRemoveAllowedCommand_WhenCmdHasSpaceOrPath_ShouldMatchByBinaryName(t *testing.T) {
	cfg := &domain.Config{AllowedCommands: []string{"ls", "cat"}}
	RemoveAllowedCommand(cfg, "ls -la /tmp")
	if len(cfg.AllowedCommands) != 1 || cfg.AllowedCommands[0] != "cat" {
		t.Errorf("expected [cat] after remove 'ls -la', got %v", cfg.AllowedCommands)
	}
	RemoveAllowedCommand(cfg, "/usr/bin/cat")
	if len(cfg.AllowedCommands) != 0 {
		t.Errorf("expected [] after remove /usr/bin/cat, got %v", cfg.AllowedCommands)
	}
}
