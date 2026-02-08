package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ironclaw/internal/brain"
	"ironclaw/internal/config"
	"ironclaw/internal/llm"
	"ironclaw/internal/memory"
	"ironclaw/internal/router"
	"ironclaw/internal/secrets"
	wa "ironclaw/internal/whatsapp"

	qrterminal "github.com/mdp/qrterminal/v3"
)

// exitFunc is the function used by main to exit; tests replace it to cover main().
var exitFunc = os.Exit

func main() {
	if err := run(); err != nil {
		log.Println(err)
		exitFunc(1)
	}
}

// newWAClientFn creates a WhatsApp client.
// The default is set in driver.go (libSQL, pure Go, no CGO required).
// Tests replace it entirely.
var newWAClientFn func(dbPath string) (wa.WAClient, error)

// startAdapterFn starts the adapter loop; tests replace it to avoid blocking.
var startAdapterFn = func(adapter *wa.Adapter, ctx context.Context) error {
	return adapter.Start(ctx)
}

// signalContextFn creates a context that cancels on OS signals; tests replace it.
var signalContextFn = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

// secretsManagerFn returns a secrets manager; tests replace it.
var secretsManagerFn = secrets.DefaultManager

// qrHandlerFn creates the QR code handler; tests replace it.
var qrHandlerFn = func() wa.QRHandler {
	return func(code string) {
		fmt.Println("\nScan this QR code with WhatsApp:")
		qrterminal.GenerateHalfBlock(code, qrterminal.L, os.Stdout)
		fmt.Println()
	}
}

func run() error {
	// 1. Determine WhatsApp database path.
	dbPath := os.Getenv("WHATSAPP_DB_PATH")
	if dbPath == "" {
		dbPath = "whatsapp.db"
	}

	// 2. Create the WhatsApp client.
	client, err := newWAClientFn(dbPath)
	if err != nil {
		return fmt.Errorf("whatsapp client init: %w", err)
	}

	// 3. Load config and build the brain.
	chatBrain, err := buildBrain()
	if err != nil {
		return fmt.Errorf("brain setup: %w", err)
	}

	// 4. Create router wrapping the brain.
	rt := router.NewRouter(chatBrain, nil)

	// 5. Create QR handler for terminal display.
	qrHandler := qrHandlerFn()

	// 6. Create and start the WhatsApp adapter.
	adapter := wa.NewAdapter(client, rt, qrHandler)

	ctx, cancel := signalContextFn()
	defer cancel()

	log.Println("WhatsApp bridge started. Press Ctrl+C to stop.")
	if startErr := startAdapterFn(adapter, ctx); startErr != nil {
		return fmt.Errorf("whatsapp bridge: %w", startErr)
	}
	log.Println("WhatsApp bridge stopped.")
	return nil
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
