package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"ironclaw/internal/domain"
)

// KeyPool manages a pool of API keys with round-robin rotation and cooldown support.
// When a key receives a rate-limit (429) error, it can be marked as "cooldown" and
// subsequent calls to Next will skip it until the cooldown period expires.
// KeyPool is safe for concurrent use.
type KeyPool struct {
	keys        []string
	mu          sync.Mutex
	nextIdx     int
	cooldowns   []time.Time   // parallel to keys — zero value means no cooldown
	cooldownDur time.Duration // how long a key stays in cooldown
	nowFunc     func() time.Time
}

// NewKeyPool creates a KeyPool from the given keys with the specified cooldown duration.
// Returns an error if keys is empty or nil.
func NewKeyPool(keys []string, cooldownDur time.Duration) (*KeyPool, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("keypool: at least one key is required")
	}
	return &KeyPool{
		keys:        keys,
		cooldowns:   make([]time.Time, len(keys)),
		cooldownDur: cooldownDur,
		nowFunc:     time.Now,
	}, nil
}

// Next returns the next available key using round-robin, skipping keys in cooldown.
// Returns the key, its index, and an error if all keys are in cooldown.
func (kp *KeyPool) Next() (string, int, error) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	now := kp.nowFunc()
	n := len(kp.keys)

	// Try each key starting from nextIdx, wrapping around
	for i := 0; i < n; i++ {
		idx := (kp.nextIdx + i) % n
		if kp.cooldowns[idx].IsZero() || now.After(kp.cooldowns[idx]) {
			// This key is available
			kp.nextIdx = (idx + 1) % n
			return kp.keys[idx], idx, nil
		}
	}

	return "", -1, fmt.Errorf("keypool: all %d keys are in cooldown", n)
}

// MarkCooldown puts the key at the given index into cooldown for the configured duration.
// Out-of-range indices are silently ignored.
func (kp *KeyPool) MarkCooldown(idx int) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	if idx < 0 || idx >= len(kp.keys) {
		return
	}
	kp.cooldowns[idx] = kp.nowFunc().Add(kp.cooldownDur)
}

// Len returns the total number of keys in the pool.
func (kp *KeyPool) Len() int {
	return len(kp.keys)
}

// Available returns the number of keys not currently in cooldown.
func (kp *KeyPool) Available() int {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	now := kp.nowFunc()
	count := 0
	for _, cd := range kp.cooldowns {
		if cd.IsZero() || now.After(cd) {
			count++
		}
	}
	return count
}

// =============================================================================
// Rate-limit detection
// =============================================================================

// isRateLimitError returns true when the error indicates a 429 / rate-limit response.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "rate limit")
}

// =============================================================================
// KeyPoolProvider (LLMProvider decorator)
// =============================================================================

// KeyPoolProvider wraps multiple LLMProviders (one per API key) and rotates between
// them using a KeyPool. On a 429 rate-limit error, the current key is marked as
// cooldown and the request is retried with the next available key.
type KeyPoolProvider struct {
	pool      *KeyPool
	providers []domain.LLMProvider
}

// NewKeyPoolProvider creates a KeyPoolProvider. The pool and providers must have matching lengths.
func NewKeyPoolProvider(pool *KeyPool, providers []domain.LLMProvider) (*KeyPoolProvider, error) {
	if pool == nil {
		return nil, fmt.Errorf("keypool provider: pool must not be nil")
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("keypool provider: at least one provider is required")
	}
	if pool.Len() != len(providers) {
		return nil, fmt.Errorf("keypool provider: pool size (%d) must match providers count (%d)", pool.Len(), len(providers))
	}
	return &KeyPoolProvider{
		pool:      pool,
		providers: providers,
	}, nil
}

// Generate implements domain.LLMProvider. It selects the next available key/provider
// via round-robin, and on a 429 error marks the key as cooldown and retries once
// with the next available key.
func (kpp *KeyPoolProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	_, idx, err := kpp.pool.Next()
	if err != nil {
		return "", err
	}

	result, genErr := kpp.providers[idx].Generate(ctx, prompt)
	if genErr == nil {
		return result, nil
	}

	// Only retry on rate-limit errors
	if !isRateLimitError(genErr) {
		return "", genErr
	}

	// Mark the rate-limited key as cooldown
	kpp.pool.MarkCooldown(idx)

	// Try once more with the next available key
	_, idx2, err := kpp.pool.Next()
	if err != nil {
		// All keys in cooldown
		return "", fmt.Errorf("all keys in cooldown after rate limit: %w", genErr)
	}

	return kpp.providers[idx2].Generate(ctx, prompt)
}

// Compile-time check that KeyPoolProvider implements LLMProvider.
var _ domain.LLMProvider = (*KeyPoolProvider)(nil)
