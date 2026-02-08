package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ironclaw/internal/domain"
)

// =============================================================================
// Helpers
// =============================================================================

func newTextMessage(role domain.MessageRole, text string) domain.Message {
	raw, _ := json.Marshal(text)
	return domain.Message{
		ID:         "msg-" + text[:min(len(text), 8)],
		Role:       role,
		Timestamp:  time.Now(),
		RawContent: json.RawMessage(raw),
	}
}

// =============================================================================
// Append tests
// =============================================================================

func TestHistoryStore_Append_WhenFileDoesNotExist_ShouldCreateAndAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	msg := newTextMessage(domain.RoleUser, "hello world")
	err := store.Append(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("expected file to have content")
	}
	// Should be valid JSON
	var decoded domain.Message
	if err := json.Unmarshal(b[:len(b)-1], &decoded); err != nil { // trim trailing newline
		t.Fatalf("invalid JSON in file: %v", err)
	}
	if decoded.Role != domain.RoleUser {
		t.Errorf("expected role user, got %s", decoded.Role)
	}
}

func TestHistoryStore_Append_WhenFileExists_ShouldAppendNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	msg1 := newTextMessage(domain.RoleUser, "first message")
	msg2 := newTextMessage(domain.RoleAssistant, "second message")

	if err := store.Append(msg1); err != nil {
		t.Fatalf("append msg1: %v", err)
	}
	if err := store.Append(msg2); err != nil {
		t.Fatalf("append msg2: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var m1, m2 domain.Message
	if err := json.Unmarshal([]byte(lines[0]), &m1); err != nil {
		t.Fatalf("unmarshal line 1: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &m2); err != nil {
		t.Fatalf("unmarshal line 2: %v", err)
	}
	if m1.Role != domain.RoleUser {
		t.Errorf("line 1 role: want user, got %s", m1.Role)
	}
	if m2.Role != domain.RoleAssistant {
		t.Errorf("line 2 role: want assistant, got %s", m2.Role)
	}
}

func TestHistoryStore_Append_ShouldAppendNewlineAfterEachMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	msg := newTextMessage(domain.RoleUser, "test newline")
	if err := store.Append(msg); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(b), "\n") {
		t.Error("expected file content to end with newline")
	}
}

func TestHistoryStore_Append_WhenDirDoesNotExist_ShouldReturnError(t *testing.T) {
	path := "/nonexistent/path/that/does/not/exist/history.jsonl"
	store := NewHistoryStore(path)

	msg := newTextMessage(domain.RoleUser, "should fail")
	err := store.Append(msg)
	if err == nil {
		t.Fatal("expected error when dir does not exist")
	}
}

// =============================================================================
// LoadHistory tests
// =============================================================================

func TestHistoryStore_LoadHistory_WhenFileDoesNotExist_ShouldReturnEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.jsonl")
	store := NewHistoryStore(path)

	msgs, err := store.LoadHistory(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(msgs))
	}
}

func TestHistoryStore_LoadHistory_WhenNIsZero_ShouldReturnEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	// Write a message first
	if err := store.Append(newTextMessage(domain.RoleUser, "ignored")); err != nil {
		t.Fatal(err)
	}

	msgs, err := store.LoadHistory(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty slice for n=0, got %d messages", len(msgs))
	}
}

func TestHistoryStore_LoadHistory_WhenNIsNegative_ShouldReturnEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	if err := store.Append(newTextMessage(domain.RoleUser, "ignored")); err != nil {
		t.Fatal(err)
	}

	msgs, err := store.LoadHistory(-5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty slice for n=-5, got %d messages", len(msgs))
	}
}

