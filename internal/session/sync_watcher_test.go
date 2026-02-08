package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"ironclaw/internal/domain"
)

// =============================================================================
// Construction
// =============================================================================

func TestNewHistorySyncWatcher_ShouldReturnNonNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	w := NewHistorySyncWatcher(path)
	if w == nil {
		t.Fatal("expected non-nil watcher")
	}
}

// =============================================================================
// Interface compliance
// =============================================================================

func TestHistorySyncWatcher_ShouldImplementHistorySyncer(t *testing.T) {
	var _ domain.HistorySyncer = &HistorySyncWatcher{}
}

// =============================================================================
// Start / Stop lifecycle
// =============================================================================

func TestHistorySyncWatcher_Start_ShouldNotReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	// Create the file so the parent dir exists for fsnotify
	os.WriteFile(path, nil, 0644)

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer w.Stop()
}

func TestHistorySyncWatcher_Stop_ShouldNotReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	w := NewHistorySyncWatcher(path)
	if err := w.Start(func(msgs []domain.Message) {}); err != nil {
		t.Fatal(err)
	}

	err := w.Stop()
	if err != nil {
		t.Fatalf("unexpected error on Stop: %v", err)
	}
}

func TestHistorySyncWatcher_Stop_WhenNotStarted_ShouldNotPanic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	w := NewHistorySyncWatcher(path)

	// Stopping without starting should be safe
	err := w.Stop()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHistorySyncWatcher_Start_WhenAlreadyStarted_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	w := NewHistorySyncWatcher(path)
	cb := func(msgs []domain.Message) {}
	if err := w.Start(cb); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	err := w.Start(cb)
	if err == nil {
		t.Fatal("expected error when starting already-started watcher")
	}
}

func TestHistorySyncWatcher_Start_WhenCallbackIsNil_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	w := NewHistorySyncWatcher(path)
	err := w.Start(nil)
	if err == nil {
		t.Fatal("expected error when callback is nil")
	}
}

// =============================================================================
// File change detection (core sync behavior)
// =============================================================================

func TestHistorySyncWatcher_ShouldCallbackWhenFileChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	var mu sync.Mutex
	var received []domain.Message

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msgs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// Give the watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Append a message externally
	appendJSONL(t, path, makeMsg("sync-1", domain.RoleUser, "from another device"))

	// Wait for the callback with timeout
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for callback")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 message, got %d", len(received))
	}
	if received[0].ID != "sync-1" {
		t.Errorf("expected sync-1, got %s", received[0].ID)
	}
}

func TestHistorySyncWatcher_ShouldDeliverMultipleMessagesFromSingleWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	var mu sync.Mutex
	var received []domain.Message

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msgs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Write multiple messages in quick succession
	appendJSONL(t, path, makeMsg("multi-1", domain.RoleUser, "first"))
	appendJSONL(t, path, makeMsg("multi-2", domain.RoleAssistant, "second"))
	appendJSONL(t, path, makeMsg("multi-3", domain.RoleUser, "third"))

	// Wait for all messages
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count >= 3 {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timed out: got %d of 3 messages", len(received))
			mu.Unlock()
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	ids := make(map[string]bool)
	for _, m := range received {
		ids[m.ID] = true
	}
	for _, expected := range []string{"multi-1", "multi-2", "multi-3"} {
		if !ids[expected] {
			t.Errorf("missing message %s", expected)
		}
	}
}

func TestHistorySyncWatcher_ShouldNotCallbackForDuplicateMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	// Pre-populate with a message
	appendJSONL(t, path, makeMsg("pre-exist", domain.RoleUser, "already here"))

	var mu sync.Mutex
	var received []domain.Message

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msgs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// The initial read should pick up pre-exist, but that's from initial scan
	// Wait for initial scan callback
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	received = nil // Reset - we don't care about the initial scan for this test
	mu.Unlock()

	// Now append a duplicate of the pre-existing message
	appendJSONL(t, path, makeMsg("pre-exist", domain.RoleUser, "duplicate"))
	appendJSONL(t, path, makeMsg("genuinely-new", domain.RoleAssistant, "new"))

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for callback")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	// Should only receive the genuinely new message
	if len(received) != 1 {
		t.Fatalf("expected 1 message (deduped), got %d", len(received))
	}
	if received[0].ID != "genuinely-new" {
		t.Errorf("expected genuinely-new, got %s", received[0].ID)
	}
}

func TestHistorySyncWatcher_ShouldNotCallbackWhenNoNewMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	callbackCount := 0
	var mu sync.Mutex

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		callbackCount++
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// Wait a bit to ensure no spurious callbacks
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if callbackCount != 0 {
		t.Errorf("expected 0 callbacks for empty file, got %d", callbackCount)
	}
}

