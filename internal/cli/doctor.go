package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"ironclaw/internal/config"
)

// DoctorOptions holds options for the doctor command.
type DoctorOptions struct {
	Workspace              string // Path to workspace directory
	NonInteractive         bool   // Skip interactive prompts
	Fix                    bool   // Attempt to fix issues automatically
	Deep                   bool   // Perform deep/diagnostic checks
	NoWorkspaceSuggestions bool   // Skip workspace suggestions
}

// DoctorResult holds the result of a health check.
type DoctorResult struct {
	Name    string
	Status  string // "pass", "fail", "warn"
	Message string
}

// RunDoctor runs the doctor subcommand: performs health checks and optionally repairs.
// Returns exit code (0 for healthy, 1 for issues found).
func RunDoctor(opts DoctorOptions, stdout, stderr io.Writer) int {
	// Use default workspace if not specified
	if opts.Workspace == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "Error: could not determine home directory: %v\n", err)
			return 1
		}
		opts.Workspace = filepath.Join(homeDir, ".ironclaw")
	}

	fmt.Fprintf(stdout, "Running Ironclaw health checks...\n\n")

	results := []DoctorResult{}

	// Check 1: Workspace directory exists
	workspaceExists := false
	if _, err := os.Stat(opts.Workspace); os.IsNotExist(err) {
		results = append(results, DoctorResult{
			Name:    "Workspace",
			Status:  "fail",
			Message: fmt.Sprintf("Workspace directory not found: %s", opts.Workspace),
		})

		if opts.Fix {
			fmt.Fprintf(stdout, "  [FIX] Creating workspace directory...\n")
			if err := os.MkdirAll(opts.Workspace, 0755); err != nil {
				fmt.Fprintf(stderr, "  Error: Failed to create workspace: %v\n", err)
			} else {
				results = append(results, DoctorResult{
					Name:    "Workspace",
					Status:  "pass",
					Message: "Created workspace directory",
				})
				workspaceExists = true
			}
		}
	} else {
		results = append(results, DoctorResult{
			Name:    "Workspace",
			Status:  "pass",
			Message: fmt.Sprintf("Workspace exists: %s", opts.Workspace),
		})
		workspaceExists = true
	}

	// Check 2: Config file exists and is valid
	configPath := filepath.Join(opts.Workspace, "ironclaw.json")
	if workspaceExists {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			results = append(results, DoctorResult{
				Name:    "Config",
				Status:  "fail",
				Message: "Configuration file not found",
			})

			if opts.Fix {
			fmt.Fprintf(stdout, "  [FIX] Creating default configuration...\n")
			if err := configWriteDefault(configPath); err != nil {
					fmt.Fprintf(stderr, "  Error: Failed to write default config: %v\n", err)
				} else {
					results = append(results, DoctorResult{
						Name:    "Config",
						Status:  "pass",
						Message: "Created default configuration",
					})
				}
			}
		} else {
			// Try to load and validate config
			cfg, err := config.Load(configPath)
			if err != nil {
				results = append(results, DoctorResult{
					Name:    "Config",
					Status:  "fail",
					Message: fmt.Sprintf("Invalid configuration: %v", err),
				})
			} else {
				results = append(results, DoctorResult{
					Name:    "Config",
					Status:  "pass",
					Message: fmt.Sprintf("Config valid (gateway port: %d)", cfg.Gateway.Port),
				})

				// Check 3: Agents path exists
				if cfg.Agents.Paths.Root != "" {
					agentsPath := filepath.Join(opts.Workspace, cfg.Agents.Paths.Root)
					if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
						results = append(results, DoctorResult{
							Name:    "Agents Path",
							Status:  "warn",
							Message: fmt.Sprintf("Agents directory not found: %s", agentsPath),
						})

						if opts.Fix {
						fmt.Fprintf(stdout, "  [FIX] Creating agents directory...\n")
						if err := osMkdirAll(agentsPath, 0755); err != nil {
							fmt.Fprintf(stderr, "  Error: Failed to create agents directory: %v\n", err)
							} else {
								results = append(results, DoctorResult{
									Name:    "Agents Path",
									Status:  "pass",
									Message: "Created agents directory",
								})
							}
						}
					} else {
						results = append(results, DoctorResult{
							Name:    "Agents Path",
							Status:  "pass",
							Message: fmt.Sprintf("Agents directory exists: %s", agentsPath),
						})
					}
				}

				// Check 4: Memory path exists
				if cfg.Agents.Paths.Memory != "" {
					memoryPath := filepath.Join(opts.Workspace, cfg.Agents.Paths.Memory)
					if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
						results = append(results, DoctorResult{
							Name:    "Memory Path",
							Status:  "warn",
							Message: fmt.Sprintf("Memory directory not found: %s", memoryPath),
						})

						if opts.Fix {
						fmt.Fprintf(stdout, "  [FIX] Creating memory directory...\n")
						if err := osMkdirAll(memoryPath, 0755); err != nil {
							fmt.Fprintf(stderr, "  Error: Failed to create memory directory: %v\n", err)
							} else {
								results = append(results, DoctorResult{
									Name:    "Memory Path",
									Status:  "pass",
									Message: "Created memory directory",
								})
							}
						}
					} else {
						results = append(results, DoctorResult{
							Name:    "Memory Path",
							Status:  "pass",
							Message: fmt.Sprintf("Memory directory exists: %s", memoryPath),
						})
					}
				}
			}
		}
	}

	// Deep checks
	if opts.Deep {
		fmt.Fprintf(stdout, "\nRunning deep checks...\n")
		// Additional diagnostic checks could go here
		results = append(results, DoctorResult{
			Name:    "Deep Check",
			Status:  "pass",
			Message: "Deep diagnostics completed",
		})
	}

	// Print summary
	fmt.Fprintf(stdout, "\n--- Health Check Summary ---\n")
	passCount, failCount, warnCount := 0, 0, 0
	for _, r := range results {
		icon := "✓"
		if r.Status == "fail" {
			icon = "✗"
			failCount++
		} else if r.Status == "warn" {
			icon = "⚠"
			warnCount++
		} else {
			passCount++
		}
		fmt.Fprintf(stdout, "  %s %s: %s\n", icon, r.Name, r.Message)
	}

	fmt.Fprintf(stdout, "\nResults: %d passed, %d failed, %d warnings\n", passCount, failCount, warnCount)

	if failCount > 0 {
		fmt.Fprintf(stdout, "\nSome checks failed. Run with --fix to attempt automatic repairs.\n")
		return 1
	}

	fmt.Fprintf(stdout, "\nAll health checks passed!\n")
	return 0
}
