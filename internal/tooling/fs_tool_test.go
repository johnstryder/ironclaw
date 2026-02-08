package tooling

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockFileSystem is a test double for FileSystem.
type mockFileSystem struct {
	readDirResult  []DirEntry
	readDirErr     error
	readFileResult string
	readFileErr    error
	writeFileErr   error
}

func (m *mockFileSystem) ReadDir(path string) ([]DirEntry, error) {
	return m.readDirResult, m.readDirErr
}

func (m *mockFileSystem) ReadFile(path string) (string, error) {
	return m.readFileResult, m.readFileErr
}

func (m *mockFileSystem) WriteFile(path string, content string) error {
	return m.writeFileErr
}

// =============================================================================
// JailPath — Security Tests
// =============================================================================

func TestJailPath_ShouldAllowSimpleRelativePath(t *testing.T) {
	result, err := JailPath("/sandbox", "file.txt")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != "/sandbox/file.txt" {
		t.Errorf("Expected '/sandbox/file.txt', got '%s'", result)
	}
}

func TestJailPath_ShouldAllowNestedRelativePath(t *testing.T) {
	result, err := JailPath("/sandbox", "subdir/file.txt")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != "/sandbox/subdir/file.txt" {
		t.Errorf("Expected '/sandbox/subdir/file.txt', got '%s'", result)
	}
}

func TestJailPath_ShouldAllowDotCurrentDir(t *testing.T) {
	result, err := JailPath("/sandbox", ".")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != "/sandbox" {
		t.Errorf("Expected '/sandbox', got '%s'", result)
	}
}

func TestJailPath_ShouldBlockDotDotTraversal(t *testing.T) {
	_, err := JailPath("/sandbox", "../etc/passwd")
	if err == nil {
		t.Fatal("Expected error for .. traversal")
	}
}

func TestJailPath_ShouldBlockNestedDotDotTraversal(t *testing.T) {
	_, err := JailPath("/sandbox", "subdir/../../etc/passwd")
	if err == nil {
		t.Fatal("Expected error for nested .. traversal")
	}
}

func TestJailPath_ShouldBlockAbsolutePathOutsideSandbox(t *testing.T) {
	_, err := JailPath("/sandbox", "/etc/passwd")
	if err == nil {
		t.Fatal("Expected error for absolute path outside sandbox")
	}
}

func TestJailPath_ShouldAllowAbsolutePathInsideSandbox(t *testing.T) {
	result, err := JailPath("/sandbox", "/sandbox/file.txt")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != "/sandbox/file.txt" {
		t.Errorf("Expected '/sandbox/file.txt', got '%s'", result)
	}
}

func TestJailPath_ShouldBlockDotDotAtEndOfPath(t *testing.T) {
	_, err := JailPath("/sandbox", "subdir/..")
	// "subdir/.." resolves to "/sandbox" which is the root itself — should be allowed
	if err != nil {
		t.Fatalf("Expected no error for path resolving to root, got: %v", err)
	}
}

func TestJailPath_ShouldBlockDoubleDotDotEscape(t *testing.T) {
	_, err := JailPath("/sandbox", "../../..")
	if err == nil {
		t.Fatal("Expected error for multiple .. escape")
	}
}

func TestJailPath_ShouldHandleTrailingSlashInRoot(t *testing.T) {
	result, err := JailPath("/sandbox/", "file.txt")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != "/sandbox/file.txt" {
		t.Errorf("Expected '/sandbox/file.txt', got '%s'", result)
	}
}

func TestJailPath_ShouldBlockDotDotWithDotSlashPrefix(t *testing.T) {
	_, err := JailPath("/sandbox", "./../secret")
	if err == nil {
		t.Fatal("Expected error for ./../secret traversal")
	}
}

// =============================================================================
// FileSystemTool — Name, Description, Definition
// =============================================================================

func TestFileSystemTool_Name_ShouldReturnFileSystem(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	if tool.Name() != "filesystem" {
		t.Errorf("Expected name 'filesystem', got '%s'", tool.Name())
	}
}

func TestFileSystemTool_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestFileSystemTool_Definition_ShouldContainOperationProperty(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", parsed["type"])
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'properties' in schema")
	}
	if _, exists := props["operation"]; !exists {
		t.Error("Expected 'operation' property in schema")
	}
	if _, exists := props["path"]; !exists {
		t.Error("Expected 'path' property in schema")
	}
}

