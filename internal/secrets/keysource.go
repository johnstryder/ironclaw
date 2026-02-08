package secrets

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Hooks for tests.
var (
	keySourceReadFile      = os.ReadFile
	keySourceUserConfigDir = os.UserConfigDir
	keySourceMkdirAll      = os.MkdirAll
)

// DefaultKeySource returns a 32-byte key from IRONCLAW_SECRETS_PASSPHRASE or, on Linux, /etc/machine-id.
// Callers must not modify the returned slice.
func DefaultKeySource() ([]byte, error) {
	if s := os.Getenv("IRONCLAW_SECRETS_PASSPHRASE"); s != "" {
		return deriveKey(s), nil
	}
	const machineIDPath = "/etc/machine-id"
	b, err := keySourceReadFile(machineIDPath)
	if err != nil {
		return nil, fmt.Errorf("secrets: set IRONCLAW_SECRETS_PASSPHRASE or ensure %s exists: %w", machineIDPath, err)
	}
	// Use first line; machine-id is often one line
	for i, c := range b {
		if c == '\n' || c == '\r' {
			b = b[:i]
			break
		}
	}
	if len(b) == 0 {
		return nil, errors.New("secrets: machine-id is empty")
	}
	return deriveKey(string(b)), nil
}

// DeriveKeyFromPassphrase returns a 32-byte key from a passphrase (e.g. for tests).
// Do not use for production when a proper KDF is preferred; this is acceptable for file secrets with a strong passphrase.
func DeriveKeyFromPassphrase(passphrase string) []byte {
	return deriveKey(passphrase)
}

func deriveKey(input string) []byte {
	const salt = "ironclaw-secrets-v1"
	h := sha256.Sum256([]byte(salt + input))
	return h[:]
}

// SecretsDir returns the directory for the .secrets file (same base as prefs: UserConfigDir/ironclaw).
func SecretsDir() (string, error) {
	base, err := keySourceUserConfigDir()
	if err != nil {
		return "", fmt.Errorf("secrets dir: %w", err)
	}
	dir := filepath.Join(base, "ironclaw")
	if err := keySourceMkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("secrets dir mkdir: %w", err)
	}
	return dir, nil
}

// DefaultSecretsPath returns the path to the default .secrets file.
func DefaultSecretsPath() (string, error) {
	dir, err := SecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".secrets"), nil
}
