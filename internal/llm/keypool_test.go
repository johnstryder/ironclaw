package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"ironclaw/internal/domain"
)

// =============================================================================
// KeyPool Creation
// =============================================================================

func TestNewKeyPool_WhenValidKeys_ShouldCreatePool(t *testing.T) {
	pool, err := NewKeyPool([]string{"key1", "key2"}, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool == nil {
		t.Fatal("pool should not be nil")
	}
	if pool.Len() != 2 {
		t.Errorf("want 2 keys, got %d", pool.Len())
	}
}

func TestNewKeyPool_WhenSingleKey_ShouldCreatePool(t *testing.T) {
	pool, err := NewKeyPool([]string{"only-key"}, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.Len() != 1 {
		t.Errorf("want 1 key, got %d", pool.Len())
	}
}

func TestNewKeyPool_WhenEmptyKeys_ShouldReturnError(t *testing.T) {
	_, err := NewKeyPool([]string{}, 60*time.Second)
	if err == nil {
		t.Error("expected error for empty keys")
	}
}

func TestNewKeyPool_WhenNilKeys_ShouldReturnError(t *testing.T) {
	_, err := NewKeyPool(nil, 60*time.Second)
	if err == nil {
		t.Error("expected error for nil keys")
	}
}

// =============================================================================
// Round-Robin Rotation
// =============================================================================

func TestKeyPool_Next_ShouldReturnFirstKeyOnFirstCall(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a", "key-b", "key-c"}, 60*time.Second)

	key, idx, err := pool.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "key-a" {
		t.Errorf("want key-a, got %q", key)
	}
	if idx != 0 {
		t.Errorf("want idx 0, got %d", idx)
	}
}

func TestKeyPool_Next_ShouldRotateKeysRoundRobin(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a", "key-b", "key-c"}, 60*time.Second)

	expected := []struct {
		key string
		idx int
	}{
		{"key-a", 0},
		{"key-b", 1},
		{"key-c", 2},
		{"key-a", 0}, // wraps around
		{"key-b", 1},
	}

	for i, want := range expected {
		key, idx, err := pool.Next()
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if key != want.key {
			t.Errorf("call %d: want key %q, got %q", i, want.key, key)
		}
		if idx != want.idx {
			t.Errorf("call %d: want idx %d, got %d", i, want.idx, idx)
		}
	}
}

func TestKeyPool_Next_WhenSingleKey_ShouldAlwaysReturnSameKey(t *testing.T) {
	pool, _ := NewKeyPool([]string{"only-key"}, 60*time.Second)

	for i := 0; i < 5; i++ {
		key, idx, err := pool.Next()
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if key != "only-key" {
			t.Errorf("call %d: want only-key, got %q", i, key)
		}
		if idx != 0 {
			t.Errorf("call %d: want idx 0, got %d", i, idx)
		}
	}
}

// =============================================================================
// Cooldown Behavior
// =============================================================================

func TestKeyPool_MarkCooldown_ShouldSkipCoolingDownKey(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b", "key-c"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	// First call returns key-a (idx=0)
	pool.Next()

	// Mark key-a as cooldown
	pool.MarkCooldown(0)

	// Next call should skip key-a and return key-b
	key, idx, err := pool.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "key-b" {
		t.Errorf("want key-b (skipping cooled-down key-a), got %q", key)
	}
	if idx != 1 {
		t.Errorf("want idx 1, got %d", idx)
	}
}

func TestKeyPool_MarkCooldown_ShouldSkipMultipleCooledKeys(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b", "key-c"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	// Mark key-a and key-b as cooldown
	pool.MarkCooldown(0)
	pool.MarkCooldown(1)

	// Next should skip both and return key-c
	key, idx, err := pool.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "key-c" {
		t.Errorf("want key-c, got %q", key)
	}
	if idx != 2 {
		t.Errorf("want idx 2, got %d", idx)
	}
}

func TestKeyPool_Next_WhenAllKeysInCooldown_ShouldReturnError(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	pool.MarkCooldown(0)
	pool.MarkCooldown(1)

	_, _, err := pool.Next()
	if err == nil {
		t.Error("expected error when all keys are in cooldown")
	}
}

func TestKeyPool_Next_ShouldRecoverKeyAfterCooldownExpires(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	// Use key-a, then mark it cooldown
	pool.Next() // key-a
	pool.MarkCooldown(0)

	// Advance time past cooldown
	now = now.Add(61 * time.Second)
	pool.nowFunc = func() time.Time { return now }

	// key-b would be next in rotation, but after that key-a should be available again
	pool.Next() // key-b (idx=1)

	key, idx, err := pool.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "key-a" {
		t.Errorf("want key-a (recovered from cooldown), got %q", key)
	}
	if idx != 0 {
		t.Errorf("want idx 0, got %d", idx)
	}
}

func TestKeyPool_MarkCooldown_WhenInvalidIndex_ShouldNotPanic(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a"}, 60*time.Second)

	// Should not panic with out-of-range indices
	pool.MarkCooldown(-1)
	pool.MarkCooldown(5)
	pool.MarkCooldown(100)

	// Pool should still work
	key, _, err := pool.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "key-a" {
		t.Errorf("want key-a, got %q", key)
	}
}

// =============================================================================
// Available Keys Count
// =============================================================================

func TestKeyPool_Available_ShouldReturnAllKeysWhenNoneCooledDown(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a", "key-b", "key-c"}, 60*time.Second)
	if pool.Available() != 3 {
		t.Errorf("want 3 available, got %d", pool.Available())
	}
}

func TestKeyPool_Available_ShouldExcludeCooledDownKeys(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b", "key-c"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	pool.MarkCooldown(0)
	if pool.Available() != 2 {
		t.Errorf("want 2 available after 1 cooldown, got %d", pool.Available())
	}
}

func TestKeyPool_Available_ShouldRecoverAfterCooldownExpires(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	pool.MarkCooldown(0)
	if pool.Available() != 1 {
		t.Errorf("want 1 available, got %d", pool.Available())
	}

	// Advance time
	now = now.Add(61 * time.Second)
	pool.nowFunc = func() time.Time { return now }

	if pool.Available() != 2 {
		t.Errorf("want 2 available after cooldown expired, got %d", pool.Available())
	}
}

// =============================================================================
// Thread Safety
// =============================================================================

func TestKeyPool_Next_ShouldBeThreadSafe(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a", "key-b", "key-c"}, 60*time.Second)

	var wg sync.WaitGroup
	results := make(chan string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key, _, err := pool.Next()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			results <- key
		}()
	}

	wg.Wait()
	close(results)

	counts := make(map[string]int)
	for key := range results {
		counts[key]++
	}

	// With round-robin over 3 keys and 100 calls, distribution should be roughly equal
	for key, count := range counts {
		if count < 30 || count > 37 {
			t.Errorf("key %q got %d calls, expected ~33", key, count)
		}
	}
}

func TestKeyPool_MarkCooldown_ShouldBeThreadSafe(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b", "key-c", "key-d", "key-e"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	var wg sync.WaitGroup
	// Concurrently cooldown and read
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			pool.MarkCooldown(idx % 5)
		}(i)
		go func() {
			defer wg.Done()
			pool.Available()
		}()
	}
	wg.Wait()

	// Should not have panicked; just verify pool is still usable
	_ = pool.Len()
}

