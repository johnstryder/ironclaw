package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const nonceSizeGCM = 12

// defaultKeySource is used by NewFileManager; tests may replace to force errors.
var defaultKeySource = DefaultKeySource

// fileWriteFile is used by writeMap; tests may replace to force errors.
var fileWriteFile = os.WriteFile

// fileMarshal is used by writeMap; tests may replace to force errors.
var fileMarshal = json.Marshal

// fileRandReader is used by writeMap for nonce; tests may replace to force errors.
var fileRandReader io.Reader = rand.Reader

// fileCipherNewGCM is used by Get and writeMap; tests may replace to force errors.
var fileCipherNewGCM = cipher.NewGCM

// NewFileManager returns a SecretsManager that stores secrets in an AES-GCM encrypted file.
// The key is obtained from DefaultKeySource (passphrase env or machine-id).
func NewFileManager(path string) (SecretsManager, error) {
	key, err := defaultKeySource()
	if err != nil {
		return nil, err
	}
	return NewFileManagerWithKey(path, key)
}

// NewFileManagerWithKey returns a SecretsManager with an explicit 32-byte key (for tests).
func NewFileManagerWithKey(path string, key []byte) (SecretsManager, error) {
	if len(key) != 32 {
		return nil, errors.New("secrets: key must be 32 bytes")
	}
	return &fileManager{path: path, key: key}, nil
}

type fileManager struct {
	path string
	key  []byte
}

func (f *fileManager) Get(key string) (string, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("secrets read: %w", err)
	}
	if len(data) < nonceSizeGCM {
		return "", fmt.Errorf("secrets file truncated")
	}
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return "", err
	}
	gcm, err := fileCipherNewGCM(block)
	if err != nil {
		return "", err
	}
	nonce, ciphertext := data[:nonceSizeGCM], data[nonceSizeGCM:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("secrets decrypt: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(plain, &m); err != nil {
		return "", fmt.Errorf("secrets parse: %w", err)
	}
	v, ok := m[key]
	if !ok || v == "" {
		return "", ErrNotFound
	}
	return v, nil
}

func (f *fileManager) Set(key, value string) error {
	m := make(map[string]string)
	data, err := os.ReadFile(f.path)
	if err == nil {
		if len(data) >= nonceSizeGCM {
			block, _ := aes.NewCipher(f.key)
			gcm, _ := cipher.NewGCM(block)
			nonce, ciphertext := data[:nonceSizeGCM], data[nonceSizeGCM:]
			plain, err := gcm.Open(nil, nonce, ciphertext, nil)
			if err == nil {
				_ = json.Unmarshal(plain, &m)
			}
		}
	}
	m[key] = value
	return f.writeMap(m)
}

func (f *fileManager) Delete(key string) error {
	m := make(map[string]string)
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("secrets read: %w", err)
	}
	if len(data) < nonceSizeGCM {
		return f.writeEmptyMap()
	}
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return err
	}
	gcm, err := fileCipherNewGCM(block)
	if err != nil {
		return err
	}
	nonce, ciphertext := data[:nonceSizeGCM], data[nonceSizeGCM:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		e := f.deleteDecryptFail()
		return e
	}
	_ = json.Unmarshal(plain, &m)
	delete(m, key)
	return f.writeMap(m)
}

func (f *fileManager) writeEmptyMap() error {
	return f.writeMap(map[string]string{})
}

func (f *fileManager) deleteDecryptFail() error {
	return f.writeEmptyMap()
}

func (f *fileManager) writeMap(m map[string]string) error {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("secrets mkdir: %w", err)
	}
	plain, err := fileMarshal(m)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return err
	}
	gcm, err := fileCipherNewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, nonceSizeGCM)
	if _, err := io.ReadFull(fileRandReader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nonce, nonce, plain, nil)
	return fileWriteFile(f.path, ciphertext, 0600)
}
