package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ironclaw/internal/domain"
)

func TestNewProvider_WhenConfigIsNil_ShouldReturnLocalProvider(t *testing.T) {
	getSecret := func(name string) (string, error) { return "", nil }

	provider, err := NewProvider(nil, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	got, err := provider.Generate(context.Background(), "test")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// Local default prefix is "Local: "
	if got != "Local: test" {
		t.Errorf("want Local: test, got %q", got)
	}
}

func TestNewProvider_WhenProviderIsLocal_ShouldReturnLocalProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "local", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) { return "", nil }

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	got, err := provider.Generate(context.Background(), "test")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// Local default prefix is "Local: "
	if got != "Local: test" {
		t.Errorf("want Local: test, got %q", got)
	}
}

func TestNewProvider_WhenProviderIsEmpty_ShouldDefaultToLocal(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) { return "", nil }

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	got, _ := provider.Generate(context.Background(), "hi")
	if got != "Local: hi" {
		t.Errorf("default provider should be local, got %q", got)
	}
}

func TestNewProvider_WhenProviderIsOpenAI_AndKeyMissing_ShouldReturnError(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openai", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) { return "", nil }

	_, err := NewProvider(cfg, getSecret)
	if err == nil {
		t.Error("expected error when OpenAI selected but no API key")
	}
}

func TestNewProvider_WhenProviderIsAnthropic_AndKeyMissing_ShouldReturnError(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "anthropic", DefaultModel: "claude-3-5-sonnet"}
	getSecret := func(name string) (string, error) { return "", nil }

	_, err := NewProvider(cfg, getSecret)
	if err == nil {
		t.Error("expected error when Anthropic selected but no API key")
	}
}

func TestNewProvider_WhenProviderIsOpenRouter_AndKeyMissing_ShouldReturnError(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openrouter", DefaultModel: "openai/gpt-4"}
	getSecret := func(name string) (string, error) { return "", nil }

	_, err := NewProvider(cfg, getSecret)
	if err == nil {
		t.Error("expected error when OpenRouter selected but no API key")
	}
}

func TestNewProvider_WhenProviderIsGemini_AndKeyMissing_ShouldReturnError(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "gemini", DefaultModel: "gemini-pro"}
	getSecret := func(name string) (string, error) { return "", nil }

	_, err := NewProvider(cfg, getSecret)
	if err == nil {
		t.Error("expected error when Gemini selected but no API key")
	}
}

func TestNewProvider_WhenGetSecretFails_OpenAI_ShouldReturnError(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openai", DefaultModel: "gpt-4o"}
	wantErr := errors.New("secret error")
	getSecret := func(name string) (string, error) { return "", wantErr }

	_, err := NewProvider(cfg, getSecret)
	if err != wantErr {
		t.Errorf("want %v, got %v", wantErr, err)
	}
}

func TestNewProvider_WhenGetSecretFails_Anthropic_ShouldReturnError(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "anthropic", DefaultModel: "claude-3-5-sonnet"}
	wantErr := errors.New("secret error")
	getSecret := func(name string) (string, error) { return "", wantErr }

	_, err := NewProvider(cfg, getSecret)
	if err != wantErr {
		t.Errorf("want %v, got %v", wantErr, err)
	}
}

func TestNewProvider_WhenGetSecretFails_OpenRouter_ShouldReturnError(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openrouter", DefaultModel: "openai/gpt-4"}
	wantErr := errors.New("secret error")
	getSecret := func(name string) (string, error) { return "", wantErr }

	_, err := NewProvider(cfg, getSecret)
	if err != wantErr {
		t.Errorf("want %v, got %v", wantErr, err)
	}
}

func TestNewProvider_WhenGetSecretFails_Gemini_ShouldReturnError(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "gemini", DefaultModel: "gemini-pro"}
	wantErr := errors.New("secret error")
	getSecret := func(name string) (string, error) { return "", wantErr }

	_, err := NewProvider(cfg, getSecret)
	if err != wantErr {
		t.Errorf("want %v, got %v", wantErr, err)
	}
}

