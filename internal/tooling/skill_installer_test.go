package tooling

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// stubFetcher — test double for HTTPFetcher (SkillInstaller-specific)
// =============================================================================

type stubFetcher struct {
	data []byte
	err  error
}

func (s *stubFetcher) Fetch(url string) ([]byte, error) {
	return s.data, s.err
}

// =============================================================================
// NewSkillInstaller Tests
// =============================================================================

func TestNewSkillInstaller_ShouldReturnValidInstaller(t *testing.T) {
	dir := t.TempDir()
	reg := NewToolRegistry()
	installer := NewSkillInstaller(dir, reg, &stubFetcher{})
	if installer == nil {
		t.Fatal("expected non-nil installer")
	}
}

func TestNewSkillInstaller_ShouldStoreSkillsDir(t *testing.T) {
	dir := t.TempDir()
	reg := NewToolRegistry()
	installer := NewSkillInstaller(dir, reg, &stubFetcher{})
	if installer.skillsDir != dir {
		t.Errorf("expected skillsDir %q, got %q", dir, installer.skillsDir)
	}
}

func TestNewSkillInstaller_ShouldStoreRegistry(t *testing.T) {
	dir := t.TempDir()
	reg := NewToolRegistry()
	installer := NewSkillInstaller(dir, reg, &stubFetcher{})
	if installer.registry != reg {
		t.Error("expected installer to store the provided registry")
	}
}

func TestNewSkillInstaller_ShouldStoreFetcher(t *testing.T) {
	dir := t.TempDir()
	reg := NewToolRegistry()
	fetcher := &stubFetcher{}
	installer := NewSkillInstaller(dir, reg, fetcher)
	if installer.fetcher == nil {
		t.Error("expected installer to store the provided fetcher")
	}
}

// =============================================================================
// isURL Tests
// =============================================================================

func TestIsURL_ShouldReturnTrueForHTTP(t *testing.T) {
	if !isURL("http://example.com/skill.md") {
		t.Error("expected true for http:// URL")
	}
}

func TestIsURL_ShouldReturnTrueForHTTPS(t *testing.T) {
	if !isURL("https://raw.githubusercontent.com/user/repo/main/skill.md") {
		t.Error("expected true for https:// URL")
	}
}

func TestIsURL_ShouldReturnFalseForLocalPath(t *testing.T) {
	if isURL("/home/user/skills/greet.md") {
		t.Error("expected false for absolute local path")
	}
}

func TestIsURL_ShouldReturnFalseForRelativePath(t *testing.T) {
	if isURL("skills/greet.md") {
		t.Error("expected false for relative local path")
	}
}

func TestIsURL_ShouldReturnFalseForEmptyString(t *testing.T) {
	if isURL("") {
		t.Error("expected false for empty string")
	}
}

// =============================================================================
// Install from local path Tests
// =============================================================================

const validSkillContent = `---
name: test_skill
description: "A test skill"
args:
  - name: input
    type: string
    description: "Test input"
    required: true
---
# Test Skill

This is a test skill body.
`

func writeTestSkillFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test skill file: %v", err)
	}
	return path
}

func TestInstall_ShouldCopyLocalFileToSkillsDir(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "test_skill.md", validSkillContent)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.Install(sourcePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should exist in skills dir
	destPath := filepath.Join(skillsDir, "test_skill.md")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("expected skill file to be copied to skills directory")
	}
}

func TestInstall_ShouldReturnParsedSkillFromLocalFile(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "test_skill.md", validSkillContent)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	skill, err := installer.Install(sourcePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name() != "test_skill" {
		t.Errorf("expected skill name 'test_skill', got %q", skill.Name())
	}
	if skill.Description() != "A test skill" {
		t.Errorf("expected description 'A test skill', got %q", skill.Description())
	}
}

func TestInstall_ShouldRegisterSkillInRegistry(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "test_skill.md", validSkillContent)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.Install(sourcePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Skill should be in the registry
	tool, err := reg.Get("test_skill")
	if err != nil {
		t.Fatalf("expected skill to be registered: %v", err)
	}
	if tool.Name() != "test_skill" {
		t.Errorf("expected 'test_skill', got %q", tool.Name())
	}
}

