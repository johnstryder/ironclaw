package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockLLMProvider is a test double for domain.LLMProvider.
type mockLLMProvider struct {
	response string
	err      error
}

func (m *mockLLMProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}

// spyLLMProvider captures the prompt passed to Generate.
type spyLLMProvider struct {
	capturedPrompt string
	response       string
	err            error
}

func (s *spyLLMProvider) Generate(ctx context.Context, prompt string) (string, error) {
	s.capturedPrompt = prompt
	return s.response, s.err
}

// captureWriteFS records the path and content of WriteFile calls.
type captureWriteFS struct {
	writtenPath    string
	writtenContent string
	writeErr       error
}

func (c *captureWriteFS) ReadDir(path string) ([]DirEntry, error) { return nil, nil }
func (c *captureWriteFS) ReadFile(path string) (string, error)    { return "", nil }
func (c *captureWriteFS) WriteFile(path string, content string) error {
	c.writtenPath = path
	c.writtenContent = content
	return c.writeErr
}

// =============================================================================
// BuildSkillPrompt Tests
// =============================================================================

func TestBuildSkillPrompt_ShouldContainUserDescription(t *testing.T) {
	prompt := BuildSkillPrompt("a skill that fetches the weather", "")
	if !strings.Contains(prompt, "a skill that fetches the weather") {
		t.Error("expected prompt to contain the user's description")
	}
}

func TestBuildSkillPrompt_ShouldContainFormatInstructions(t *testing.T) {
	prompt := BuildSkillPrompt("any skill", "")
	if !strings.Contains(prompt, "---") {
		t.Error("expected prompt to contain frontmatter delimiter instructions")
	}
	if !strings.Contains(prompt, "name:") {
		t.Error("expected prompt to contain 'name:' field instruction")
	}
	if !strings.Contains(prompt, "description:") {
		t.Error("expected prompt to contain 'description:' field instruction")
	}
	if !strings.Contains(prompt, "args:") {
		t.Error("expected prompt to contain 'args:' field instruction")
	}
}

func TestBuildSkillPrompt_ShouldIncludeSkillNameWhenProvided(t *testing.T) {
	prompt := BuildSkillPrompt("fetch weather data", "weather_fetch")
	if !strings.Contains(prompt, "weather_fetch") {
		t.Error("expected prompt to include the specified skill name")
	}
}

func TestBuildSkillPrompt_ShouldNotIncludeNameInstructionWhenEmpty(t *testing.T) {
	prompt := BuildSkillPrompt("fetch weather data", "")
	// Should NOT contain a "Use the name:" directive when no name provided
	if strings.Contains(prompt, "Use the name:") {
		t.Error("expected prompt to not contain name directive when skill name is empty")
	}
}

func TestBuildSkillPrompt_ShouldContainFewShotExample(t *testing.T) {
	prompt := BuildSkillPrompt("any skill", "")
	// Should contain at least one complete example with frontmatter
	if !strings.Contains(prompt, "Example") {
		t.Error("expected prompt to contain a few-shot example")
	}
}

func TestBuildSkillPrompt_ShouldMentionSnakeCase(t *testing.T) {
	prompt := BuildSkillPrompt("any skill", "")
	if !strings.Contains(prompt, "snake_case") {
		t.Error("expected prompt to instruct the LLM to use snake_case for names")
	}
}

// =============================================================================
// ExtractSkillMarkdown Tests
// =============================================================================

func TestExtractSkillMarkdown_ShouldReturnContentStartingWithFrontmatter(t *testing.T) {
	input := `---
name: weather
description: "Fetch weather"
---
Body text.
`
	result, err := ExtractSkillMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result, "---") {
		t.Errorf("expected result to start with ---, got %q", result[:10])
	}
}

