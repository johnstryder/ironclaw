package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

// mockProvider implements domain.LLMProvider for tests.
type mockProvider struct {
	response string
	err      error
	prompt   string // last prompt passed to Generate
}

func (m *mockProvider) Generate(ctx context.Context, prompt string) (string, error) {
	m.prompt = prompt
	return m.response, m.err
}

// mockMemoryStore implements domain.MemoryStore for tests.
type mockMemoryStore struct {
	memory      string
	loadErr     error
	remembered  []string
	rememberErr error
}

func (m *mockMemoryStore) Append(date string, content string) error { return nil }
func (m *mockMemoryStore) Remember(content string) error {
	if m.rememberErr != nil {
		return m.rememberErr
	}
	m.remembered = append(m.remembered, content)
	return nil
}
func (m *mockMemoryStore) LoadMemory() (string, error) {
	return m.memory, m.loadErr
}

func TestBrain_Generate_WhenProviderReturnsResponse_ShouldReturnResponse(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "Hello from mock"}
	brain := NewBrain(provider)

	got, err := brain.Generate(ctx, "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "Hello from mock" {
		t.Errorf("want response %q, got %q", "Hello from mock", got)
	}
	if provider.prompt != "hi" {
		t.Errorf("want prompt %q, got %q", "hi", provider.prompt)
	}
}

func TestBrain_Generate_WhenProviderReturnsError_ShouldReturnError(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("provider failed")
	provider := &mockProvider{err: wantErr}
	brain := NewBrain(provider)

	got, err := brain.Generate(ctx, "anything")
	if err != wantErr {
		t.Errorf("want err %v, got %v", wantErr, err)
	}
	if got != "" {
		t.Errorf("want empty string on error, got %q", got)
	}
}

// contextAwareMock returns context error when ctx is done (e.g. canceled).
type contextAwareMock struct {
	response string
}

func (m *contextAwareMock) Generate(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return m.response, nil
}

func TestBrain_Generate_WhenContextCanceled_ShouldReturnContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := &contextAwareMock{response: "never used"}
	brain := NewBrain(provider)

	_, err := brain.Generate(ctx, "hi")
	if err == nil {
		t.Error("expected error when context is canceled")
	}
	if ctx.Err() == nil {
		t.Error("expected context to be canceled")
	}
}

func TestNewBrain_WhenProviderIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewBrain(nil) should panic")
		}
	}()
	NewBrain(nil)
}

func TestBrain_Generate_DelegatesToProvider_WithCorrectPrompt(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "ok"}
	brain := NewBrain(provider)
	prompt := "What is 2+2?"

	_, _ = brain.Generate(ctx, prompt)
	if provider.prompt != prompt {
		t.Errorf("provider should receive prompt %q, got %q", prompt, provider.prompt)
	}
}

// =============================================================================
// Memory injection tests
// =============================================================================

func TestBrain_Generate_WhenMemoryExists_ShouldPrependMemoryToPrompt(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "Blue!"}
	mem := &mockMemoryStore{memory: "- Favorite color is blue.\n"}
	brain := NewBrain(provider, WithMemory(mem))

	_, err := brain.Generate(ctx, "What is my favorite color?")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(provider.prompt, "Favorite color is blue.") {
		t.Errorf("expected prompt to contain memory, got %q", provider.prompt)
	}
	if !strings.Contains(provider.prompt, "What is my favorite color?") {
		t.Errorf("expected prompt to contain original question, got %q", provider.prompt)
	}
}

func TestBrain_Generate_WhenMemoryIsEmpty_ShouldPassPromptUnchanged(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "ok"}
	mem := &mockMemoryStore{memory: ""}
	brain := NewBrain(provider, WithMemory(mem))

	_, err := brain.Generate(ctx, "hello")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if provider.prompt != "hello" {
		t.Errorf("expected prompt %q unchanged, got %q", "hello", provider.prompt)
	}
}