func TestInstall_ShouldReturnErrorForNonexistentLocalFile(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.Install("/nonexistent/path/skill.md")
	if err == nil {
		t.Error("expected error for nonexistent local file")
	}
}

func TestInstall_ShouldReturnErrorForInvalidSkillFile(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "bad.md", "no frontmatter here")

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.Install(sourcePath)
	if err == nil {
		t.Error("expected error for invalid skill file")
	}
}

func TestInstall_ShouldReturnErrorForEmptySource(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.Install("")
	if err == nil {
		t.Error("expected error for empty source")
	}
}

func TestInstall_ShouldUseFilenameFromSourcePath(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "my_custom_skill.md", validSkillContent)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.Install(sourcePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use the original filename
	destPath := filepath.Join(skillsDir, "my_custom_skill.md")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("expected file to keep original filename in skills dir")
	}
}

// =============================================================================
// Install from URL Tests
// =============================================================================

func TestInstall_ShouldDownloadFromURLAndSave(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	fetcher := &stubFetcher{data: []byte(validSkillContent)}
	installer := NewSkillInstaller(skillsDir, reg, fetcher)

	skill, err := installer.Install("https://raw.githubusercontent.com/user/repo/main/test_skill.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name() != "test_skill" {
		t.Errorf("expected skill name 'test_skill', got %q", skill.Name())
	}

	// File should exist in skills dir
	destPath := filepath.Join(skillsDir, "test_skill.md")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("expected skill file to be saved to skills directory")
	}
}

func TestInstall_ShouldRegisterSkillFromURL(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	fetcher := &stubFetcher{data: []byte(validSkillContent)}
	installer := NewSkillInstaller(skillsDir, reg, fetcher)

	_, err := installer.Install("https://example.com/test_skill.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool, err := reg.Get("test_skill")
	if err != nil {
		t.Fatalf("expected skill to be registered: %v", err)
	}
	if tool.Name() != "test_skill" {
		t.Errorf("expected 'test_skill', got %q", tool.Name())
	}
}

func TestInstall_ShouldReturnErrorOnHTTPFailure(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	fetcher := &stubFetcher{err: fmt.Errorf("network error")}
	installer := NewSkillInstaller(skillsDir, reg, fetcher)

	_, err := installer.Install("https://example.com/skill.md")
	if err == nil {
		t.Error("expected error when HTTP fetch fails")
	}
}

func TestInstall_ShouldReturnErrorOnInvalidContentFromURL(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	fetcher := &stubFetcher{data: []byte("not valid frontmatter")}
	installer := NewSkillInstaller(skillsDir, reg, fetcher)

	_, err := installer.Install("https://example.com/skill.md")
	if err == nil {
		t.Error("expected error for invalid skill content from URL")
	}
}

func TestInstall_ShouldExtractFilenameFromURL(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	fetcher := &stubFetcher{data: []byte(validSkillContent)}
	installer := NewSkillInstaller(skillsDir, reg, fetcher)

	_, err := installer.Install("https://raw.githubusercontent.com/user/repo/main/my_remote_skill.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	destPath := filepath.Join(skillsDir, "my_remote_skill.md")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("expected file to use filename from URL path")
	}
}

// =============================================================================
// filenameFromURL Tests
// =============================================================================

func TestFilenameFromURL_ShouldExtractFilename(t *testing.T) {
	name := filenameFromURL("https://example.com/skills/greet.md")
	if name != "greet.md" {
		t.Errorf("expected 'greet.md', got %q", name)
	}
}

func TestFilenameFromURL_ShouldFallbackToSkillMd(t *testing.T) {
	name := filenameFromURL("https://example.com/")
	if name != "skill.md" {
		t.Errorf("expected 'skill.md' fallback, got %q", name)
	}
}

func TestFilenameFromURL_ShouldHandleTrailingSlash(t *testing.T) {
	name := filenameFromURL("https://example.com/skills/")
	// "skills" has no dot so should fallback
	if name != "skill.md" {
		t.Errorf("expected 'skill.md' fallback for no-extension path, got %q", name)
	}
}

func TestFilenameFromURL_ShouldHandleDeepPath(t *testing.T) {
	name := filenameFromURL("https://raw.githubusercontent.com/user/repo/main/skills/translate.md")
	if name != "translate.md" {
		t.Errorf("expected 'translate.md', got %q", name)
	}
}

// =============================================================================
// ReloadSkills Tests
// =============================================================================

func TestReloadSkills_ShouldLoadAllSkillsFromDir(t *testing.T) {
	skillsDir := t.TempDir()
	writeTestSkillFile(t, skillsDir, "alpha.md", `---
name: alpha
description: "Alpha skill"
---
Alpha body.
`)
	writeTestSkillFile(t, skillsDir, "beta.md", `---
name: beta
description: "Beta skill"
---
Beta body.
`)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	names, err := installer.ReloadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 newly registered skills, got %d", len(names))
	}
}

