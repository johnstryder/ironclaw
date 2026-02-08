package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ironclaw/internal/domain"
)

// =============================================================================
// Helpers
// =============================================================================

// appendJSONL writes a Message as a JSONL line to the file at path.
func appendJSONL(t *testing.T, path string, msg domain.Message) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Fatalf("write message: %v", err)
	}
}

func makeMsg(id string, role domain.MessageRole, text string) domain.Message {
	raw, _ := json.Marshal(text)
	return domain.Message{
		ID:         id,
		Role:       role,
		Timestamp:  time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
		RawContent: json.RawMessage(raw),
	}
}

// =============================================================================
// Construction
// =============================================================================

func TestNewSyncReader_ShouldReturnNonNilReader(t *testing.T) {
	r := NewSyncReader("/tmp/test.jsonl")
	if r == nil {
		t.Fatal("expected non-nil SyncReader")
	}
}

func TestNewSyncReader_ShouldStartAtOffsetZero(t *testing.T) {
	r := NewSyncReader("/tmp/test.jsonl")
	if r.Offset() != 0 {
		t.Errorf("expected offset 0, got %d", r.Offset())
	}
}

// =============================================================================
// ReadNew: basic scenarios
// =============================================================================

func TestSyncReader_ReadNew_WhenFileDoesNotExist_ShouldReturnEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.jsonl")
	r := NewSyncReader(path)

	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(msgs))
	}
}

func TestSyncReader_ReadNew_WhenFileIsEmpty_ShouldReturnEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewSyncReader(path)
	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(msgs))
	}
}

func TestSyncReader_ReadNew_ShouldReturnNewMessagesFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Append two messages before creating reader
	appendJSONL(t, path, makeMsg("msg-1", domain.RoleUser, "hello"))
	appendJSONL(t, path, makeMsg("msg-2", domain.RoleAssistant, "hi"))

	r := NewSyncReader(path)
	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != "msg-1" {
		t.Errorf("msgs[0].ID: want msg-1, got %s", msgs[0].ID)
	}
	if msgs[1].ID != "msg-2" {
		t.Errorf("msgs[1].ID: want msg-2, got %s", msgs[1].ID)
	}
}

func TestSyncReader_ReadNew_ShouldOnlyReturnNewLinesSinceLastRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Write initial messages
	appendJSONL(t, path, makeMsg("msg-1", domain.RoleUser, "first"))
	appendJSONL(t, path, makeMsg("msg-2", domain.RoleAssistant, "second"))

	r := NewSyncReader(path)

	// First read: should get both
	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("first read: expected 2, got %d", len(msgs))
	}

	// Second read without changes: should get nothing
	msgs, err = r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Errorf("second read: expected 0, got %d", len(msgs))
	}

	// Append a new message externally
	appendJSONL(t, path, makeMsg("msg-3", domain.RoleUser, "third"))

	// Third read: should get only the new message
	msgs, err = r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("third read: expected 1, got %d", len(msgs))
	}
	if msgs[0].ID != "msg-3" {
		t.Errorf("expected msg-3, got %s", msgs[0].ID)
	}
}

// =============================================================================
// ReadNew: deduplication
// =============================================================================

func TestSyncReader_ReadNew_ShouldDeduplicateByMessageID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Write a message, read it, then externally re-append the same message ID
	appendJSONL(t, path, makeMsg("dup-1", domain.RoleUser, "original"))

	r := NewSyncReader(path)
	msgs, _ := r.ReadNew()
	if len(msgs) != 1 {
		t.Fatalf("first read: expected 1, got %d", len(msgs))
	}

	// Simulate sync conflict: same ID appended again
	appendJSONL(t, path, makeMsg("dup-1", domain.RoleUser, "duplicate"))
	appendJSONL(t, path, makeMsg("new-1", domain.RoleAssistant, "new"))

	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	// Should only return the genuinely new message, not the duplicate
	if len(msgs) != 1 {
		t.Fatalf("second read: expected 1 (deduped), got %d", len(msgs))
	}
	if msgs[0].ID != "new-1" {
		t.Errorf("expected new-1, got %s", msgs[0].ID)
	}
}

func TestSyncReader_ReadNew_ShouldDeduplicateWithinSameBatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Write duplicate IDs in the same batch
	appendJSONL(t, path, makeMsg("dup-1", domain.RoleUser, "first"))
	appendJSONL(t, path, makeMsg("dup-1", domain.RoleUser, "second"))
	appendJSONL(t, path, makeMsg("unique", domain.RoleAssistant, "unique"))

	r := NewSyncReader(path)
	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	// Should return 2: first occurrence of dup-1 and unique
	if len(msgs) != 2 {
		t.Fatalf("expected 2 (deduped), got %d", len(msgs))
	}
	if msgs[0].ID != "dup-1" {
		t.Errorf("msgs[0].ID: want dup-1, got %s", msgs[0].ID)
	}
	if msgs[1].ID != "unique" {
		t.Errorf("msgs[1].ID: want unique, got %s", msgs[1].ID)
	}
}

// =============================================================================
// ReadNew: edge cases
// =============================================================================