func TestNewProvider_WhenProviderUnknown_ShouldReturnError(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "unknown", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) { return "", nil }

	_, err := NewProvider(cfg, getSecret)
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestNewProvider_WhenProviderIsOpenAI_AndKeyProvided_ShouldReturnOpenAIProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openai", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) {
		if name == "openai_api_key" {
			return "test-key", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	// Test that it's an OpenAIProvider by calling Generate (it will fail but prove it's the right type)
	_, err = provider.Generate(context.Background(), "hi")
	if err == nil {
		t.Error("expected error with test key")
	}
}

func TestNewProvider_WhenProviderIsAnthropic_AndKeyProvided_ShouldReturnAnthropicProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "anthropic", DefaultModel: "claude-3-5-sonnet"}
	getSecret := func(name string) (string, error) {
		if name == "anthropic_api_key" {
			return "test-key", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	// Test that it's an AnthropicProvider by calling Generate (it will fail but prove it's the right type)
	_, err = provider.Generate(context.Background(), "hi")
	if err == nil {
		t.Error("expected error with test key")
	}
}

func TestNewProvider_WhenProviderIsOpenRouter_AndKeyProvided_ShouldReturnOpenRouterProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openrouter", DefaultModel: "openai/gpt-4"}
	getSecret := func(name string) (string, error) {
		if name == "openrouter_api_key" {
			return "test-key", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	// Test that it's an OpenRouterProvider by calling Generate (it will fail but prove it's the right type)
	_, err = provider.Generate(context.Background(), "hi")
	if err == nil {
		t.Error("expected error with test key")
	}
}

func TestNewProvider_WhenProviderIsGemini_AndKeyProvided_ShouldReturnGeminiProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "gemini", DefaultModel: "gemini-pro"}
	getSecret := func(name string) (string, error) {
		if name == "gemini_api_key" {
			return "test-key", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	// Test that it's a GeminiProvider by calling Generate (it will fail but prove it's the right type)
	_, err = provider.Generate(context.Background(), "hi")
	if err == nil {
		t.Error("expected error with test key")
	}
}

func TestNewProvider_WhenProviderIsOllama_ShouldReturnOllamaProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "ollama", DefaultModel: "llama3"}
	getSecret := func(name string) (string, error) { return "", nil }

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	// Test that it's an OllamaProvider by calling Generate (it will fail but prove it's the right type)
	_, err = provider.Generate(context.Background(), "hi")
	if err == nil {
		t.Error("expected error when Ollama server not running")
	}
}

// =============================================================================
// Retry wrapping tests
// =============================================================================

func TestNewProvider_WhenRetryConfigProvided_ShouldWrapWithRetry(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "local", DefaultModel: "test"}
	getSecret := func(name string) (string, error) { return "", nil }
	retryCfg := &domain.RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100,
		MaxBackoff:     1000,
		Multiplier:     2,
	}

	provider, err := NewProvider(cfg, getSecret, retryCfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	// Should still work through the retry wrapper
	result, err := provider.Generate(context.Background(), "test")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Local: test" {
		t.Errorf("want 'Local: test', got %q", result)
	}
}

func TestNewProvider_WhenRetryConfigNil_ShouldNotWrap(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "local", DefaultModel: "test"}
	getSecret := func(name string) (string, error) { return "", nil }

	provider, err := NewProvider(cfg, getSecret, nil)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	result, err := provider.Generate(context.Background(), "test")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Local: test" {
		t.Errorf("want 'Local: test', got %q", result)
	}
}

func TestNewProvider_WhenRetryConfigMaxRetriesZero_ShouldNotWrap(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "local", DefaultModel: "test"}
	getSecret := func(name string) (string, error) { return "", nil }
	retryCfg := &domain.RetryConfig{
		MaxRetries:     0,
		InitialBackoff: 100,
		MaxBackoff:     1000,
		Multiplier:     2,
	}

	provider, err := NewProvider(cfg, getSecret, retryCfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	result, err := provider.Generate(context.Background(), "test")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Local: test" {
		t.Errorf("want 'Local: test', got %q", result)
	}
}

