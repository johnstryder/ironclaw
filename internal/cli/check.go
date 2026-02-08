package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"ironclaw/internal/config"
)

const checkCmd = "check"

// CheckOptions holds options for the check command.
type CheckOptions struct {
	Fix bool // if true, write default config when missing
}

// RunCheck runs the check subcommand: checks config, paths, gateway; optionally repairs. Returns exit code.
func RunCheck(args []string, stdout, stderr io.Writer) int {
	opts := parseCheckOptions(args)
	cfgPath := "ironclaw.json"
	if p := os.Getenv("IRONCLAW_CONFIG"); p != "" {
		cfgPath = p
	}

	note := func(section, message string) {
		fmt.Fprintf(stdout, "  [%s] %s\n", section, message)
	}

	// 1. Config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			note("Config", fmt.Sprintf("No config at %s.", cfgPath))
			if opts.Fix {
				if writeErr := writeDefaultConfig(cfgPath); writeErr != nil {
					fmt.Fprintf(stderr, "  failed to write default config: %v\n", writeErr)
					return 1
				}
				note("Config", fmt.Sprintf("Wrote default config to %s.", cfgPath))
			} else {
				note("Config", "Run with --fix to create a default ironclaw.json.")
			}
		} else {
			note("Config", err.Error())
			return 1
		}
	} else {
		note("Config", fmt.Sprintf("Loaded %s.", cfgPath))

		// 2. Gateway
		note("Gateway", fmt.Sprintf("port=%d auth=%s", cfg.Gateway.Port, cfg.Gateway.Auth.Mode))
		if cfg.Gateway.Auth.Mode == "none" {
			note("Gateway", "Auth is disabled. Consider setting gateway.auth.mode to \"password\" or \"token\" for production.")
		}

		// 3. Paths
		root := cfg.Agents.Paths.Root
		mem := cfg.Agents.Paths.Memory
		if root != "" {
			if err := ensureDir(root, "agents.root"); err != nil {
				note("Paths", err.Error())
			} else {
				note("Paths", fmt.Sprintf("agents.root %s ok.", root))
			}
		}
		if mem != "" {
			if err := ensureDir(mem, "agents.memory"); err != nil {
				note("Paths", err.Error())
			} else {
				note("Paths", fmt.Sprintf("agents.memory %s ok.", mem))
			}
		}
	}

	fmt.Fprintln(stdout, "  Check complete.")
	return 0
}

func parseCheckOptions(args []string) CheckOptions {
	var opts CheckOptions
	for _, a := range args {
		if a == "--fix" || a == "-fix" {
			opts.Fix = true
			break
		}
	}
	return opts
}

func ensureDir(dir, label string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(abs, 0755); mkErr != nil {
				return fmt.Errorf("%s %q: mkdir failed: %w", label, abs, mkErr)
			}
			return nil
		}
		return fmt.Errorf("%s %q: %w", label, abs, err)
	}
	var notDirErr error
	if !info.IsDir() {
		notDirErr = fmt.Errorf("%s %q: not a directory", label, abs)
	}
	if notDirErr != nil {
		return notDirErr
	}
	return nil
}

func writeDefaultConfig(path string) error {
	return config.WriteDefault(path)
}
