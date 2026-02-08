package context

import (
	"fmt"

	"ironclaw/internal/domain"
)

// Manager implements domain.ContextManager using a sliding-window strategy.
// It counts tokens for each message and drops the oldest messages first
// when the total exceeds the configured maximum token budget.
type Manager struct {
	tokenizer domain.Tokenizer
	maxTokens int
}

// NewManager creates a Manager with the given tokenizer and max token limit.
// Panics if tokenizer is nil or maxTokens <= 0.
func NewManager(tokenizer domain.Tokenizer, maxTokens int) *Manager {
	if tokenizer == nil {
		panic("context: tokenizer must not be nil")
	}
	if maxTokens <= 0 {
		panic("context: maxTokens must be > 0")
	}
	return &Manager{
		tokenizer: tokenizer,
		maxTokens: maxTokens,
	}
}

// FitToWindow applies a sliding-window strategy: it reserves tokens for the
// system prompt, then walks messages from newest to oldest, keeping as many
// recent messages as fit within the remaining budget.
//
// Returns an error if the system prompt alone exceeds maxTokens or if the
// tokenizer returns an error.
func (m *Manager) FitToWindow(messages []domain.Message, systemPrompt string) ([]domain.Message, error) {
	if len(messages) == 0 {
		return []domain.Message{}, nil
	}

	// Reserve tokens for the system prompt.
	sysTokens, err := m.countPromptTokens(systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("context: counting system prompt tokens: %w", err)
	}
	if sysTokens > m.maxTokens {
		return nil, fmt.Errorf("context: system prompt (%d tokens) exceeds limit (%d tokens)", sysTokens, m.maxTokens)
	}

	budget := m.maxTokens - sysTokens

	// Count tokens for each message.
	tokenCounts := make([]int, len(messages))
	for i, msg := range messages {
		text := MessageText(msg)
		count, err := m.tokenizer.CountTokens(text)
		if err != nil {
			return nil, fmt.Errorf("context: counting tokens for message %d: %w", i, err)
		}
		tokenCounts[i] = count
	}

	// Walk from the end (most recent) backwards, accumulating messages that fit.
	total := 0
	startIdx := len(messages) // will be decremented
	for i := len(messages) - 1; i >= 0; i-- {
		if total+tokenCounts[i] > budget {
			break
		}
		total += tokenCounts[i]
		startIdx = i
	}

	// Return the kept slice (preserves original order).
	return messages[startIdx:], nil
}

// countPromptTokens counts tokens for a system prompt. Empty prompt = 0 tokens.
func (m *Manager) countPromptTokens(prompt string) (int, error) {
	if prompt == "" {
		return 0, nil
	}
	return m.tokenizer.CountTokens(prompt)
}

// Ensure Manager implements domain.ContextManager.
var _ domain.ContextManager = (*Manager)(nil)
