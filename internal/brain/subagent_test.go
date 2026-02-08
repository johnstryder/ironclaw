package brain

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// =============================================================================
// SubAgent — Construction
// =============================================================================

func TestNewSubAgent_ShouldReturnSubAgentWithProviderAndSystemPrompt(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	sa := NewSubAgent(provider, "You are a Python Expert")

	if sa == nil {
		t.Fatal("expected non-nil SubAgent")
	}
}

func TestNewSubAgent_WhenProviderIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSubAgent(nil, ...) should panic")
		}
	}()
	NewSubAgent(nil, "any role")
}

func TestNewSubAgent_ShouldStoreSystemPrompt(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	sa := NewSubAgent(provider, "You are a Python Expert")

	if sa.SystemPrompt() != "You are a Python Expert" {
		t.Errorf("expected system prompt %q, got %q", "You are a Python Expert", sa.SystemPrompt())
	}
}

func TestNewSubAgent_WhenEmptySystemPrompt_ShouldAllowIt(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	sa := NewSubAgent(provider, "")

	if sa.SystemPrompt() != "" {
		t.Errorf("expected empty system prompt, got %q", sa.SystemPrompt())
	}
}

// =============================================================================
// SubAgent — Run
// =============================================================================

func TestSubAgent_Run_ShouldReturnLLMResponse(t *testing.T) {
	provider := &mockProvider{response: "Python is dynamically typed"}
	sa := NewSubAgent(provider, "You are a Python Expert")

	result, err := sa.Run(context.Background(), "Explain Python typing")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "Python is dynamically typed" {
		t.Errorf("expected %q, got %q", "Python is dynamically typed", result)
	}
}

func TestSubAgent_Run_ShouldIncludeSystemPromptInLLMCall(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	sa := NewSubAgent(provider, "You are a Python Expert")

	_, err := sa.Run(context.Background(), "Explain decorators")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(provider.prompt, "You are a Python Expert") {
		t.Errorf("expected prompt to contain system prompt, got %q", provider.prompt)
	}
}

func TestSubAgent_Run_ShouldIncludeTaskInLLMCall(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	sa := NewSubAgent(provider, "You are a Python Expert")

	_, err := sa.Run(context.Background(), "Explain decorators")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(provider.prompt, "Explain decorators") {
		t.Errorf("expected prompt to contain task, got %q", provider.prompt)
	}
}

func TestSubAgent_Run_WhenProviderReturnsError_ShouldReturnError(t *testing.T) {
	wantErr := errors.New("LLM is down")
	provider := &mockProvider{err: wantErr}
	sa := NewSubAgent(provider, "You are an Expert")

	_, err := sa.Run(context.Background(), "do something")
	if err == nil {
		t.Fatal("expected error when provider fails")
	}
	if !strings.Contains(err.Error(), "LLM is down") {
		t.Errorf("expected error to contain 'LLM is down', got %q", err.Error())
	}
}

func TestSubAgent_Run_WhenProviderReturnsError_ShouldReturnEmptyString(t *testing.T) {
	provider := &mockProvider{err: errors.New("fail")}
	sa := NewSubAgent(provider, "role")

	result, _ := sa.Run(context.Background(), "task")
	if result != "" {
		t.Errorf("expected empty string on error, got %q", result)
	}
}

func TestSubAgent_Run_WhenContextCanceled_ShouldReturnContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := &contextAwareMock{response: "should not return"}
	sa := NewSubAgent(provider, "role")

	_, err := sa.Run(ctx, "task")
	if err == nil {
		t.Fatal("expected error when context is canceled")
	}
}

func TestSubAgent_Run_ShouldFormatPromptWithSystemAndTask(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	sa := NewSubAgent(provider, "You are a Summarizer")

	_, _ = sa.Run(context.Background(), "Summarize this text")

	// System prompt should appear before the task in the prompt
	sysIdx := strings.Index(provider.prompt, "You are a Summarizer")
	taskIdx := strings.Index(provider.prompt, "Summarize this text")
	if sysIdx == -1 || taskIdx == -1 {
		t.Fatalf("expected both system prompt and task in prompt, got %q", provider.prompt)
	}
	if sysIdx >= taskIdx {
		t.Errorf("expected system prompt to appear before task in prompt")
	}
}

func TestSubAgent_Run_ShouldRunInIsolation_NoMemoryInjection(t *testing.T) {
	// SubAgent should NOT use memory injection — it only uses the system prompt + task.
	// We verify by checking the prompt doesn't contain memory headers.
	provider := &mockProvider{response: "ok"}
	sa := NewSubAgent(provider, "role")

	_, _ = sa.Run(context.Background(), "task")

	if strings.Contains(provider.prompt, "[Long-term Memory]") {
		t.Error("SubAgent should not inject memory — it runs in isolation")
	}
}