func TestSyncReader_ReadNew_ShouldSkipInvalidJSONLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Write a valid message, then inject garbage, then another valid message
	appendJSONL(t, path, makeMsg("valid-1", domain.RoleUser, "good"))
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("this is not valid json\n")
	f.Close()
	appendJSONL(t, path, makeMsg("valid-2", domain.RoleAssistant, "also good"))

	r := NewSyncReader(path)
	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

func TestSyncReader_ReadNew_ShouldHandleEmptyLinesGracefully(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	appendJSONL(t, path, makeMsg("msg-1", domain.RoleUser, "hello"))
	// Inject empty lines
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("\n\n\n")
	f.Close()
	appendJSONL(t, path, makeMsg("msg-2", domain.RoleAssistant, "world"))

	r := NewSyncReader(path)
	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestSyncReader_ReadNew_WhenSeekFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	appendJSONL(t, path, makeMsg("msg-1", domain.RoleUser, "hello"))

	r := NewSyncReader(path)
	// First read to advance offset past 0
	if _, err := r.ReadNew(); err != nil {
		t.Fatal(err)
	}
	if r.Offset() == 0 {
		t.Fatal("offset should be > 0 after reading")
	}

	// Append more data so file size >= offset (avoids truncation reset)
	appendJSONL(t, path, makeMsg("msg-2", domain.RoleAssistant, "world"))

	// Inject a seek function that always fails
	r.seekFn = func(f *os.File, offset int64, whence int) (int64, error) {
		return 0, fmt.Errorf("injected seek error")
	}

	_, err := r.ReadNew()
	if err == nil {
		t.Fatal("expected error when seek fails")
	}
	if err.Error() != "injected seek error" {
		t.Errorf("expected injected seek error, got %v", err)
	}
}

func TestSyncReader_ReadNew_ShouldSkipMessagesWithEmptyID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	appendJSONL(t, path, makeMsg("", domain.RoleUser, "no id"))
	appendJSONL(t, path, makeMsg("has-id", domain.RoleAssistant, "has id"))

	r := NewSyncReader(path)
	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	// Empty-ID messages should still be returned (they just can't be deduped)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestSyncReader_ReadNew_WhenFilePermissionDenied_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"x","role":"user","content":"hi"}`+"\n"), 0000); err != nil {
		t.Fatal(err)
	}

	r := NewSyncReader(path)
	_, err := r.ReadNew()
	if err == nil {
		t.Fatal("expected error when file has no read permissions")
	}
}

// =============================================================================
// ReadNew: multiple sequential reads with interleaved appends
// =============================================================================

func TestSyncReader_ReadNew_MultipleExternalAppends_ShouldTrackOffsetCorrectly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	r := NewSyncReader(path)

	// First: file doesn't exist yet
	msgs, _ := r.ReadNew()
	if len(msgs) != 0 {
		t.Fatalf("round 1: expected 0, got %d", len(msgs))
	}

	// External device writes 2 messages
	appendJSONL(t, path, makeMsg("ext-1", domain.RoleUser, "from device A"))
	appendJSONL(t, path, makeMsg("ext-2", domain.RoleAssistant, "reply from A"))

	msgs, _ = r.ReadNew()
	if len(msgs) != 2 {
		t.Fatalf("round 2: expected 2, got %d", len(msgs))
	}

	// No changes
	msgs, _ = r.ReadNew()
	if len(msgs) != 0 {
		t.Fatalf("round 3: expected 0, got %d", len(msgs))
	}

	// Another device writes 1 message
	appendJSONL(t, path, makeMsg("ext-3", domain.RoleUser, "from device B"))

	msgs, _ = r.ReadNew()
	if len(msgs) != 1 {
		t.Fatalf("round 4: expected 1, got %d", len(msgs))
	}
	if msgs[0].ID != "ext-3" {
		t.Errorf("round 4: expected ext-3, got %s", msgs[0].ID)
	}
}

func TestSyncReader_ReadNew_ShouldHandleManyMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	for i := 0; i < 100; i++ {
		appendJSONL(t, path, makeMsg(fmt.Sprintf("m-%d", i), domain.RoleUser, fmt.Sprintf("msg %d", i)))
	}

	r := NewSyncReader(path)
	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 100 {
		t.Fatalf("expected 100, got %d", len(msgs))
	}

	// Append 50 more
	for i := 100; i < 150; i++ {
		appendJSONL(t, path, makeMsg(fmt.Sprintf("m-%d", i), domain.RoleUser, fmt.Sprintf("msg %d", i)))
	}

	msgs, err = r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 50 {
		t.Fatalf("expected 50 new, got %d", len(msgs))
	}
}

// =============================================================================
// Offset
// =============================================================================

func TestSyncReader_Offset_ShouldAdvanceAfterRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	appendJSONL(t, path, makeMsg("msg-1", domain.RoleUser, "hello"))

	r := NewSyncReader(path)
	if r.Offset() != 0 {
		t.Errorf("initial offset: expected 0, got %d", r.Offset())
	}

	r.ReadNew()
	if r.Offset() == 0 {
		t.Error("offset should have advanced after reading")
	}
}