func TestBrain_Generate_WhenNoMemoryStore_ShouldPassPromptUnchanged(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "ok"}
	brain := NewBrain(provider)

	_, err := brain.Generate(ctx, "hello")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if provider.prompt != "hello" {
		t.Errorf("expected prompt %q unchanged, got %q", "hello", provider.prompt)
	}
}

func TestBrain_Generate_WhenMemoryLoadFails_ShouldStillGenerateWithOriginalPrompt(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "ok"}
	mem := &mockMemoryStore{loadErr: errors.New("disk error")}
	brain := NewBrain(provider, WithMemory(mem))

	got, err := brain.Generate(ctx, "hello")
	if err != nil {
		t.Fatalf("Generate should not fail: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected response 'ok', got %q", got)
	}
	if provider.prompt != "hello" {
		t.Errorf("expected original prompt on memory load failure, got %q", provider.prompt)
	}
}

func TestBrain_Generate_WhenMemoryExists_ShouldContainMemoryHeader(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "ok"}
	mem := &mockMemoryStore{memory: "- Some fact\n"}
	brain := NewBrain(provider, WithMemory(mem))

	_, _ = brain.Generate(ctx, "question")
	if !strings.Contains(provider.prompt, "[Long-term Memory]") {
		t.Errorf("expected prompt to contain memory header, got %q", provider.prompt)
	}
}

func TestNewBrain_WithMemory_ShouldAcceptNilMemoryStore(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	// WithMemory(nil) should not panic
	brain := NewBrain(provider, WithMemory(nil))
	got, err := brain.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected 'ok', got %q", got)
	}
	if provider.prompt != "hi" {
		t.Errorf("expected 'hi', got %q", provider.prompt)
	}
}

// =============================================================================
// Mock ContextManager
// =============================================================================

type mockContextManager struct {
	fitResult []domain.Message
	fitErr    error
	fitCalled bool
	gotMsgs   []domain.Message
	gotPrompt string
}

func (m *mockContextManager) FitToWindow(msgs []domain.Message, systemPrompt string) ([]domain.Message, error) {
	m.fitCalled = true
	m.gotMsgs = msgs
	m.gotPrompt = systemPrompt
	return m.fitResult, m.fitErr
}

// textMsg creates a Message with TextBlock for testing.
func textMsg(role domain.MessageRole, text string) domain.Message {
	raw, _ := json.Marshal(text)
	return domain.Message{
		Role:       role,
		RawContent: raw,
		ContentBlocks: []domain.ContentBlock{
			domain.TextBlock{Text: text},
		},
	}
}

// =============================================================================
// GenerateWithContext Tests
// =============================================================================

func TestBrain_GenerateWithContext_WhenContextManagerFits_ShouldSendTrimmedMessages(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "I see 2 messages"}
	cm := &mockContextManager{
		fitResult: []domain.Message{
			textMsg(domain.RoleUser, "recent question"),
			textMsg(domain.RoleAssistant, "recent answer"),
		},
	}
	brain := NewBrain(provider, WithContextManager(cm))

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "old question"),
		textMsg(domain.RoleAssistant, "old answer"),
		textMsg(domain.RoleUser, "recent question"),
		textMsg(domain.RoleAssistant, "recent answer"),
	}

	got, err := brain.GenerateWithContext(ctx, msgs, "system prompt")
	if err != nil {
		t.Fatalf("GenerateWithContext: %v", err)
	}
	if got != "I see 2 messages" {
		t.Errorf("expected 'I see 2 messages', got %q", got)
	}
	if !cm.fitCalled {
		t.Error("expected ContextManager.FitToWindow to be called")
	}
	if cm.gotPrompt != "system prompt" {
		t.Errorf("expected system prompt 'system prompt', got %q", cm.gotPrompt)
	}
}

