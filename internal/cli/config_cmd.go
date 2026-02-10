package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfigOptions holds options for the config command.
type ConfigOptions struct {
	Workspace string // Path to workspace directory
	Action    string // "get", "set", or "unset"
	Path      string // Config path using dot notation (e.g., "gateway.port")
	Value     string // Value to set (for set action)
}

// RunConfig runs the config subcommand: non-interactive get/set/unset.
// Returns exit code (0 for success, 1 for error).
func RunConfig(opts ConfigOptions, stdout, stderr io.Writer) int {
	// Use default workspace if not specified
	if opts.Workspace == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "Error: could not determine home directory: %v\n", err)
			return 1
		}
		opts.Workspace = filepath.Join(homeDir, ".ironclaw")
	}

	configPath := filepath.Join(opts.Workspace, "ironclaw.json")

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Fprintf(stderr, "Error: no configuration found at %s\n", configPath)
		fmt.Fprintf(stderr, "Run 'ironclaw setup' first to create a workspace.\n")
		return 1
	}

	// Load config as generic map for path-based access
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: failed to read config: %v\n", err)
		return 1
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(stderr, "Error: failed to parse config: %v\n", err)
		return 1
	}

	switch opts.Action {
	case "get":
		return runConfigGet(cfg, opts.Path, stdout, stderr)
	case "set":
		return runConfigSet(&cfg, opts.Path, opts.Value, configPath, stdout, stderr)
	case "unset":
		return runConfigUnset(&cfg, opts.Path, configPath, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Error: unknown action %q (use 'get', 'set', or 'unset')\n", opts.Action)
		return 1
	}
}

// runConfigGet retrieves a value from the config using dot notation path.
func runConfigGet(cfg map[string]interface{}, path string, stdout, stderr io.Writer) int {
	parts := strings.Split(path, ".")
	value := getValueAtPath(cfg, parts)

	if value == nil {
		fmt.Fprintf(stderr, "Error: path %q not found in config\n", path)
		return 1
	}

	// Print the value based on its type
	switch v := value.(type) {
	case string:
		fmt.Fprintln(stdout, v)
	case float64:
		// Check if it's an integer
		if v == float64(int64(v)) {
			fmt.Fprintf(stdout, "%d\n", int64(v))
		} else {
			fmt.Fprintf(stdout, "%g\n", v)
		}
	case bool:
		fmt.Fprintf(stdout, "%t\n", v)
	default:
		// For complex types, print as JSON
		jsonBytes, _ := json.Marshal(v)
		fmt.Fprintln(stdout, string(jsonBytes))
	}

	return 0
}

// runConfigSet sets a value in the config using dot notation path.
func runConfigSet(cfg *map[string]interface{}, path, value, configPath string, stdout, stderr io.Writer) int {
	parts := strings.Split(path, ".")

	// Try to parse the value as number or bool, otherwise keep as string
	var parsedValue interface{} = value
	if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
		parsedValue = float64(intVal)
	} else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		parsedValue = floatVal
	} else if boolVal, err := strconv.ParseBool(value); err == nil {
		parsedValue = boolVal
	}

	if err := setValueAtPathFn(*cfg, parts, parsedValue); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// Save the updated config
	if err := saveConfig(configPath, *cfg); err != nil {
		fmt.Fprintf(stderr, "Error: failed to save config: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "ok\n")
	return 0
}

// runConfigUnset removes a value from the config using dot notation path.
func runConfigUnset(cfg *map[string]interface{}, path, configPath string, stdout, stderr io.Writer) int {
	parts := strings.Split(path, ".")

	if err := unsetValueAtPath(*cfg, parts); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// Save the updated config
	if err := saveConfig(configPath, *cfg); err != nil {
		fmt.Fprintf(stderr, "Error: failed to save config: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "ok\n")
	return 0
}

// getValueAtPath retrieves a value from a nested map using a path.
func getValueAtPath(data map[string]interface{}, path []string) interface{} {
	if len(path) == 0 {
		return nil
	}

	value, exists := data[path[0]]
	if !exists {
		return nil
	}

	if len(path) == 1 {
		return value
	}

	// Continue traversing
	nextMap, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	return getValueAtPath(nextMap, path[1:])
}

// setValueAtPath sets a value in a nested map using a path.
func setValueAtPath(data map[string]interface{}, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}

	if len(path) == 1 {
		data[path[0]] = value
		return nil
	}

	// Ensure the next level exists
	nextValue, exists := data[path[0]]
	if !exists {
		// Create the nested map
		data[path[0]] = make(map[string]interface{})
		nextValue = data[path[0]]
	}

	nextMap, ok := nextValue.(map[string]interface{})
	if !ok {
		// Convert to map if it's not already
		data[path[0]] = make(map[string]interface{})
		nextMap = data[path[0]].(map[string]interface{})
	}

	return setValueAtPath(nextMap, path[1:], value)
}

// unsetValueAtPath removes a value from a nested map using a path.
func unsetValueAtPath(data map[string]interface{}, path []string) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}

	if len(path) == 1 {
		delete(data, path[0])
		return nil
	}

	// Continue traversing
	nextValue, exists := data[path[0]]
	if !exists {
		return fmt.Errorf("path %q not found", strings.Join(path, "."))
	}

	nextMap, ok := nextValue.(map[string]interface{})
	if !ok {
		return fmt.Errorf("path %q is not an object", strings.Join(path[:len(path)-1], "."))
	}

	return unsetValueAtPath(nextMap, path[1:])
}

// saveConfig writes the config map back to a file.
func saveConfig(path string, cfg map[string]interface{}) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
