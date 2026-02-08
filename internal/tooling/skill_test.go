package tooling

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// ParseFrontmatter Tests
// =============================================================================

func TestParseFrontmatter_ShouldExtractNameAndDescription(t *testing.T) {
	content := `---
name: search
description: "Search the web for information"
---
You are a search assistant.
`
	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Name != "search" {
		t.Errorf("expected name 'search', got %q", fm.Name)
	}
	if fm.Description != "Search the web for information" {
		t.Errorf("expected description 'Search the web for information', got %q", fm.Description)
	}
	if body != "You are a search assistant." {
		t.Errorf("expected body 'You are a search assistant.', got %q", body)
	}
}

func TestParseFrontmatter_ShouldExtractArgs(t *testing.T) {
	content := `---
name: lookup
description: "Look up a term"
args:
  - name: query
    type: string
    description: "The search query"
    required: true
  - name: limit
    type: number
    description: "Max results"
    required: false
---
Body text here.
`
	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(fm.Args))
	}

	arg0 := fm.Args[0]
	if arg0.Name != "query" {
		t.Errorf("expected arg[0].Name='query', got %q", arg0.Name)
	}
	if arg0.Type != "string" {
		t.Errorf("expected arg[0].Type='string', got %q", arg0.Type)
	}
	if arg0.Description != "The search query" {
		t.Errorf("expected arg[0].Description, got %q", arg0.Description)
	}
	if !arg0.Required {
		t.Error("expected arg[0].Required=true")
	}

	arg1 := fm.Args[1]
	if arg1.Name != "limit" {
		t.Errorf("expected arg[1].Name='limit', got %q", arg1.Name)
	}
	if arg1.Required {
		t.Error("expected arg[1].Required=false")
	}
}

func TestParseFrontmatter_ShouldReturnErrorWhenNoFrontmatter(t *testing.T) {
	content := `Just some markdown without frontmatter.`
	_, _, err := ParseFrontmatter(content)
	if err == nil {
		t.Error("expected error when no frontmatter delimiters found")
	}
}

func TestParseFrontmatter_ShouldReturnErrorWhenMissingClosingDelimiter(t *testing.T) {
	content := `---
name: broken
description: "No closing delimiter"
`
	_, _, err := ParseFrontmatter(content)
	if err == nil {
		t.Error("expected error when closing --- is missing")
	}
}

func TestParseFrontmatter_ShouldReturnErrorWhenNameMissing(t *testing.T) {
	content := `---
description: "No name field"
---
Body.
`
	_, _, err := ParseFrontmatter(content)
	if err == nil {
		t.Error("expected error when name is missing from frontmatter")
	}
}

func TestParseFrontmatter_ShouldReturnErrorWhenDescriptionMissing(t *testing.T) {
	content := `---
name: orphan
---
Body.
`
	_, _, err := ParseFrontmatter(content)
	if err == nil {
		t.Error("expected error when description is missing from frontmatter")
	}
}

func TestParseFrontmatter_ShouldHandleEmptyBody(t *testing.T) {
	content := `---
name: minimal
description: "A minimal skill"
---
`
	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Name != "minimal" {
		t.Errorf("expected name 'minimal', got %q", fm.Name)
	}
	if body != "" {
		t.Errorf("expected empty body, got %q", body)
	}
}

func TestParseFrontmatter_ShouldHandleNoArgs(t *testing.T) {
	content := `---
name: greet
description: "Greet the user"
---
Hello!
`
	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Args != nil && len(fm.Args) != 0 {
		t.Errorf("expected nil or empty args, got %d", len(fm.Args))
	}
}

// =============================================================================
// BuildJSONSchema Tests
// =============================================================================

func TestBuildJSONSchema_ShouldReturnValidJSONWithArgs(t *testing.T) {
	args := []SkillArg{
		{Name: "query", Type: "string", Description: "The search query", Required: true},
		{Name: "limit", Type: "number", Description: "Max results", Required: false},
	}
	schema := BuildJSONSchema(args)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("schema should be valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("expected type 'object', got %v", parsed["type"])
	}

	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'properties' to be an object")
	}
	if _, exists := props["query"]; !exists {
		t.Error("expected 'query' in properties")
	}
	if _, exists := props["limit"]; !exists {
		t.Error("expected 'limit' in properties")
	}

	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("expected 'required' to be an array")
	}
	if len(required) != 1 || required[0] != "query" {
		t.Errorf("expected required=['query'], got %v", required)
	}
}

func TestBuildJSONSchema_ShouldReturnEmptyObjectWhenNoArgs(t *testing.T) {
	schema := BuildJSONSchema(nil)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("schema should be valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("expected type 'object', got %v", parsed["type"])
	}
}