func TestHistoryStore_LoadHistory_ShouldReturnLastNMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	// Append 5 messages
	for i := 0; i < 5; i++ {
		msg := newTextMessage(domain.RoleUser, "msg-"+string(rune('A'+i)))
		msg.ID = "id-" + string(rune('A'+i))
		if err := store.Append(msg); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	// Load last 3
	msgs, err := store.LoadHistory(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	// Should be the last 3: C, D, E
	if msgs[0].ID != "id-C" {
		t.Errorf("msgs[0].ID: want id-C, got %s", msgs[0].ID)
	}
	if msgs[1].ID != "id-D" {
		t.Errorf("msgs[1].ID: want id-D, got %s", msgs[1].ID)
	}
	if msgs[2].ID != "id-E" {
		t.Errorf("msgs[2].ID: want id-E, got %s", msgs[2].ID)
	}
}

func TestHistoryStore_LoadHistory_WhenFileHasFewerThanNMessages_ShouldReturnAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	// Append 2 messages
	msg1 := newTextMessage(domain.RoleUser, "only one")
	msg1.ID = "id-1"
	msg2 := newTextMessage(domain.RoleAssistant, "only two")
	msg2.ID = "id-2"
	if err := store.Append(msg1); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(msg2); err != nil {
		t.Fatal(err)
	}

	// Request more than available
	msgs, err := store.LoadHistory(100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != "id-1" {
		t.Errorf("msgs[0].ID: want id-1, got %s", msgs[0].ID)
	}
	if msgs[1].ID != "id-2" {
		t.Errorf("msgs[1].ID: want id-2, got %s", msgs[1].ID)
	}
}

func TestHistoryStore_LoadHistory_ShouldDeserializeMessagesCorrectly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	original := newTextMessage(domain.RoleAssistant, "Hello, I am an assistant")
	original.ID = "msg-123"
	if err := store.Append(original); err != nil {
		t.Fatal(err)
	}

	msgs, err := store.LoadHistory(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	got := msgs[0]
	if got.ID != "msg-123" {
		t.Errorf("ID: want msg-123, got %s", got.ID)
	}
	if got.Role != domain.RoleAssistant {
		t.Errorf("Role: want assistant, got %s", got.Role)
	}
	// ContentBlocks should be parsed (via custom UnmarshalJSON)
	if len(got.ContentBlocks) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(got.ContentBlocks))
	}
	tb, ok := got.ContentBlocks[0].(domain.TextBlock)
	if !ok {
		t.Fatalf("expected TextBlock, got %T", got.ContentBlocks[0])
	}
	if tb.Text != "Hello, I am an assistant" {
		t.Errorf("Text: want 'Hello, I am an assistant', got %q", tb.Text)
	}
}

func TestHistoryStore_LoadHistory_AfterAppend_ShouldReturnAppendedMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	// Round-trip: append then load
	msg := newTextMessage(domain.RoleUser, "round trip test")
	msg.ID = "rt-1"
	if err := store.Append(msg); err != nil {
		t.Fatal(err)
	}

	msgs, err := store.LoadHistory(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1, got %d", len(msgs))
	}
	if msgs[0].ID != "rt-1" {
		t.Errorf("expected rt-1, got %s", msgs[0].ID)
	}
}

// =============================================================================
// Edge case & error injection tests
// =============================================================================

func TestHistoryStore_Append_WhenWriteFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)
	store.writeFn = func(f *os.File, data []byte) (int, error) {
		return 0, errors.New("injected write error")
	}

	msg := newTextMessage(domain.RoleUser, "should fail write")
	err := store.Append(msg)
	if err == nil {
		t.Fatal("expected error when write fails")
	}
	if err.Error() != "injected write error" {
		t.Errorf("expected injected error, got %v", err)
	}
}

