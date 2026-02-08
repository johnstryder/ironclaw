package tooling

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ironclaw/internal/domain"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// Frontmatter Types
// =============================================================================

// SkillArg describes a single argument for a Markdown-defined skill.
type SkillArg struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

// SkillFrontmatter holds the YAML frontmatter parsed from a skill .md file.
type SkillFrontmatter struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Args        []SkillArg `yaml:"args"`
}

// =============================================================================
// ParseFrontmatter
// =============================================================================

// ParseFrontmatter splits a Markdown string into its YAML frontmatter and body.
// Frontmatter must be delimited by "---" on lines by themselves.
// Returns the parsed frontmatter, the trimmed body, and any error.
func ParseFrontmatter(content string) (*SkillFrontmatter, string, error) {
	const delimiter = "---"

	// Must start with ---
	if !strings.HasPrefix(strings.TrimSpace(content), delimiter) {
		return nil, "", fmt.Errorf("no frontmatter found: content must start with ---")
	}

	// Find the opening and closing delimiters
	trimmed := strings.TrimSpace(content)
	// Remove leading ---
	rest := trimmed[len(delimiter):]

	// Find the closing ---
	closingIdx := strings.Index(rest, "\n"+delimiter)
	if closingIdx == -1 {
		return nil, "", fmt.Errorf("no closing --- delimiter found")
	}

	yamlContent := rest[:closingIdx]
	body := strings.TrimSpace(rest[closingIdx+len("\n"+delimiter):])

	var fm SkillFrontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, "", fmt.Errorf("invalid YAML frontmatter: %w", err)
	}

	if fm.Name == "" {
		return nil, "", fmt.Errorf("frontmatter missing required field: name")
	}
	if fm.Description == "" {
		return nil, "", fmt.Errorf("frontmatter missing required field: description")
	}

	return &fm, body, nil
}

// =============================================================================
// BuildJSONSchema
// =============================================================================

// BuildJSONSchema converts a slice of SkillArg into a JSON Schema string
// suitable for LLM function-calling. Returns a JSON object schema.
func BuildJSONSchema(args []SkillArg) string {
	properties := make(map[string]map[string]string)
	var required []string

	for _, arg := range args {
		prop := map[string]string{
			"type": arg.Type,
		}
		if arg.Description != "" {
			prop["description"] = arg.Description
		}
		properties[arg.Name] = prop

		if arg.Required {
			required = append(required, arg.Name)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	b, _ := json.Marshal(schema)
	return string(b)
}

// =============================================================================
// MarkdownSkill — implements SchemaTool
// =============================================================================

// MarkdownSkill is a dynamically loaded tool defined by a Markdown file with
// YAML frontmatter. It implements SchemaTool so it can be registered in the
// ToolRegistry and used by the Brain.
type MarkdownSkill struct {
	name        string
	description string
	schema      string // JSON Schema string built from frontmatter args
	body        string // Markdown body (system prompt / few-shot examples)
}

// Name returns the tool name from the frontmatter.
func (s *MarkdownSkill) Name() string { return s.name }

// Description returns the tool description from the frontmatter.
func (s *MarkdownSkill) Description() string { return s.description }

// Definition returns the JSON Schema string for the tool's arguments.
func (s *MarkdownSkill) Definition() string { return s.schema }

// Body returns the Markdown body (system prompt / few-shot examples).
func (s *MarkdownSkill) Body() string { return s.body }

// Call validates the arguments against the schema and returns the Markdown body
// as the tool result. This provides the LLM with the skill's system prompt or
// few-shot examples.
func (s *MarkdownSkill) Call(args json.RawMessage) (*domain.ToolResult, error) {
	if err := ValidateAgainstSchema(args, s.schema); err != nil {
		return nil, fmt.Errorf("skill %q input validation failed: %w", s.name, err)
	}
	return &domain.ToolResult{
		Data: s.body,
		Metadata: map[string]string{
			"skill": s.name,
		},
	}, nil
}

// =============================================================================
// ParseSkillFile — reads a single .md skill file
// =============================================================================

// ParseSkillFile reads a Markdown skill file from disk, parses its YAML
// frontmatter, and returns a MarkdownSkill ready for registration.
func ParseSkillFile(path string) (*MarkdownSkill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file %q: %w", path, err)
	}

	fm, body, err := ParseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill file %q: %w", path, err)
	}

	schema := BuildJSONSchema(fm.Args)

	return &MarkdownSkill{
		name:        fm.Name,
		description: fm.Description,
		schema:      schema,
		body:        body,
	}, nil
}

// =============================================================================
// LoadSkillsFromDir — loads all .md skill files from a directory
// =============================================================================

// LoadSkillsFromDir reads all .md files from the given directory, parses each
// as a skill definition, and returns the resulting MarkdownSkill slice. Non-.md
// files are ignored. Returns an error if the directory does not exist or any
// .md file fails to parse.
func LoadSkillsFromDir(dir string) ([]*MarkdownSkill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills directory %q: %w", dir, err)
	}

	var skills []*MarkdownSkill
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		skill, err := ParseSkillFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load skill from %q: %w", entry.Name(), err)
		}
		skills = append(skills, skill)
	}

	return skills, nil
}