func TestBuildJSONSchema_ShouldIncludeDescriptionsInProperties(t *testing.T) {
	args := []SkillArg{
		{Name: "input", Type: "string", Description: "The input text", Required: true},
	}
	schema := BuildJSONSchema(args)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("schema should be valid JSON: %v", err)
	}
	props := parsed["properties"].(map[string]interface{})
	inputProp := props["input"].(map[string]interface{})
	if inputProp["description"] != "The input text" {
		t.Errorf("expected description 'The input text', got %v", inputProp["description"])
	}
}

// =============================================================================
// MarkdownSkill SchemaTool Interface Tests
// =============================================================================

func TestMarkdownSkill_ShouldImplementSchemaTool(t *testing.T) {
	// Compile-time interface check
	var _ SchemaTool = (*MarkdownSkill)(nil)
}

func TestMarkdownSkill_Name_ShouldReturnSkillName(t *testing.T) {
	skill := &MarkdownSkill{
		name:        "search",
		description: "Search the web",
		schema:      `{"type":"object"}`,
		body:        "You are a search assistant.",
	}
	if skill.Name() != "search" {
		t.Errorf("expected 'search', got %q", skill.Name())
	}
}

func TestMarkdownSkill_Description_ShouldReturnSkillDescription(t *testing.T) {
	skill := &MarkdownSkill{
		name:        "search",
		description: "Search the web",
		schema:      `{"type":"object"}`,
		body:        "You are a search assistant.",
	}
	if skill.Description() != "Search the web" {
		t.Errorf("expected 'Search the web', got %q", skill.Description())
	}
}

func TestMarkdownSkill_Definition_ShouldReturnJSONSchema(t *testing.T) {
	skill := &MarkdownSkill{
		name:        "search",
		description: "Search the web",
		schema:      `{"type":"object","properties":{"q":{"type":"string"}}}`,
		body:        "prompt",
	}
	def := skill.Definition()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(def), &parsed); err != nil {
		t.Fatalf("Definition() should return valid JSON: %v", err)
	}
}

