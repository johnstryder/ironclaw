package context

import (
	"encoding/json"
	"errors"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// Mock Tokenizer
// =============================================================================

// mockTokenizer implements domain.Tokenizer using a simple word-count heuristic.
// Each "word" costs 1 token. This makes tests deterministic without real tiktoken.
type mockTokenizer struct {
	countFn func(text string) (int, error)
}

func newWordCountTokenizer() *mockTokenizer {
	return &mockTokenizer{
		countFn: func(text string) (int, error) {
			if text == "" {
				return 0, nil
			}
			count := 1
			for _, c := range text {
				if c == ' ' || c == '\n' {
					count++
				}
			}
			return count, nil
		},
	}
}

func (m *mockTokenizer) CountTokens(text string) (int, error) {
	return m.countFn(text)
}

// helper to create a text message with role and string content.
func textMsg(role domain.MessageRole, text string) domain.Message {
	raw, _ := json.Marshal(text)
	msg := domain.Message{
		Role:       role,
		RawContent: raw,
		ContentBlocks: []domain.ContentBlock{
			domain.TextBlock{Text: text},
		},
	}
	return msg
}

// =============================================================================
// Manager Constructor Tests
// =============================================================================

func TestNewManager_WhenValidArgs_ShouldReturnManager(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 100)
	if mgr == nil {
		t.Fatal("expected non-nil Manager")
	}
}

func TestNewManager_WhenTokenizerIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when tokenizer is nil")
		}
	}()
	NewManager(nil, 100)
}

func TestNewManager_WhenMaxTokensIsZero_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when maxTokens is 0")
		}
	}()
	NewManager(newWordCountTokenizer(), 0)
}

func TestNewManager_WhenMaxTokensIsNegative_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when maxTokens is negative")
		}
	}()
	NewManager(newWordCountTokenizer(), -5)
}

// =============================================================================
// FitToWindow Tests - Basic Behavior
// =============================================================================

func TestFitToWindow_WhenMessagesUnderLimit_ShouldReturnAllMessages(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 100) // 100 token limit

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "hello"),     // ~1 token
		textMsg(domain.RoleAssistant, "world"), // ~1 token
	}

	got, err := mgr.FitToWindow(msgs, "system prompt") // ~2 tokens for system
	if err != nil {
		t.Fatalf("FitToWindow: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 messages, got %d", len(got))
	}
}

func TestFitToWindow_WhenMessagesOverLimit_ShouldDropOldestMessages(t *testing.T) {
	tok := newWordCountTokenizer()
	// Max 10 tokens. System prompt "be helpful" = 2 tokens.
	// That leaves 8 tokens for messages.
	mgr := NewManager(tok, 10)

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "one two three four five"),       // 5 tokens
		textMsg(domain.RoleAssistant, "six seven eight nine ten"), // 5 tokens
		textMsg(domain.RoleUser, "just two"),                      // 2 tokens
	}

	got, err := mgr.FitToWindow(msgs, "be helpful")
	if err != nil {
		t.Fatalf("FitToWindow: %v", err)
	}

	// Should keep only the most recent messages that fit.
	// System = 2, "just two" = 2, "six seven eight nine ten" = 5 => total 9 <= 10
	// Adding "one two three four five" = 5 => 14 > 10, so drop it.
	if len(got) != 2 {
		t.Errorf("expected 2 messages (dropped oldest), got %d", len(got))
	}
	// The first returned message should be the second original message
	if len(got) > 0 {
		text := MessageText(got[0])
		if text != "six seven eight nine ten" {
			t.Errorf("first kept message should be 'six seven eight nine ten', got %q", text)
		}
	}
}

func TestFitToWindow_WhenEmptyMessages_ShouldReturnEmptySlice(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 100)

	got, err := mgr.FitToWindow(nil, "system")
	if err != nil {
		t.Fatalf("FitToWindow: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 messages, got %d", len(got))
	}
}

func TestFitToWindow_WhenEmptySystemPrompt_ShouldUseFullBudgetForMessages(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 5)

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "one two three"), // 3 tokens
		textMsg(domain.RoleAssistant, "four five"), // 2 tokens
	}

	got, err := mgr.FitToWindow(msgs, "")
	if err != nil {
		t.Fatalf("FitToWindow: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 messages (all fit with 0-token system prompt), got %d", len(got))
	}
}

func TestFitToWindow_WhenSystemPromptExceedsLimit_ShouldReturnError(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 3) // only 3 tokens total

	// System prompt alone is 5 tokens
	_, err := mgr.FitToWindow(
		[]domain.Message{textMsg(domain.RoleUser, "hi")},
		"one two three four five",
	)
	if err == nil {
		t.Error("expected error when system prompt exceeds limit")
	}
}

