package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ironclaw/internal/domain"
	"ironclaw/internal/secrets"
	"ironclaw/internal/telegram"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockBotAPI implements telegram.BotAPI for tests.
type mockBotAPI struct {
	updates chan tgbotapi.Update
}

func (m *mockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return tgbotapi.Message{}, nil
}

func (m *mockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return m.updates
}

func (m *mockBotAPI) StopReceivingUpdates() {}

// mockSecretsManager implements secrets.SecretsManager for tests.
type mockSecretsManager struct {
	store map[string]string
	err   error
}

func (m *mockSecretsManager) Get(key string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	v, ok := m.store[key]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return v, nil
}

func (m *mockSecretsManager) Set(key, value string) error { return nil }
func (m *mockSecretsManager) Delete(key string) error     { return nil }

// =============================================================================
// Default function variable tests
// =============================================================================

func TestDefaultNewBotAPIFn_WhenInvalidToken_ShouldReturnError(t *testing.T) {
	// Call the default production newBotAPIFn with an invalid token.
	_, err := newBotAPIFn("invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestDefaultStartAdapterFn_ShouldCallStart(t *testing.T) {
	bot := &mockBotAPI{updates: make(chan tgbotapi.Update, 1)}
	rtr := &mockRouter{response: "ok"}
	adapter := telegram.NewAdapter(bot, rtr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so Start returns right away

	// The default startAdapterFn just calls adapter.Start(ctx).
	startAdapterFn(adapter, ctx)
}

func TestDefaultSignalContextFn_ShouldReturnCancelableContext(t *testing.T) {
	ctx, cancel := signalContextFn()
	defer cancel()

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	// Cancel should work without panic.
	cancel()
}

// mockRouter for the startAdapterFn test.
type mockRouter struct {
	response string
}

func (m *mockRouter) Route(ctx context.Context, channelID, prompt string) (string, error) {
	return m.response, nil
}

// =============================================================================
// main tests
// =============================================================================

func TestMain_WhenRunFails_ShouldCallExitWithOne(t *testing.T) {
	oldExit := exitFunc
	oldNewBot := newBotAPIFn
	defer func() {
		exitFunc = oldExit
		newBotAPIFn = oldNewBot
	}()

	// Set token so loadToken succeeds, but make newBotAPIFn fail.
	t.Setenv("TELEGRAM_BOT_TOKEN", "bad-token")
	newBotAPIFn = func(token string) (telegram.BotAPI, error) {
		return nil, errors.New("invalid token")
	}

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	main()

	if exitCode != 1 {
		t.Errorf("want exit code 1, got %d", exitCode)
	}
}

// =============================================================================
// loadToken tests
// =============================================================================

func TestLoadToken_WhenEnvVarSet_ShouldReturnToken(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token-123")

	token, err := loadToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "test-token-123" {
		t.Errorf("want 'test-token-123', got %q", token)
	}
}

func TestLoadToken_WhenEnvVarEmpty_ShouldFallbackToSecrets(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")

	// Without a valid secrets store configured, this should error
	_, err := loadToken()
	if err == nil {
		t.Fatal("expected error when no token available")
	}
}

func TestLoadToken_WhenSecretsManagerFails_ShouldReturnError(t *testing.T) {
	oldSM := secretsManagerFn
	defer func() { secretsManagerFn = oldSM }()

	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	secretsManagerFn = func() (secrets.SecretsManager, error) {
		return nil, errors.New("keyring unavailable")
	}

	_, err := loadToken()
	if err == nil {
		t.Fatal("expected error when secrets manager fails")
	}
	if !strings.Contains(err.Error(), "secrets manager") {
		t.Errorf("error should mention secrets manager, got: %v", err)
	}
}

func TestLoadToken_WhenSecretExists_ShouldReturnTokenFromStore(t *testing.T) {
	oldSM := secretsManagerFn
	defer func() { secretsManagerFn = oldSM }()

	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	secretsManagerFn = func() (secrets.SecretsManager, error) {
		return &mockSecretsManager{
			store: map[string]string{"telegram_bot_token": "secret-token-456"},
		}, nil
	}

	token, err := loadToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "secret-token-456" {
		t.Errorf("want 'secret-token-456', got %q", token)
	}
}

func TestLoadToken_WhenSecretNotFound_ShouldReturnError(t *testing.T) {
	oldSM := secretsManagerFn
	defer func() { secretsManagerFn = oldSM }()

	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	secretsManagerFn = func() (secrets.SecretsManager, error) {
		return &mockSecretsManager{store: map[string]string{}}, nil
	}

	_, err := loadToken()
	if err == nil {
		t.Fatal("expected error when secret not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found, got: %v", err)
	}
}

// =============================================================================
// buildBrain tests
// =============================================================================

func TestBuildBrain_WhenConfigMissing_ShouldReturnError(t *testing.T) {
	t.Setenv("IRONCLAW_CONFIG", "/nonexistent/ironclaw.json")

	_, err := buildBrain()
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "config load") {
		t.Errorf("error should mention config load, got: %v", err)
	}
}

func TestBuildBrain_WhenLocalProvider_ShouldSucceed(t *testing.T) {
	// Write a minimal config with provider "local" (no API keys needed).
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test-config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
		Retry: domain.RetryConfig{
			MaxRetries:     0,
			InitialBackoff: 500,
			MaxBackoff:     5000,
			Multiplier:     2,
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)

	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	b, err := buildBrain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil brain")
	}
}