func TestExtractSkillMarkdown_ShouldExtractFromCodeBlock(t *testing.T) {
	input := "Here is the skill:\n```markdown\n---\nname: weather\ndescription: \"Fetch weather\"\n---\nBody.\n```\n"
	result, err := ExtractSkillMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result, "---") {
		t.Errorf("expected result to start with ---, got %q", result)
	}
	if strings.Contains(result, "```") {
		t.Error("expected result to not contain code block markers")
	}
}

func TestExtractSkillMarkdown_ShouldExtractFromCodeBlockWithoutLangTag(t *testing.T) {
	input := "Here is the skill:\n```\n---\nname: weather\ndescription: \"Fetch weather\"\n---\nBody.\n```\n"
	result, err := ExtractSkillMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result, "---") {
		t.Errorf("expected result to start with ---, got %q", result)
	}
}

func TestExtractSkillMarkdown_ShouldExtractWhenPrefixedWithText(t *testing.T) {
	input := "Sure! Here's the skill definition:\n\n---\nname: weather\ndescription: \"Fetch weather\"\n---\nBody.\n"
	result, err := ExtractSkillMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result, "---") {
		t.Errorf("expected result to start with ---, got %q", result)
	}
}

func TestExtractSkillMarkdown_ShouldReturnErrorWhenNoFrontmatterFound(t *testing.T) {
	input := "This is just some text with no skill definition."
	_, err := ExtractSkillMarkdown(input)
	if err == nil {
		t.Error("expected error when no frontmatter found in response")
	}
}

func TestExtractSkillMarkdown_ShouldReturnErrorForEmptyInput(t *testing.T) {
	_, err := ExtractSkillMarkdown("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestExtractSkillMarkdown_ShouldPreserveBodyContent(t *testing.T) {
	input := `---
name: test
description: "Test"
---
# System Prompt

You are a helpful assistant.

## Rules
- Be accurate
- Be concise
`
	result, err := ExtractSkillMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "# System Prompt") {
		t.Error("expected result to contain body header")
	}
	if !strings.Contains(result, "- Be accurate") {
		t.Error("expected result to preserve body content")
	}
}

func TestExtractSkillMarkdown_ShouldStripTrailingCodeBlockMarker(t *testing.T) {
	input := "```markdown\n---\nname: test\ndescription: \"Test\"\n---\nBody.\n```"
	result, err := ExtractSkillMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasSuffix(strings.TrimSpace(result), "```") {
		t.Error("expected result to strip trailing code block marker")
	}
}

func TestExtractSkillMarkdown_ShouldFindFrontmatterViaLineScanFallback(t *testing.T) {
	// The line scanner is the final fallback. It's reached when:
	// 1. Content doesn't start with ---
	// 2. No code blocks found
	// 3. \n---\n pattern not found
	// This happens when --- is at the very end of input with no trailing newline.
	input := "Here is the generated skill output\n---"
	result, err := ExtractSkillMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result, "---") {
		t.Errorf("expected result to start with ---, got %q", result)
	}
}

// =============================================================================
// NewSkillGenerator Tests
// =============================================================================

func TestNewSkillGenerator_ShouldReturnNonNilGenerator(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	if gen == nil {
		t.Fatal("expected non-nil generator")
	}
}

func TestNewSkillGenerator_ShouldStoreSkillsDir(t *testing.T) {
	dir := t.TempDir()
	gen := NewSkillGenerator(dir, NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	if gen.skillsDir != dir {
		t.Errorf("expected skillsDir %q, got %q", dir, gen.skillsDir)
	}
}

func TestNewSkillGenerator_ShouldStoreRegistry(t *testing.T) {
	reg := NewToolRegistry()
	gen := NewSkillGenerator(t.TempDir(), reg, &mockLLMProvider{}, &captureWriteFS{})
	if gen.registry != reg {
		t.Error("expected generator to store the provided registry")
	}
}

func TestNewSkillGenerator_ShouldStoreLLM(t *testing.T) {
	llm := &mockLLMProvider{}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})
	if gen.llm != llm {
		t.Error("expected generator to store the provided LLM provider")
	}
}