func TestMarkdownSkill_Call_ShouldReturnBodyAsResult(t *testing.T) {
	skill := &MarkdownSkill{
		name:        "greet",
		description: "Greet",
		schema:      `{"type":"object","properties":{}}`,
		body:        "Hello, I am your assistant!",
	}
	result, err := skill.Call(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data != "Hello, I am your assistant!" {
		t.Errorf("expected body in result, got %q", result.Data)
	}
}

func TestMarkdownSkill_Call_ShouldValidateArgsAgainstSchema(t *testing.T) {
	schema := `{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`
	skill := &MarkdownSkill{
		name:        "search",
		description: "Search",
		schema:      schema,
		body:        "Search prompt",
	}
	// Missing required field "query"
	_, err := skill.Call(json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected validation error when required field is missing")
	}
}

func TestMarkdownSkill_Call_ShouldSucceedWithValidArgs(t *testing.T) {
	schema := `{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`
	skill := &MarkdownSkill{
		name:        "search",
		description: "Search",
		schema:      schema,
		body:        "Search prompt",
	}
	result, err := skill.Call(json.RawMessage(`{"query":"test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data != "Search prompt" {
		t.Errorf("expected 'Search prompt', got %q", result.Data)
	}
}

func TestMarkdownSkill_Body_ShouldReturnMarkdownBody(t *testing.T) {
	skill := &MarkdownSkill{
		name:        "test",
		description: "Test",
		schema:      `{"type":"object"}`,
		body:        "# System Prompt\nYou are helpful.",
	}
	if skill.Body() != "# System Prompt\nYou are helpful." {
		t.Errorf("expected body content, got %q", skill.Body())
	}
}

// =============================================================================
// ParseSkillFile Tests (file I/O)
// =============================================================================

func writeSkillFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return path
}

func TestParseSkillFile_ShouldReturnMarkdownSkillFromValidFile(t *testing.T) {
	dir := t.TempDir()
	path := writeSkillFile(t, dir, "search.md", `---
name: search
description: "Search the web"
args:
  - name: query
    type: string
    description: "Search query"
    required: true
---
You are a search assistant. Find information for the user.
`)

	skill, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name() != "search" {
		t.Errorf("expected name 'search', got %q", skill.Name())
	}
	if skill.Description() != "Search the web" {
		t.Errorf("expected description 'Search the web', got %q", skill.Description())
	}
	if skill.Body() != "You are a search assistant. Find information for the user." {
		t.Errorf("unexpected body: %q", skill.Body())
	}

	// Schema should have required query arg
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(skill.Definition()), &schema); err != nil {
		t.Fatalf("schema should be valid JSON: %v", err)
	}
	required := schema["required"].([]interface{})
	if len(required) != 1 || required[0] != "query" {
		t.Errorf("expected required=['query'], got %v", required)
	}
}

func TestParseSkillFile_ShouldReturnErrorForNonexistentFile(t *testing.T) {
	_, err := ParseSkillFile("/nonexistent/path/skill.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseSkillFile_ShouldReturnErrorForInvalidFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := writeSkillFile(t, dir, "bad.md", `Just some text without frontmatter`)

	_, err := ParseSkillFile(path)
	if err == nil {
		t.Error("expected error for file without frontmatter")
	}
}

func TestParseSkillFile_ShouldReturnErrorForMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeSkillFile(t, dir, "malformed.md", `---
name: [invalid yaml
  - broken: {
---
Body
`)

	_, err := ParseSkillFile(path)
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}

// =============================================================================
// LoadSkillsFromDir Tests
// =============================================================================

func TestLoadSkillsFromDir_ShouldLoadAllMdFiles(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "search.md", `---
name: search
description: "Search the web"
---
Search prompt.
`)
	writeSkillFile(t, dir, "translate.md", `---
name: translate
description: "Translate text"
---
Translation prompt.
`)

	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name()] = true
	}
	if !names["search"] {
		t.Error("expected 'search' skill")
	}
	if !names["translate"] {
		t.Error("expected 'translate' skill")
	}
}

func TestLoadSkillsFromDir_ShouldIgnoreNonMdFiles(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "skill.md", `---
name: real_skill
description: "A real skill"
---
Prompt.
`)
	writeSkillFile(t, dir, "notes.txt", `Just a text file`)
	writeSkillFile(t, dir, "config.yaml", `key: value`)

	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (ignoring non-.md), got %d", len(skills))
	}
	if skills[0].Name() != "real_skill" {
		t.Errorf("expected 'real_skill', got %q", skills[0].Name())
	}
}

func TestLoadSkillsFromDir_ShouldReturnEmptyForEmptyDir(t *testing.T) {
	dir := t.TempDir()
	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadSkillsFromDir_ShouldReturnErrorForNonexistentDir(t *testing.T) {
	_, err := LoadSkillsFromDir("/nonexistent/skills/dir")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestLoadSkillsFromDir_ShouldReturnErrorForInvalidSkillFile(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "broken.md", `No frontmatter here`)

	_, err := LoadSkillsFromDir(dir)
	if err == nil {
		t.Error("expected error when a .md file has invalid frontmatter")
	}
}

func TestLoadSkillsFromDir_ShouldIgnoreSubdirectories(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "valid.md", `---
name: valid
description: "Valid skill"
---
Prompt.
`)
	// Create a subdirectory (should be ignored)
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (ignoring subdir), got %d", len(skills))
	}
}

func TestLoadSkillsFromDir_ShouldRegisterIntoToolRegistry(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "greet.md", `---
name: greet
description: "Greet the user"
---
Hello! How can I help you?
`)

	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reg := NewToolRegistry()
	for _, s := range skills {
		if err := reg.Register(s); err != nil {
			t.Fatalf("failed to register skill: %v", err)
		}
	}

	tool, err := reg.Get("greet")
	if err != nil {
		t.Fatalf("expected to find 'greet' in registry: %v", err)
	}
	if tool.Name() != "greet" {
		t.Errorf("expected name 'greet', got %q", tool.Name())
	}
	if tool.Description() != "Greet the user" {
		t.Errorf("expected description 'Greet the user', got %q", tool.Description())
	}
}

// =============================================================================
// End-to-End Integration: Load from file → register → call
// =============================================================================

func TestE2E_LoadSkillFile_RegisterAndCall_ShouldReturnBody(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "summarize.md", `---
name: summarize
description: "Summarize text into key points"
args:
  - name: text
    type: string
    description: "The text to summarize"
    required: true
  - name: max_points
    type: number
    description: "Maximum number of bullet points"
    required: false
---
# Summarization Skill

You are an expert summarizer. Given text, extract the key points.

## Few-Shot Example
Input: "The quick brown fox jumps over the lazy dog."
Output: "- A fox jumped over a dog."
`)

	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	reg := NewToolRegistry()
	if err := reg.Register(skills[0]); err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// Verify the tool appears in definitions for LLM
	defs := reg.Definitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if defs[0].Name != "summarize" {
		t.Errorf("expected definition name 'summarize', got %q", defs[0].Name)
	}

	// Call the skill with valid args
	tool, _ := reg.Get("summarize")
	result, err := tool.Call(json.RawMessage(`{"text":"Hello world"}`))
	if err != nil {
		t.Fatalf("unexpected error calling skill: %v", err)
	}
	if !strings.Contains(result.Data, "Summarization Skill") {
		t.Errorf("expected body to contain 'Summarization Skill', got %q", result.Data)
	}
	if !strings.Contains(result.Data, "Few-Shot Example") {
		t.Errorf("expected body to contain 'Few-Shot Example', got %q", result.Data)
	}

	// Call with missing required arg should fail
	_, err = tool.Call(json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected validation error when required 'text' arg is missing")
	}
}

func TestE2E_MultipleSkills_ShouldAllRegisterAndBeCallable(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "alpha.md", `---
name: alpha
description: "Alpha skill"
---
Alpha body.
`)
	writeSkillFile(t, dir, "beta.md", `---
name: beta
description: "Beta skill"
args:
  - name: input
    type: string
    description: "Input"
    required: true
---
Beta body.
`)

	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reg := NewToolRegistry()
	for _, s := range skills {
		if err := reg.Register(s); err != nil {
			t.Fatalf("failed to register %q: %v", s.Name(), err)
		}
	}

	// Both should be in the registry
	if len(reg.List()) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(reg.List()))
	}

	// Alpha (no args required)
	alpha, _ := reg.Get("alpha")
	result, err := alpha.Call(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("alpha call failed: %v", err)
	}
	if result.Data != "Alpha body." {
		t.Errorf("expected 'Alpha body.', got %q", result.Data)
	}

	// Beta (requires input)
	beta, _ := reg.Get("beta")
	_, err = beta.Call(json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected beta call to fail without required 'input'")
	}

	result, err = beta.Call(json.RawMessage(`{"input":"hello"}`))
	if err != nil {
		t.Fatalf("beta call with valid args failed: %v", err)
	}
	if result.Data != "Beta body." {
		t.Errorf("expected 'Beta body.', got %q", result.Data)
	}
}

// =============================================================================
// Edge Cases: Call metadata, multiline body, special characters
// =============================================================================

func TestMarkdownSkill_Call_ShouldIncludeSkillNameInMetadata(t *testing.T) {
	skill := &MarkdownSkill{
		name:        "test_skill",
		description: "Test",
		schema:      `{"type":"object","properties":{}}`,
		body:        "body",
	}
	result, err := skill.Call(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metadata["skill"] != "test_skill" {
		t.Errorf("expected metadata skill='test_skill', got %q", result.Metadata["skill"])
	}
}

func TestParseFrontmatter_ShouldPreserveMultilineBody(t *testing.T) {
	content := `---
name: multi
description: "Multiline body"
---
# Title

Paragraph one.

Paragraph two.

- Bullet one
- Bullet two
`
	_, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "# Title") {
		t.Error("expected body to contain '# Title'")
	}
	if !strings.Contains(body, "Paragraph one.") {
		t.Error("expected body to contain 'Paragraph one.'")
	}
	if !strings.Contains(body, "- Bullet two") {
		t.Error("expected body to contain '- Bullet two'")
	}
}

func TestParseFrontmatter_ShouldHandleSpecialCharsInDescription(t *testing.T) {
	content := `---
name: special
description: "Handle 'quotes' and \"escapes\" & symbols <>"
---
Body.
`
	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Name != "special" {
		t.Errorf("expected name 'special', got %q", fm.Name)
	}
}

func TestBuildJSONSchema_ShouldHandleMultipleRequiredArgs(t *testing.T) {
	args := []SkillArg{
		{Name: "a", Type: "string", Required: true},
		{Name: "b", Type: "string", Required: true},
		{Name: "c", Type: "string", Required: false},
	}
	schema := BuildJSONSchema(args)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("schema should be valid JSON: %v", err)
	}
	required := parsed["required"].([]interface{})
	if len(required) != 2 {
		t.Errorf("expected 2 required args, got %d", len(required))
	}
}

func TestBuildJSONSchema_ShouldOmitRequiredWhenNoneRequired(t *testing.T) {
	args := []SkillArg{
		{Name: "a", Type: "string", Required: false},
	}
	schema := BuildJSONSchema(args)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("schema should be valid JSON: %v", err)
	}
	if _, exists := parsed["required"]; exists {
		t.Error("expected no 'required' key when no args are required")
	}
}

func TestParseFrontmatter_ShouldReturnErrorForInvalidYAML(t *testing.T) {
	content := `---
name: bad
description: [
  unclosed bracket
---
Body.
`
	_, _, err := ParseFrontmatter(content)
	if err == nil {
		t.Error("expected error for invalid YAML syntax")
	}
}
