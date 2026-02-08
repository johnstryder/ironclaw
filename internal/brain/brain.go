package brain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	ironctx "ironclaw/internal/context"
	"ironclaw/internal/domain"
)

// Option is a functional option for configuring Brain.
type Option func(*Brain)

// WithMemory sets the memory store for the Brain. If ms is nil it is ignored.
func WithMemory(ms domain.MemoryStore) Option {
	return func(b *Brain) {
		if ms != nil {
			b.memory = ms
		}
	}
}

// WithContextManager sets the context manager for adaptive context chunking.
// If cm is nil it is ignored (all messages will be sent without truncation).
func WithContextManager(cm domain.ContextManager) Option {
	return func(b *Brain) {
		if cm != nil {
			b.contextMgr = cm
		}
	}
}

// WithLogger sets a structured logger for the Brain. If l is nil it is ignored
// and the default slog logger is used.
func WithLogger(l *slog.Logger) Option {
	return func(b *Brain) {
		if l != nil {
			b.logger = l
		}
	}
}

// WithFallbacks adds fallback LLM providers that are tried in order if the
// primary provider fails. Nil entries are silently skipped.
func WithFallbacks(providers ...domain.LLMProvider) Option {
	return func(b *Brain) {
		for _, p := range providers {
			if p != nil {
				b.fallbacks = append(b.fallbacks, p)
			}
		}
	}
}

// Brain holds an LLM provider and exposes Generate to application logic.
// Callers are unaware of the underlying implementation (OpenAI, Anthropic, local).
type Brain struct {
	provider   domain.LLMProvider
	fallbacks  []domain.LLMProvider   // optional; tried in order when provider fails
	memory     domain.MemoryStore     // optional; nil means no persistent memory
	contextMgr domain.ContextManager  // optional; nil means no context window management
	logger     *slog.Logger           // optional; nil uses slog.Default()
}

// NewBrain returns a Brain that uses the given provider. Provider must not be nil.
// Options (e.g. WithMemory, WithContextManager) may be passed to configure optional features.
func NewBrain(provider domain.LLMProvider, opts ...Option) *Brain {
	if provider == nil {
		panic("brain: provider must not be nil")
	}
	b := &Brain{provider: provider}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Generate calls the underlying LLM provider with the given prompt and returns the response.
// If a MemoryStore is configured, its content is prepended to the prompt as context.
// When fallbacks are configured, they are tried in order if the primary provider fails.
func (b *Brain) Generate(ctx context.Context, prompt string) (string, error) {
	enriched := b.enrichPrompt(prompt)
	return b.generateWithFailover(ctx, enriched)
}

// log returns the Brain's logger, falling back to the default slog logger.
func (b *Brain) log() *slog.Logger {
	if b.logger != nil {
		return b.logger
	}
	return slog.Default()
}

// generateWithFailover tries the primary provider, then each fallback in order.
// Returns the first successful response, or an aggregated error if all fail.
func (b *Brain) generateWithFailover(ctx context.Context, prompt string) (string, error) {
	result, err := b.provider.Generate(ctx, prompt)
	if err == nil {
		return result, nil
	}

	// No fallbacks configured — return primary error directly.
	if len(b.fallbacks) == 0 {
		return "", err
	}

	// Collect all errors for aggregated reporting.
	errs := []error{err}

	for i, fb := range b.fallbacks {
		// Stop iterating if the context has been canceled.
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		b.log().Warn("provider failed, trying fallback",
			"provider_index", i,
			"error", err,
		)

		result, fbErr := fb.Generate(ctx, prompt)
		if fbErr == nil {
			return result, nil
		}
		errs = append(errs, fbErr)
		err = fbErr
	}

	return "", fmt.Errorf("brain: all %d providers failed: %w", len(errs), errors.Join(errs...))
}

// GenerateWithContext takes a message history and system prompt, applies adaptive
// context chunking (if a ContextManager is configured), then sends the result to
// the LLM provider. Memory is injected into the system prompt before chunking.
func (b *Brain) GenerateWithContext(ctx context.Context, messages []domain.Message, systemPrompt string) (string, error) {
	// Enrich the system prompt with long-term memory.
	enrichedSystem := b.enrichPrompt(systemPrompt)

	// Apply context window management if configured.
	fittedMessages := messages
	if b.contextMgr != nil && len(messages) > 0 {
		fitted, err := b.contextMgr.FitToWindow(messages, enrichedSystem)
		if err != nil {
			return "", fmt.Errorf("brain: context fitting failed: %w", err)
		}
		fittedMessages = fitted
	}

	// Build the final prompt from system prompt + fitted messages.
	prompt := buildPrompt(enrichedSystem, fittedMessages)
	return b.generateWithFailover(ctx, prompt)
}

// enrichPrompt prepends long-term memory to the prompt when available.
// On load error the memory is silently skipped (best-effort).
func (b *Brain) enrichPrompt(prompt string) string {
	if b.memory == nil {
		return prompt
	}
	content, err := b.memory.LoadMemory()
	if err != nil || content == "" {
		return prompt
	}
	return fmt.Sprintf("[Long-term Memory]\n%s\n[End Memory]\n\n%s", content, prompt)
}

// buildPrompt assembles the final prompt from system prompt and message history.
func buildPrompt(systemPrompt string, messages []domain.Message) string {
	var sb strings.Builder

	if systemPrompt != "" {
		sb.WriteString("[System]\n")
		sb.WriteString(systemPrompt)
		sb.WriteString("\n[End System]\n\n")
	}

	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("[%s]\n", msg.Role))
		sb.WriteString(ironctx.MessageText(msg))
		sb.WriteString("\n\n")
	}

	return sb.String()
}
