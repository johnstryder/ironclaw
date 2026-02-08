package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"ironclaw/internal/domain"
	"ironclaw/internal/session"
)

func TestHistoryManual(t *testing.T) {
	path := "history.jsonl"
	os.Remove(path) // start fresh
	defer os.Remove(path)

	store := session.NewHistoryStore(path)

	// Simulate a conversation
	msgs := []domain.Message{
		{ID: "1", Role: domain.RoleUser, Timestamp: time.Now(), RawContent: json.RawMessage(`"Hello, who are you?"`)},
		{ID: "2", Role: domain.RoleAssistant, Timestamp: time.Now(), RawContent: json.RawMessage(`"I am Ironclaw, your AI assistant."`)},
		{ID: "3", Role: domain.RoleUser, Timestamp: time.Now(), RawContent: json.RawMessage(`"What can you do?"`)},
		{ID: "4", Role: domain.RoleAssistant, Timestamp: time.Now(), RawContent: json.RawMessage(`"I can help with many things!"`)},
	}

	t.Log("=== Appending messages ===")
	for _, m := range msgs {
		if err := store.Append(m); err != nil {
			t.Fatalf("append error: %v", err)
		}
		t.Logf("  Appended: [%s] %s", m.Role, string(m.RawContent))
	}

	t.Log("\n=== Raw JSONL file content ===")
	b, _ := os.ReadFile(path)
	t.Log(string(b))

	t.Log("=== Verifying each line is valid JSON ===")
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	for i, line := range lines {
		var check map[string]interface{}
		if err := json.Unmarshal([]byte(line), &check); err != nil {
			t.Errorf("  Line %d: INVALID JSON - %v", i+1, err)
		} else {
			t.Logf("  Line %d: valid JSON", i+1)
		}
	}

	t.Log("\n=== Loading last 2 messages (simulating restart) ===")
	loaded, err := store.LoadHistory(2)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	for _, m := range loaded {
		t.Logf("  Loaded: [%s] id=%s content=%s", m.Role, m.ID, string(m.RawContent))
		if len(m.ContentBlocks) > 0 {
			t.Logf("    ContentBlocks parsed: %d block(s)", len(m.ContentBlocks))
		}
	}

	t.Log("\n=== Loading ALL messages ===")
	all, _ := store.LoadHistory(100)
	t.Logf("  Total messages loaded: %d", len(all))
	for _, m := range all {
		t.Logf("  [%s] %s: %s", m.Timestamp.Format("15:04:05"), m.Role, string(m.RawContent))
	}

	t.Logf("\nDone! %d lines written to %s", len(lines), path)
}
