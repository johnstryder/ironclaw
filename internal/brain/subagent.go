package brain

import (
	"context"
	"fmt"

	"ironclaw/internal/domain"
)

// SubAgent runs a secondary LLM generation in isolation with a specialized
// system prompt (role). It inherits the parent's LLMProvider but does NOT
// share memory, history, or context management — guaranteeing isolation.
type SubAgent struct {
	provider     domain.LLMProvider
	systemPrompt string
}

// NewSubAgent creates a SubAgent with the given provider and system prompt.
// The provider must not be nil; the system prompt may be empty.
func NewSubAgent(provider domain.LLMProvider, systemPrompt string) *SubAgent {
	if provider == nil {
		panic("subagent: provider must not be nil")
	}
	return &SubAgent{
		provider:     provider,
		systemPrompt: systemPrompt,
	}
}

// SystemPrompt returns the specialized system prompt this sub-agent uses.
func (sa *SubAgent) SystemPrompt() string {
	return sa.systemPrompt
}

// Run executes a task using the sub-agent's system prompt and returns the LLM
// response. The prompt is formatted as [System] + systemPrompt + [Task] + task.
// No memory injection or context management is applied — the sub-agent runs in
// full isolation.
func (sa *SubAgent) Run(ctx context.Context, task string) (string, error) {
	prompt := sa.buildPrompt(task)
	result, err := sa.provider.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("subagent: %w", err)
	}
	return result, nil
}

// buildPrompt assembles the prompt from the system prompt and task.
func (sa *SubAgent) buildPrompt(task string) string {
	return fmt.Sprintf("[System]\n%s\n[End System]\n\n[Task]\n%s\n[End Task]", sa.systemPrompt, task)
}

// SubAgentRunner implements domain.SubAgentRunner by creating a fresh SubAgent
// for each invocation. It shares the LLMProvider but nothing else.
type SubAgentRunner struct {
	provider domain.LLMProvider
}

// NewSubAgentRunner creates a runner that spawns isolated sub-agents using the
// given LLM provider. Panics if provider is nil.
func NewSubAgentRunner(provider domain.LLMProvider) *SubAgentRunner {
	if provider == nil {
		panic("subagent_runner: provider must not be nil")
	}
	return &SubAgentRunner{provider: provider}
}

// RunSubAgent creates a new SubAgent with the given system prompt and executes
// the task. Each invocation is fully isolated.
func (r *SubAgentRunner) RunSubAgent(ctx context.Context, systemPrompt string, task string) (string, error) {
	sa := NewSubAgent(r.provider, systemPrompt)
	return sa.Run(ctx, task)
}