func TestSubAgent_Run_WhenEmptyTask_ShouldStillCallProvider(t *testing.T) {
	provider := &mockProvider{response: "no task given"}
	sa := NewSubAgent(provider, "role")

	result, err := sa.Run(context.Background(), "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "no task given" {
		t.Errorf("expected %q, got %q", "no task given", result)
	}
}

// =============================================================================
// SubAgentRunner — Construction
// =============================================================================

func TestNewSubAgentRunner_ShouldReturnNonNilRunner(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	runner := NewSubAgentRunner(provider)
	if runner == nil {
		t.Fatal("expected non-nil SubAgentRunner")
	}
}

func TestNewSubAgentRunner_WhenProviderIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSubAgentRunner(nil) should panic")
		}
	}()
	NewSubAgentRunner(nil)
}

// =============================================================================
// SubAgentRunner — RunSubAgent
// =============================================================================

func TestSubAgentRunner_RunSubAgent_ShouldReturnLLMResponse(t *testing.T) {
	provider := &mockProvider{response: "summarized text"}
	runner := NewSubAgentRunner(provider)

	result, err := runner.RunSubAgent(context.Background(), "You are a Summarizer", "Summarize this")
	if err != nil {
		t.Fatalf("RunSubAgent: %v", err)
	}
	if result != "summarized text" {
		t.Errorf("expected %q, got %q", "summarized text", result)
	}
}

func TestSubAgentRunner_RunSubAgent_ShouldPassSystemPromptToSubAgent(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	runner := NewSubAgentRunner(provider)

	_, err := runner.RunSubAgent(context.Background(), "You are a Go Expert", "explain goroutines")
	if err != nil {
		t.Fatalf("RunSubAgent: %v", err)
	}
	if !strings.Contains(provider.prompt, "You are a Go Expert") {
		t.Errorf("expected system prompt in provider prompt, got %q", provider.prompt)
	}
}

func TestSubAgentRunner_RunSubAgent_ShouldPassTaskToSubAgent(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	runner := NewSubAgentRunner(provider)

	_, err := runner.RunSubAgent(context.Background(), "Expert", "explain goroutines")
	if err != nil {
		t.Fatalf("RunSubAgent: %v", err)
	}
	if !strings.Contains(provider.prompt, "explain goroutines") {
		t.Errorf("expected task in provider prompt, got %q", provider.prompt)
	}
}

func TestSubAgentRunner_RunSubAgent_WhenProviderFails_ShouldReturnError(t *testing.T) {
	provider := &mockProvider{err: errors.New("api timeout")}
	runner := NewSubAgentRunner(provider)

	_, err := runner.RunSubAgent(context.Background(), "Expert", "task")
	if err == nil {
		t.Fatal("expected error when provider fails")
	}
	if !strings.Contains(err.Error(), "api timeout") {
		t.Errorf("expected error to contain 'api timeout', got %q", err.Error())
	}
}

func TestSubAgentRunner_RunSubAgent_WhenContextCanceled_ShouldReturnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := &contextAwareMock{response: "should not return"}
	runner := NewSubAgentRunner(provider)

	_, err := runner.RunSubAgent(ctx, "Expert", "task")
	if err == nil {
		t.Fatal("expected error when context is canceled")
	}
}

func TestSubAgentRunner_RunSubAgent_ShouldCreateIsolatedSubAgent(t *testing.T) {
	// Each call creates a fresh SubAgent — no state leaks between calls.
	provider := &mockProvider{response: "first"}
	runner := NewSubAgentRunner(provider)

	r1, _ := runner.RunSubAgent(context.Background(), "Role A", "Task 1")
	firstPrompt := provider.prompt

	provider.response = "second"
	r2, _ := runner.RunSubAgent(context.Background(), "Role B", "Task 2")
	secondPrompt := provider.prompt

	if r1 != "first" {
		t.Errorf("first call: expected 'first', got %q", r1)
	}
	if r2 != "second" {
		t.Errorf("second call: expected 'second', got %q", r2)
	}
	if !strings.Contains(firstPrompt, "Role A") {
		t.Errorf("first prompt should contain 'Role A', got %q", firstPrompt)
	}
	if !strings.Contains(secondPrompt, "Role B") {
		t.Errorf("second prompt should contain 'Role B', got %q", secondPrompt)
	}
}

// =============================================================================
// SubAgentRunner — Implements domain.SubAgentRunner interface
// =============================================================================

func TestSubAgentRunner_ShouldImplementDomainSubAgentRunnerInterface(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	var _ interface {
		RunSubAgent(ctx context.Context, systemPrompt string, task string) (string, error)
	} = NewSubAgentRunner(provider)
}
