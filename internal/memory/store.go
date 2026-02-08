package memory

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"ironclaw/internal/domain"
)

// memoryFile is the filename for long-term persistent memory.
const memoryFile = "memory.md"

// writeStringFunc is used to write content so tests can inject a failing implementation.
type writeStringFunc func(io.Writer, string) (int, error)

// FileMemoryStore writes memory entries to files under dir as YYYY-MM-DD.md, append-only.
// It also supports a persistent long-term memory file (memory.md) for facts the agent remembers.
type FileMemoryStore struct {
	dir      string
	writeStr writeStringFunc // nil means use file.WriteString
}

// NewFileMemoryStore returns a MemoryStore that writes to dir. Date is sanitized with filepath.Base
// so path traversal (e.g. "../../../etc/passwd") cannot escape dir.
func NewFileMemoryStore(dir string) *FileMemoryStore {
	return &FileMemoryStore{dir: filepath.Clean(dir)}
}

// Append implements domain.MemoryStore. It appends content to dir/<date>.md (date sanitized to basename).
func (f *FileMemoryStore) Append(date string, content string) error {
	base := filepath.Base(date)
	if base == "." || base == ".." {
		base = "default"
	}
	path := filepath.Join(f.dir, base+".md")
	return f.appendToFile(path, content)
}

// Remember implements domain.MemoryStore. It appends a fact to the persistent memory.md file.
// Each entry is written on its own line with a "- " prefix for readability.
func (f *FileMemoryStore) Remember(content string) error {
	if content == "" {
		return nil
	}
	path := filepath.Join(f.dir, memoryFile)
	return f.appendToFile(path, "- "+content+"\n")
}

// LoadMemory implements domain.MemoryStore. It reads the entire memory.md file.
// Returns empty string (not error) when the file does not exist.
func (f *FileMemoryStore) LoadMemory() (string, error) {
	path := filepath.Join(f.dir, memoryFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// appendToFile opens a file in append mode and writes content.
func (f *FileMemoryStore) appendToFile(path string, content string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	var writeErr error
	if f.writeStr != nil {
		_, writeErr = f.writeStr(file, content)
	} else {
		_, writeErr = file.WriteString(content)
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// Ensure FileMemoryStore implements domain.MemoryStore.
var _ domain.MemoryStore = (*FileMemoryStore)(nil)