func TestNewSkillGenerator_ShouldStoreFileSystem(t *testing.T) {
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, fs)
	if gen.fs != fs {
		t.Error("expected generator to store the provided filesystem")
	}
}

// =============================================================================
// SkillGenerator.Generate Tests (full pipeline with mocked LLM)
// =============================================================================

const validGeneratedSkill = `---
name: weather_fetch
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
`

func TestGenerate_ShouldReturnParsedSkillFromLLMResponse(t *testing.T) {
	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, fs)

	skill, err := gen.Generate(context.Background(), "a skill that fetches weather data", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name() != "weather_fetch" {
		t.Errorf("expected skill name 'weather_fetch', got %q", skill.Name())
	}
	if skill.Description() != "Fetch current weather data for a location" {
		t.Errorf("expected description, got %q", skill.Description())
	}
}

func TestGenerate_ShouldWriteSkillFileToSkillsDir(t *testing.T) {
	dir := t.TempDir()
	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(dir, NewToolRegistry(), llm, fs)

	_, err := gen.Generate(context.Background(), "fetch weather", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := filepath.Join(dir, "weather_fetch.md")
	if fs.writtenPath != expectedPath {
		t.Errorf("expected file written to %q, got %q", expectedPath, fs.writtenPath)
	}
}

func TestGenerate_ShouldWriteCorrectContent(t *testing.T) {
	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, fs)

	_, err := gen.Generate(context.Background(), "fetch weather", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(fs.writtenContent, "---") {
		t.Error("expected written content to start with frontmatter")
	}
	if !strings.Contains(fs.writtenContent, "weather_fetch") {
		t.Error("expected written content to contain skill name")
	}
}

func TestGenerate_ShouldRegisterSkillInRegistry(t *testing.T) {
	reg := NewToolRegistry()
	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(t.TempDir(), reg, llm, fs)

	_, err := gen.Generate(context.Background(), "fetch weather", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool, err := reg.Get("weather_fetch")
	if err != nil {
		t.Fatalf("expected skill to be registered: %v", err)
	}
	if tool.Name() != "weather_fetch" {
		t.Errorf("expected 'weather_fetch', got %q", tool.Name())
	}
}

func TestGenerate_ShouldPassDescriptionToLLM(t *testing.T) {
	spy := &spyLLMProvider{response: validGeneratedSkill}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), spy, &captureWriteFS{})

	_, err := gen.Generate(context.Background(), "a skill that fetches weather data", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(spy.capturedPrompt, "a skill that fetches weather data") {
		t.Error("expected LLM prompt to contain the user's description")
	}
}

func TestGenerate_ShouldPassSkillNameToLLMWhenProvided(t *testing.T) {
	spy := &spyLLMProvider{response: validGeneratedSkill}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), spy, &captureWriteFS{})

	_, err := gen.Generate(context.Background(), "fetch weather data", "weather_fetch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(spy.capturedPrompt, "weather_fetch") {
		t.Error("expected LLM prompt to contain the specified skill name")
	}
}

func TestGenerate_ShouldReturnErrorForEmptyDescription(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})

	_, err := gen.Generate(context.Background(), "", "")
	if err == nil {
		t.Error("expected error for empty description")
	}
}

func TestGenerate_ShouldReturnErrorWhenLLMFails(t *testing.T) {
	llm := &mockLLMProvider{err: fmt.Errorf("LLM service unavailable")}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	_, err := gen.Generate(context.Background(), "any skill", "")
	if err == nil {
		t.Error("expected error when LLM fails")
	}
	if !strings.Contains(err.Error(), "LLM generation failed") {
		t.Errorf("expected 'LLM generation failed' in error, got %q", err.Error())
	}
}

func TestGenerate_ShouldReturnErrorWhenLLMReturnsInvalidContent(t *testing.T) {
	llm := &mockLLMProvider{response: "Sorry, I can't generate that skill."}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	_, err := gen.Generate(context.Background(), "any skill", "")
	if err == nil {
		t.Error("expected error when LLM returns invalid content")
	}
	if !strings.Contains(err.Error(), "failed to extract skill") {
		t.Errorf("expected 'failed to extract skill' in error, got %q", err.Error())
	}
}