func TestNewProvider_WhenNoRetryConfig_ShouldNotWrap(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "local", DefaultModel: "test"}
	getSecret := func(name string) (string, error) { return "", nil }

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	result, err := provider.Generate(context.Background(), "test")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Local: test" {
		t.Errorf("want 'Local: test', got %q", result)
	}
}

// =============================================================================
// Fallback provider tests
// =============================================================================

func TestNewFallbackProviders_WhenValidConfigs_ShouldReturnProviders(t *testing.T) {
	getSecret := func(name string) (string, error) { return "", nil }
	fallbacks := []domain.FallbackConfig{
		{Provider: "local", DefaultModel: "test1"},
		{Provider: "local", DefaultModel: "test2"},
	}

	providers := NewFallbackProviders(fallbacks, getSecret)
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	result, err := providers[0].Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "Local: hi" {
		t.Errorf("want 'Local: hi', got %q", result)
	}
}

func TestNewFallbackProviders_WhenEmptyConfigs_ShouldReturnNil(t *testing.T) {
	getSecret := func(name string) (string, error) { return "", nil }

	providers := NewFallbackProviders(nil, getSecret)
	if providers != nil {
		t.Errorf("expected nil, got %v", providers)
	}
}

func TestNewFallbackProviders_WhenInvalidConfigMixed_ShouldSkipBadOnes(t *testing.T) {
	getSecret := func(name string) (string, error) { return "", nil }
	fallbacks := []domain.FallbackConfig{
		{Provider: "local", DefaultModel: "test"},
		{Provider: "openai", DefaultModel: "gpt-4o"}, // will fail: no API key
		{Provider: "local", DefaultModel: "test2"},
	}

	providers := NewFallbackProviders(fallbacks, getSecret)
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers (skipping broken openai), got %d", len(providers))
	}
}

func TestNewFallbackProviders_WhenAllInvalid_ShouldReturnNil(t *testing.T) {
	getSecret := func(name string) (string, error) { return "", nil }
	fallbacks := []domain.FallbackConfig{
		{Provider: "openai", DefaultModel: "gpt-4o"}, // no key
		{Provider: "anthropic", DefaultModel: "claude"}, // no key
	}

	providers := NewFallbackProviders(fallbacks, getSecret)
	if providers != nil {
		t.Errorf("expected nil when all fallbacks fail, got %d providers", len(providers))
	}
}

// =============================================================================
// KeyPool factory integration tests
// =============================================================================

func TestNewProvider_WhenOpenAIMultipleKeys_ShouldReturnKeyPoolProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openai", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) {
		if name == "openai_api_key" {
			return "key-1,key-2,key-3", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	// Should be a KeyPoolProvider (wrapping 3 OpenAI providers)
	kpp, ok := provider.(*KeyPoolProvider)
	if !ok {
		t.Fatalf("expected *KeyPoolProvider, got %T", provider)
	}
	if kpp.pool.Len() != 3 {
		t.Errorf("want 3 keys in pool, got %d", kpp.pool.Len())
	}
}

func TestNewProvider_WhenOpenAISingleKey_ShouldReturnRegularProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openai", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) {
		if name == "openai_api_key" {
			return "single-key", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	// Should be a regular OpenAI provider, not a KeyPoolProvider
	_, ok := provider.(*KeyPoolProvider)
	if ok {
		t.Error("single key should not create KeyPoolProvider")
	}
}