func TestBuildBrain_WhenMemoryPathSet_ShouldConfigureMemory(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0755)
	cfgPath := filepath.Join(dir, "test-config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: memDir},
		},
		Retry: domain.RetryConfig{
			MaxRetries:     0,
			InitialBackoff: 500,
			MaxBackoff:     5000,
			Multiplier:     2,
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)

	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	b, err := buildBrain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil brain")
	}
}

func TestBuildBrain_WhenSecretsManagerFails_ShouldReturnError(t *testing.T) {
	oldSM := secretsManagerFn
	defer func() { secretsManagerFn = oldSM }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	secretsManagerFn = func() (secrets.SecretsManager, error) {
		return nil, errors.New("keyring broken")
	}

	_, err := buildBrain()
	if err == nil {
		t.Fatal("expected error when secrets manager fails")
	}
	if !strings.Contains(err.Error(), "secrets manager") {
		t.Errorf("error should mention secrets manager, got: %v", err)
	}
}

func TestBuildBrain_WhenProviderNeedsApiKey_ShouldReturnError(t *testing.T) {
	oldSM := secretsManagerFn
	defer func() { secretsManagerFn = oldSM }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "anthropic", // needs an API key
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	// Mock secrets manager that has no api key
	secretsManagerFn = func() (secrets.SecretsManager, error) {
		return &mockSecretsManager{store: map[string]string{}}, nil
	}

	_, err := buildBrain()
	if err == nil {
		t.Fatal("expected error when provider needs API key")
	}
	if !strings.Contains(err.Error(), "llm provider") {
		t.Errorf("error should mention llm provider, got: %v", err)
	}
}

func TestBuildBrain_WhenFallbacksConfigured_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test-config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
			Fallbacks: []domain.FallbackConfig{
				{Provider: "local", DefaultModel: "fallback1"},
				{Provider: "local", DefaultModel: "fallback2"},
			},
		},
		Retry: domain.RetryConfig{
			MaxRetries:     0,
			InitialBackoff: 500,
			MaxBackoff:     5000,
			Multiplier:     2,
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	b, err := buildBrain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil brain with fallbacks")
	}
}

