package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"

	"ironclaw/internal/domain"
)

// openFunc opens a file for reading; tests may replace it to inject errors.
type openFunc func(path string) (*os.File, error)

// seekFunc seeks within a file; tests may replace it to inject errors.
type seekFunc func(f *os.File, offset int64, whence int) (int64, error)

// SyncReader tracks a JSONL history file's read position and detects newly
// appended messages. It is the core primitive for cross-platform sync:
// another device (via Syncthing/Dropbox) appends lines to the same file,
// and SyncReader picks up only the new content.
type SyncReader struct {
	path   string
	offset int64
	known  map[string]bool // message IDs already seen (for dedup)
	openFn openFunc        // nil means use os.Open
	seekFn seekFunc        // nil means use f.Seek
}

// NewSyncReader creates a SyncReader for the given JSONL file path.
// The reader starts at offset 0 and has no known message IDs.
func NewSyncReader(path string) *SyncReader {
	return &SyncReader{
		path:  path,
		known: make(map[string]bool),
	}
}

// Offset returns the current byte offset into the file.
func (s *SyncReader) Offset() int64 {
	return s.offset
}

// KnownIDs returns a copy of the set of message IDs the reader has already seen.
func (s *SyncReader) KnownIDs() map[string]bool {
	out := make(map[string]bool, len(s.known))
	for k, v := range s.known {
		out[k] = v
	}
	return out
}

// MarkKnown registers a message ID as already seen, so it will be skipped
// during future ReadNew calls. Use this for locally-generated messages.
func (s *SyncReader) MarkKnown(id string) {
	if id != "" {
		s.known[id] = true
	}
}

// ReadNew reads any lines appended after the current offset, parses them as
// Messages, deduplicates by ID, advances the offset, and returns only new
// messages. Invalid JSON lines and empty lines are silently skipped.
//
// If the file has been truncated (i.e. is now smaller than the offset),
// the reader resets to the beginning and re-reads the entire file, still
// deduplicating against previously known IDs.
func (s *SyncReader) ReadNew() ([]domain.Message, error) {
	open := os.Open
	if s.openFn != nil {
		open = s.openFn
	}
	f, err := open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	// Detect truncation: if the file is now smaller than our offset, reset.
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < s.offset {
		s.offset = 0
	}

	// Seek to where we left off.
	if s.offset > 0 {
		seek := f.Seek
		if s.seekFn != nil {
			seek = func(offset int64, whence int) (int64, error) {
				return s.seekFn(f, offset, whence)
			}
		}
		if _, err := seek(s.offset, 0); err != nil {
			return nil, err
		}
	}

	scanner := bufio.NewScanner(f)
	var msgs []domain.Message
	var bytesRead int64

	for scanner.Scan() {
		line := scanner.Text()
		bytesRead += int64(len(scanner.Bytes())) + 1 // +1 for newline

		if line == "" {
			continue
		}

		var msg domain.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue // skip corrupt lines
		}

		// Deduplicate: skip if we've seen this ID before.
		if msg.ID != "" && s.known[msg.ID] {
			continue
		}

		// Track the ID.
		if msg.ID != "" {
			s.known[msg.ID] = true
		}

		msgs = append(msgs, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	s.offset += bytesRead
	return msgs, nil
}
