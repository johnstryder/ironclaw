package secrets

// DefaultManager returns a SecretsManager for the default location (UserConfigDir/ironclaw/.secrets).
// Uses IRONCLAW_SECRETS_PASSPHRASE or machine-id (Linux) for the encryption key.
func DefaultManager() (SecretsManager, error) {
	path, err := DefaultSecretsPath()
	if err != nil {
		return nil, err
	}
	return NewFileManager(path)
}
