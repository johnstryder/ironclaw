package tooling

import (
	"encoding/json"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"

	"ironclaw/internal/domain"
)

// defaultSkillFilename is the fallback filename when one cannot be derived
// from a URL path.
const defaultSkillFilename = "skill.md"

// =============================================================================
// SkillInstaller — plug-and-play skill installation
// =============================================================================

// SkillInstaller downloads or copies skill files into the skills directory and
// registers them in the ToolRegistry. It implements SchemaTool so the LLM can
// invoke it directly.
type SkillInstaller struct {
	skillsDir string
	registry  *ToolRegistry
	fetcher   HTTPFetcher
}

// NewSkillInstaller creates a SkillInstaller that saves skills to skillsDir
// and registers them in the given registry. The fetcher is used for URL downloads.
func NewSkillInstaller(skillsDir string, registry *ToolRegistry, fetcher HTTPFetcher) *SkillInstaller {
	return &SkillInstaller{
		skillsDir: skillsDir,
		registry:  registry,
		fetcher:   fetcher,
	}
}

// =============================================================================
// SchemaTool Interface — so the LLM can invoke install_skill
// =============================================================================

// skillInstallerInput is the JSON input structure for the install_skill tool.
type skillInstallerInput struct {
	Source string `json:"source"`
}

// Name returns the tool name for function-calling.
func (si *SkillInstaller) Name() string { return "install_skill" }

// Description returns a human-readable description for the LLM.
func (si *SkillInstaller) Description() string {
	return "Install a skill from a URL or local file path into the skills directory and register it immediately."
}

// Definition returns the JSON Schema string for the tool's input.
func (si *SkillInstaller) Definition() string {
	return `{"type":"object","properties":{"source":{"type":"string","description":"URL or local file path to a skill .md file"}},"required":["source"]}`
}

// Call executes the install_skill tool with the given JSON arguments.
func (si *SkillInstaller) Call(args json.RawMessage) (*domain.ToolResult, error) {
	var input skillInstallerInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	if input.Source == "" {
		return nil, fmt.Errorf("source must not be empty")
	}

	skill, err := si.Install(input.Source)
	if err != nil {
		return nil, err
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Successfully installed skill %q (%s)", skill.Name(), skill.Description()),
		Metadata: map[string]string{
			"skill_name":   skill.Name(),
			"skill_source": input.Source,
		},
	}, nil
}

// =============================================================================
// Install — fetch / copy, validate, persist, register
// =============================================================================

// isURL returns true when the source string looks like an HTTP(S) URL.
func isURL(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}

// Install installs a skill from a URL or local file path. It saves the file to
// the skills directory, parses it, and registers it in the ToolRegistry.
// Returns the parsed MarkdownSkill or an error.
func (si *SkillInstaller) Install(source string) (*MarkdownSkill, error) {
	if source == "" {
		return nil, fmt.Errorf("source must not be empty")
	}

	content, filename, err := si.resolveSource(source)
	if err != nil {
		return nil, err
	}

	// Validate by parsing before writing
	fm, body, err := ParseFrontmatter(string(content))
	if err != nil {
		return nil, fmt.Errorf("invalid skill content: %w", err)
	}

	// Write to skills directory
	destPath := filepath.Join(si.skillsDir, filename)
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write skill file to %q: %w", destPath, err)
	}

	// Build and register the skill
	schema := BuildJSONSchema(fm.Args)
	skill := &MarkdownSkill{
		name:        fm.Name,
		description: fm.Description,
		schema:      schema,
		body:        body,
	}

	if err := si.registry.Register(skill); err != nil {
		return nil, fmt.Errorf("failed to register skill %q: %w", skill.Name(), err)
	}

	return skill, nil
}

// resolveSource fetches content from a URL or reads from a local path.
// Returns the raw bytes and the destination filename.
func (si *SkillInstaller) resolveSource(source string) ([]byte, string, error) {
	if isURL(source) {
		data, err := si.fetcher.Fetch(source)
		if err != nil {
			return nil, "", fmt.Errorf("failed to fetch skill from URL %q: %w", source, err)
		}
		return data, filenameFromURL(source), nil
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read local skill file %q: %w", source, err)
	}
	return data, filepath.Base(source), nil
}

// =============================================================================
// ReloadSkills — hot-reload all .md skills from the directory
// =============================================================================

// ReloadSkills scans the skills directory, parses every .md file, and registers
// any skills that are not already present in the ToolRegistry. Returns the names
// of newly registered skills.
func (si *SkillInstaller) ReloadSkills() ([]string, error) {
	skills, err := LoadSkillsFromDir(si.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("reload: %w", err)
	}

	var registered []string
	for _, skill := range skills {
		if err := si.registry.Register(skill); err != nil {
			// Already registered — skip
			continue
		}
		registered = append(registered, skill.Name())
	}
	return registered, nil
}

// =============================================================================
// Helpers
// =============================================================================

// filenameFromURL extracts a filename from a URL path. Falls back to
// defaultSkillFilename if no filename with an extension can be determined.
func filenameFromURL(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return defaultSkillFilename
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		return defaultSkillFilename
	}
	base := filepath.Base(path)
	if base != "" && strings.Contains(base, ".") {
		return base
	}
	return defaultSkillFilename
}