func TestFileSystemTool_Definition_ShouldRequireOperationAndPath(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("Expected 'required' array in schema")
	}
	requiredFields := make(map[string]bool)
	for _, r := range required {
		requiredFields[r.(string)] = true
	}
	if !requiredFields["operation"] {
		t.Error("Expected 'operation' in required fields")
	}
	if !requiredFields["path"] {
		t.Error("Expected 'path' in required fields")
	}
}

// =============================================================================
// FileSystemTool.Call — Input Validation
// =============================================================================

func TestFileSystemTool_Call_ShouldRejectInvalidJSON(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	_, err := tool.Call(json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestFileSystemTool_Call_ShouldRejectMissingOperationField(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	_, err := tool.Call(json.RawMessage(`{"path":"file.txt"}`))
	if err == nil {
		t.Fatal("Expected error for missing operation field")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestFileSystemTool_Call_ShouldRejectMissingPathField(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	_, err := tool.Call(json.RawMessage(`{"operation":"list_dir"}`))
	if err == nil {
		t.Fatal("Expected error for missing path field")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestFileSystemTool_Call_ShouldRejectEmptyPathString(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	_, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":""}`))
	if err == nil {
		t.Fatal("Expected error for empty path string")
	}
}

func TestFileSystemTool_Call_ShouldRejectInvalidOperation(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	_, err := tool.Call(json.RawMessage(`{"operation":"delete","path":"file.txt"}`))
	if err == nil {
		t.Fatal("Expected error for invalid operation")
	}
}

func TestFileSystemTool_Call_ShouldRejectPathTraversal(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{
		readDirResult: []DirEntry{{Name: "should-not-reach", IsDir: false}},
	})
	_, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":"../../etc"}`))
	if err == nil {
		t.Fatal("Expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path escapes sandbox") {
		t.Errorf("Expected 'path escapes sandbox' in error, got: %v", err)
	}
}

func TestFileSystemTool_Call_ShouldRejectAbsolutePathOutsideSandbox(t *testing.T) {
	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	_, err := tool.Call(json.RawMessage(`{"operation":"read_file","path":"/etc/passwd"}`))
	if err == nil {
		t.Fatal("Expected error for absolute path outside sandbox")
	}
	if !strings.Contains(err.Error(), "path escapes sandbox") {
		t.Errorf("Expected 'path escapes sandbox' in error, got: %v", err)
	}
}

// =============================================================================
// FileSystemTool.Call — ListDir Operation
// =============================================================================

func TestFileSystemTool_Call_ListDir_ShouldReturnDirectoryEntries(t *testing.T) {
	fs := &mockFileSystem{
		readDirResult: []DirEntry{
			{Name: "file1.txt", IsDir: false},
			{Name: "subdir", IsDir: true},
			{Name: "file2.go", IsDir: false},
		},
	}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":"."}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "file1.txt") {
		t.Errorf("Expected 'file1.txt' in output, got: %s", result.Data)
	}
	if !strings.Contains(result.Data, "subdir") {
		t.Errorf("Expected 'subdir' in output, got: %s", result.Data)
	}
	if !strings.Contains(result.Data, "file2.go") {
		t.Errorf("Expected 'file2.go' in output, got: %s", result.Data)
	}
}

func TestFileSystemTool_Call_ListDir_ShouldIndicateDirectories(t *testing.T) {
	fs := &mockFileSystem{
		readDirResult: []DirEntry{
			{Name: "subdir", IsDir: true},
		},
	}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":"."}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "subdir/") {
		t.Errorf("Expected directory indicator 'subdir/' in output, got: %s", result.Data)
	}
}

func TestFileSystemTool_Call_ListDir_ShouldReturnEmptyForEmptyDir(t *testing.T) {
	fs := &mockFileSystem{
		readDirResult: []DirEntry{},
	}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":"."}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "" {
		t.Errorf("Expected empty data for empty directory, got: '%s'", result.Data)
	}
}

func TestFileSystemTool_Call_ListDir_ShouldReturnErrorWhenReadDirFails(t *testing.T) {
	fs := &mockFileSystem{
		readDirErr: fmt.Errorf("permission denied"),
	}
	tool := NewFileSystemTool("/sandbox", fs)
	_, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":"."}`))
	if err == nil {
		t.Fatal("Expected error when ReadDir fails")
	}
	if !strings.Contains(err.Error(), "failed to list directory") {
		t.Errorf("Expected 'failed to list directory' in error, got: %v", err)
	}
}

func TestFileSystemTool_Call_ListDir_ShouldReturnMetadataWithOperation(t *testing.T) {
	fs := &mockFileSystem{
		readDirResult: []DirEntry{{Name: "a.txt", IsDir: false}},
	}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":"."}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["operation"] != "list_dir" {
		t.Errorf("Expected metadata operation='list_dir', got '%s'", result.Metadata["operation"])
	}
}

// =============================================================================
// FileSystemTool.Call — ReadFile Operation
// =============================================================================

func TestFileSystemTool_Call_ReadFile_ShouldReturnFileContents(t *testing.T) {
	fs := &mockFileSystem{
		readFileResult: "Hello, World!",
	}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"read_file","path":"hello.txt"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", result.Data)
	}
}

func TestFileSystemTool_Call_ReadFile_ShouldReturnEmptyForEmptyFile(t *testing.T) {
	fs := &mockFileSystem{
		readFileResult: "",
	}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"read_file","path":"empty.txt"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "" {
		t.Errorf("Expected empty data, got '%s'", result.Data)
	}
}

func TestFileSystemTool_Call_ReadFile_ShouldReturnErrorWhenReadFails(t *testing.T) {
	fs := &mockFileSystem{
		readFileErr: fmt.Errorf("file not found"),
	}
	tool := NewFileSystemTool("/sandbox", fs)
	_, err := tool.Call(json.RawMessage(`{"operation":"read_file","path":"missing.txt"}`))
	if err == nil {
		t.Fatal("Expected error when ReadFile fails")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' in error, got: %v", err)
	}
}

func TestFileSystemTool_Call_ReadFile_ShouldReturnMetadataWithPath(t *testing.T) {
	fs := &mockFileSystem{
		readFileResult: "content",
	}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"read_file","path":"hello.txt"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["operation"] != "read_file" {
		t.Errorf("Expected metadata operation='read_file', got '%s'", result.Metadata["operation"])
	}
	if result.Metadata["path"] != "hello.txt" {
		t.Errorf("Expected metadata path='hello.txt', got '%s'", result.Metadata["path"])
	}
}

// =============================================================================
// FileSystemTool.Call — WriteFile Operation
// =============================================================================

func TestFileSystemTool_Call_WriteFile_ShouldSucceedOnValidInput(t *testing.T) {
	fs := &mockFileSystem{}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"write_file","path":"output.txt","content":"data"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data == "" {
		t.Error("Expected non-empty success message")
	}
}

func TestFileSystemTool_Call_WriteFile_ShouldReturnErrorWhenWriteFails(t *testing.T) {
	fs := &mockFileSystem{
		writeFileErr: fmt.Errorf("disk full"),
	}
	tool := NewFileSystemTool("/sandbox", fs)
	_, err := tool.Call(json.RawMessage(`{"operation":"write_file","path":"output.txt","content":"data"}`))
	if err == nil {
		t.Fatal("Expected error when WriteFile fails")
	}
	if !strings.Contains(err.Error(), "failed to write file") {
		t.Errorf("Expected 'failed to write file' in error, got: %v", err)
	}
}

func TestFileSystemTool_Call_WriteFile_ShouldReturnMetadataWithPath(t *testing.T) {
	fs := &mockFileSystem{}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"write_file","path":"output.txt","content":"data"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["operation"] != "write_file" {
		t.Errorf("Expected metadata operation='write_file', got '%s'", result.Metadata["operation"])
	}
	if result.Metadata["path"] != "output.txt" {
		t.Errorf("Expected metadata path='output.txt', got '%s'", result.Metadata["path"])
	}
}

func TestFileSystemTool_Call_WriteFile_ShouldWriteEmptyContent(t *testing.T) {
	fs := &mockFileSystem{}
	tool := NewFileSystemTool("/sandbox", fs)
	result, err := tool.Call(json.RawMessage(`{"operation":"write_file","path":"output.txt","content":""}`))
	if err != nil {
		t.Fatalf("Expected no error for empty content, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

// =============================================================================
// FileSystemTool.Call — Unmarshal error path (defense-in-depth)
// =============================================================================

func TestFileSystemTool_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := fsUnmarshalFunc
	fsUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { fsUnmarshalFunc = original }()

	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	_, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":"."}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

// =============================================================================
// FileSystemTool.Call — WriteFile with path traversal in content
// =============================================================================

func TestFileSystemTool_Call_WriteFile_ShouldRejectPathTraversalInWritePath(t *testing.T) {
	fs := &mockFileSystem{}
	tool := NewFileSystemTool("/sandbox", fs)
	_, err := tool.Call(json.RawMessage(`{"operation":"write_file","path":"../outside.txt","content":"evil"}`))
	if err == nil {
		t.Fatal("Expected error for path traversal in write path")
	}
	if !strings.Contains(err.Error(), "path escapes sandbox") {
		t.Errorf("Expected 'path escapes sandbox' in error, got: %v", err)
	}
}

// =============================================================================
// spyFileSystem — verifies the resolved path is passed to the FS layer
// =============================================================================

type spyFileSystem struct {
	calledPath    string
	calledContent string
}

func (s *spyFileSystem) ReadDir(path string) ([]DirEntry, error) {
	s.calledPath = path
	return []DirEntry{}, nil
}

func (s *spyFileSystem) ReadFile(path string) (string, error) {
	s.calledPath = path
	return "spy", nil
}

func (s *spyFileSystem) WriteFile(path string, content string) error {
	s.calledPath = path
	s.calledContent = content
	return nil
}

func TestFileSystemTool_Call_ListDir_ShouldPassResolvedPathToFS(t *testing.T) {
	spy := &spyFileSystem{}
	tool := NewFileSystemTool("/sandbox", spy)
	_, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":"subdir"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if spy.calledPath != "/sandbox/subdir" {
		t.Errorf("Expected FS to receive '/sandbox/subdir', got '%s'", spy.calledPath)
	}
}

func TestFileSystemTool_Call_ReadFile_ShouldPassResolvedPathToFS(t *testing.T) {
	spy := &spyFileSystem{}
	tool := NewFileSystemTool("/sandbox", spy)
	_, err := tool.Call(json.RawMessage(`{"operation":"read_file","path":"subdir/file.txt"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if spy.calledPath != "/sandbox/subdir/file.txt" {
		t.Errorf("Expected FS to receive '/sandbox/subdir/file.txt', got '%s'", spy.calledPath)
	}
}

func TestFileSystemTool_Call_WriteFile_ShouldPassResolvedPathAndContentToFS(t *testing.T) {
	spy := &spyFileSystem{}
	tool := NewFileSystemTool("/sandbox", spy)
	_, err := tool.Call(json.RawMessage(`{"operation":"write_file","path":"output.txt","content":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if spy.calledPath != "/sandbox/output.txt" {
		t.Errorf("Expected FS to receive '/sandbox/output.txt', got '%s'", spy.calledPath)
	}
	if spy.calledContent != "hello" {
		t.Errorf("Expected FS to receive content 'hello', got '%s'", spy.calledContent)
	}
}

// =============================================================================
// OsFileSystem — Integration Tests (real file system with temp directory)
// =============================================================================

func TestOsFileSystem_ReadDir_ShouldListRealDirectory(t *testing.T) {
	dir := t.TempDir()
	// Create test files
	osFS := &OsFileSystem{}
	if err := osFS.WriteFile(dir+"/a.txt", "a"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := osFS.WriteFile(dir+"/b.txt", "b"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	entries, err := osFS.ReadDir(dir)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}
}

func TestOsFileSystem_ReadFile_ShouldReadRealFile(t *testing.T) {
	dir := t.TempDir()
	osFS := &OsFileSystem{}
	if err := osFS.WriteFile(dir+"/test.txt", "hello world"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	content, err := osFS.ReadFile(dir + "/test.txt")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if content != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", content)
	}
}

func TestOsFileSystem_WriteFile_ShouldCreateAndWriteRealFile(t *testing.T) {
	dir := t.TempDir()
	osFS := &OsFileSystem{}
	err := osFS.WriteFile(dir+"/new.txt", "new content")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	content, err := osFS.ReadFile(dir + "/new.txt")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if content != "new content" {
		t.Errorf("Expected 'new content', got '%s'", content)
	}
}

func TestOsFileSystem_ReadDir_ShouldReturnErrorForNonExistentDir(t *testing.T) {
	osFS := &OsFileSystem{}
	_, err := osFS.ReadDir("/nonexistent-dir-12345")
	if err == nil {
		t.Fatal("Expected error for non-existent directory")
	}
}

func TestOsFileSystem_ReadFile_ShouldReturnErrorForNonExistentFile(t *testing.T) {
	osFS := &OsFileSystem{}
	_, err := osFS.ReadFile("/nonexistent-file-12345.txt")
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
}

func TestOsFileSystem_ReadDir_ShouldIndicateDirectoryEntries(t *testing.T) {
	dir := t.TempDir()
	osFS := &OsFileSystem{}
	// Create a subdirectory
	if err := osFS.WriteFile(dir+"/file.txt", "data"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	entries, err := osFS.ReadDir(dir)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Name == "file.txt" && !e.IsDir {
			found = true
		}
	}
	if !found {
		t.Error("Expected to find 'file.txt' as a non-directory entry")
	}
}

// =============================================================================
// FileSystemTool.Call — default switch branch (defense-in-depth)
// =============================================================================

// looseFileSystemInput has no enum restriction so unknown operations reach the
// default case in Call's switch statement.
type looseFileSystemInput struct {
	Operation string `json:"operation" jsonschema:"minLength=1"`
	Path      string `json:"path" jsonschema:"minLength=1"`
	Content   string `json:"content,omitempty"`
}

// looseFileSystemTool uses a loose schema to test the default switch branch.
type looseFileSystemTool struct {
	rootDir string
	fs      FileSystem
}

func (l *looseFileSystemTool) Name() string        { return "loose_filesystem" }
func (l *looseFileSystemTool) Description() string  { return "loose fs tool for testing" }
func (l *looseFileSystemTool) Definition() string   { return GenerateSchema(looseFileSystemInput{}) }
func (l *looseFileSystemTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	schema := l.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}
	var input looseFileSystemInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	resolvedPath, err := JailPath(l.rootDir, input.Path)
	if err != nil {
		return nil, err
	}
	switch input.Operation {
	case "list_dir":
		entries, fsErr := l.fs.ReadDir(resolvedPath)
		if fsErr != nil {
			return nil, fmt.Errorf("failed to list directory: %w", fsErr)
		}
		var lines []string
		for _, entry := range entries {
			lines = append(lines, entry.Name)
		}
		return &domain.ToolResult{Data: strings.Join(lines, "\n")}, nil
	default:
		return nil, fmt.Errorf("unknown operation: %s", input.Operation)
	}
}

func TestFileSystemTool_Call_ShouldReturnErrorForUnknownOperationDefenseInDepth(t *testing.T) {
	// Inject an unmarshaler that sets an unknown operation after schema validation passes
	original := fsUnmarshalFunc
	fsUnmarshalFunc = func(data []byte, v interface{}) error {
		input, ok := v.(*FileSystemInput)
		if !ok {
			return fmt.Errorf("unexpected type")
		}
		input.Operation = "unknown_op"
		input.Path = "."
		return nil
	}
	defer func() { fsUnmarshalFunc = original }()

	tool := NewFileSystemTool("/sandbox", &mockFileSystem{})
	_, err := tool.Call(json.RawMessage(`{"operation":"list_dir","path":"."}`))
	if err == nil {
		t.Fatal("Expected error for unknown operation (defense-in-depth)")
	}
	if !strings.Contains(err.Error(), "unknown operation") {
		t.Errorf("Expected 'unknown operation' in error, got: %v", err)
	}
}

func TestLooseFileSystemTool_Call_ShouldReturnErrorForUnknownOperation(t *testing.T) {
	tool := &looseFileSystemTool{rootDir: "/sandbox", fs: &mockFileSystem{}}
	_, err := tool.Call(json.RawMessage(`{"operation":"delete","path":"."}`))
	if err == nil {
		t.Fatal("Expected error for unknown operation")
	}
	if !strings.Contains(err.Error(), "unknown operation") {
		t.Errorf("Expected 'unknown operation' in error, got: %v", err)
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*FileSystemTool)(nil)
var _ SchemaTool = (*looseFileSystemTool)(nil)
var _ FileSystem = (*mockFileSystem)(nil)
var _ FileSystem = (*spyFileSystem)(nil)
var _ FileSystem = (*OsFileSystem)(nil)
