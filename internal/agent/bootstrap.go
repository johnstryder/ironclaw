package agent

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"ironclaw/internal/domain"
)

// LoadAgentContext reads AGENTS.md, SOUL.md, IDENTITY.md, and TOOLS.md from root
// and returns an AgentContext. Root is cleaned with filepath.Clean. Missing files
// result in empty fields; only root not existing or not being a directory returns an error.
func LoadAgentContext(root string) (*domain.AgentContext, error) {
	root = filepath.Clean(root)
	fi, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, os.ErrNotExist
	}

	ctx := &domain.AgentContext{}

	if b, err := os.ReadFile(filepath.Join(root, "SOUL.md")); err == nil {
		ctx.Soul = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(root, "IDENTITY.md")); err == nil {
		ctx.Identity = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(root, "AGENTS.md")); err == nil {
		_ = b
		// AGENTS.md is high-level directives; could be merged into Soul or kept separate. Leave Soul as primary.
	}
	if b, err := os.ReadFile(filepath.Join(root, "TOOLS.md")); err == nil {
		ctx.Tools = parseToolNames(string(b))
	}

	return ctx, nil
}

// parseToolNames extracts tool names from TOOLS.md (lines like "- name" or "name").
func parseToolNames(content string) []string {
	var names []string
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimSpace(line[2:])
		}
		if line != "" {
			names = append(names, line)
		}
	}
	return names
}