func TestBrain_GenerateWithContext_WhenNoContextManager_ShouldSendAllMessages(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "ok"}
	brain := NewBrain(provider) // no context manager

	msgs := []domain.Message{
		textMsg(domain.RoleUser, "hello"),
		textMsg(domain.RoleAssistant, "hi"),
	}

	got, err := brain.GenerateWithContext(ctx, msgs, "system")
	if err != nil {
		t.Fatalf("GenerateWithContext: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected 'ok', got %q", got)
	}
	// Should contain both messages in the prompt
	if !strings.Contains(provider.prompt, "hello") {
		t.Errorf("expected prompt to contain 'hello', got %q", provider.prompt)
	}
	if !strings.Contains(provider.prompt, "hi") {
		t.Errorf("expected prompt to contain 'hi', got %q", provider.prompt)
	}
}

func TestBrain_GenerateWithContext_WhenContextManagerReturnsError_ShouldReturnError(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "unused"}
	cm := &mockContextManager{
		fitErr: errors.New("context overflow"),
	}
	brain := NewBrain(provider, WithContextManager(cm))

	_, err := brain.GenerateWithContext(ctx, []domain.Message{textMsg(domain.RoleUser, "x")}, "sys")
	if err == nil {
		t.Error("expected error when ContextManager fails")
	}
	if !strings.Contains(err.Error(), "context overflow") {
		t.Errorf("expected error to contain 'context overflow', got %q", err.Error())
	}
}

func TestBrain_GenerateWithContext_ShouldIncludeSystemPromptInFinalPrompt(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "ok"}
	cm := &mockContextManager{
		fitResult: []domain.Message{
			textMsg(domain.RoleUser, "question"),
		},
	}
	brain := NewBrain(provider, WithContextManager(cm))

	_, err := brain.GenerateWithContext(ctx, []domain.Message{textMsg(domain.RoleUser, "question")}, "You are helpful")
	if err != nil {
		t.Fatalf("GenerateWithContext: %v", err)
	}
	if !strings.Contains(provider.prompt, "You are helpful") {
		t.Errorf("expected system prompt in final prompt, got %q", provider.prompt)
	}
}

func TestBrain_GenerateWithContext_WhenEmptyMessages_ShouldSendSystemPromptOnly(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "ok"}
	brain := NewBrain(provider)

	got, err := brain.GenerateWithContext(ctx, nil, "system only")
	if err != nil {
		t.Fatalf("GenerateWithContext: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected 'ok', got %q", got)
	}
	if !strings.Contains(provider.prompt, "system only") {
		t.Errorf("expected prompt to contain 'system only', got %q", provider.prompt)
	}
}

func TestBrain_GenerateWithContext_WhenMemoryAndContextManager_ShouldUseMemoryInSystemPrompt(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{response: "ok"}
	mem := &mockMemoryStore{memory: "- User likes cats.\n"}
	cm := &mockContextManager{
		fitResult: []domain.Message{
			textMsg(domain.RoleUser, "tell me a story"),
		},
	}
	brain := NewBrain(provider, WithMemory(mem), WithContextManager(cm))

	_, err := brain.GenerateWithContext(ctx, []domain.Message{textMsg(domain.RoleUser, "tell me a story")}, "be creative")
	if err != nil {
		t.Fatalf("GenerateWithContext: %v", err)
	}
	// The system prompt passed to FitToWindow should include memory
	if !strings.Contains(cm.gotPrompt, "User likes cats") {
		t.Errorf("expected memory in system prompt passed to context manager, got %q", cm.gotPrompt)
	}
}

func TestWithContextManager_WhenNil_ShouldBeIgnored(t *testing.T) {
	provider := &mockProvider{response: "ok"}
	brain := NewBrain(provider, WithContextManager(nil))
	got, err := brain.GenerateWithContext(context.Background(), nil, "sys")
	if err != nil {
		t.Fatalf("GenerateWithContext: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected 'ok', got %q", got)
	}
}

// =============================================================================
// Failover Tests
// =============================================================================

// failoverMock tracks whether Generate was called.
type failoverMock struct {
	response string
	err      error
	called   bool
	prompt   string
}

