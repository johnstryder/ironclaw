package memory

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

func TestFileMemoryStore_Append_WhenFileDoesNotExist_ShouldCreateAndWrite(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	err := s.Append("2024-01-15", "first line\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "2024-01-15.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(b) != "first line\n" {
		t.Errorf("content: want %q, got %q", "first line\n", string(b))
	}
}

func TestFileMemoryStore_Append_WhenFileExists_ShouldAppendNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	if err := s.Append("2024-01-15", "first\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.Append("2024-01-15", "second\n"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "2024-01-15.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	want := "first\nsecond\n"
	if string(b) != want {
		t.Errorf("content: want %q, got %q", want, string(b))
	}
}

func TestFileMemoryStore_Append_WhenDateContainsPathTraversal_ShouldNotEscapeDir(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	err := s.Append("../../../etc/passwd", "x")
	if err != nil {
		t.Logf("append rejected or failed: %v", err)
		return
	}
	// Implementation must sanitize to basename: only passwd.md should exist under dir
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "passwd.md" {
		t.Errorf("expected single file passwd.md under dir, got %v", entries[0].Name())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "passwd.md"))
	if string(b) != "x" {
		t.Errorf("content: want x, got %q", string(b))
	}
}

func TestFileMemoryStore_Append_WhenDateIsCleaned_ShouldUseSafeFilename(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	// Only YYYY-MM-DD is safe; implementation should use filepath.Base or reject
	err := s.Append("2024-01-15", "ok\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	path := filepath.Join(dir, "2024-01-15.md")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file %s to exist: %v", path, err)
	}
}

func TestFileMemoryStore_Append_EmptyContent_ShouldStillAppend(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	if err := s.Append("2024-01-15", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "2024-01-15.md"))
	if string(b) != "" {
		t.Errorf("expected empty file, got %q", string(b))
	}
	if err := s.Append("2024-01-15", "after\n"); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, "2024-01-15.md"))
	if string(b) != "after\n" {
		t.Errorf("content: want %q, got %q", "after\n", string(b))
	}
}

func TestFileMemoryStore_implements_MemoryStore(t *testing.T) {
	var _ domain.MemoryStore = NewFileMemoryStore(t.TempDir())
}

func TestFileMemoryStore_Append_WhenDateIsDotOrDotDot_ShouldUseDefaultFilename(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	if err := s.Append(".", "from-dot\n"); err != nil {
		t.Fatalf("append date .: %v", err)
	}
	if err := s.Append("..", "from-dotdot\n"); err != nil {
		t.Fatalf("append date ..: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "default.md"))
	if err != nil {
		t.Fatalf("read default.md: %v", err)
	}
	if string(b) != "from-dot\nfrom-dotdot\n" {
		t.Errorf("content: want from-dot + from-dotdot, got %q", string(b))
	}
}

func TestFileMemoryStore_Append_WhenDirIsFile_ShouldReturnError(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewFileMemoryStore(f)
	err := s.Append("2024-01-15", "x")
	if err == nil {
		t.Fatal("expected error when store dir is a file")
	}
}

func TestFileMemoryStore_Append_WhenWriteFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	s.writeStr = func(io.Writer, string) (int, error) { return 0, errors.New("injected write error") }
	err := s.Append("2024-01-15", "x")
	if err == nil {
		t.Fatal("expected error when write fails")
	}
	if err.Error() != "injected write error" {
		t.Errorf("expected injected error, got %v", err)
	}
}

// =============================================================================
// Remember tests
// =============================================================================

func TestFileMemoryStore_Remember_WhenFileDoesNotExist_ShouldCreateAndWrite(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	err := s.Remember("My favorite color is blue.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "memory.md"))
	if err != nil {
		t.Fatalf("read memory.md: %v", err)
	}
	if !strings.Contains(string(b), "My favorite color is blue.") {
		t.Errorf("expected content to contain fact, got %q", string(b))
	}
}

func TestFileMemoryStore_Remember_WhenCalledMultipleTimes_ShouldAppendNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	if err := s.Remember("Fact one."); err != nil {
		t.Fatal(err)
	}
	if err := s.Remember("Fact two."); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "memory.md"))
	if err != nil {
		t.Fatalf("read memory.md: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, "Fact one.") {
		t.Error("expected content to contain 'Fact one.'")
	}
	if !strings.Contains(content, "Fact two.") {
		t.Error("expected content to contain 'Fact two.'")
	}
}

func TestFileMemoryStore_Remember_WhenContentIsEmpty_ShouldNotError(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	if err := s.Remember(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFileMemoryStore_Remember_WhenDirDoesNotExist_ShouldReturnError(t *testing.T) {
	s := NewFileMemoryStore("/nonexistent/path/that/does/not/exist")
	err := s.Remember("anything")
	if err == nil {
		t.Fatal("expected error when dir does not exist")
	}
}

func TestFileMemoryStore_Remember_WhenWriteFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	s.writeStr = func(io.Writer, string) (int, error) { return 0, errors.New("injected write error") }
	err := s.Remember("anything")
	if err == nil {
		t.Fatal("expected error when write fails")
	}
	if err.Error() != "injected write error" {
		t.Errorf("expected injected error, got %v", err)
	}
}

func TestFileMemoryStore_Remember_ShouldAppendWithNewline(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	if err := s.Remember("first"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remember("second"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "memory.md"))
	// Each entry should be on its own line
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines, got %d: %q", len(lines), string(b))
	}
}

// =============================================================================
// LoadMemory tests
// =============================================================================

func TestFileMemoryStore_LoadMemory_WhenFileDoesNotExist_ShouldReturnEmptyString(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	content, err := s.LoadMemory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string, got %q", content)
	}
}

func TestFileMemoryStore_LoadMemory_WhenFileExists_ShouldReturnContent(t *testing.T) {
	dir := t.TempDir()
	// Pre-populate memory.md
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("- My favorite color is blue.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewFileMemoryStore(dir)
	content, err := s.LoadMemory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "My favorite color is blue.") {
		t.Errorf("expected content to contain fact, got %q", content)
	}
}

func TestFileMemoryStore_LoadMemory_AfterRemember_ShouldReturnRememberedContent(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	if err := s.Remember("User's name is John."); err != nil {
		t.Fatal(err)
	}
	content, err := s.LoadMemory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "User's name is John.") {
		t.Errorf("expected content to contain remembered fact, got %q", content)
	}
}

func TestFileMemoryStore_LoadMemory_WhenDirIsFile_ShouldReturnError(t *testing.T) {
	f := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create store pointing at a file, so memory.md path becomes notadir/memory.md
	s := NewFileMemoryStore(f)
	_, err := s.LoadMemory()
	// Reading from a path where dir is a file should error
	if err == nil {
		t.Fatal("expected error when store dir is a file")
	}
}

func TestFileMemoryStore_LoadMemory_WhenFileIsEmpty_ShouldReturnEmptyString(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewFileMemoryStore(dir)
	content, err := s.LoadMemory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string, got %q", content)
	}
}

func TestFileMemoryStore_LoadMemory_WhenMultipleRemember_ShouldReturnAll(t *testing.T) {
	dir := t.TempDir()
	s := NewFileMemoryStore(dir)
	facts := []string{
		"Favorite color is blue.",
		"Birthday is January 1.",
		"Prefers dark mode.",
	}
	for _, f := range facts {
		if err := s.Remember(f); err != nil {
			t.Fatal(err)
		}
	}
	content, err := s.LoadMemory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range facts {
		if !strings.Contains(content, f) {
			t.Errorf("expected content to contain %q, got %q", f, content)
		}
	}
}
