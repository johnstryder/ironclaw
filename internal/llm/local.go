package llm

import (
	"context"
	"fmt"

	"ironclaw/internal/domain"
)

// LocalProvider is a model-agnostic stub that returns a deterministic response
// for manual testing without API keys. It implements domain.LLMProvider.
type LocalProvider struct {
	Prefix string // prepended to the prompt in the response
}

// NewLocalProvider returns a local provider that echoes the prompt with an optional prefix.
func NewLocalProvider(prefix string) *LocalProvider {
	return &LocalProvider{Prefix: prefix}
}

// Generate implements domain.LLMProvider.
func (p *LocalProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if p.Prefix == "" {
		return prompt, nil
	}
	return fmt.Sprintf("%s%s", p.Prefix, prompt), nil
}

// Ensure LocalProvider implements domain.LLMProvider at compile time.
var _ domain.LLMProvider = (*LocalProvider)(nil)
