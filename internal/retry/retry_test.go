package retry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"ironclaw/internal/domain"
)

// =============================================================================
// RetryConfig Tests
// =============================================================================

func TestDefaultRetryConfig_ShouldHaveReasonableDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("want MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialBackoff != 500*time.Millisecond {
		t.Errorf("want InitialBackoff=500ms, got %v", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("want MaxBackoff=30s, got %v", cfg.MaxBackoff)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("want Multiplier=2.0, got %v", cfg.Multiplier)
	}
}

func TestRetryConfig_Validate_WhenValid_ShouldReturnNil(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestRetryConfig_Validate_WhenMaxRetriesNegative_ShouldReturnError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetries = -1
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative MaxRetries")
	}
}

func TestRetryConfig_Validate_WhenInitialBackoffZero_ShouldReturnError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialBackoff = 0
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for zero InitialBackoff")
	}
}

func TestRetryConfig_Validate_WhenMaxBackoffZero_ShouldReturnError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBackoff = 0
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for zero MaxBackoff")
	}
}

func TestRetryConfig_Validate_WhenMultiplierLessThanOne_ShouldReturnError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Multiplier = 0.5
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for Multiplier < 1")
	}
}

func TestRetryConfig_Validate_WhenMaxRetriesZero_ShouldReturnNil(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetries = 0
	if err := cfg.Validate(); err != nil {
		t.Errorf("MaxRetries=0 (no retries) should be valid, got: %v", err)
	}
}

// =============================================================================
// IsRetryable Tests
// =============================================================================

func TestIsRetryable_WhenNilError_ShouldReturnFalse(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("nil error should not be retryable")
	}
}

func TestIsRetryable_When500Error_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("anthropic api: 500 Internal Server Error")
	if !IsRetryable(err) {
		t.Error("500 error should be retryable")
	}
}

func TestIsRetryable_When502Error_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("openai api: 502 Bad Gateway")
	if !IsRetryable(err) {
		t.Error("502 error should be retryable")
	}
}

func TestIsRetryable_When503Error_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("anthropic api: 503 Service Unavailable")
	if !IsRetryable(err) {
		t.Error("503 error should be retryable")
	}
}

func TestIsRetryable_When504Error_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("gemini api: 504 Gateway Timeout")
	if !IsRetryable(err) {
		t.Error("504 error should be retryable")
	}
}

func TestIsRetryable_When529Error_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("anthropic api: 529 Overloaded")
	if !IsRetryable(err) {
		t.Error("529 (overloaded) error should be retryable")
	}
}

func TestIsRetryable_When429Error_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("anthropic api: 429 Too Many Requests")
	if !IsRetryable(err) {
		t.Error("429 rate limit error should be retryable")
	}
}

func TestIsRetryable_When400Error_ShouldReturnFalse(t *testing.T) {
	err := fmt.Errorf("anthropic api: 400 Bad Request")
	if IsRetryable(err) {
		t.Error("400 error should NOT be retryable")
	}
}

func TestIsRetryable_When401Error_ShouldReturnFalse(t *testing.T) {
	err := fmt.Errorf("openai api: 401 Unauthorized")
	if IsRetryable(err) {
		t.Error("401 error should NOT be retryable")
	}
}

func TestIsRetryable_When403Error_ShouldReturnFalse(t *testing.T) {
	err := fmt.Errorf("anthropic api: 403 Forbidden")
	if IsRetryable(err) {
		t.Error("403 error should NOT be retryable")
	}
}

func TestIsRetryable_When404Error_ShouldReturnFalse(t *testing.T) {
	err := fmt.Errorf("openai api: 404 Not Found")
	if IsRetryable(err) {
		t.Error("404 error should NOT be retryable")
	}
}

func TestIsRetryable_WhenTimeoutError_ShouldReturnTrue(t *testing.T) {
	err := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &timeoutErr{},
	}
	if !IsRetryable(err) {
		t.Error("timeout error should be retryable")
	}
}