func TestReloadSkills_ShouldRegisterNewSkillsInRegistry(t *testing.T) {
	skillsDir := t.TempDir()
	writeTestSkillFile(t, skillsDir, "gamma.md", `---
name: gamma
description: "Gamma skill"
---
Gamma body.
`)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.ReloadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool, err := reg.Get("gamma")
	if err != nil {
		t.Fatalf("expected 'gamma' to be registered: %v", err)
	}
	if tool.Name() != "gamma" {
		t.Errorf("expected 'gamma', got %q", tool.Name())
	}
}

func TestReloadSkills_ShouldSkipAlreadyRegisteredSkills(t *testing.T) {
	skillsDir := t.TempDir()
	writeTestSkillFile(t, skillsDir, "existing.md", `---
name: existing
description: "Already registered"
---
Body.
`)

	reg := NewToolRegistry()
	// Pre-register the skill
	reg.Register(&MarkdownSkill{
		name:        "existing",
		description: "Already registered",
		schema:      `{"type":"object","properties":{}}`,
		body:        "Body.",
	})

	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})
	names, err := installer.ReloadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return 0 newly registered (it was already there)
	if len(names) != 0 {
		t.Errorf("expected 0 newly registered skills, got %d: %v", len(names), names)
	}
}

func TestReloadSkills_ShouldReturnNewlyRegisteredNames(t *testing.T) {
	skillsDir := t.TempDir()
	writeTestSkillFile(t, skillsDir, "new_one.md", `---
name: new_one
description: "A new skill"
---
New body.
`)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	names, err := installer.ReloadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "new_one" {
		t.Errorf("expected [new_one], got %v", names)
	}
}

func TestReloadSkills_ShouldReturnEmptyForEmptyDir(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	names, err := installer.ReloadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 skills, got %d", len(names))
	}
}

func TestReloadSkills_ShouldReturnErrorForNonexistentDir(t *testing.T) {
	reg := NewToolRegistry()
	installer := NewSkillInstaller("/nonexistent/skills/dir", reg, &stubFetcher{})

	_, err := installer.ReloadSkills()
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// =============================================================================
// SchemaTool Interface Tests
// =============================================================================

func TestSkillInstaller_ShouldImplementSchemaTool(t *testing.T) {
	var _ SchemaTool = (*SkillInstaller)(nil)
}

func TestSkillInstaller_Name_ShouldReturnInstallSkill(t *testing.T) {
	installer := NewSkillInstaller(t.TempDir(), NewToolRegistry(), &stubFetcher{})
	if installer.Name() != "install_skill" {
		t.Errorf("expected 'install_skill', got %q", installer.Name())
	}
}

func TestSkillInstaller_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	installer := NewSkillInstaller(t.TempDir(), NewToolRegistry(), &stubFetcher{})
	desc := installer.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
	if !strings.Contains(strings.ToLower(desc), "skill") {
		t.Errorf("expected description to mention 'skill', got %q", desc)
	}
}