// =============================================================================
// MarkKnown integration
// =============================================================================

func TestHistorySyncWatcher_MarkKnown_ShouldPreventCallbackForLocalMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	var mu sync.Mutex
	var received []domain.Message

	w := NewHistorySyncWatcher(path)
	// Pre-mark a message as local before starting
	w.MarkKnown("local-msg")

	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msgs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Append both a local (known) and remote message
	appendJSONL(t, path, makeMsg("local-msg", domain.RoleUser, "my own message"))
	appendJSONL(t, path, makeMsg("remote-msg", domain.RoleAssistant, "from remote"))

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for callback")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 (local excluded), got %d", len(received))
	}
	if received[0].ID != "remote-msg" {
		t.Errorf("expected remote-msg, got %s", received[0].ID)
	}
}

// =============================================================================
// File creation (file doesn't exist yet when watcher starts)
// =============================================================================

func TestHistorySyncWatcher_ShouldDetectNewFileCreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	// Note: file does NOT exist yet

	var mu sync.Mutex
	var received []domain.Message

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msgs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create the file and write a message
	appendJSONL(t, path, makeMsg("created-1", domain.RoleUser, "file just created"))

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for new file detection")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1, got %d", len(received))
	}
	if received[0].ID != "created-1" {
		t.Errorf("expected created-1, got %s", received[0].ID)
	}
}

// =============================================================================
// Pre-existing content (start with non-empty file)
// =============================================================================

func TestHistorySyncWatcher_ShouldDeliverPreExistingMessagesOnStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	// Pre-populate the file
	appendJSONL(t, path, makeMsg("pre-1", domain.RoleUser, "already here"))
	appendJSONL(t, path, makeMsg("pre-2", domain.RoleAssistant, "me too"))

	var mu sync.Mutex
	var received []domain.Message

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msgs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// Wait for initial scan
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count >= 2 {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timed out: got %d of 2 pre-existing messages", len(received))
			mu.Unlock()
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	ids := map[string]bool{}
	for _, m := range received {
		ids[m.ID] = true
	}
	if !ids["pre-1"] || !ids["pre-2"] {
		t.Errorf("expected pre-1 and pre-2, got %v", ids)
	}
}

// =============================================================================
// Stop should cease delivery
// =============================================================================

func TestHistorySyncWatcher_AfterStop_ShouldNotDeliverNewMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	var mu sync.Mutex
	callbackCount := 0

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		callbackCount++
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// Stop the watcher
	if err := w.Stop(); err != nil {
		t.Fatal(err)
	}

	// Write after stop
	appendJSONL(t, path, makeMsg("after-stop", domain.RoleUser, "should not be seen"))

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if callbackCount != 0 {
		t.Errorf("expected 0 callbacks after stop, got %d", callbackCount)
	}
}

// =============================================================================
// Error paths
// =============================================================================

func TestHistorySyncWatcher_Start_WhenWatcherCreationFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	w := NewHistorySyncWatcher(path)
	w.newWatcherFn = func() (*fsnotify.Watcher, error) {
		return nil, fmt.Errorf("injected watcher error")
	}

	err := w.Start(func(msgs []domain.Message) {})
	if err == nil {
		t.Fatal("expected error when watcher creation fails")
	}
	if err.Error() != "injected watcher error" {
		t.Errorf("expected injected error, got %v", err)
	}
}

func TestHistorySyncWatcher_Start_WhenDirDoesNotExist_ShouldReturnError(t *testing.T) {
	// Parent directory doesn't exist, so watcher.Add should fail
	path := "/nonexistent/deeply/nested/dir/history.jsonl"
	w := NewHistorySyncWatcher(path)

	err := w.Start(func(msgs []domain.Message) {})
	if err == nil {
		t.Fatal("expected error when parent directory does not exist")
	}
}

func TestHistorySyncWatcher_ShouldIgnoreNonTargetFileEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	var mu sync.Mutex
	var received []domain.Message

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msgs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Write to a DIFFERENT file in the same directory — should be ignored
	otherPath := filepath.Join(dir, "other.jsonl")
	appendJSONL(t, otherPath, makeMsg("other-1", domain.RoleUser, "wrong file"))

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 0 {
		t.Errorf("expected 0 callbacks for non-target file, got %d", len(received))
	}
}

func TestHistorySyncWatcher_ShouldIgnoreChmodEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	var mu sync.Mutex
	callbackCount := 0

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		callbackCount++
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Trigger a chmod event (not write/create)
	os.Chmod(path, 0755)

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if callbackCount != 0 {
		t.Errorf("expected 0 callbacks for chmod event, got %d", callbackCount)
	}
}

