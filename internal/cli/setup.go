package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SetupOptions holds options for the setup command.
type SetupOptions struct {
	Workspace      string // Path to workspace directory
	Wizard         bool   // Run interactive wizard
	NonInteractive bool   // Skip interactive prompts
	Mode           string // Setup mode: "local", "server", "remote"
	RemoteURL      string // Remote API URL
	RemoteToken    string // Remote authentication token
}

// RunSetup runs the setup subcommand: initializes config and workspace.
// Returns exit code (0 for success, 1 for error).
func RunSetup(opts SetupOptions, stdout, stderr io.Writer) int {
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

	configPath := filepath.Join(opts.Workspace, "ironclaw.json")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Fprintf(stdout, "Workspace already exists at %s\n", opts.Workspace)
		fmt.Fprintf(stdout, "Using existing configuration.\n")
		return 0
	}

	// Create default config
	if err := configWriteDefault(configPath); err != nil {
		fmt.Fprintf(stderr, "Error: failed to write default config: %v\n", err)
		return 1
	}

	// Load config and update with mode/remote settings
	cfg, err := configLoad(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: failed to load config for updates: %v\n", err)
		return 1
	}

	// Apply mode setting
	if opts.Mode != "" {
		cfg.Mode = opts.Mode
		fmt.Fprintf(stdout, "Setup mode: %s\n", opts.Mode)
	}

	// Apply remote configuration
	if opts.RemoteURL != "" {
		cfg.RemoteURL = opts.RemoteURL
		fmt.Fprintf(stdout, "Remote URL configured: %s\n", opts.RemoteURL)
	}
	if opts.RemoteToken != "" {
		cfg.RemoteToken = opts.RemoteToken
	}

	// Save updated config
	if err := configSave(configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "Error: failed to save config: %v\n", err)
		return 1
	}

	if opts.Wizard && !opts.NonInteractive {
		fmt.Fprintf(stdout, "Running interactive setup wizard...\n")
		// Note: Full wizard implementation would prompt for values here
	}

	fmt.Fprintf(stdout, "Ironclaw workspace initialized at %s\n", opts.Workspace)
	fmt.Fprintf(stdout, "Configuration written to %s\n", configPath)

	return 0
}
