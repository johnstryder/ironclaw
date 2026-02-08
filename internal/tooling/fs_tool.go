package tooling

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ironclaw/internal/domain"
)

// FileSystem abstracts file system operations for testability.
type FileSystem interface {
	ReadDir(path string) ([]DirEntry, error)
	ReadFile(path string) (string, error)
	WriteFile(path string, content string) error
}

// DirEntry represents a directory entry returned by ReadDir.
type DirEntry struct {
	Name  string
	IsDir bool
}

// JailPath resolves the given userPath relative to root and ensures the result
// stays inside root. Returns the clean absolute path or an error if the path
// escapes the sandbox.
func JailPath(root, userPath string) (string, error) {
	cleanRoot := filepath.Clean(root)

	// If userPath is absolute, check if it's inside the sandbox
	if filepath.IsAbs(userPath) {
		cleanUser := filepath.Clean(userPath)
		if cleanUser == cleanRoot || strings.HasPrefix(cleanUser, cleanRoot+string(filepath.Separator)) {
			return cleanUser, nil
		}
		return "", fmt.Errorf("path escapes sandbox: %s", userPath)
	}

	// Resolve relative path against root
	resolved := filepath.Join(cleanRoot, userPath)
	resolved = filepath.Clean(resolved)

	// Verify the resolved path is inside (or equal to) root
	if resolved == cleanRoot || strings.HasPrefix(resolved, cleanRoot+string(filepath.Separator)) {
		return resolved, nil
	}

	return "", fmt.Errorf("path escapes sandbox: %s", userPath)
}

// FileSystemInput represents the input structure for filesystem operations.
type FileSystemInput struct {
	Operation string `json:"operation" jsonschema:"enum=list_dir,enum=read_file,enum=write_file"`
	Path      string `json:"path" jsonschema:"minLength=1"`
	Content   string `json:"content,omitempty"`
}

// fsUnmarshalFunc is the JSON unmarshaler used by Call. Package-level so
// tests can inject a failing unmarshaler to cover the defense-in-depth error path.
var fsUnmarshalFunc = json.Unmarshal

// FileSystemTool provides sandboxed file system access (list, read, write).
// All paths are jailed to the configured root directory.
type FileSystemTool struct {
	rootDir string
	fs      FileSystem
}

// NewFileSystemTool creates a FileSystemTool with the given root directory and file system.
func NewFileSystemTool(rootDir string, fs FileSystem) *FileSystemTool {
	return &FileSystemTool{rootDir: rootDir, fs: fs}
}

// Name returns the tool name used in function-calling.
func (f *FileSystemTool) Name() string { return "filesystem" }

// Description returns a human-readable description for the LLM.
func (f *FileSystemTool) Description() string {
	return "Provides sandboxed file system access: list directories, read files, and write files within the allowed working directory"
}

// Definition returns the JSON Schema for filesystem input.
func (f *FileSystemTool) Definition() string {
	return GenerateSchema(FileSystemInput{})
}

// Call validates the input and dispatches to the appropriate filesystem operation.
func (f *FileSystemTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	// 1. Validate input against JSON schema
	schema := f.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 2. Unmarshal input
	var input FileSystemInput
	if err := fsUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// 3. Jail the path — ensure it stays inside the sandbox
	resolvedPath, err := JailPath(f.rootDir, input.Path)
	if err != nil {
		return nil, err
	}

	// 4. Dispatch to the appropriate operation
	switch input.Operation {
	case "list_dir":
		return f.listDir(resolvedPath)
	case "read_file":
		return f.readFile(resolvedPath, input.Path)
	case "write_file":
		return f.writeFile(resolvedPath, input.Path, input.Content)
	default:
		return nil, fmt.Errorf("unknown operation: %s", input.Operation)
	}
}

func (f *FileSystemTool) listDir(resolvedPath string) (*domain.ToolResult, error) {
	entries, err := f.fs.ReadDir(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	var lines []string
	for _, entry := range entries {
		name := entry.Name
		if entry.IsDir {
			name += "/"
		}
		lines = append(lines, name)
	}

	return &domain.ToolResult{
		Data: strings.Join(lines, "\n"),
		Metadata: map[string]string{
			"operation": "list_dir",
			"path":      resolvedPath,
		},
	}, nil
}

func (f *FileSystemTool) readFile(resolvedPath, userPath string) (*domain.ToolResult, error) {
	content, err := f.fs.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &domain.ToolResult{
		Data: content,
		Metadata: map[string]string{
			"operation": "read_file",
			"path":      userPath,
		},
	}, nil
}

func (f *FileSystemTool) writeFile(resolvedPath, userPath, content string) (*domain.ToolResult, error) {
	if err := f.fs.WriteFile(resolvedPath, content); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), userPath),
		Metadata: map[string]string{
			"operation":    "write_file",
			"path":         userPath,
			"bytes_written": fmt.Sprintf("%d", len(content)),
		},
	}, nil
}

// OsFileSystem implements FileSystem using the real os package.
type OsFileSystem struct{}

// ReadDir reads a real directory and returns its entries.
func (o *OsFileSystem) ReadDir(path string) ([]DirEntry, error) {
	osEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	entries := make([]DirEntry, 0, len(osEntries))
	for _, e := range osEntries {
		entries = append(entries, DirEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
		})
	}
	return entries, nil
}

// ReadFile reads a real file and returns its contents as a string.
func (o *OsFileSystem) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile writes content to a real file, creating or truncating it.
func (o *OsFileSystem) WriteFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