func TestSkillInstaller_Definition_ShouldReturnValidJSONSchema(t *testing.T) {
	installer := NewSkillInstaller(t.TempDir(), NewToolRegistry(), &stubFetcher{})
	def := installer.Definition()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(def), &parsed); err != nil {
		t.Fatalf("Definition() should return valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("expected type 'object', got %v", parsed["type"])
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'properties' to be an object")
	}
	if _, exists := props["source"]; !exists {
		t.Error("expected 'source' property in schema")
	}
}

func TestSkillInstaller_Call_ShouldInstallFromLocalPath(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "call_test.md", validSkillContent)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	args := fmt.Sprintf(`{"source":%q}`, sourcePath)
	result, err := installer.Call(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !strings.Contains(result.Data, "test_skill") {
		t.Errorf("expected result to contain skill name, got %q", result.Data)
	}
}

func TestSkillInstaller_Call_ShouldInstallFromURL(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	fetcher := &stubFetcher{data: []byte(validSkillContent)}
	installer := NewSkillInstaller(skillsDir, reg, fetcher)

	result, err := installer.Call(json.RawMessage(`{"source":"https://example.com/remote_skill.md"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Skill should be registered
	_, err = reg.Get("test_skill")
	if err != nil {
		t.Fatalf("expected skill to be registered: %v", err)
	}
}

func TestSkillInstaller_Call_ShouldReturnErrorForMissingSource(t *testing.T) {
	installer := NewSkillInstaller(t.TempDir(), NewToolRegistry(), &stubFetcher{})

	_, err := installer.Call(json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error when source is missing")
	}
}

func TestSkillInstaller_Call_ShouldReturnErrorForInvalidJSON(t *testing.T) {
	installer := NewSkillInstaller(t.TempDir(), NewToolRegistry(), &stubFetcher{})

	_, err := installer.Call(json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestSkillInstaller_Call_ShouldForwardInstallError(t *testing.T) {
	installer := NewSkillInstaller(t.TempDir(), NewToolRegistry(), &stubFetcher{})

	// Source file doesn't exist, so Install will fail
	args := `{"source":"/nonexistent/path/skill.md"}`
	_, err := installer.Call(json.RawMessage(args))
	if err == nil {
		t.Error("expected error when Install fails")
	}
}

func TestSkillInstaller_Call_ShouldReturnMetadata(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "meta_test.md", validSkillContent)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	args := fmt.Sprintf(`{"source":%q}`, sourcePath)
	result, err := installer.Call(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metadata["skill_name"] != "test_skill" {
		t.Errorf("expected metadata skill_name='test_skill', got %q", result.Metadata["skill_name"])
	}
	if result.Metadata["skill_source"] != sourcePath {
		t.Errorf("expected metadata skill_source=%q, got %q", sourcePath, result.Metadata["skill_source"])
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestInstall_ShouldReturnErrorWhenRegistryRejectsSkill(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "duplicate.md", validSkillContent)

	reg := NewToolRegistry()
	// Pre-register a skill with the same name
	reg.Register(&MarkdownSkill{
		name:        "test_skill",
		description: "Pre-existing",
		schema:      `{"type":"object","properties":{}}`,
		body:        "body",
	})

	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.Install(sourcePath)
	if err == nil {
		t.Error("expected error when registry rejects duplicate skill")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected 'already registered' in error, got %q", err.Error())
	}
}

func TestInstall_ShouldReturnErrorWhenSkillsDirNotWritable(t *testing.T) {
	// Create a read-only directory
	skillsDir := t.TempDir()
	os.Chmod(skillsDir, 0444)
	defer os.Chmod(skillsDir, 0755) // cleanup

	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "write_fail.md", validSkillContent)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.Install(sourcePath)
	if err == nil {
		t.Error("expected error when skills directory is not writable")
	}
}

func TestFilenameFromURL_ShouldHandlePathWithoutExtension(t *testing.T) {
	name := filenameFromURL("https://example.com/skills/no-extension")
	if name != "skill.md" {
		t.Errorf("expected 'skill.md' fallback for no-extension path, got %q", name)
	}
}

func TestFilenameFromURL_ShouldHandleMalformedURL(t *testing.T) {
	// Control characters make URL parsing fail
	name := filenameFromURL("://\x00invalid")
	if name != "skill.md" {
		t.Errorf("expected 'skill.md' fallback for malformed URL, got %q", name)
	}
}

func TestInstall_ShouldPreserveFileContentExactly(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := writeTestSkillFile(t, sourceDir, "preserve.md", validSkillContent)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	_, err := installer.Install(sourcePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	destPath := filepath.Join(skillsDir, "preserve.md")
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(got) != validSkillContent {
		t.Error("expected file content to be preserved exactly")
	}
}

// =============================================================================
// E2E Integration: Install → ReloadSkills → Call
// =============================================================================

func TestE2E_InstallThenReloadSkills_ShouldMakeSkillCallable(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()

	// Create a skill file in source directory
	skillContent := `---
name: e2e_skill
description: "End-to-end test skill"
args:
  - name: message
    type: string
    description: "A message"
    required: true
---
# E2E Skill

You are a test skill for end-to-end testing.
`
	sourcePath := writeTestSkillFile(t, sourceDir, "e2e_skill.md", skillContent)

	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	// Step 1: Install the skill
	skill, err := installer.Install(sourcePath)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if skill.Name() != "e2e_skill" {
		t.Errorf("expected 'e2e_skill', got %q", skill.Name())
	}

	// Step 2: Verify it's in the registry
	tool, err := reg.Get("e2e_skill")
	if err != nil {
		t.Fatalf("expected skill in registry: %v", err)
	}

	// Step 3: Call it with valid args
	result, err := tool.Call(json.RawMessage(`{"message":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error calling skill: %v", err)
	}
	if !strings.Contains(result.Data, "E2E Skill") {
		t.Errorf("expected body to contain 'E2E Skill', got %q", result.Data)
	}

	// Step 4: ReloadSkills should NOT re-register it (already exists)
	names, err := installer.ReloadSkills()
	if err != nil {
		t.Fatalf("ReloadSkills failed: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 newly registered (already exists), got %d: %v", len(names), names)
	}
}

func TestE2E_InstallFromURL_ThenCallViaSchemaTool(t *testing.T) {
	skillsDir := t.TempDir()

	urlSkillContent := `---
name: url_e2e
description: "URL-sourced skill"
---
You are installed from a URL.
`
	reg := NewToolRegistry()
	fetcher := &stubFetcher{data: []byte(urlSkillContent)}
	installer := NewSkillInstaller(skillsDir, reg, fetcher)

	// Install via the SchemaTool.Call interface
	args := `{"source":"https://example.com/skills/url_e2e.md"}`
	result, err := installer.Call(json.RawMessage(args))
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if !strings.Contains(result.Data, "url_e2e") {
		t.Errorf("expected result to mention skill name, got %q", result.Data)
	}

	// Verify the skill is callable
	tool, err := reg.Get("url_e2e")
	if err != nil {
		t.Fatalf("expected skill in registry: %v", err)
	}
	callResult, err := tool.Call(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(callResult.Data, "installed from a URL") {
		t.Errorf("expected body content, got %q", callResult.Data)
	}
}

func TestE2E_ReloadPicksUpNewFiles(t *testing.T) {
	skillsDir := t.TempDir()
	reg := NewToolRegistry()
	installer := NewSkillInstaller(skillsDir, reg, &stubFetcher{})

	// Initially empty
	names, err := installer.ReloadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("expected 0, got %d", len(names))
	}

	// Add a skill file directly to the skills dir (simulating external addition)
	writeTestSkillFile(t, skillsDir, "dynamic.md", `---
name: dynamic
description: "Dynamically added"
---
Dynamic body.
`)

	// Reload should pick it up
	names, err = installer.ReloadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "dynamic" {
		t.Errorf("expected [dynamic], got %v", names)
	}

	// Verify it's callable
	tool, err := reg.Get("dynamic")
	if err != nil {
		t.Fatalf("expected 'dynamic' in registry: %v", err)
	}
	result, err := tool.Call(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data != "Dynamic body." {
		t.Errorf("expected 'Dynamic body.', got %q", result.Data)
	}
}