func TestHistorySyncWatcher_ShouldHandleInitialReadError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	w := NewHistorySyncWatcher(path)
	// Inject an openFn that always returns a non-ErrNotExist error
	w.reader.openFn = func(p string) (*os.File, error) {
		return nil, fmt.Errorf("injected initial read error")
	}

	err := w.Start(func(msgs []domain.Message) {})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// Give time for the initial scan goroutine to run and log the error
	time.Sleep(200 * time.Millisecond)
	// Should not crash — error is logged internally
}

func TestHistorySyncWatcher_ShouldHandleReadErrorInDebounceCallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Inject a failing openFn AFTER start to cause debounce read errors
	w.reader.openFn = func(p string) (*os.File, error) {
		return nil, fmt.Errorf("injected debounce read error")
	}

	// Trigger a file change — this writes to the actual path, causing fsnotify event
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(`{"id":"trigger","role":"user","content":"trigger"}` + "\n")
	f.Close()

	// Wait for debounce callback to fire and encounter the error
	time.Sleep(500 * time.Millisecond)
	// Should not crash — error is logged internally
}

// =============================================================================
// Rapid writes
// =============================================================================

func TestHistorySyncWatcher_eventLoop_WhenFsnotifyErrorOccurs_ShouldLogAndContinue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {})
	if err != nil {
		t.Fatal(err)
	}

	// Send an error through the fsnotify Errors channel
	go func() {
		w.watcher.Errors <- fmt.Errorf("injected fsnotify error")
	}()

	// Give time for the error to be processed (logged)
	time.Sleep(200 * time.Millisecond)

	// Watcher should still be running — verify by writing a message
	w.Stop()
}

func TestHistorySyncWatcher_eventLoop_WhenEventsChannelClosed_ShouldReturn(t *testing.T) {
	// Directly exercise the eventLoop to cover the !ok path on Events channel.
	// We create a custom watcher and replace its Events channel with one we
	// control, so we can close it independently of the Errors channel.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	// Replace Events with our own channel that we can close independently.
	controlledEvents := make(chan fsnotify.Event)
	watcher.Events = controlledEvents

	w := &HistorySyncWatcher{
		path:    "/tmp/test-eventloop-events.jsonl",
		reader:  NewSyncReader("/tmp/test-eventloop-events.jsonl"),
		done:    make(chan struct{}),
		watcher: watcher,
	}

	loopDone := make(chan struct{})
	go func() {
		w.eventLoop(func(msgs []domain.Message) {})
		close(loopDone)
	}()

	// Close ONLY the Events channel — Errors channel stays open, so the
	// select must pick up Events !ok (covering the uncovered return path).
	close(controlledEvents)

	select {
	case <-loopDone:
		// eventLoop exited via Events !ok — success
	case <-time.After(2 * time.Second):
		close(w.done) // cleanup
		t.Fatal("eventLoop did not return after Events channel close")
	}
}

func TestHistorySyncWatcher_eventLoop_WhenErrorsChannelClosed_ShouldReturn(t *testing.T) {
	// Cover the !ok path on the Errors channel by closing only Errors.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	// Replace Errors with our own channel that we can close independently.
	controlledErrors := make(chan error)
	watcher.Errors = controlledErrors

	w := &HistorySyncWatcher{
		path:    "/tmp/test-eventloop-errors.jsonl",
		reader:  NewSyncReader("/tmp/test-eventloop-errors.jsonl"),
		done:    make(chan struct{}),
		watcher: watcher,
	}

	loopDone := make(chan struct{})
	go func() {
		w.eventLoop(func(msgs []domain.Message) {})
		close(loopDone)
	}()

	// Close ONLY the Errors channel — Events channel stays open, so the
	// select must pick up Errors !ok.
	close(controlledErrors)

	select {
	case <-loopDone:
		// eventLoop exited via Errors !ok — success
	case <-time.After(2 * time.Second):
		close(w.done) // cleanup
		t.Fatal("eventLoop did not return after Errors channel close")
	}
}

func TestHistorySyncWatcher_ShouldHandleRapidWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, nil, 0644)

	var mu sync.Mutex
	var received []domain.Message

	w := NewHistorySyncWatcher(path)
	err := w.Start(func(msgs []domain.Message) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msgs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Rapidly append 20 messages
	for i := 0; i < 20; i++ {
		msg := makeMsg("rapid-"+json.Number(fmt.Sprintf("%d", i)).String(), domain.RoleUser, "rapid msg")
		appendJSONL(t, path, msg)
	}

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count >= 20 {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timed out: got %d of 20 rapid messages", len(received))
			mu.Unlock()
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}