func TestIsRetryable_WhenConnectionRefused_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("anthropic do: dial tcp: connect: connection refused")
	if !IsRetryable(err) {
		t.Error("connection refused error should be retryable")
	}
}

func TestIsRetryable_WhenContextCanceled_ShouldReturnFalse(t *testing.T) {
	if IsRetryable(context.Canceled) {
		t.Error("context.Canceled should NOT be retryable")
	}
}

func TestIsRetryable_WhenContextDeadlineExceeded_ShouldReturnFalse(t *testing.T) {
	if IsRetryable(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded should NOT be retryable")
	}
}

func TestIsRetryable_WhenWrappedRetryableError_ShouldReturnTrue(t *testing.T) {
	inner := fmt.Errorf("anthropic api: 503 Service Unavailable")
	wrapped := fmt.Errorf("brain generate: %w", inner)
	if !IsRetryable(wrapped) {
		t.Error("wrapped 503 error should be retryable")
	}
}

func TestIsRetryable_WhenGenericError_ShouldReturnFalse(t *testing.T) {
	err := errors.New("something went wrong")
	if IsRetryable(err) {
		t.Error("generic error should NOT be retryable")
	}
}

func TestIsRetryable_WhenEOFError_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("anthropic do: %w", fmt.Errorf("unexpected EOF"))
	if !IsRetryable(err) {
		t.Error("EOF error should be retryable (connection reset)")
	}
}

// =============================================================================
// RetryableProvider Tests
// =============================================================================

// mockLLM implements domain.LLMProvider for tests.
type mockLLM struct {
	calls     int32
	responses []string
	errs      []error
}

func (m *mockLLM) Generate(ctx context.Context, prompt string) (string, error) {
	idx := int(atomic.AddInt32(&m.calls, 1)) - 1
	if idx < len(m.errs) && m.errs[idx] != nil {
		return "", m.errs[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return "default", nil
}

// timeoutErr is a test helper that implements net.Error with Timeout() = true.
type timeoutErr struct{}

func (t *timeoutErr) Error() string   { return "i/o timeout" }
func (t *timeoutErr) Timeout() bool   { return true }
func (t *timeoutErr) Temporary() bool { return true }

// noopSleep replaces time.Sleep in tests to avoid real delays.
func noopSleep(d time.Duration) {}

func TestNewRetryableProvider_ShouldReturnProvider(t *testing.T) {
	inner := &mockLLM{responses: []string{"ok"}}
	cfg := DefaultConfig()
	p := NewRetryableProvider(inner, cfg)
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewRetryableProvider_WhenInnerIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil inner provider")
		}
	}()
	NewRetryableProvider(nil, DefaultConfig())
}

func TestRetryableProvider_Generate_WhenNoError_ShouldReturnResponseWithoutRetry(t *testing.T) {
	inner := &mockLLM{responses: []string{"hello"}}
	cfg := DefaultConfig()
	p := NewRetryableProvider(inner, cfg)
	p.sleepFunc = noopSleep

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("want 'hello', got %q", result)
	}
	if atomic.LoadInt32(&inner.calls) != 1 {
		t.Errorf("expected 1 call, got %d", atomic.LoadInt32(&inner.calls))
	}
}

func TestRetryableProvider_Generate_WhenRetryableErrorThenSuccess_ShouldRetryAndSucceed(t *testing.T) {
	inner := &mockLLM{
		responses: []string{"", "success"},
		errs:      []error{fmt.Errorf("anthropic api: 503 Service Unavailable"), nil},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 3
	p := NewRetryableProvider(inner, cfg)
	p.sleepFunc = noopSleep

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("want 'success', got %q", result)
	}
	if atomic.LoadInt32(&inner.calls) != 2 {
		t.Errorf("expected 2 calls (1 fail + 1 success), got %d", atomic.LoadInt32(&inner.calls))
	}
}

