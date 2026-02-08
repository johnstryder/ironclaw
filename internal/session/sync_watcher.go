package session

import (
	"errors"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"ironclaw/internal/domain"
)

// debounceDelay is the time to wait after a file event before reading new content.
// This coalesces rapid successive writes into a single read.
var debounceDelay = 100 * time.Millisecond

// newWatcherFunc creates an fsnotify watcher; tests may replace it to inject errors.
type newWatcherFunc func() (*fsnotify.Watcher, error)

// HistorySyncWatcher watches a history JSONL file for external changes (e.g.
// from Syncthing, Dropbox, or another device writing to the same file) and
// delivers newly appended messages via a callback. It implements domain.HistorySyncer.
type HistorySyncWatcher struct {
	path         string
	reader       *SyncReader
	watcher      *fsnotify.Watcher
	done         chan struct{}
	mu           sync.Mutex
	running      bool
	newWatcherFn newWatcherFunc // nil means use fsnotify.NewWatcher
}

// NewHistorySyncWatcher creates a watcher for the given JSONL history file.
// Call Start to begin watching and Stop to release resources.
func NewHistorySyncWatcher(path string) *HistorySyncWatcher {
	return &HistorySyncWatcher{
		path:   path,
		reader: NewSyncReader(path),
	}
}

// MarkKnown registers a message ID as already seen, so it will be skipped
// by the sync reader. Use this for locally-generated messages.
func (w *HistorySyncWatcher) MarkKnown(id string) {
	w.reader.MarkKnown(id)
}

// Start begins watching the history file for changes. The callback is invoked
// (on a separate goroutine) whenever new messages are detected. Start must not
// be called more than once without an intervening Stop.
func (w *HistorySyncWatcher) Start(callback func([]domain.Message)) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if callback == nil {
		return errors.New("sync watcher: callback must not be nil")
	}
	if w.running {
		return errors.New("sync watcher: already started")
	}

	// Watch the parent directory so we catch file creation events too
	// (the file may not exist yet when the watcher starts).
	dir := filepath.Dir(w.path)
	newWatcher := fsnotify.NewWatcher
	if w.newWatcherFn != nil {
		newWatcher = w.newWatcherFn
	}
	watcher, err := newWatcher()
	if err != nil {
		return err
	}
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return err
	}

	w.watcher = watcher
	w.done = make(chan struct{})
	w.running = true

	// Perform an initial scan for pre-existing content.
	go func() {
		msgs, err := w.reader.ReadNew()
		if err != nil {
			log.Printf("sync watcher: initial read error: %v", err)
		}
		if len(msgs) > 0 {
			callback(msgs)
		}
	}()

	// Start the event loop.
	go w.eventLoop(callback)

	return nil
}

// Stop ceases watching and releases resources. Safe to call even if not started.
func (w *HistorySyncWatcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	close(w.done)
	err := w.watcher.Close()
	w.running = false
	return err
}

// eventLoop listens for fsnotify events and reads new content with debouncing.
func (w *HistorySyncWatcher) eventLoop(callback func([]domain.Message)) {
	target := filepath.Base(w.path)
	var debounceTimer *time.Timer

	for {
		select {
		case <-w.done:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			// Only react to our specific file.
			if filepath.Base(event.Name) != target {
				continue
			}
			// We care about writes and creates.
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			// Debounce: reset the timer on every qualifying event.
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				msgs, err := w.reader.ReadNew()
				if err != nil {
					log.Printf("sync watcher: read error: %v", err)
					return
				}
				if len(msgs) > 0 {
					callback(msgs)
				}
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("sync watcher: fsnotify error: %v", err)
		}
	}
}

// Ensure HistorySyncWatcher implements domain.HistorySyncer.
var _ domain.HistorySyncer = (*HistorySyncWatcher)(nil)