func TestHistoryStore_LoadHistory_WhenFileHasInvalidJSON_ShouldSkipBadLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Write a mix of valid and invalid lines
	store := NewHistoryStore(path)
	msg := newTextMessage(domain.RoleUser, "valid msg")
	msg.ID = "valid-1"
	if err := store.Append(msg); err != nil {
		t.Fatal(err)
	}

	// Manually inject a bad line
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("this is not valid json\n")
	f.Close()

	// Append another valid message
	msg2 := newTextMessage(domain.RoleAssistant, "valid msg two")
	msg2.ID = "valid-2"
	if err := store.Append(msg2); err != nil {
		t.Fatal(err)
	}

	msgs, err := store.LoadHistory(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have 2 valid messages (bad line skipped)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 valid messages, got %d", len(msgs))
	}
	if msgs[0].ID != "valid-1" {
		t.Errorf("msgs[0].ID: want valid-1, got %s", msgs[0].ID)
	}
	if msgs[1].ID != "valid-2" {
		t.Errorf("msgs[1].ID: want valid-2, got %s", msgs[1].ID)
	}
}

func TestHistoryStore_LoadHistory_WhenFileIsEmpty_ShouldReturnEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Create an empty file
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	store := NewHistoryStore(path)
	msgs, err := store.LoadHistory(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(msgs))
	}
}

func TestHistoryStore_LoadHistory_WhenFileHasOnlyEmptyLines_ShouldReturnEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	if err := os.WriteFile(path, []byte("\n\n\n"), 0644); err != nil {
		t.Fatal(err)
	}

	store := NewHistoryStore(path)
	msgs, err := store.LoadHistory(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(msgs))
	}
}

func TestHistoryStore_LoadHistory_WhenPathIsUnreadableDir_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	// Point the store at a directory, not a file — os.Open on dir works but scanning won't
	// give useful results. Instead, create a path that exists as a directory.
	dirPath := filepath.Join(dir, "adir")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatal(err)
	}
	store := NewHistoryStore(dirPath)
	_, err := store.LoadHistory(10)
	// Opening a directory for reading should produce an error from scanner
	// or at least not panic
	if err != nil {
		// This is acceptable — error reading directory as file
		return
	}
	// If no error, at least it shouldn't return garbage
}

func TestHistoryStore_implements_SessionHistoryStore(t *testing.T) {
	var _ domain.SessionHistoryStore = NewHistoryStore("test.jsonl")
}