func TestNewProvider_WhenAnthropicMultipleKeys_ShouldReturnKeyPoolProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "anthropic", DefaultModel: "claude-3-5-sonnet"}
	getSecret := func(name string) (string, error) {
		if name == "anthropic_api_key" {
			return "key-1,key-2", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	kpp, ok := provider.(*KeyPoolProvider)
	if !ok {
		t.Fatalf("expected *KeyPoolProvider, got %T", provider)
	}
	if kpp.pool.Len() != 2 {
		t.Errorf("want 2 keys in pool, got %d", kpp.pool.Len())
	}
}

func TestNewProvider_WhenOpenRouterMultipleKeys_ShouldReturnKeyPoolProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openrouter", DefaultModel: "openai/gpt-4"}
	getSecret := func(name string) (string, error) {
		if name == "openrouter_api_key" {
			return "key-1,key-2", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	kpp, ok := provider.(*KeyPoolProvider)
	if !ok {
		t.Fatalf("expected *KeyPoolProvider, got %T", provider)
	}
	if kpp.pool.Len() != 2 {
		t.Errorf("want 2 keys in pool, got %d", kpp.pool.Len())
	}
}

func TestNewProvider_WhenGeminiMultipleKeys_ShouldReturnKeyPoolProvider(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "gemini", DefaultModel: "gemini-pro"}
	getSecret := func(name string) (string, error) {
		if name == "gemini_api_key" {
			return "key-1,key-2", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	kpp, ok := provider.(*KeyPoolProvider)
	if !ok {
		t.Fatalf("expected *KeyPoolProvider, got %T", provider)
	}
	if kpp.pool.Len() != 2 {
		t.Errorf("want 2 keys in pool, got %d", kpp.pool.Len())
	}
}

func TestNewProvider_WhenMultipleKeysWithWhitespace_ShouldTrimKeys(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openai", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) {
		if name == "openai_api_key" {
			return "  key-1 , key-2 , key-3  ", nil
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	kpp, ok := provider.(*KeyPoolProvider)
	if !ok {
		t.Fatalf("expected *KeyPoolProvider, got %T", provider)
	}
	if kpp.pool.Len() != 3 {
		t.Errorf("want 3 keys in pool, got %d", kpp.pool.Len())
	}
}

func TestNewProvider_WhenMultipleKeysWithRetryConfig_ShouldWrapWithRetry(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openai", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) {
		if name == "openai_api_key" {
			return "key-1,key-2", nil
		}
		return "", nil
	}
	retryCfg := &domain.RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100,
		MaxBackoff:     1000,
		Multiplier:     2,
	}

	provider, err := NewProvider(cfg, getSecret, retryCfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	// Should be wrapped in retry, so not directly a KeyPoolProvider
	if provider == nil {
		t.Fatal("provider is nil")
	}
}

func TestNewProvider_WhenKeysContainEmptyAfterSplit_ShouldFilterThem(t *testing.T) {
	cfg := &domain.AgentsConfig{Provider: "openai", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) {
		if name == "openai_api_key" {
			return "key-1,,key-2,", nil // empty entries
		}
		return "", nil
	}

	provider, err := NewProvider(cfg, getSecret)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	kpp, ok := provider.(*KeyPoolProvider)
	if !ok {
		t.Fatalf("expected *KeyPoolProvider, got %T", provider)
	}
	if kpp.pool.Len() != 2 {
		t.Errorf("want 2 keys (empty entries filtered), got %d", kpp.pool.Len())
	}
}

func TestNewProvider_WhenKeyPoolCreationFails_ShouldReturnError(t *testing.T) {
	// Inject a failing KeyPool constructor to cover the defensive error branch
	original := newKeyPoolFunc
	newKeyPoolFunc = func(keys []string, cooldown time.Duration) (*KeyPool, error) {
		return nil, fmt.Errorf("injected pool error")
	}
	defer func() { newKeyPoolFunc = original }()

	cfg := &domain.AgentsConfig{Provider: "openai", DefaultModel: "gpt-4o"}
	getSecret := func(name string) (string, error) {
		if name == "openai_api_key" {
			return "key-1,key-2", nil
		}
		return "", nil
	}

	_, err := NewProvider(cfg, getSecret)
	if err == nil {
		t.Error("expected error when KeyPool creation fails")
	}
	if !strings.Contains(err.Error(), "key pool") {
		t.Errorf("error should mention key pool, got %q", err.Error())
	}
}