// =============================================================================
// isRateLimitError
// =============================================================================

func TestIsRateLimitError_WhenContains429_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("openai api: 429 Too Many Requests")
	if !isRateLimitError(err) {
		t.Error("expected true for 429 error")
	}
}

func TestIsRateLimitError_WhenContainsRateLimit_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("rate limit exceeded")
	if !isRateLimitError(err) {
		t.Error("expected true for rate limit error")
	}
}

func TestIsRateLimitError_WhenContainsRateLimitMixedCase_ShouldReturnTrue(t *testing.T) {
	err := fmt.Errorf("Rate Limit exceeded")
	if !isRateLimitError(err) {
		t.Error("expected true for Rate Limit error")
	}
}

func TestIsRateLimitError_WhenNormalError_ShouldReturnFalse(t *testing.T) {
	err := fmt.Errorf("openai api: 500 Internal Server Error")
	if isRateLimitError(err) {
		t.Error("expected false for 500 error")
	}
}

func TestIsRateLimitError_WhenNilError_ShouldReturnFalse(t *testing.T) {
	if isRateLimitError(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsRateLimitError_WhenAuthError_ShouldReturnFalse(t *testing.T) {
	err := fmt.Errorf("openai api: 401 Unauthorized")
	if isRateLimitError(err) {
		t.Error("expected false for 401 error")
	}
}

// =============================================================================
// KeyPoolProvider
// =============================================================================

// mockProvider is a test double that records calls and returns configurable results.
type mockProvider struct {
	name     string
	response string
	err      error
	calls    int
}

func (m *mockProvider) Generate(_ context.Context, prompt string) (string, error) {
	m.calls++
	if m.err != nil {
		return "", m.err
	}
	return m.response + ": " + prompt, nil
}

var _ domain.LLMProvider = (*mockProvider)(nil)

func TestNewKeyPoolProvider_WhenValidInputs_ShouldCreateProvider(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	providers := []domain.LLMProvider{
		&mockProvider{name: "a", response: "resp-a"},
		&mockProvider{name: "b", response: "resp-b"},
	}

	kpp, err := NewKeyPoolProvider(pool, providers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kpp == nil {
		t.Fatal("provider should not be nil")
	}
}

func TestNewKeyPoolProvider_WhenNilPool_ShouldReturnError(t *testing.T) {
	providers := []domain.LLMProvider{
		&mockProvider{name: "a", response: "resp-a"},
	}

	_, err := NewKeyPoolProvider(nil, providers)
	if err == nil {
		t.Error("expected error for nil pool")
	}
}

func TestNewKeyPoolProvider_WhenNilProviders_ShouldReturnError(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a"}, 60*time.Second)

	_, err := NewKeyPoolProvider(pool, nil)
	if err == nil {
		t.Error("expected error for nil providers")
	}
}

func TestNewKeyPoolProvider_WhenMismatchedLengths_ShouldReturnError(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	providers := []domain.LLMProvider{
		&mockProvider{name: "a", response: "resp-a"},
	}

	_, err := NewKeyPoolProvider(pool, providers)
	if err == nil {
		t.Error("expected error when pool size != providers length")
	}
}

func TestKeyPoolProvider_Generate_ShouldForwardToCurrentProvider(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a"}, 60*time.Second)
	mock := &mockProvider{name: "a", response: "resp-a"}
	providers := []domain.LLMProvider{mock}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	result, err := kpp.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "resp-a: hello" {
		t.Errorf("want 'resp-a: hello', got %q", result)
	}
	if mock.calls != 1 {
		t.Errorf("want 1 call, got %d", mock.calls)
	}
}

func TestKeyPoolProvider_Generate_ShouldRotateProviders(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	mockA := &mockProvider{name: "a", response: "resp-a"}
	mockB := &mockProvider{name: "b", response: "resp-b"}
	providers := []domain.LLMProvider{mockA, mockB}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	// First call: provider A
	result, _ := kpp.Generate(context.Background(), "1")
	if result != "resp-a: 1" {
		t.Errorf("call 1: want 'resp-a: 1', got %q", result)
	}

	// Second call: provider B
	result, _ = kpp.Generate(context.Background(), "2")
	if result != "resp-b: 2" {
		t.Errorf("call 2: want 'resp-b: 2', got %q", result)
	}

	// Third call: provider A again
	result, _ = kpp.Generate(context.Background(), "3")
	if result != "resp-a: 3" {
		t.Errorf("call 3: want 'resp-a: 3', got %q", result)
	}
}

func TestKeyPoolProvider_Generate_WhenRateLimited_ShouldCooldownAndRetryNextKey(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	mockA := &mockProvider{name: "a", err: fmt.Errorf("openai api: 429 Too Many Requests")}
	mockB := &mockProvider{name: "b", response: "resp-b"}
	providers := []domain.LLMProvider{mockA, mockB}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	result, err := kpp.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "resp-b: hello" {
		t.Errorf("want 'resp-b: hello', got %q", result)
	}
	// Provider A should have been called once (and failed)
	if mockA.calls != 1 {
		t.Errorf("mockA: want 1 call, got %d", mockA.calls)
	}
	// Provider B should have been called once (retry succeeded)
	if mockB.calls != 1 {
		t.Errorf("mockB: want 1 call, got %d", mockB.calls)
	}
	// Key A should now be in cooldown
	if pool.Available() != 1 {
		t.Errorf("want 1 available key (A in cooldown), got %d", pool.Available())
	}
}

func TestKeyPoolProvider_Generate_WhenAllKeysRateLimited_ShouldReturnError(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	rateLimitErr := fmt.Errorf("openai api: 429 Too Many Requests")
	mockA := &mockProvider{name: "a", err: rateLimitErr}
	mockB := &mockProvider{name: "b", err: rateLimitErr}
	providers := []domain.LLMProvider{mockA, mockB}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	_, err := kpp.Generate(context.Background(), "hello")
	if err == nil {
		t.Error("expected error when all keys are rate-limited")
	}
}

func TestKeyPoolProvider_Generate_WhenNon429Error_ShouldReturnErrorWithoutCooldown(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)

	authErr := fmt.Errorf("openai api: 401 Unauthorized")
	mockA := &mockProvider{name: "a", err: authErr}
	mockB := &mockProvider{name: "b", response: "resp-b"}
	providers := []domain.LLMProvider{mockA, mockB}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	_, err := kpp.Generate(context.Background(), "hello")
	if err == nil {
		t.Error("expected error to be returned")
	}
	if err.Error() != authErr.Error() {
		t.Errorf("want original error, got %q", err.Error())
	}
	// Key A should NOT be in cooldown (was a 401, not 429)
	if pool.Available() != 2 {
		t.Errorf("want 2 available keys (no cooldown for 401), got %d", pool.Available())
	}
	// Provider B should not have been called
	if mockB.calls != 0 {
		t.Errorf("mockB should not have been called, got %d calls", mockB.calls)
	}
}

func TestKeyPoolProvider_Generate_WhenContextCanceled_ShouldReturnContextError(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a"}, 60*time.Second)
	mock := &mockProvider{name: "a", response: "resp-a"}
	providers := []domain.LLMProvider{mock}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := kpp.Generate(ctx, "hello")
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestKeyPoolProvider_Generate_ShouldImplementLLMProvider(t *testing.T) {
	pool, _ := NewKeyPool([]string{"key-a"}, 60*time.Second)
	mock := &mockProvider{name: "a", response: "resp-a"}
	providers := []domain.LLMProvider{mock}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	// Compile-time check
	var _ domain.LLMProvider = kpp
}

func TestKeyPoolProvider_Generate_WhenRateLimitedAndRetryAlsoFails_ShouldReturnRetryError(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	rateLimitErr := fmt.Errorf("openai api: 429 Too Many Requests")
	genericErr := fmt.Errorf("openai api: 500 Internal Server Error")
	mockA := &mockProvider{name: "a", err: rateLimitErr}
	mockB := &mockProvider{name: "b", err: genericErr}
	providers := []domain.LLMProvider{mockA, mockB}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	_, err := kpp.Generate(context.Background(), "hello")
	if err == nil {
		t.Error("expected error")
	}
	// Should return the error from the retry attempt (provider B's error)
	if err.Error() != genericErr.Error() {
		t.Errorf("want provider B's error, got %q", err.Error())
	}
}

func TestKeyPoolProvider_Generate_WhenRateLimitedAndAllOtherKeysAlreadyCooledDown_ShouldReturnCooldownError(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a", "key-b"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	rateLimitErr := fmt.Errorf("openai api: 429 Too Many Requests")
	mockA := &mockProvider{name: "a", err: rateLimitErr}
	mockB := &mockProvider{name: "b", response: "resp-b"}
	providers := []domain.LLMProvider{mockA, mockB}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	// Pre-cooldown key-b so when key-a gets 429, there's no fallback
	pool.MarkCooldown(1)

	_, err := kpp.Generate(context.Background(), "hello")
	if err == nil {
		t.Error("expected error when all keys in cooldown after rate limit")
	}
	// Error message should indicate all keys are in cooldown
	if !strings.Contains(err.Error(), "cooldown") {
		t.Errorf("error should mention cooldown, got %q", err.Error())
	}
	// Provider B should not have been called (it was already cooled down)
	if mockB.calls != 0 {
		t.Errorf("mockB should not have been called, got %d calls", mockB.calls)
	}
}

func TestKeyPoolProvider_Generate_WhenPoolExhaustedBeforeCall_ShouldReturnError(t *testing.T) {
	now := time.Now()
	pool, _ := NewKeyPool([]string{"key-a"}, 60*time.Second)
	pool.nowFunc = func() time.Time { return now }

	mock := &mockProvider{name: "a", response: "resp-a"}
	providers := []domain.LLMProvider{mock}

	kpp, _ := NewKeyPoolProvider(pool, providers)

	// Put the only key in cooldown before calling Generate
	pool.MarkCooldown(0)

	_, err := kpp.Generate(context.Background(), "hello")
	if err == nil {
		t.Error("expected error when pool is exhausted before Generate")
	}
	// Provider should not have been called at all
	if mock.calls != 0 {
		t.Errorf("provider should not have been called, got %d calls", mock.calls)
	}
}

// =============================================================================
// splitKeys tests
// =============================================================================

func TestSplitKeys_WhenSingleKey_ShouldReturnSingleElement(t *testing.T) {
	keys := splitKeys("my-api-key")
	if len(keys) != 1 {
		t.Fatalf("want 1 key, got %d", len(keys))
	}
	if keys[0] != "my-api-key" {
		t.Errorf("want my-api-key, got %q", keys[0])
	}
}

func TestSplitKeys_WhenMultipleKeys_ShouldSplitByComma(t *testing.T) {
	keys := splitKeys("key1,key2,key3")
	if len(keys) != 3 {
		t.Fatalf("want 3 keys, got %d", len(keys))
	}
	expected := []string{"key1", "key2", "key3"}
	for i, want := range expected {
		if keys[i] != want {
			t.Errorf("key %d: want %q, got %q", i, want, keys[i])
		}
	}
}

func TestSplitKeys_WhenWhitespace_ShouldTrim(t *testing.T) {
	keys := splitKeys("  key1 , key2 , key3  ")
	if len(keys) != 3 {
		t.Fatalf("want 3 keys, got %d", len(keys))
	}
	if keys[0] != "key1" || keys[1] != "key2" || keys[2] != "key3" {
		t.Errorf("expected trimmed keys, got %v", keys)
	}
}

func TestSplitKeys_WhenEmptyEntries_ShouldFilter(t *testing.T) {
	keys := splitKeys("key1,,key2,")
	if len(keys) != 2 {
		t.Fatalf("want 2 keys (empty filtered), got %d", len(keys))
	}
}

func TestSplitKeys_WhenAllEmpty_ShouldReturnEmpty(t *testing.T) {
	keys := splitKeys(",,,")
	if len(keys) != 0 {
		t.Fatalf("want 0 keys, got %d", len(keys))
	}
}

func TestSplitKeys_WhenEmptyString_ShouldReturnEmpty(t *testing.T) {
	keys := splitKeys("")
	if len(keys) != 0 {
		t.Fatalf("want 0 keys, got %d", len(keys))
	}
}
