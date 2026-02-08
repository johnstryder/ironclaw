package config

import (
	"errors"
	"path/filepath"
	"strings"

	"ironclaw/internal/domain"
)

// AddAllowedCommand adds cmd to cfg.AllowedCommands if not already present (by binary name).
func AddAllowedCommand(cfg *domain.Config, cmd string) {
	if cfg == nil {
		return
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	first := cmd
	if i := strings.IndexAny(cmd, " \t"); i >= 0 {
		first = cmd[:i]
	}
	bin := filepath.Base(first)
	if cfg.AllowedCommands == nil {
		cfg.AllowedCommands = []string{}
	}
	for _, c := range cfg.AllowedCommands {
		if filepath.Base(c) == bin {
			return
		}
	}
	cfg.AllowedCommands = append(cfg.AllowedCommands, bin)
}

// RemoveAllowedCommand removes the command (by binary name) from cfg.AllowedCommands.
func RemoveAllowedCommand(cfg *domain.Config, cmd string) {
	if cfg == nil || len(cfg.AllowedCommands) == 0 {
		return
	}
	cmd = strings.TrimSpace(cmd)
	first := cmd
	if i := strings.IndexAny(cmd, " \t"); i >= 0 {
		first = cmd[:i]
	}
	bin := filepath.Base(first)
	out := make([]string, 0, len(cfg.AllowedCommands))
	for _, c := range cfg.AllowedCommands {
		if filepath.Base(c) != bin {
			out = append(out, c)
		}
	}
	cfg.AllowedCommands = out
}

// ErrCommandNotAllowed is returned when the requested command is not in the allowlist.
var ErrCommandNotAllowed = errors.New("command not allowed by policy")

// ValidateCommand checks cfg.AllowedCommands. If the allowlist is empty or nil, any command is allowed.
// Otherwise the command's binary name (first token, base of path) must be in the allowlist.
func ValidateCommand(cfg *domain.Config, cmd string) error {
	if cfg == nil || len(cfg.AllowedCommands) == 0 {
		return nil
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ErrCommandNotAllowed
	}
	first := cmd
	if i := strings.IndexAny(cmd, " \t"); i >= 0 {
		first = cmd[:i]
	}
	bin := filepath.Base(first)
	for _, allowed := range cfg.AllowedCommands {
		if filepath.Base(allowed) == bin {
			return nil
		}
	}
	return ErrCommandNotAllowed
}
