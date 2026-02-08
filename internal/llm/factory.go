package llm

import (
	"fmt"
	"strings"
	"time"

	"ironclaw/internal/domain"
	"ironclaw/internal/retry"
)

// defaultCooldownDuration is the time a rate-limited key stays in cooldown.
const defaultCooldownDuration = 60 * time.Second

// SecretGetter returns a secret by name (e.g. "openai_api_key"). Used to resolve API keys.
type SecretGetter func(name string) (string, error)

// NewProvider returns an LLMProvider for the given agents config, optionally wrapped with retry logic.
// Provider may be "local", "openai", "anthropic", "openrouter", "ollama", or "gemini". Empty provider defaults to "local".
// getSecret is used to resolve API keys for openai/anthropic/openrouter/gemini.
// retryCfg, if non-nil, wraps the provider with exponential-backoff retry on transient errors.
func NewProvider(agents *domain.AgentsConfig, getSecret SecretGetter, retryCfg ...*domain.RetryConfig) (domain.LLMProvider, error) {
	base, err := newBaseProvider(agents, getSecret)
	if err != nil {
		return nil, err
	}
	return wrapWithRetry(base, retryCfg...), nil
}

// newBaseProvider creates the raw LLM provider without retry wrapping.
// When a secret contains comma-separated keys, a KeyPoolProvider is created with
// round-robin rotation and 429-cooldown support.
func newBaseProvider(agents *domain.AgentsConfig, getSecret SecretGetter) (domain.LLMProvider, error) {
	if agents == nil {
		return NewLocalProvider("Local: "), nil
	}
	provider := agents.Provider
	if provider == "" {
		provider = "local"
	}
	switch provider {
	case "local":
		return NewLocalProvider("Local: "), nil
	case "openai":
		return resolveKeyedProvider("openai", "openai_api_key", getSecret, func(key string) domain.LLMProvider {
			return NewOpenAIProvider(key, agents.DefaultModel)
		})
	case "anthropic":
		return resolveKeyedProvider("anthropic", "anthropic_api_key", getSecret, func(key string) domain.LLMProvider {
			return NewAnthropicProvider(key, agents.DefaultModel)
		})
	case "openrouter":
		return resolveKeyedProvider("openrouter", "openrouter_api_key", getSecret, func(key string) domain.LLMProvider {
			return NewOpenRouterProvider(key, agents.DefaultModel)
		})
	case "ollama":
		return NewOllamaProvider(agents.DefaultModel), nil
	case "gemini":
		return resolveKeyedProvider("gemini", "gemini_api_key", getSecret, func(key string) domain.LLMProvider {
			return NewGeminiProvider(key, agents.DefaultModel)
		})
	default:
		return nil, fmt.Errorf("unknown LLM provider %q (use: local, openai, anthropic, openrouter, ollama, gemini)", provider)
	}
}

// splitKeys splits a raw secret value by commas, trims whitespace, and filters empty entries.
func splitKeys(raw string) []string {
	parts := strings.Split(raw, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	return keys
}

// newKeyPoolFunc is the KeyPool constructor. Package-level var for test injection.
var newKeyPoolFunc = NewKeyPool

// resolveKeyedProvider fetches the secret, splits it into one or more keys, and returns either
// a single provider (one key) or a KeyPoolProvider (multiple keys).
// providerName is used in error messages. secretName is the key to fetch from secrets.
// makeProvider is a factory function that creates a provider for a single API key.
func resolveKeyedProvider(providerName, secretName string, getSecret SecretGetter, makeProvider func(key string) domain.LLMProvider) (domain.LLMProvider, error) {
	raw, err := getSecret(secretName)
	if err != nil {
		return nil, err
	}
	keys := splitKeys(raw)
	if len(keys) == 0 {
		return nil, fmt.Errorf("%s provider: API key not set (store with: ironclaw secrets set %s <key>)", providerName, secretName)
	}
	if len(keys) == 1 {
		return makeProvider(keys[0]), nil
	}
	// Multiple keys: create a KeyPoolProvider
	pool, err := newKeyPoolFunc(keys, defaultCooldownDuration)
	if err != nil {
		return nil, fmt.Errorf("%s key pool: %w", providerName, err)
	}
	providers := make([]domain.LLMProvider, len(keys))
	for i, k := range keys {
		providers[i] = makeProvider(k)
	}
	return NewKeyPoolProvider(pool, providers)
}

// NewFallbackProviders creates LLM providers for each fallback config entry.
// Failed fallback configurations are silently skipped (logged but not fatal).
func NewFallbackProviders(fallbacks []domain.FallbackConfig, getSecret SecretGetter, retryCfg ...*domain.RetryConfig) []domain.LLMProvider {
	var providers []domain.LLMProvider
	for _, fb := range fallbacks {
		cfg := &domain.AgentsConfig{
			Provider:     fb.Provider,
			DefaultModel: fb.DefaultModel,
		}
		p, err := NewProvider(cfg, getSecret, retryCfg...)
		if err != nil {
			// Skip failed fallback configs — they are best-effort.
			continue
		}
		providers = append(providers, p)
	}
	return providers
}

// wrapWithRetry decorates a provider with retry logic when config is supplied.
func wrapWithRetry(provider domain.LLMProvider, retryCfg ...*domain.RetryConfig) domain.LLMProvider {
	if len(retryCfg) == 0 || retryCfg[0] == nil || retryCfg[0].MaxRetries <= 0 {
		return provider
	}
	rc := retryCfg[0]
	cfg := retry.Config{
		MaxRetries:     rc.MaxRetries,
		InitialBackoff: time.Duration(rc.InitialBackoff) * time.Millisecond,
		MaxBackoff:     time.Duration(rc.MaxBackoff) * time.Millisecond,
		Multiplier:     float64(rc.Multiplier),
	}
	return retry.NewRetryableProvider(provider, cfg)
}