func TestRetryableProvider_Generate_WhenNonRetryableError_ShouldNotRetry(t *testing.T) {
	inner := &mockLLM{
		errs: []error{fmt.Errorf("anthropic api: 401 Unauthorized")},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 3
	p := NewRetryableProvider(inner, cfg)
	p.sleepFunc = noopSleep

	_, err := p.Generate(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&inner.calls) != 1 {
		t.Errorf("expected 1 call (no retry for 401), got %d", atomic.LoadInt32(&inner.calls))
	}
}

func TestRetryableProvider_Generate_WhenMaxRetriesExhausted_ShouldReturnLastError(t *testing.T) {
	serverErr := fmt.Errorf("anthropic api: 500 Internal Server Error")
	inner := &mockLLM{
		errs: []error{serverErr, serverErr, serverErr, serverErr},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 3
	p := NewRetryableProvider(inner, cfg)
	p.sleepFunc = noopSleep

	_, err := p.Generate(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// 1 initial + 3 retries = 4 calls
	if atomic.LoadInt32(&inner.calls) != 4 {
		t.Errorf("expected 4 calls (1 initial + 3 retries), got %d", atomic.LoadInt32(&inner.calls))
	}
}

func TestRetryableProvider_Generate_WhenMaxRetriesZero_ShouldNotRetry(t *testing.T) {
	inner := &mockLLM{
		errs: []error{fmt.Errorf("anthropic api: 503 Service Unavailable")},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 0
	p := NewRetryableProvider(inner, cfg)
	p.sleepFunc = noopSleep

	_, err := p.Generate(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&inner.calls) != 1 {
		t.Errorf("expected 1 call (no retries), got %d", atomic.LoadInt32(&inner.calls))
	}
}

func TestRetryableProvider_Generate_WhenContextCanceledDuringRetry_ShouldReturnContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inner := &mockLLM{
		errs: []error{
			fmt.Errorf("anthropic api: 503 Service Unavailable"),
			fmt.Errorf("anthropic api: 503 Service Unavailable"),
		},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 5
	p := NewRetryableProvider(inner, cfg)
	// Cancel context during sleep
	p.sleepFunc = func(d time.Duration) {
		cancel()
	}

	_, err := p.Generate(ctx, "hi")
	if err == nil {
		t.Fatal("expected error when context canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestRetryableProvider_Generate_ShouldUseExponentialBackoff(t *testing.T) {
	serverErr := fmt.Errorf("anthropic api: 500 Internal Server Error")
	inner := &mockLLM{
		errs: []error{serverErr, serverErr, serverErr, serverErr},
	}
	cfg := Config{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
	}
	p := NewRetryableProvider(inner, cfg)

	var sleepDurations []time.Duration
	p.sleepFunc = func(d time.Duration) {
		sleepDurations = append(sleepDurations, d)
	}

	_, _ = p.Generate(context.Background(), "hi")

	if len(sleepDurations) != 3 {
		t.Fatalf("expected 3 sleeps, got %d", len(sleepDurations))
	}
	// Backoff: 100ms, 200ms, 400ms
	expected := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	for i, want := range expected {
		if sleepDurations[i] != want {
			t.Errorf("sleep[%d]: want %v, got %v", i, want, sleepDurations[i])
		}
	}
}

func TestRetryableProvider_Generate_BackoffShouldCapAtMaxBackoff(t *testing.T) {
	serverErr := fmt.Errorf("anthropic api: 500 Internal Server Error")
	inner := &mockLLM{
		errs: []error{serverErr, serverErr, serverErr, serverErr, serverErr, serverErr},
	}
	cfg := Config{
		MaxRetries:     5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     300 * time.Millisecond,
		Multiplier:     2.0,
	}
	p := NewRetryableProvider(inner, cfg)

	var sleepDurations []time.Duration
	p.sleepFunc = func(d time.Duration) {
		sleepDurations = append(sleepDurations, d)
	}

	_, _ = p.Generate(context.Background(), "hi")

	// Backoff: 100ms, 200ms, 300ms (capped), 300ms (capped), 300ms (capped)
	for i, d := range sleepDurations {
		if d > 300*time.Millisecond {
			t.Errorf("sleep[%d] = %v exceeds MaxBackoff 300ms", i, d)
		}
	}
}

func TestRetryableProvider_Generate_ShouldReturnClearErrorMessageAfterExhaustion(t *testing.T) {
	serverErr := fmt.Errorf("anthropic api: 503 Service Unavailable")
	inner := &mockLLM{
		errs: []error{serverErr, serverErr, serverErr, serverErr},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 3
	p := NewRetryableProvider(inner, cfg)
	p.sleepFunc = noopSleep

	_, err := p.Generate(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	errMsg := err.Error()
	// Error message should mention retries exhausted and include the original error
	if !containsAll(errMsg, "retries exhausted", "503") {
		t.Errorf("error should mention retries exhausted and original error, got: %q", errMsg)
	}
}

func TestRetryableProvider_Generate_WhenTimeoutError_ShouldRetry(t *testing.T) {
	timeoutError := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &timeoutErr{},
	}
	inner := &mockLLM{
		responses: []string{"", "success after timeout"},
		errs:      []error{timeoutError, nil},
	}
	cfg := DefaultConfig()
	p := NewRetryableProvider(inner, cfg)
	p.sleepFunc = noopSleep

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success after timeout" {
		t.Errorf("want 'success after timeout', got %q", result)
	}
	if atomic.LoadInt32(&inner.calls) != 2 {
		t.Errorf("expected 2 calls, got %d", atomic.LoadInt32(&inner.calls))
	}
}

func TestRetryableProvider_Generate_WhenConnectionRefused_ShouldRetry(t *testing.T) {
	inner := &mockLLM{
		responses: []string{"", "connected"},
		errs:      []error{fmt.Errorf("anthropic do: dial tcp: connect: connection refused"), nil},
	}
	cfg := DefaultConfig()
	p := NewRetryableProvider(inner, cfg)
	p.sleepFunc = noopSleep

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "connected" {
		t.Errorf("want 'connected', got %q", result)
	}
}

func TestRetryableProvider_Generate_SucceedsOnThirdAttempt_ShouldReturnSuccess(t *testing.T) {
	serverErr := fmt.Errorf("anthropic api: 500 Internal Server Error")
	inner := &mockLLM{
		responses: []string{"", "", "third time lucky"},
		errs:      []error{serverErr, serverErr, nil},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 5
	p := NewRetryableProvider(inner, cfg)
	p.sleepFunc = noopSleep

	result, err := p.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "third time lucky" {
		t.Errorf("want 'third time lucky', got %q", result)
	}
	if atomic.LoadInt32(&inner.calls) != 3 {
		t.Errorf("expected 3 calls, got %d", atomic.LoadInt32(&inner.calls))
	}
}

func TestRetryableProvider_ImplementsLLMProvider(t *testing.T) {
	inner := &mockLLM{responses: []string{"ok"}}
	var _ domain.LLMProvider = NewRetryableProvider(inner, DefaultConfig())
}

func TestRetryableProvider_Generate_ShouldPassPromptToInner(t *testing.T) {
	var capturedPrompt string
	inner := &promptCapturingLLM{captured: &capturedPrompt}
	p := NewRetryableProvider(inner, DefaultConfig())
	p.sleepFunc = noopSleep

	_, _ = p.Generate(context.Background(), "what is 2+2?")
	if capturedPrompt != "what is 2+2?" {
		t.Errorf("want prompt 'what is 2+2?', got %q", capturedPrompt)
	}
}

// promptCapturingLLM captures the last prompt for verification.
type promptCapturingLLM struct {
	captured *string
}

func (p *promptCapturingLLM) Generate(ctx context.Context, prompt string) (string, error) {
	*p.captured = prompt
	return "ok", nil
}

// =============================================================================
// Helpers
// =============================================================================

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