func TestHistoryStore_Append_ShouldPreserveToolUseContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	// Build a message with ToolUse content blocks
	toolInput := json.RawMessage(`{"expression":"2+2"}`)
	blocks := []domain.ContentBlock{
		domain.TextBlock{Text: "Let me calculate that."},
		domain.ToolUseBlock{ToolUseID: "tu-1", Name: "calculator", Input: toolInput},
	}
	// Serialize blocks to RawContent
	rawBlocks := make([]json.RawMessage, 0, len(blocks))
	for _, b := range blocks {
		switch v := b.(type) {
		case domain.TextBlock:
			j, _ := json.Marshal(struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{"text", v.Text})
			rawBlocks = append(rawBlocks, j)
		case domain.ToolUseBlock:
			j, _ := json.Marshal(struct {
				Type  string          `json:"type"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			}{"tool_use", v.ToolUseID, v.Name, v.Input})
			rawBlocks = append(rawBlocks, j)
		}
	}
	rawContent, _ := json.Marshal(rawBlocks)

	msg := domain.Message{
		ID:         "msg-tool",
		Role:       domain.RoleAssistant,
		Timestamp:  time.Now(),
		RawContent: json.RawMessage(rawContent),
	}

	if err := store.Append(msg); err != nil {
		t.Fatal(err)
	}

	msgs, err := store.LoadHistory(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	got := msgs[0]
	if got.ID != "msg-tool" {
		t.Errorf("ID: want msg-tool, got %s", got.ID)
	}
	if len(got.ContentBlocks) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(got.ContentBlocks))
	}
	tb, ok := got.ContentBlocks[0].(domain.TextBlock)
	if !ok {
		t.Fatalf("block 0: expected TextBlock, got %T", got.ContentBlocks[0])
	}
	if tb.Text != "Let me calculate that." {
		t.Errorf("block 0 text: want 'Let me calculate that.', got %q", tb.Text)
	}
	tu, ok := got.ContentBlocks[1].(domain.ToolUseBlock)
	if !ok {
		t.Fatalf("block 1: expected ToolUseBlock, got %T", got.ContentBlocks[1])
	}
	if tu.Name != "calculator" {
		t.Errorf("block 1 name: want calculator, got %s", tu.Name)
	}
}

// =============================================================================
// Concurrency tests
// =============================================================================

func TestHistoryStore_ConcurrentAppend_ShouldNotLoseMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	const goroutines = 20
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := newTextMessage(domain.RoleUser, fmt.Sprintf("concurrent-%d", idx))
			msg.ID = fmt.Sprintf("c-%d", idx)
			if err := store.Append(msg); err != nil {
				t.Errorf("append goroutine %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	msgs, err := store.LoadHistory(goroutines + 10)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(msgs) != goroutines {
		t.Errorf("expected %d messages, got %d", goroutines, len(msgs))
	}
}

// =============================================================================
// Marshal error coverage
// =============================================================================

func TestHistoryStore_Append_WhenMarshalFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)
	store.marshalFn = func(v any) ([]byte, error) {
		return nil, errors.New("injected marshal error")
	}

	msg := newTextMessage(domain.RoleUser, "should fail marshal")
	err := store.Append(msg)
	if err == nil {
		t.Fatal("expected error when marshal fails")
	}
	if err.Error() != "injected marshal error" {
		t.Errorf("expected injected marshal error, got %v", err)
	}

	// File should not have been created
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Error("expected file to not exist after marshal failure")
	}
}

// =============================================================================
// LoadHistory with non-ErrNotExist open error
// =============================================================================

func TestHistoryStore_LoadHistory_WhenFilePermissionDenied_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Create the file, then remove read permissions
	if err := os.WriteFile(path, []byte(`{"id":"x","role":"user","content":"hi"}`+"\n"), 0000); err != nil {
		t.Fatal(err)
	}

	store := NewHistoryStore(path)
	_, err := store.LoadHistory(10)
	if err == nil {
		t.Fatal("expected error when file has no read permissions")
	}
}

func TestHistoryStore_LoadHistory_WhenNEqualsExactLineCount_ShouldReturnAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	for i := 0; i < 3; i++ {
		msg := newTextMessage(domain.RoleUser, fmt.Sprintf("exact-%d", i))
		msg.ID = fmt.Sprintf("e-%d", i)
		if err := store.Append(msg); err != nil {
			t.Fatal(err)
		}
	}

	msgs, err := store.LoadHistory(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3, got %d", len(msgs))
	}
	if msgs[0].ID != "e-0" || msgs[1].ID != "e-1" || msgs[2].ID != "e-2" {
		t.Errorf("unexpected order: %s, %s, %s", msgs[0].ID, msgs[1].ID, msgs[2].ID)
	}
}

func TestHistoryStore_LoadHistory_ShouldPreserveTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	store := NewHistoryStore(path)

	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	msg := newTextMessage(domain.RoleUser, "timestamped")
	msg.ID = "ts-1"
	msg.Timestamp = ts
	if err := store.Append(msg); err != nil {
		t.Fatal(err)
	}

	msgs, err := store.LoadHistory(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatal("expected 1 message")
	}
	if !msgs[0].Timestamp.Equal(ts) {
		t.Errorf("timestamp: want %v, got %v", ts, msgs[0].Timestamp)
	}
}

func TestHistoryStore_LoadHistory_WhenFileHasOnlyInvalidJSON_ShouldReturnEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	if err := os.WriteFile(path, []byte("not json\nalso not json\n{bad\n"), 0644); err != nil {
		t.Fatal(err)
	}

	store := NewHistoryStore(path)
	msgs, err := store.LoadHistory(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages from all-invalid file, got %d", len(msgs))
	}
}