func TestGenerate_ShouldReturnErrorWhenLLMReturnsBadFrontmatter(t *testing.T) {
	badSkill := `---
name: [invalid yaml structure
---
Body.
`
	llm := &mockLLMProvider{response: badSkill}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	_, err := gen.Generate(context.Background(), "any skill", "")
	if err == nil {
		t.Error("expected error when LLM returns skill with bad frontmatter")
	}
	if !strings.Contains(err.Error(), "invalid frontmatter") {
		t.Errorf("expected 'invalid frontmatter' in error, got %q", err.Error())
	}
}

func TestGenerate_ShouldReturnErrorWhenWriteFails(t *testing.T) {
	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{writeErr: fmt.Errorf("disk full")}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, fs)

	_, err := gen.Generate(context.Background(), "any skill", "")
	if err == nil {
		t.Error("expected error when file write fails")
	}
	if !strings.Contains(err.Error(), "failed to write skill file") {
		t.Errorf("expected 'failed to write skill file' in error, got %q", err.Error())
	}
}

func TestGenerate_ShouldReturnErrorWhenRegistryRejectsDuplicate(t *testing.T) {
	reg := NewToolRegistry()
	// Pre-register a skill with the same name
	reg.Register(&MarkdownSkill{
		name:        "weather_fetch",
		description: "Existing",
		schema:      `{"type":"object","properties":{}}`,
		body:        "body",
	})

	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(t.TempDir(), reg, llm, fs)

	_, err := gen.Generate(context.Background(), "fetch weather", "")
	if err == nil {
		t.Error("expected error when registry rejects duplicate skill")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected 'already registered' in error, got %q", err.Error())
	}
}

func TestGenerate_ShouldHandleLLMResponseWrappedInCodeBlock(t *testing.T) {
	wrappedResponse := "Here is the generated skill:\n```markdown\n" + validGeneratedSkill + "\n```\n"
	llm := &mockLLMProvider{response: wrappedResponse}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	skill, err := gen.Generate(context.Background(), "fetch weather", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name() != "weather_fetch" {
		t.Errorf("expected 'weather_fetch', got %q", skill.Name())
	}
}

func TestGenerate_ShouldReturnSkillWithValidJSONSchema(t *testing.T) {
	llm := &mockLLMProvider{response: validGeneratedSkill}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	skill, err := gen.Generate(context.Background(), "fetch weather", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(skill.Definition()), &schema); err != nil {
		t.Fatalf("skill schema should be valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", schema["type"])
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'properties' in schema")
	}
	if _, exists := props["location"]; !exists {
		t.Error("expected 'location' in schema properties")
	}
}

func TestGenerate_ShouldReturnSkillWithBody(t *testing.T) {
	llm := &mockLLMProvider{response: validGeneratedSkill}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	skill, err := gen.Generate(context.Background(), "fetch weather", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(skill.Body(), "Weather Skill") {
		t.Errorf("expected body to contain 'Weather Skill', got %q", skill.Body())
	}
}

// =============================================================================
// SchemaTool Interface Tests — Name, Description, Definition, Call
// =============================================================================

func TestSkillGenerator_ShouldImplementSchemaTool(t *testing.T) {
	var _ SchemaTool = (*SkillGenerator)(nil)
}

func TestSkillGenerator_Name_ShouldReturnGenerateSkill(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	if gen.Name() != "generate_skill" {
		t.Errorf("expected 'generate_skill', got %q", gen.Name())
	}
}

func TestSkillGenerator_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	desc := gen.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
	if !strings.Contains(strings.ToLower(desc), "skill") {
		t.Errorf("expected description to mention 'skill', got %q", desc)
	}
}

func TestSkillGenerator_Definition_ShouldReturnValidJSONSchema(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	def := gen.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(def), &parsed); err != nil {
		t.Fatalf("Definition() should return valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("expected type 'object', got %v", parsed["type"])
	}
}

func TestSkillGenerator_Definition_ShouldRequireDescription(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	def := gen.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(def), &parsed); err != nil {
		t.Fatalf("Definition() should return valid JSON: %v", err)
	}

	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'properties' in schema")
	}
	if _, exists := props["description"]; !exists {
		t.Error("expected 'description' property in schema")
	}

	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("expected 'required' array in schema")
	}
	found := false
	for _, r := range required {
		if r.(string) == "description" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'description' in required fields")
	}
}

