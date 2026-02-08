package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ironclaw/internal/brain"
	"ironclaw/internal/config"
	"ironclaw/internal/llm"
	"ironclaw/internal/memory"
	"ironclaw/internal/router"
	"ironclaw/internal/secrets"
	"ironclaw/internal/telegram"
)

// exitFunc is the function used by main to exit; tests replace it to cover main().
var exitFunc = os.Exit

func main() {
	if err := run(); err != nil {
		log.Println(err)
		exitFunc(1)
	}
}

// newBotAPIFn creates a real Telegram BotAPI; tests replace it.
var newBotAPIFn = func(token string) (telegram.BotAPI, error) {
	return tgbotapi.NewBotAPI(token)
}

// startAdapterFn starts the adapter loop; tests replace it to avoid blocking.
var startAdapterFn = func(adapter *telegram.Adapter, ctx context.Context) {
	adapter.Start(ctx)
}

// signalContextFn creates a context that cancels on OS signals; tests replace it.
var signalContextFn = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

// secretsManagerFn returns a secrets manager; tests replace it.
var secretsManagerFn = secrets.DefaultManager

func run() error {
	// 1. Load Telegram bot token from secrets store or environment.
	token, err := loadToken()
	if err != nil {
		return fmt.Errorf("telegram token: %w\n\nSet TELEGRAM_BOT_TOKEN env var or run:\n  ironclaw secrets set telegram_bot_token YOUR_TOKEN", err)
	}

	// 2. Create the Telegram bot API.
	bot, err := newBotAPIFn(token)
	if err != nil {
		return fmt.Errorf("telegram bot init: %w", err)
	}

	// 3. Load config and build the brain.
	chatBrain, err := buildBrain()
	if err != nil {
		return fmt.Errorf("brain setup: %w", err)
	}

	// 4. Create router wrapping the brain.
	rt := router.NewRouter(chatBrain, nil)

	// 5. Create and start the Telegram adapter.
	adapter := telegram.NewAdapter(bot, rt)

	ctx, cancel := signalContextFn()
	defer cancel()

	log.Println("Telegram bridge started. Press Ctrl+C to stop.")
	startAdapterFn(adapter, ctx)
	log.Println("Telegram bridge stopped.")
	return nil
}

// loadToken retrieves the Telegram bot token from TELEGRAM_BOT_TOKEN env var or secrets store.
func loadToken() (string, error) {
	// Try environment variable first.
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		return token, nil
	}

	// Try secrets store.
	sm, err := secretsManagerFn()
	if err != nil {
		return "", fmt.Errorf("secrets manager: %w", err)
	}
	token, err := sm.Get("telegram_bot_token")
	if err != nil {
		return "", fmt.Errorf("secret 'telegram_bot_token' not found: %w", err)
	}
	return token, nil
}

// buildBrain creates a Brain from ironclaw config + secrets (same as the daemon).
func buildBrain() (*brain.Brain, error) {
	cfgPath := os.Getenv("IRONCLAW_CONFIG")
	if cfgPath == "" {
		cfgPath = "ironclaw.json"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("config load (%s): %w", cfgPath, err)
	}

	sm, err := secretsManagerFn()
	if err != nil {
		return nil, fmt.Errorf("secrets manager: %w", err)
	}

	provider, err := llm.NewProvider(&cfg.Agents, sm.Get, &cfg.Retry)
	if err != nil {
		return nil, fmt.Errorf("llm provider: %w", err)
	}

	var opts []brain.Option
	if cfg.Agents.Paths.Memory != "" {
		memStore := memory.NewFileMemoryStore(cfg.Agents.Paths.Memory)
		opts = append(opts, brain.WithMemory(memStore))
	}
	if len(cfg.Agents.Fallbacks) > 0 {
		fallbacks := llm.NewFallbackProviders(cfg.Agents.Fallbacks, sm.Get, &cfg.Retry)
		if len(fallbacks) > 0 {
			opts = append(opts, brain.WithFallbacks(fallbacks...))
		}
	}

	return brain.NewBrain(provider, opts...), nil
}
