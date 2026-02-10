package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"ironclaw/internal/config"
)

// ConfigureOptions holds options for the configure command.
type ConfigureOptions struct {
	Workspace      string   // Path to workspace directory
	NonInteractive bool     // Skip interactive prompts
	GatewayPort    int      // Gateway port number
	GatewayAuth    string   // Gateway auth mode: "none", "token", "password"
	DefaultModel   string   // Default AI model
	Provider       string   // AI provider: "openai", "anthropic", "local", "ollama"
	Channels       []string // List of channels to configure
	Skills         []string // List of skills to enable
}

// RunConfigure runs the configure subcommand: updates configuration interactively or non-interactively.
// Returns exit code (0 for success, 1 for error).
func RunConfigure(opts ConfigureOptions, stdout, stderr io.Writer) int {
	// Use default workspace if not specified
	if opts.Workspace == "" {
		homeDir, err := osUserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "Error: could not determine home directory: %v\n", err)
			return 1
		}
		opts.Workspace = filepath.Join(homeDir, ".ironclaw")
	}

	configPath := filepath.Join(opts.Workspace, "ironclaw.json")

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Fprintf(stderr, "Error: no existing configuration found at %s\n", configPath)
		fmt.Fprintf(stderr, "Run 'ironclaw setup' or 'ironclaw onboard' first to create a workspace.\n")
		return 1
	}

	// Load existing config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: failed to load config: %v\n", err)
		return 1
	}

	// Apply gateway configuration
	if opts.GatewayPort > 0 {
		cfg.Gateway.Port = opts.GatewayPort
		fmt.Fprintf(stdout, "Gateway port set to %d\n", opts.GatewayPort)
	}
	if opts.GatewayAuth != "" {
		cfg.Gateway.Auth.Mode = opts.GatewayAuth
		fmt.Fprintf(stdout, "Gateway auth mode set to %s\n", opts.GatewayAuth)
	}

	// Apply model configuration
	if opts.Provider != "" {
		cfg.Agents.Provider = opts.Provider
		fmt.Fprintf(stdout, "AI provider set to %s\n", opts.Provider)
	}
	if opts.DefaultModel != "" {
		cfg.Agents.DefaultModel = opts.DefaultModel
		fmt.Fprintf(stdout, "Default model set to %s\n", opts.DefaultModel)
	}

	// Apply channels configuration
	if len(opts.Channels) > 0 {
		cfg.Channels = opts.Channels
		fmt.Fprintf(stdout, "Channels configured: %v\n", opts.Channels)
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

	// Save updated config
	if err := config.Save(configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "Error: failed to save config: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "\nConfiguration updated successfully!\n")
	return 0
}
