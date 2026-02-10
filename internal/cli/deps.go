package cli

import (
	"os"

	"ironclaw/internal/config"
	"ironclaw/internal/domain"
)

// Function variables for dependency injection in tests.
// Default values are the real implementations; tests may temporarily swap them.
var (
	osUserHomeDir      = os.UserHomeDir
	osMkdirAll         = os.MkdirAll
	configWriteDefault = config.WriteDefault
	configLoad         = config.Load
	configSave         = func(path string, cfg *domain.Config) error { return config.Save(path, cfg) }
	setValueAtPathFn   = setValueAtPath
)
