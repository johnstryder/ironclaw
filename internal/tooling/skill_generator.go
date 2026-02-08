package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ironclaw/internal/domain"
)

// =============================================================================
// Prompt Template
// =============================================================================

// skillPromptTemplate is the system prompt used to instruct the LLM to generate
// a valid Markdown skill definition file.
const skillPromptTemplate = `You are a skill definition generator for the Ironclaw agent framework.
Generate a valid Markdown skill file with YAML frontmatter.

The skill MUST follow this exact format:

---
name: <snake_case_name>
description: "<A concise description of what the skill does>"
args:
  - name: <arg_name>
    type: <string|number|boolean>
    description: "<What this argument is for>"
    required: <true|false>
---
<Markdown body with a system prompt, rules, and optional few-shot examples>

Requirements:
- The name field MUST be snake_case (e.g. fetch_weather, translate_text)
- The description must be concise and descriptive (one sentence)
- Include all relevant args the skill would need
- Each arg must have a type of "string", "number", or "boolean"
- The body MUST contain a clear system prompt for the LLM
- Optionally include rules and few-shot examples in the body
- Output ONLY the skill markdown file content — no commentary, no wrapping

Example of a valid skill file:

---
name: fetch_weather
description: "Fetch current weather data for a location"
args:
  - name: location
    type: string
    description: "City name or coordinates"
    required: true
  - name: units
    type: string
    description: "Temperature units: celsius or fahrenheit"
    required: false
---
# Weather Skill

You are a weather information assistant. Provide current weather data for the requested location.

## Rules
- Always include temperature, conditions, and humidity
- Default to celsius if units not specified
- If location is ambiguous, ask for clarification

## Few-Shot Example

**Input:** location="London", units="celsius"
**Output:** "London: 15°C, partly cloudy, humidity 72%"
`

// BuildSkillPrompt constructs the full prompt for the LLM to generate a skill
// definition. If skillName is provided, the LLM is instructed to use that name.
func BuildSkillPrompt(description string, skillName string) string {
	var sb strings.Builder
	sb.WriteString(skillPromptTemplate)
	sb.WriteString("\n")

	if skillName != "" {
		sb.WriteString(fmt.Sprintf("Use the name: %s\n\n", skillName))
	}

	sb.WriteString(fmt.Sprintf("Generate a skill for: %s\n", description))
	return sb.String()
}

// =============================================================================
// ExtractSkillMarkdown — parse LLM output
// =============================================================================

// ExtractSkillMarkdown extracts a valid skill Markdown definition from an LLM
// response. The LLM may wrap output in code blocks or add commentary; this
// function finds and returns just the frontmatter + body content.
func ExtractSkillMarkdown(response string) (string, error) {
	if response == "" {
		return "", fmt.Errorf("empty response from LLM")
	}

	trimmed := strings.TrimSpace(response)

	// Case 1: Response starts with --- (clean output)
	if strings.HasPrefix(trimmed, "---") {
		return trimmed, nil
	}

	// Case 2: Wrapped in a code block (```markdown or ```)
	// Find the code block and extract content
	codeBlockStart := -1
	for _, marker := range []string{"```markdown\n", "```md\n", "```yaml\n", "```\n"} {
		idx := strings.Index(trimmed, marker)
		if idx != -1 {
			codeBlockStart = idx + len(marker)
			break
		}
	}

	if codeBlockStart != -1 {
		rest := trimmed[codeBlockStart:]
		// Find closing ```
		closingIdx := strings.LastIndex(rest, "```")
		if closingIdx != -1 {
			rest = rest[:closingIdx]
		}
		content := strings.TrimSpace(rest)
		if strings.HasPrefix(content, "---") {
			return content, nil
		}
	}

	// Case 3: Frontmatter starts somewhere in the middle (after commentary)
	idx := strings.Index(trimmed, "\n---\n")
	if idx != -1 {
		content := strings.TrimSpace(trimmed[idx+1:])
		if strings.HasPrefix(content, "---") {
			return content, nil
		}
	}

	// Case 4: Line-by-line scan — catches edge cases like --- at end of string
	// without a trailing newline, where \n---\n cannot match.
	lines := strings.Split(trimmed, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			content := strings.TrimSpace(strings.Join(lines[i:], "\n"))
			if strings.HasPrefix(content, "---") {
				return content, nil
			}
		}
	}

	return "", fmt.Errorf("no valid skill frontmatter found in LLM response")
}

// =============================================================================
// SkillGenerator — implements SchemaTool
// =============================================================================

// SkillGeneratorInput is the JSON input structure for the generate_skill tool.
type SkillGeneratorInput struct {
	Description string `json:"description" jsonschema:"minLength=1"`
	SkillName   string `json:"skill_name,omitempty"`
}

