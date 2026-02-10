package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"ironclaw/internal/domain"
)

// OnboardOptions holds options for the onboard command.
type OnboardOptions struct {
	Workspace      string   // Path to workspace directory
	NonInteractive bool     // Skip interactive prompts
	GatewayPort    int      // Gateway port number
	GatewayAuth    string   // Gateway auth mode: "none", "token", "password"
	AuthToken      string   // Auth token for gateway
	DefaultModel   string   // Default AI model
	Provider       string   // AI provider: "openai", "anthropic", "local", "ollama"
	Skills         []string // List of skills to enable
}

// RunOnboard runs the onboard subcommand: comprehensive setup wizard.
// Returns exit code (0 for success, 1 for error).
func RunOnboard(opts OnboardOptions, stdout, stderr io.Writer) int {
	// Use default workspace if not specified
	if opts.Workspace == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "Error: could not determine home directory: %v\n", err)
			return 1
		}
		opts.Workspace = filepath.Join(homeDir, ".ironclaw")
	}

	// Create workspace directory
	if err := os.MkdirAll(opts.Workspace, 0755); err != nil {
		fmt.Fprintf(stderr, "Error: failed to create workspace directory: %v\n", err)
		return 1
	}

	// Create skills directory if skills are specified
	if len(opts.Skills) > 0 {
		skillsDir := filepath.Join(opts.Workspace, "skills")
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			fmt.Fprintf(stderr, "Error: failed to create skills directory: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Skills directory created: %s\n", skillsDir)
	}

	configPath := filepath.Join(opts.Workspace, "ironclaw.json")

	// Check if config already exists
	var cfg *domain.Config
	if _, err := os.Stat(configPath); err == nil {
		// Load existing config
		loadedCfg, err := configLoad(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "Error: failed to load existing config: %v\n", err)
			return 1
		}
		cfg = loadedCfg
		fmt.Fprintf(stdout, "Using existing workspace at %s\n", opts.Workspace)
	} else {
		// Create new default config
		if err := configWriteDefault(configPath); err != nil {
			fmt.Fprintf(stderr, "Error: failed to write default config: %v\n", err)
			return 1
		}
		loadedCfg, err := configLoad(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "Error: failed to load new config: %v\n", err)
			return 1
		}
		cfg = loadedCfg
		fmt.Fprintf(stdout, "Created new workspace at %s\n", opts.Workspace)
	}

	// Apply gateway configuration
	if opts.GatewayPort > 0 {
		cfg.Gateway.Port = opts.GatewayPort
		fmt.Fprintf(stdout, "Gateway port configured: %d\n", opts.GatewayPort)
	}
	if opts.GatewayAuth != "" {
		cfg.Gateway.Auth.Mode = opts.GatewayAuth
		fmt.Fprintf(stdout, "Gateway auth mode: %s\n", opts.GatewayAuth)
	}
	if opts.AuthToken != "" {
		cfg.Gateway.Auth.AuthToken = opts.AuthToken
	}

	// Apply model configuration
	if opts.Provider != "" {
		cfg.Agents.Provider = opts.Provider
		fmt.Fprintf(stdout, "AI provider: %s\n", opts.Provider)
	}
	if opts.DefaultModel != "" {
		cfg.Agents.DefaultModel = opts.DefaultModel
		fmt.Fprintf(stdout, "Default model: %s\n", opts.DefaultModel)
	}

	// Save updated config
	if err := configSave(configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "Error: failed to save config: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "\nOnboarding complete!\n")
	fmt.Fprintf(stdout, "Configuration saved to %s\n", configPath)
	fmt.Fprintf(stdout, "Run 'ironclaw' to start the daemon.\n")

	return 0
}
