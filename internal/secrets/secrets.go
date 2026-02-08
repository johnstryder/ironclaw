package secrets

import "errors"

// SecretsManager stores and retrieves secrets (e.g. API keys) without writing them to config.
// Implementations use the OS keyring or an encrypted file; secrets are never stored in plain text.
type SecretsManager interface {
	// Get returns the secret for the given key (e.g. "openai", "anthropic"). Returns ErrNotFound if missing.
	Get(key string) (string, error)
	// Set stores the secret for the given key. Overwrites if the key already exists.
	Set(key, value string) error
	// Delete removes the secret for the given key. No error if the key did not exist.
	Delete(key string) error
}

// ErrNotFound is returned when a secret is not found.
var ErrNotFound = errors.New("secret not found")
