package retry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"ironclaw/internal/domain"
)

// =============================================================================
// RetryConfig
// =============================================================================

// Config controls retry behaviour for external API calls.
type Config struct {
	MaxRetries     int           `json:"maxRetries"`     // Maximum number of retry attempts (0 = no retries)
	InitialBackoff time.Duration `json:"initialBackoff"` // Delay before first retry
	MaxBackoff     time.Duration `json:"maxBackoff"`     // Upper bound on backoff duration
	Multiplier     float64       `json:"multiplier"`     // Backoff multiplier (e.g. 2.0 for exponential)
}

// DefaultConfig returns sensible retry defaults.
func DefaultConfig() Config {
	return Config{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
	}
}

// Validate checks that all Config fields are within acceptable ranges.
func (c Config) Validate() error {
	if c.MaxRetries < 0 {
		return errors.New("retry: MaxRetries must be >= 0")
	}
	if c.InitialBackoff <= 0 {
		return errors.New("retry: InitialBackoff must be > 0")
	}
	if c.MaxBackoff <= 0 {
		return errors.New("retry: MaxBackoff must be > 0")
	}
	if c.Multiplier < 1.0 {
		return errors.New("retry: Multiplier must be >= 1.0")
	}
	return nil
}

// =============================================================================
// Error Classification
// =============================================================================

// retryableStatusCodes are HTTP status codes that indicate a transient failure.
var retryableStatusCodes = []string{"429", "500", "502", "503", "504", "529"}

// IsRetryable returns true when err represents a transient failure that may
// succeed on retry (5xx, 429, timeout, connection refused, EOF).
// Context errors (Canceled, DeadlineExceeded) are never retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are never retryable — the caller chose to cancel.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// net.Error timeout (wraps OS-level i/o timeout)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	msg := err.Error()

	// HTTP status codes that are retryable
	for _, code := range retryableStatusCodes {
		if strings.Contains(msg, code) {
			return true
		}
	}

	// Connection-level transient failures
	if strings.Contains(msg, "connection refused") {
		return true
	}
	if strings.Contains(msg, "EOF") {
		return true
	}

	return false
}

// =============================================================================
// RetryableProvider (Decorator)
// =============================================================================

// RetryableProvider wraps an LLMProvider with retry-on-transient-error logic.
type RetryableProvider struct {
	inner     domain.LLMProvider
	config    Config
	sleepFunc func(time.Duration) // injectable for testing
}

// NewRetryableProvider returns a decorator that retries Generate calls on transient errors.
// inner must not be nil.
func NewRetryableProvider(inner domain.LLMProvider, cfg Config) *RetryableProvider {
	if inner == nil {
		panic("retry: inner provider must not be nil")
	}
	return &RetryableProvider{
		inner:     inner,
		config:    cfg,
		sleepFunc: time.Sleep,
	}
}

// Generate calls the inner provider and retries on transient errors with exponential backoff.
// Returns the first successful result, or the last error after retries are exhausted.
func (p *RetryableProvider) Generate(ctx context.Context, prompt string) (string, error) {
	var lastErr error
	backoff := p.config.InitialBackoff

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		result, err := p.inner.Generate(ctx, prompt)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry non-retryable errors
		if !IsRetryable(err) {
			return "", err
		}

		// Don't sleep after the last attempt
		if attempt == p.config.MaxRetries {
			break
		}

		// Sleep with exponential backoff, checking context cancellation
		p.sleepFunc(backoff)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		// Increase backoff for next iteration, capped at MaxBackoff
		next := time.Duration(float64(backoff) * p.config.Multiplier)
		if next > p.config.MaxBackoff {
			next = p.config.MaxBackoff
		}
		backoff = next
	}

	return "", fmt.Errorf("retries exhausted after %d attempts: %w", p.config.MaxRetries+1, lastErr)
}

// Compile-time check that RetryableProvider implements LLMProvider.
var _ domain.LLMProvider = (*RetryableProvider)(nil)