// SkillGenerator generates new skill definitions using an LLM and saves them
// to the skills directory. It implements SchemaTool so the LLM can invoke it
// directly as a tool.
type SkillGenerator struct {
	skillsDir string
	registry  *ToolRegistry
	llm       domain.LLMProvider
	fs        FileSystem
}

// NewSkillGenerator creates a SkillGenerator with the given dependencies.
func NewSkillGenerator(skillsDir string, registry *ToolRegistry, llm domain.LLMProvider, fs FileSystem) *SkillGenerator {
	return &SkillGenerator{
		skillsDir: skillsDir,
		registry:  registry,
		llm:       llm,
		fs:        fs,
	}
}

// Name returns the tool name for function-calling.
func (sg *SkillGenerator) Name() string { return "generate_skill" }

// Description returns a human-readable description for the LLM.
func (sg *SkillGenerator) Description() string {
	return "Generate a new skill definition from a natural language description. The skill is saved to the skills directory and registered immediately."
}

// Definition returns the JSON Schema string for the tool's input.
func (sg *SkillGenerator) Definition() string {
	return GenerateSchema(SkillGeneratorInput{})
}

// sgUnmarshalFunc is the JSON unmarshaler used by Call. Package-level so tests
// can inject a failing unmarshaler to cover defense-in-depth error paths.
var sgUnmarshalFunc = json.Unmarshal

// Call executes the generate_skill tool with the given JSON arguments.
func (sg *SkillGenerator) Call(args json.RawMessage) (*domain.ToolResult, error) {
	// 1. Validate input against JSON schema
	schema := sg.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 2. Unmarshal input
	var input SkillGeneratorInput
	if err := sgUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	if input.Description == "" {
		return nil, fmt.Errorf("description must not be empty")
	}

	// 3. Generate the skill
	skill, err := sg.Generate(context.Background(), input.Description, input.SkillName)
	if err != nil {
		return nil, err
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Successfully generated and registered skill %q (%s)", skill.Name(), skill.Description()),
		Metadata: map[string]string{
			"skill_name":        skill.Name(),
			"skill_description": skill.Description(),
		},
	}, nil
}

// Generate builds a prompt, calls the LLM, parses the response, validates
// the generated skill, saves it to disk, and registers it in the ToolRegistry.
func (sg *SkillGenerator) Generate(ctx context.Context, description string, skillName string) (*MarkdownSkill, error) {
	if description == "" {
		return nil, fmt.Errorf("description must not be empty")
	}

	// 1. Build the prompt
	prompt := BuildSkillPrompt(description, skillName)

	// 2. Call the LLM
	response, err := sg.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// 3. Extract the skill markdown from the LLM response
	markdown, err := ExtractSkillMarkdown(response)
	if err != nil {
		return nil, fmt.Errorf("failed to extract skill from LLM response: %w", err)
	}

	// 4. Parse and validate the frontmatter
	fm, body, err := ParseFrontmatter(markdown)
	if err != nil {
		return nil, fmt.Errorf("generated skill has invalid frontmatter: %w", err)
	}

	// 5. Build the skill object
	schemaStr := BuildJSONSchema(fm.Args)
	skill := &MarkdownSkill{
		name:        fm.Name,
		description: fm.Description,
		schema:      schemaStr,
		body:        body,
	}

	// 6. Save to disk
	filename := fm.Name + ".md"
	destPath := filepath.Join(sg.skillsDir, filename)
	if err := sg.fs.WriteFile(destPath, markdown); err != nil {
		return nil, fmt.Errorf("failed to write skill file to %q: %w", destPath, err)
	}

	// 7. Register in the ToolRegistry
	if err := sg.registry.Register(skill); err != nil {
		return nil, fmt.Errorf("failed to register skill %q: %w", skill.Name(), err)
	}

	return skill, nil
}

// =============================================================================
// GenerateToFile — convenience for external callers (saves using os.WriteFile)
// =============================================================================

// GenerateToFile is a convenience wrapper that generates a skill and writes it
// directly to the skills directory using os.WriteFile (bypassing the FileSystem
// interface). Useful for CLI commands that don't inject a FileSystem.
func (sg *SkillGenerator) GenerateToFile(ctx context.Context, description string, skillName string) (string, error) {
	if description == "" {
		return "", fmt.Errorf("description must not be empty")
	}

	prompt := BuildSkillPrompt(description, skillName)

	response, err := sg.llm.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	markdown, err := ExtractSkillMarkdown(response)
	if err != nil {
		return "", fmt.Errorf("failed to extract skill from LLM response: %w", err)
	}

	fm, _, err := ParseFrontmatter(markdown)
	if err != nil {
		return "", fmt.Errorf("generated skill has invalid frontmatter: %w", err)
	}

	filename := fm.Name + ".md"
	destPath := filepath.Join(sg.skillsDir, filename)
	if err := os.WriteFile(destPath, []byte(markdown), 0644); err != nil {
		return "", fmt.Errorf("failed to write skill file: %w", err)
	}

	return destPath, nil
}