func (m *failoverMock) Generate(ctx context.Context, prompt string) (string, error) {
	m.called = true
	m.prompt = prompt
	return m.response, m.err
}

func TestBrain_Generate_WhenPrimarySucceeds_ShouldNotCallFallback(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{response: "primary response"}
	fallback := &failoverMock{response: "fallback response"}

	brain := NewBrain(primary, WithFallbacks(fallback))
	got, err := brain.Generate(ctx, "hello")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "primary response" {
		t.Errorf("expected 'primary response', got %q", got)
	}
	if !primary.called {
		t.Error("expected primary to be called")
	}
	if fallback.called {
		t.Error("expected fallback NOT to be called when primary succeeds")
	}
}

func TestBrain_Generate_WhenPrimaryFails_ShouldFallbackToSecondary(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{err: errors.New("primary down")}
	fallback := &failoverMock{response: "fallback response"}

	brain := NewBrain(primary, WithFallbacks(fallback))
	got, err := brain.Generate(ctx, "hello")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "fallback response" {
		t.Errorf("expected 'fallback response', got %q", got)
	}
	if !primary.called {
		t.Error("expected primary to be called")
	}
	if !fallback.called {
		t.Error("expected fallback to be called")
	}
}

func TestBrain_Generate_WhenAllProvidersFail_ShouldReturnAggregatedError(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{err: errors.New("primary down")}
	fallback1 := &failoverMock{err: errors.New("fallback1 down")}
	fallback2 := &failoverMock{err: errors.New("fallback2 down")}

	brain := NewBrain(primary, WithFallbacks(fallback1, fallback2))
	_, err := brain.Generate(ctx, "hello")

	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
	// Should mention all providers failed
	if !strings.Contains(err.Error(), "all 3 providers failed") {
		t.Errorf("expected aggregated error message, got %q", err.Error())
	}
	// Should contain each provider error
	if !strings.Contains(err.Error(), "primary down") {
		t.Errorf("expected error to contain 'primary down', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "fallback1 down") {
		t.Errorf("expected error to contain 'fallback1 down', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "fallback2 down") {
		t.Errorf("expected error to contain 'fallback2 down', got %q", err.Error())
	}
	if !primary.called || !fallback1.called || !fallback2.called {
		t.Error("expected all providers to be called")
	}
}

func TestBrain_Generate_WhenFirstFallbackFails_ShouldTrySecondFallback(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{err: errors.New("primary down")}
	fallback1 := &failoverMock{err: errors.New("fallback1 down")}
	fallback2 := &failoverMock{response: "third time's a charm"}

	brain := NewBrain(primary, WithFallbacks(fallback1, fallback2))
	got, err := brain.Generate(ctx, "hello")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "third time's a charm" {
		t.Errorf("expected 'third time's a charm', got %q", got)
	}
	if !primary.called || !fallback1.called || !fallback2.called {
		t.Error("expected all three providers to be called in order")
	}
}

func TestBrain_GenerateWithContext_WhenPrimaryFails_ShouldFallbackToSecondary(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{err: errors.New("primary down")}
	fallback := &failoverMock{response: "fallback context response"}

	brain := NewBrain(primary, WithFallbacks(fallback))
	msgs := []domain.Message{textMsg(domain.RoleUser, "hello")}
	got, err := brain.GenerateWithContext(ctx, msgs, "system prompt")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "fallback context response" {
		t.Errorf("expected 'fallback context response', got %q", got)
	}
	if !primary.called {
		t.Error("expected primary to be called")
	}
	if !fallback.called {
		t.Error("expected fallback to be called after GenerateWithContext primary failure")
	}
}

func TestBrain_Generate_WhenFailoverOccurs_ShouldLogPrimaryError(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{err: errors.New("primary down")}
	fallback := &failoverMock{response: "fallback response"}

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(handler)

	brain := NewBrain(primary, WithFallbacks(fallback), WithLogger(logger))
	_, err := brain.Generate(ctx, "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "primary down") {
		t.Errorf("expected log to contain 'primary down', got %q", logOutput)
	}
	if !strings.Contains(logOutput, "provider failed") {
		t.Errorf("expected log to contain 'provider failed', got %q", logOutput)
	}
}

func TestBrain_Generate_WhenContextCanceled_ShouldStopFailoverAndReturnContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Primary cancels the context, simulating timeout during call
	primaryProvider := &failoverMock{err: context.Canceled}
	cancel() // cancel before we even call

	fallback := &failoverMock{response: "should not reach"}

	brain := NewBrain(primaryProvider, WithFallbacks(fallback))
	_, err := brain.Generate(ctx, "hello")

	if err == nil {
		t.Fatal("expected error when context is canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
	if fallback.called {
		t.Error("expected fallback NOT to be called when context is canceled")
	}
}

func TestBrain_Generate_WhenNoFallbacksConfigured_ShouldReturnPrimaryErrorDirectly(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("only provider down")
	primary := &failoverMock{err: wantErr}

	brain := NewBrain(primary) // no fallbacks
	_, err := brain.Generate(ctx, "hello")

	if err != wantErr {
		t.Errorf("expected direct primary error %v, got %v", wantErr, err)
	}
}

func TestWithFallbacks_WhenNilProviders_ShouldBeIgnored(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{response: "ok"}

	brain := NewBrain(primary, WithFallbacks(nil, nil))
	got, err := brain.Generate(ctx, "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "ok" {
		t.Errorf("expected 'ok', got %q", got)
	}
}

func TestWithLogger_WhenNil_ShouldUseDefaultLogger(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{err: errors.New("fail")}
	fallback := &failoverMock{response: "ok"}

	// WithLogger(nil) should not panic and should use default logger
	brain := NewBrain(primary, WithFallbacks(fallback), WithLogger(nil))
	got, err := brain.Generate(ctx, "hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "ok" {
		t.Errorf("expected 'ok', got %q", got)
	}
}

func TestBrain_Generate_WithFallbacksAndMemory_ShouldEnrichPromptForAllProviders(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{err: errors.New("primary down")}
	fallback := &failoverMock{response: "ok"}
	mem := &mockMemoryStore{memory: "- Favorite color is blue.\n"}

	brain := NewBrain(primary, WithFallbacks(fallback), WithMemory(mem))
	_, err := brain.Generate(ctx, "What is my favorite color?")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Fallback should receive the enriched prompt (with memory)
	if !strings.Contains(fallback.prompt, "Favorite color is blue.") {
		t.Errorf("expected fallback to receive enriched prompt with memory, got %q", fallback.prompt)
	}
	if !strings.Contains(fallback.prompt, "What is my favorite color?") {
		t.Errorf("expected fallback prompt to contain original question, got %q", fallback.prompt)
	}
}

func TestBrain_GenerateWithContext_WhenAllProvidersFail_ShouldReturnAggregatedError(t *testing.T) {
	ctx := context.Background()
	primary := &failoverMock{err: errors.New("primary ctx down")}
	fallback := &failoverMock{err: errors.New("fallback ctx down")}

	brain := NewBrain(primary, WithFallbacks(fallback))
	msgs := []domain.Message{textMsg(domain.RoleUser, "hello")}
	_, err := brain.GenerateWithContext(ctx, msgs, "system")

	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
	if !strings.Contains(err.Error(), "all 2 providers failed") {
		t.Errorf("expected aggregated error, got %q", err.Error())
	}
}

func TestBrain_Generate_WhenPrimaryFailsWithNilFallbacksOption_ShouldReturnError(t *testing.T) {
	// Ensure WithFallbacks with empty args still works
	ctx := context.Background()
	primary := &failoverMock{err: errors.New("down")}

	brain := NewBrain(primary, WithFallbacks()) // empty fallbacks
	_, err := brain.Generate(ctx, "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "down" {
		t.Errorf("expected 'down', got %q", err.Error())
	}
}
