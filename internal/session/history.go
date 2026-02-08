package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"

	"ironclaw/internal/domain"
)

// writeFunc is used to write content so tests can inject a failing implementation.
type writeFunc func(f *os.File, data []byte) (int, error)

// marshalFunc is the JSON marshaling function; tests may replace it to force errors.
type marshalFunc func(v any) ([]byte, error)

// HistoryStore persists session messages to a JSONL file (one JSON object per line).
// It supports appending new messages and loading the last N messages for context restoration.
type HistoryStore struct {
	path      string
	writeFn   writeFunc   // nil means use f.Write
	marshalFn marshalFunc // nil means use json.Marshal
}

// NewHistoryStore returns a HistoryStore that reads/writes to the given JSONL file path.
func NewHistoryStore(path string) *HistoryStore {
	return &HistoryStore{path: path}
}

// Append serializes a Message to JSON and appends it as a single line to the history file.
func (h *HistoryStore) Append(msg domain.Message) error {
	marshal := json.Marshal
	if h.marshalFn != nil {
		marshal = h.marshalFn
	}
	data, err := marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	var writeErr error
	if h.writeFn != nil {
		_, writeErr = h.writeFn(f, data)
	} else {
		_, writeErr = f.Write(data)
	}
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// LoadHistory reads the last n messages from the history file.
// Returns empty slice when the file does not exist or n <= 0.
func (h *HistoryStore) LoadHistory(n int) ([]domain.Message, error) {
	if n <= 0 {
		return nil, nil
	}

	f, err := os.Open(h.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	// Collect all non-empty lines from the file.
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Take only the last n lines.
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	msgs := make([]domain.Message, 0, len(lines))
	for _, line := range lines {
		var msg domain.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue // skip corrupt lines
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// Ensure HistoryStore implements domain.SessionHistoryStore.
var _ domain.SessionHistoryStore = (*HistoryStore)(nil)