func TestBuildBrain_WhenFallbacksAllInvalid_ShouldStillSucceed(t *testing.T) {
	oldSM := secretsManagerFn
	defer func() { secretsManagerFn = oldSM }()

	secretsManagerFn = func() (secrets.SecretsManager, error) {
		return &mockSecretsManager{store: map[string]string{}}, nil
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test-config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
			Fallbacks: []domain.FallbackConfig{
				{Provider: "openai", DefaultModel: "gpt-4o"},       // no key -> skipped
				{Provider: "anthropic", DefaultModel: "claude-3.5"}, // no key -> skipped
			},
		},
		Retry: domain.RetryConfig{
			MaxRetries:     0,
			InitialBackoff: 500,
			MaxBackoff:     5000,
			Multiplier:     2,
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	b, err := buildBrain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil brain even with all invalid fallbacks")
	}
}

func TestBuildBrain_WhenDefaultConfigPath_ShouldUseIronclawJson(t *testing.T) {
	// Unset IRONCLAW_CONFIG to use default "ironclaw.json" which won't exist in temp dir.
	t.Setenv("IRONCLAW_CONFIG", "")

	// Change to a temp dir where ironclaw.json doesn't exist.
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(t.TempDir())

	_, err := buildBrain()
	if err == nil {
		t.Fatal("expected error for missing default config")
	}
}

// =============================================================================
// run tests
// =============================================================================

func TestRun_WhenNoToken_ShouldReturnError(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")

	err := run()
	if err == nil {
		t.Fatal("expected error when no token")
	}
	if !strings.Contains(err.Error(), "telegram token") {
		t.Errorf("error should mention telegram token, got: %v", err)
	}
}

func TestRun_WhenBotAPIFails_ShouldReturnError(t *testing.T) {
	oldNewBot := newBotAPIFn
	defer func() { newBotAPIFn = oldNewBot }()

	t.Setenv("TELEGRAM_BOT_TOKEN", "valid-token")
	newBotAPIFn = func(token string) (telegram.BotAPI, error) {
		return nil, errors.New("auth failed")
	}

	err := run()
	if err == nil {
		t.Fatal("expected error when bot init fails")
	}
	if !strings.Contains(err.Error(), "telegram bot init") {
		t.Errorf("error should mention bot init, got: %v", err)
	}
}

func TestRun_WhenBrainBuildFails_ShouldReturnError(t *testing.T) {
	oldNewBot := newBotAPIFn
	defer func() { newBotAPIFn = oldNewBot }()

	t.Setenv("TELEGRAM_BOT_TOKEN", "valid-token")
	t.Setenv("IRONCLAW_CONFIG", "/nonexistent/config.json")

	newBotAPIFn = func(token string) (telegram.BotAPI, error) {
		return &mockBotAPI{updates: make(chan tgbotapi.Update, 1)}, nil
	}

	err := run()
	if err == nil {
		t.Fatal("expected error when brain build fails")
	}
	if !strings.Contains(err.Error(), "brain setup") {
		t.Errorf("error should mention brain setup, got: %v", err)
	}
}

func TestRun_WhenAllDepsValid_ShouldStartAndReturn(t *testing.T) {
	oldNewBot := newBotAPIFn
	oldStart := startAdapterFn
	oldSignal := signalContextFn
	defer func() {
		newBotAPIFn = oldNewBot
		startAdapterFn = oldStart
		signalContextFn = oldSignal
	}()

	// Set up valid token and config.
	t.Setenv("TELEGRAM_BOT_TOKEN", "valid-token")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	// Mock the bot creation.
	newBotAPIFn = func(token string) (telegram.BotAPI, error) {
		return &mockBotAPI{updates: make(chan tgbotapi.Update, 1)}, nil
	}

	// Mock the signal context to return an already-canceled context.
	signalContextFn = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately so Start returns right away
		return ctx, cancel
	}

	// Mock startAdapterFn to be a no-op (avoid blocking).
	startAdapterFn = func(adapter *telegram.Adapter, ctx context.Context) {
		adapter.Start(ctx)
	}

	err := run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