func TestSyncReader_Offset_ShouldNotAdvanceWhenFileDoesNotExist(t *testing.T) {
	r := NewSyncReader("/tmp/nonexistent_sync_test.jsonl")
	r.ReadNew()
	if r.Offset() != 0 {
		t.Errorf("offset should be 0 for nonexistent file, got %d", r.Offset())
	}
}

// =============================================================================
// KnownIDs
// =============================================================================

func TestSyncReader_KnownIDs_ShouldTrackSeenMessageIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	appendJSONL(t, path, makeMsg("tracked-1", domain.RoleUser, "hello"))
	appendJSONL(t, path, makeMsg("tracked-2", domain.RoleAssistant, "hi"))

	r := NewSyncReader(path)
	r.ReadNew()

	known := r.KnownIDs()
	if !known["tracked-1"] {
		t.Error("expected tracked-1 in known IDs")
	}
	if !known["tracked-2"] {
		t.Error("expected tracked-2 in known IDs")
	}
}

// =============================================================================
// MarkKnown
// =============================================================================

func TestSyncReader_MarkKnown_ShouldPreventDuplicateOnNextRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	r := NewSyncReader(path)
	// Pre-mark a message ID as known (e.g. local message we sent)
	r.MarkKnown("local-1")

	// Now external device appends a message with that same ID
	appendJSONL(t, path, makeMsg("local-1", domain.RoleUser, "from local"))
	appendJSONL(t, path, makeMsg("remote-1", domain.RoleAssistant, "from remote"))

	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	// Should only get the remote one since local-1 was pre-marked
	if len(msgs) != 1 {
		t.Fatalf("expected 1 (local-1 excluded), got %d", len(msgs))
	}
	if msgs[0].ID != "remote-1" {
		t.Errorf("expected remote-1, got %s", msgs[0].ID)
	}
}

// =============================================================================
// File truncation (sync conflict where file gets shorter)
// =============================================================================

func TestSyncReader_ReadNew_WhenStatFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	appendJSONL(t, path, makeMsg("msg-1", domain.RoleUser, "hello"))

	r := NewSyncReader(path)

	// Inject an openFn that returns a file-like object whose Stat fails.
	// We do this by opening a real file, closing it, then returning it.
	r.openFn = func(p string) (*os.File, error) {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		// Close the file so Stat will fail
		f.Close()
		return f, nil
	}

	_, err := r.ReadNew()
	if err == nil {
		t.Fatal("expected error when Stat fails on closed file")
	}
}

func TestSyncReader_ReadNew_WhenOpenFnReturnsNonExistError_ShouldReturnError(t *testing.T) {
	r := NewSyncReader("/some/path.jsonl")
	injectedErr := fmt.Errorf("injected open error")
	r.openFn = func(p string) (*os.File, error) {
		return nil, injectedErr
	}

	_, err := r.ReadNew()
	if err == nil {
		t.Fatal("expected error from injected openFn")
	}
	if err.Error() != "injected open error" {
		t.Errorf("expected injected error, got %v", err)
	}
}

func TestSyncReader_ReadNew_WhenScannerErrors_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Write a line that exceeds the default scanner buffer (64KB) to trigger scanner error
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Write a very long line without a newline (exceeds bufio.MaxScanTokenSize = 64*1024)
	longLine := make([]byte, 70*1024)
	for i := range longLine {
		longLine[i] = 'x'
	}
	f.Write(longLine)
	f.Close()

	r := NewSyncReader(path)
	_, err = r.ReadNew()
	if err == nil {
		t.Fatal("expected error from scanner exceeding buffer")
	}
}

func TestSyncReader_MarkKnown_EmptyID_ShouldBeIgnored(t *testing.T) {
	r := NewSyncReader("/tmp/test.jsonl")
	r.MarkKnown("")
	if len(r.KnownIDs()) != 0 {
		t.Error("expected empty known IDs after marking empty string")
	}
}

func TestSyncReader_ReadNew_WhenFileTruncated_ShouldResetAndRereadFromStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	appendJSONL(t, path, makeMsg("old-1", domain.RoleUser, "old message one"))
	appendJSONL(t, path, makeMsg("old-2", domain.RoleAssistant, "old message two"))

	r := NewSyncReader(path)
	msgs, _ := r.ReadNew()
	if len(msgs) != 2 {
		t.Fatalf("initial: expected 2, got %d", len(msgs))
	}

	// Simulate file being replaced (truncated + new content, smaller than before)
	os.WriteFile(path, nil, 0644)
	appendJSONL(t, path, makeMsg("new-1", domain.RoleUser, "fresh start"))

	msgs, err := r.ReadNew()
	if err != nil {
		t.Fatal(err)
	}
	// After truncation reset, should re-read from start; old-1/old-2 already known, new-1 is new
	if len(msgs) != 1 {
		t.Fatalf("after truncation: expected 1 (new-1), got %d", len(msgs))
	}
	if msgs[0].ID != "new-1" {
		t.Errorf("expected new-1, got %s", msgs[0].ID)
	}
}