func TestFitToWindow_WhenSingleMessageExceedsRemaining_ShouldDropIt(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 5) // 5 token limit

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "this message has way too many words to fit"), // 9 tokens
	}

	got, err := mgr.FitToWindow(msgs, "sys") // sys = 1 token, 4 remaining
	if err != nil {
		t.Fatalf("FitToWindow: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 messages (single message too large), got %d", len(got))
	}
}

func TestFitToWindow_WhenAllMessagesDropped_ShouldReturnEmptySlice(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 3)

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "too many words here"),
		textMsg(domain.RoleAssistant, "also too many words"),
	}

	got, err := mgr.FitToWindow(msgs, "a") // a = 1 token, 2 remaining
	if err != nil {
		t.Fatalf("FitToWindow: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 messages, got %d", len(got))
	}
}

// =============================================================================
// FitToWindow Tests - Sliding Window (keeps most recent)
// =============================================================================

func TestFitToWindow_ShouldPreserveMostRecentMessages(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 15)

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "old message one"),         // 3 tokens
		textMsg(domain.RoleAssistant, "old response two"),   // 3 tokens
		textMsg(domain.RoleUser, "recent question three"),   // 3 tokens
		textMsg(domain.RoleAssistant, "recent answer four"), // 3 tokens
	}

	// System "sys" = 1 token. Budget = 14 tokens.
	// All messages = 12 tokens. All fit.
	got, err := mgr.FitToWindow(msgs, "sys")
	if err != nil {
		t.Fatalf("FitToWindow: %v", err)
	}
	if len(got) != 4 {
		t.Errorf("expected 4 messages (all fit), got %d", len(got))
	}
}

func TestFitToWindow_WhenTight_ShouldKeepOnlyLatest(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 8)

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "alpha beta gamma"),          // 3 tokens
		textMsg(domain.RoleAssistant, "delta epsilon zeta"),   // 3 tokens
		textMsg(domain.RoleUser, "eta theta"),                 // 2 tokens
	}

	// System "x" = 1 token. Budget = 7 tokens.
	// From end: "eta theta"=2 (total=2), "delta epsilon zeta"=3 (total=5),
	//           "alpha beta gamma"=3 (total=8>7) => drop first
	got, err := mgr.FitToWindow(msgs, "x")
	if err != nil {
		t.Fatalf("FitToWindow: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 messages, got %d", len(got))
	}
	if len(got) == 2 {
		if MessageText(got[0]) != "delta epsilon zeta" {
			t.Errorf("expected first kept message 'delta epsilon zeta', got %q", MessageText(got[0]))
		}
		if MessageText(got[1]) != "eta theta" {
			t.Errorf("expected second kept message 'eta theta', got %q", MessageText(got[1]))
		}
	}
}

func TestFitToWindow_ShouldPreserveMessageOrder(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 50)

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "first"),
		textMsg(domain.RoleAssistant, "second"),
		textMsg(domain.RoleUser, "third"),
	}

	got, err := mgr.FitToWindow(msgs, "")
	if err != nil {
		t.Fatalf("FitToWindow: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}
	for i, want := range []string{"first", "second", "third"} {
		if MessageText(got[i]) != want {
			t.Errorf("message[%d]: expected %q, got %q", i, want, MessageText(got[i]))
		}
	}
}

// =============================================================================
// FitToWindow Tests - Error Handling
// =============================================================================

func TestFitToWindow_WhenTokenizerFails_ShouldReturnError(t *testing.T) {
	tok := &mockTokenizer{
		countFn: func(text string) (int, error) {
			return 0, errors.New("tokenizer exploded")
		},
	}
	mgr := NewManager(tok, 100)

	_, err := mgr.FitToWindow(
		[]domain.Message{textMsg(domain.RoleUser, "hi")},
		"system",
	)
	if err == nil {
		t.Error("expected error when tokenizer fails")
	}
}

func TestFitToWindow_WhenTokenizerFailsOnMessage_ShouldReturnError(t *testing.T) {
	callCount := 0
	tok := &mockTokenizer{
		countFn: func(text string) (int, error) {
			callCount++
			if callCount > 1 {
				return 0, errors.New("tokenizer failed on message")
			}
			return 1, nil // first call succeeds (system prompt)
		},
	}
	mgr := NewManager(tok, 100)

	_, err := mgr.FitToWindow(
		[]domain.Message{textMsg(domain.RoleUser, "hi")},
		"system",
	)
	if err == nil {
		t.Error("expected error when tokenizer fails on message")
	}
}

// =============================================================================
// Interface compliance
// =============================================================================

func TestManager_ShouldImplementContextManagerInterface(t *testing.T) {
	tok := newWordCountTokenizer()
	mgr := NewManager(tok, 100)
	var _ domain.ContextManager = mgr
}