func TestSkillGenerator_Definition_ShouldIncludeSkillNameProperty(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	def := gen.Definition()

	var parsed map[string]interface{}
	json.Unmarshal([]byte(def), &parsed)
	props := parsed["properties"].(map[string]interface{})
	if _, exists := props["skill_name"]; !exists {
		t.Error("expected 'skill_name' property in schema")
	}
}

func TestSkillGenerator_Call_ShouldGenerateAndReturnResult(t *testing.T) {
	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, fs)

	result, err := gen.Call(json.RawMessage(`{"description":"fetch weather data"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !strings.Contains(result.Data, "weather_fetch") {
		t.Errorf("expected result to contain skill name, got %q", result.Data)
	}
}

func TestSkillGenerator_Call_ShouldReturnMetadata(t *testing.T) {
	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, fs)

	result, err := gen.Call(json.RawMessage(`{"description":"fetch weather data"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metadata["skill_name"] != "weather_fetch" {
		t.Errorf("expected metadata skill_name='weather_fetch', got %q", result.Metadata["skill_name"])
	}
	if result.Metadata["skill_description"] != "Fetch current weather data for a location" {
		t.Errorf("expected metadata skill_description, got %q", result.Metadata["skill_description"])
	}
}

func TestSkillGenerator_Call_ShouldRejectInvalidJSON(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})

	_, err := gen.Call(json.RawMessage(`{bad json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("expected 'input validation failed' in error, got %q", err.Error())
	}
}

func TestSkillGenerator_Call_ShouldRejectMissingDescription(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})

	_, err := gen.Call(json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error when description is missing")
	}
}

func TestSkillGenerator_Call_ShouldRejectEmptyDescription(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})

	_, err := gen.Call(json.RawMessage(`{"description":""}`))
	if err == nil {
		t.Error("expected error for empty description")
	}
}

func TestSkillGenerator_Call_ShouldPassOptionalSkillName(t *testing.T) {
	spy := &spyLLMProvider{response: validGeneratedSkill}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), spy, &captureWriteFS{})

	_, err := gen.Call(json.RawMessage(`{"description":"fetch weather","skill_name":"weather_fetch"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(spy.capturedPrompt, "weather_fetch") {
		t.Error("expected skill_name to be passed to LLM prompt")
	}
}

func TestSkillGenerator_Call_ShouldForwardLLMErrors(t *testing.T) {
	llm := &mockLLMProvider{err: fmt.Errorf("LLM unavailable")}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	_, err := gen.Call(json.RawMessage(`{"description":"any skill"}`))
	if err == nil {
		t.Error("expected error when LLM fails")
	}
}

// =============================================================================
// SkillGenerator.Call — Unmarshal error path (defense-in-depth)
// =============================================================================

func TestSkillGenerator_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := sgUnmarshalFunc
	sgUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { sgUnmarshalFunc = original }()

	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	_, err := gen.Call(json.RawMessage(`{"description":"test"}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

func TestSkillGenerator_Call_ShouldReturnErrorWhenDescriptionEmptyDefenseInDepth(t *testing.T) {
	// Bypass schema validation to reach the defense-in-depth empty description check
	original := sgUnmarshalFunc
	sgUnmarshalFunc = func(data []byte, v interface{}) error {
		input, ok := v.(*SkillGeneratorInput)
		if !ok {
			return fmt.Errorf("unexpected type")
		}
		input.Description = "" // Force empty description past schema validation
		return nil
	}
	defer func() { sgUnmarshalFunc = original }()

	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	_, err := gen.Call(json.RawMessage(`{"description":"test"}`))
	if err == nil {
		t.Fatal("Expected error for empty description (defense-in-depth)")
	}
	if !strings.Contains(err.Error(), "description must not be empty") {
		t.Errorf("Expected 'description must not be empty' in error, got: %v", err)
	}
}

// =============================================================================
// E2E Integration: Generate → validate → save → register → call
// =============================================================================

func TestE2E_GenerateSkill_ShouldBeCallableAfterGeneration(t *testing.T) {
	reg := NewToolRegistry()
	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(t.TempDir(), reg, llm, fs)

	// Step 1: Generate the skill
	skill, err := gen.Generate(context.Background(), "fetch weather data", "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if skill.Name() != "weather_fetch" {
		t.Errorf("expected 'weather_fetch', got %q", skill.Name())
	}

	// Step 2: Verify it's in the registry
	tool, err := reg.Get("weather_fetch")
	if err != nil {
		t.Fatalf("expected skill in registry: %v", err)
	}

	// Step 3: Call it with valid args
	result, err := tool.Call(json.RawMessage(`{"location":"London"}`))
	if err != nil {
		t.Fatalf("unexpected error calling skill: %v", err)
	}
	if !strings.Contains(result.Data, "Weather Skill") {
		t.Errorf("expected body to contain 'Weather Skill', got %q", result.Data)
	}

	// Step 4: Verify metadata
	if result.Metadata["skill"] != "weather_fetch" {
		t.Errorf("expected metadata skill='weather_fetch', got %q", result.Metadata["skill"])
	}
}

func TestE2E_GenerateSkill_ViaCallInterface_ShouldMakeSkillCallable(t *testing.T) {
	reg := NewToolRegistry()
	llm := &mockLLMProvider{response: validGeneratedSkill}
	fs := &captureWriteFS{}
	gen := NewSkillGenerator(t.TempDir(), reg, llm, fs)

	// Use the SchemaTool.Call interface
	result, err := gen.Call(json.RawMessage(`{"description":"fetch weather"}`))
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if !strings.Contains(result.Data, "weather_fetch") {
		t.Errorf("expected result to mention skill name, got %q", result.Data)
	}

	// Verify the generated skill is callable
	tool, err := reg.Get("weather_fetch")
	if err != nil {
		t.Fatalf("expected skill in registry: %v", err)
	}

	callResult, err := tool.Call(json.RawMessage(`{"location":"Tokyo"}`))
	if err != nil {
		t.Fatalf("unexpected error calling generated skill: %v", err)
	}
	if !strings.Contains(callResult.Data, "Weather Skill") {
		t.Errorf("expected body content, got %q", callResult.Data)
	}
}

func TestE2E_GenerateSkill_ShouldAppearInRegistryDefinitions(t *testing.T) {
	reg := NewToolRegistry()
	llm := &mockLLMProvider{response: validGeneratedSkill}
	gen := NewSkillGenerator(t.TempDir(), reg, llm, &captureWriteFS{})

	_, err := gen.Generate(context.Background(), "fetch weather", "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	defs := reg.Definitions()
	found := false
	for _, d := range defs {
		if d.Name == "weather_fetch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'weather_fetch' in registry definitions for LLM")
	}
}

func TestE2E_GenerateSkillMinimal_NoArgs(t *testing.T) {
	minimalSkill := `---
name: hello_world
description: "Say hello to the world"
---
# Hello Skill

Simply greet the world warmly.
`
	reg := NewToolRegistry()
	llm := &mockLLMProvider{response: minimalSkill}
	gen := NewSkillGenerator(t.TempDir(), reg, llm, &captureWriteFS{})

	skill, err := gen.Generate(context.Background(), "a simple hello skill", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name() != "hello_world" {
		t.Errorf("expected 'hello_world', got %q", skill.Name())
	}

	// Call with empty args (no args defined)
	tool, _ := reg.Get("hello_world")
	result, err := tool.Call(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Data, "Hello Skill") {
		t.Errorf("expected body content, got %q", result.Data)
	}
}

// =============================================================================
// GenerateToFile Tests (os.WriteFile path)
// =============================================================================

func TestGenerateToFile_ShouldWriteToSkillsDir(t *testing.T) {
	dir := t.TempDir()
	llm := &mockLLMProvider{response: validGeneratedSkill}
	gen := NewSkillGenerator(dir, NewToolRegistry(), llm, &captureWriteFS{})

	path, err := gen.GenerateToFile(context.Background(), "fetch weather", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "weather_fetch.md")
	if path != expected {
		t.Errorf("expected path %q, got %q", expected, path)
	}

	// Verify file was actually written
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if !strings.Contains(string(data), "weather_fetch") {
		t.Error("expected file to contain skill name")
	}
}

func TestGenerateToFile_ShouldReturnErrorForEmptyDescription(t *testing.T) {
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), &mockLLMProvider{}, &captureWriteFS{})
	_, err := gen.GenerateToFile(context.Background(), "", "")
	if err == nil {
		t.Error("expected error for empty description")
	}
}

func TestGenerateToFile_ShouldReturnErrorWhenLLMFails(t *testing.T) {
	llm := &mockLLMProvider{err: fmt.Errorf("LLM unavailable")}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	_, err := gen.GenerateToFile(context.Background(), "fetch weather", "")
	if err == nil {
		t.Error("expected error when LLM fails")
	}
}

func TestGenerateToFile_ShouldReturnErrorWhenLLMReturnsInvalidContent(t *testing.T) {
	llm := &mockLLMProvider{response: "not a skill"}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	_, err := gen.GenerateToFile(context.Background(), "fetch weather", "")
	if err == nil {
		t.Error("expected error when LLM returns invalid content")
	}
}

func TestGenerateToFile_ShouldReturnErrorWhenFrontmatterInvalid(t *testing.T) {
	badSkill := "---\nname: [broken\n---\nBody.\n"
	llm := &mockLLMProvider{response: badSkill}
	gen := NewSkillGenerator(t.TempDir(), NewToolRegistry(), llm, &captureWriteFS{})

	_, err := gen.GenerateToFile(context.Background(), "fetch weather", "")
	if err == nil {
		t.Error("expected error when frontmatter is invalid")
	}
}

func TestGenerateToFile_ShouldReturnErrorWhenDirNotWritable(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0444)
	defer os.Chmod(dir, 0755)

	llm := &mockLLMProvider{response: validGeneratedSkill}
	gen := NewSkillGenerator(dir, NewToolRegistry(), llm, &captureWriteFS{})

	_, err := gen.GenerateToFile(context.Background(), "fetch weather", "")
	if err == nil {
		t.Error("expected error when directory is not writable")
	}
}

func TestGenerateToFile_ShouldProduceLoadableSkill(t *testing.T) {
	dir := t.TempDir()
	llm := &mockLLMProvider{response: validGeneratedSkill}
	gen := NewSkillGenerator(dir, NewToolRegistry(), llm, &captureWriteFS{})

	path, err := gen.GenerateToFile(context.Background(), "fetch weather", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The generated file should be parseable by ParseSkillFile
	skill, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("generated file should be parseable: %v", err)
	}
	if skill.Name() != "weather_fetch" {
		t.Errorf("expected 'weather_fetch', got %q", skill.Name())
	}

	// And loadable by LoadSkillsFromDir
	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("skills should be loadable from dir: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*SkillGenerator)(nil)
var _ domain.LLMProvider = (*mockLLMProvider)(nil)
var _ domain.LLMProvider = (*spyLLMProvider)(nil)
var _ FileSystem = (*captureWriteFS)(nil)
