package context

import (
	"encoding/json"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// MessageText Helper Tests
// =============================================================================

func TestMessageText_WhenStringContent_ShouldReturnText(t *testing.T) {
	raw, _ := json.Marshal("Hello, world!")
	msg := domain.Message{
		Role:       domain.RoleUser,
		RawContent: raw,
	}
	// Parse content blocks via unmarshal round-trip
	data, _ := json.Marshal(msg)
	var parsed domain.Message
	_ = json.Unmarshal(data, &parsed)

	got := MessageText(parsed)
	if got != "Hello, world!" {
		t.Errorf("expected %q, got %q", "Hello, world!", got)
	}
}

func TestMessageText_WhenTextBlocks_ShouldConcatenateText(t *testing.T) {
	msg := domain.Message{
		Role: domain.RoleAssistant,
		ContentBlocks: []domain.ContentBlock{
			domain.TextBlock{Text: "First part."},
			domain.TextBlock{Text: "Second part."},
		},
	}
	got := MessageText(msg)
	want := "First part.\nSecond part."
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestMessageText_WhenToolUseBlock_ShouldIncludeNameAndInput(t *testing.T) {
	msg := domain.Message{
		Role: domain.RoleAssistant,
		ContentBlocks: []domain.ContentBlock{
			domain.ToolUseBlock{
				ToolUseID: "tool_1",
				Name:      "shell",
				Input:     json.RawMessage(`{"command":"ls"}`),
			},
		},
	}
	got := MessageText(msg)
	if got == "" {
		t.Error("expected non-empty text for tool_use block")
	}
	if !containsSubstring(got, "shell") {
		t.Errorf("expected tool name 'shell' in text, got %q", got)
	}
	if !containsSubstring(got, "ls") {
		t.Errorf("expected tool input 'ls' in text, got %q", got)
	}
}

func TestMessageText_WhenToolResultBlock_ShouldIncludeContent(t *testing.T) {
	msg := domain.Message{
		Role: domain.RoleAssistant,
		ContentBlocks: []domain.ContentBlock{
			domain.ToolResultBlock{
				ToolUseID: "tool_1",
				Content:   "file1.txt\nfile2.txt",
			},
		},
	}
	got := MessageText(msg)
	if got != "file1.txt\nfile2.txt" {
		t.Errorf("expected tool result content, got %q", got)
	}
}

func TestMessageText_WhenEmptyMessage_ShouldReturnEmptyString(t *testing.T) {
	msg := domain.Message{Role: domain.RoleUser}
	got := MessageText(msg)
	if got != "" {
		t.Errorf("expected empty string for empty message, got %q", got)
	}
}

func TestMessageText_WhenImageBlock_ShouldReturnPlaceholder(t *testing.T) {
	msg := domain.Message{
		Role: domain.RoleUser,
		ContentBlocks: []domain.ContentBlock{
			domain.ImageBlock{
				Source: domain.MediaType{
					Type:      "base64",
					MediaType: "image/jpeg",
					Data:      "abc123",
				},
			},
		},
	}
	got := MessageText(msg)
	if got != "[image]" {
		t.Errorf("expected '[image]' placeholder, got %q", got)
	}
}

func TestMessageText_WhenMixedBlocks_ShouldConcatenateAll(t *testing.T) {
	msg := domain.Message{
		Role: domain.RoleAssistant,
		ContentBlocks: []domain.ContentBlock{
			domain.TextBlock{Text: "Running command..."},
			domain.ToolUseBlock{
				ToolUseID: "t1",
				Name:      "shell",
				Input:     json.RawMessage(`{"cmd":"pwd"}`),
			},
		},
	}
	got := MessageText(msg)
	if !containsSubstring(got, "Running command...") {
		t.Errorf("expected text block content in result, got %q", got)
	}
	if !containsSubstring(got, "shell") {
		t.Errorf("expected tool name in result, got %q", got)
	}
}

func TestMessageText_WhenRawContentOnly_ShouldFallbackToRawContent(t *testing.T) {
	msg := domain.Message{
		Role:       domain.RoleUser,
		RawContent: json.RawMessage(`"just raw text"`),
	}
	got := MessageText(msg)
	if got != "just raw text" {
		t.Errorf("expected raw text fallback, got %q", got)
	}
}

func TestMessageText_WhenRawContentIsArrayJSON_ShouldReturnRawString(t *testing.T) {
	msg := domain.Message{
		Role:       domain.RoleUser,
		RawContent: json.RawMessage(`[{"type":"text","text":"hello"}]`),
	}
	got := MessageText(msg)
	// Without parsed ContentBlocks, should return the raw JSON as-is
	if got == "" {
		t.Error("expected non-empty result for raw JSON array")
	}
}

// containsSubstring is a test helper.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
