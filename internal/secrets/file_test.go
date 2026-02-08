package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const testService = "ironclaw"

func TestFileManager_SetThenGet_ShouldReturnStoredValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	err = m.Set("openai", "sk-secret123")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := m.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "sk-secret123" {
		t.Errorf("Get: want sk-secret123, got %q", got)
	}
}

func TestFileManager_WhenKeyMissing_GetShouldReturnErrNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	_, err = m.Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Get: want ErrNotFound, got %v", err)
	}
}

func TestFileManager_NewFileManagerWithKey_WhenKeyWrongLength_ShouldError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	_, err := NewFileManagerWithKey(path, []byte("short"))
	if err == nil {
		t.Error("NewFileManagerWithKey: expected error for key length != 32")
	}
}

func TestFileManager_AfterSet_FileShouldNotContainPlainText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	secret := "sk-my-secret-api-key"
	err = m.Set("openai", secret)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("secrets file should not be empty after Set")
	}
	if bytesContain(data, []byte(secret)) {
		t.Error("secrets file must not contain the secret in plain text")
	}
}

func TestFileManager_Get_WhenKeyInvalidLength_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	// File exists with enough bytes so Get tries to decrypt; key length 33 makes aes.NewCipher fail in Get.
	fm := &fileManager{path: path, key: make([]byte, 33)}
	if err := os.WriteFile(path, make([]byte, 20), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := fm.Get("k")
	if err == nil {
		t.Fatal("Get with invalid key length: expected error")
	}
}

func TestFileManager_Get_WhenCiphertextCorrupt_ShouldReturnDecryptError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	if err := m.Set("openai", "sk-secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Corrupt ciphertext (after 12-byte nonce) so gcm.Open fails
	if len(data) > 13 {
		data[13] ^= 0xff
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err = m.Get("openai")
	if err == nil {
		t.Fatal("Get with corrupt ciphertext: expected error")
	}
	if !bytesContain([]byte(err.Error()), []byte("decrypt")) {
		t.Errorf("error should mention decrypt: %v", err)
	}
}

func TestFileManager_Get_WhenNewGCMFails_ShouldReturnError(t *testing.T) {
	prev := fileCipherNewGCM
	defer func() { fileCipherNewGCM = prev }()
	fileCipherNewGCM = func(cipher.Block) (cipher.AEAD, error) {
		return nil, fmt.Errorf("injected NewGCM error")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	fm := &fileManager{path: path, key: key}
	if err := os.WriteFile(path, make([]byte, 20), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := fm.Get("k")
	if err == nil {
		t.Fatal("Get when NewGCM fails: expected error")
	}
	if !bytesContain([]byte(err.Error()), []byte("injected")) {
		t.Errorf("error should mention injected: %v", err)
	}
}

func TestFileManager_AfterRestart_GetShouldReturnStoredValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m1, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	err = m1.Set("openai", "sk-persisted")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	// Simulate restart: new manager instance, same path and key
	m2, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey (restart): %v", err)
	}
	got, err := m2.Get("openai")
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if got != "sk-persisted" {
		t.Errorf("Get after restart: want sk-persisted, got %q", got)
	}
}

func TestFileManager_Set_WhenFileAlreadyExists_ShouldMergeAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	if err := m.Set("openai", "sk-first"); err != nil {
		t.Fatalf("Set openai: %v", err)
	}
	// Set second key: should read existing file, merge, write (covers read+decrypt+merge path)
	if err := m.Set("anthropic", "sk-second"); err != nil {
		t.Fatalf("Set anthropic: %v", err)
	}
	// Overwrite first key
	if err := m.Set("openai", "sk-overwrite"); err != nil {
		t.Fatalf("Set openai overwrite: %v", err)
	}
	got1, err := m.Get("openai")
	if err != nil || got1 != "sk-overwrite" {
		t.Errorf("Get openai: want sk-overwrite, got %q err=%v", got1, err)
	}
	got2, err := m.Get("anthropic")
	if err != nil || got2 != "sk-second" {
		t.Errorf("Get anthropic: want sk-second, got %q err=%v", got2, err)
	}
}

func TestFileManager_Get_WhenKeyLengthInvalid_ShouldReturnError(t *testing.T) {
	// Get uses aes.NewCipher(f.key); with key length != 16/24/32 it returns error.
	path := filepath.Join(t.TempDir(), ".secrets")
	fm := &fileManager{path: path, key: make([]byte, 31)}
	_, err := fm.Get("k")
	if err == nil {
		t.Fatal("Get with 31-byte key (invalid for AES): expected error")
	}
}

func TestFileManager_Get_WhenFileTruncated_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	if err := os.WriteFile(path, []byte("short"), 0600); err != nil {
		t.Fatal(err)
	}
	m, _ := NewFileManagerWithKey(path, key)
	_, err := m.Get("any")
	if err == nil {
		t.Fatal("Get truncated file: expected error")
	}
	if !bytesContain([]byte(err.Error()), []byte("truncated")) {
		t.Errorf("error should mention truncated: %v", err)
	}
}

func TestFileManager_Get_WhenDecryptFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key1 := DeriveKeyFromPassphrase("pass1")
	key2 := DeriveKeyFromPassphrase("pass2")
	m1, _ := NewFileManagerWithKey(path, key1)
	if err := m1.Set("k", "v"); err != nil {
		t.Fatal(err)
	}
	m2, _ := NewFileManagerWithKey(path, key2)
	_, err := m2.Get("k")
	if err == nil {
		t.Fatal("Get with wrong key: expected error")
	}
	if !bytesContain([]byte(err.Error()), []byte("decrypt")) {
		t.Errorf("error should mention decrypt: %v", err)
	}
}

func TestFileManager_Delete_WhenFileNotExist_ShouldReturnNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, _ := NewFileManagerWithKey(path, key)
	if err := m.Delete("any"); err != nil {
		t.Errorf("Delete when file not exist: want nil, got %v", err)
	}
}

func TestFileManager_Delete_ShouldRemoveKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	_ = m.Set("openai", "sk-x")
	err = m.Delete("openai")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = m.Get("openai")
	if err != ErrNotFound {
		t.Errorf("Get after Delete: want ErrNotFound, got %v", err)
	}
}

func TestFileManager_Get_WhenKeyExistsButValueEmpty_ShouldReturnErrNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	if err := m.Set("emptykey", ""); err != nil {
		t.Fatalf("Set empty value: %v", err)
	}
	_, err = m.Get("emptykey")
	if err != ErrNotFound {
		t.Errorf("Get key with empty value: want ErrNotFound, got %v", err)
	}
}

func TestFileManager_Get_WhenFileReadFailsWithNonNotExist_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Skip("chmod 0000 not supported")
	}
	defer os.Chmod(path, 0644)
	m, _ := NewFileManagerWithKey(path, key)
	_, err := m.Get("any")
	if err == nil {
		t.Fatal("Get when file unreadable: expected error")
	}
	if err == ErrNotFound {
		t.Error("expected read error, not ErrNotFound")
	}
}

func TestFileManager_Get_WhenPlainIsInvalidJSON_ShouldReturnParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	// Write valid GCM structure but plaintext is not JSON
	plain := []byte("not json")
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, nonceSizeGCM)
	copy(nonce, "123456789012")
	ciphertext := gcm.Seal(nil, nonce, plain, nil)
	if err := os.WriteFile(path, append(nonce, ciphertext...), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = m.Get("k")
	if err == nil {
		t.Fatal("Get when plain invalid JSON: expected error")
	}
	if !bytesContain([]byte(err.Error()), []byte("parse")) {
		t.Errorf("error should mention parse: %v", err)
	}
}

func TestFileManager_Delete_WhenFileTruncated_ShouldWriteEmptyMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	if err := os.WriteFile(path, []byte("tooshort"), 0600); err != nil {
		t.Fatal(err)
	}
	m, _ := NewFileManagerWithKey(path, key)
	if err := m.Delete("any"); err != nil {
		t.Errorf("Delete truncated file: want nil, got %v", err)
	}
}

func TestFileManager_Delete_WhenDecryptFails_ShouldWriteEmptyMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key1 := DeriveKeyFromPassphrase("pass1")
	key2 := DeriveKeyFromPassphrase("pass2")
	m1, _ := NewFileManagerWithKey(path, key1)
	if err := m1.Set("k", "v"); err != nil {
		t.Fatal(err)
	}
	m2, _ := NewFileManagerWithKey(path, key2)
	if err := m2.Delete("k"); err != nil {
		t.Errorf("Delete with wrong key (decrypt fails): want nil, got %v", err)
	}
}

func TestFileManager_Delete_WhenCiphertextIsGarbage_ShouldWriteEmptyMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	fm := &fileManager{path: path, key: key}
	// Write exactly nonceSizeGCM+1 bytes of garbage so len >= nonceSizeGCM but gcm.Open will fail
	if err := os.WriteFile(path, make([]byte, nonceSizeGCM+1), 0600); err != nil {
		t.Fatal(err)
	}
	if err := fm.Delete("k"); err != nil {
		t.Errorf("Delete when ciphertext is garbage (decrypt fails): want nil, got %v", err)
	}
}

func TestFileManager_Delete_WhenNewGCMFails_ShouldReturnError(t *testing.T) {
	prev := fileCipherNewGCM
	defer func() { fileCipherNewGCM = prev }()
	fileCipherNewGCM = func(cipher.Block) (cipher.AEAD, error) {
		return nil, fmt.Errorf("injected NewGCM error")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	fm := &fileManager{path: path, key: key}
	if err := os.WriteFile(path, make([]byte, 20), 0600); err != nil {
		t.Fatal(err)
	}
	err := fm.Delete("k")
	if err == nil {
		t.Fatal("Delete when NewGCM fails: expected error")
	}
	if !bytesContain([]byte(err.Error()), []byte("injected")) {
		t.Errorf("error should mention injected: %v", err)
	}
}

func TestFileManager_Delete_WhenFileExistsButUnreadable_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, _ := NewFileManagerWithKey(path, key)
	if err := m.Set("k", "v"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Skip("chmod 0000 not supported")
	}
	defer os.Chmod(path, 0600)
	err := m.Delete("k")
	if err == nil {
		t.Fatal("Delete when file unreadable: expected error")
	}
}

func TestFileManager_writeMap_WhenParentDirIsFile_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	fileAsParent := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fileAsParent, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, _ := NewFileManagerWithKey(path, key)
	err := m.Set("k", "v")
	if err == nil {
		t.Fatal("Set when parent is file: expected error")
	}
	if !bytesContain([]byte(err.Error()), []byte("mkdir")) {
		t.Errorf("error should mention mkdir: %v", err)
	}
}

func TestNewFileManager_WhenDefaultKeySourceFails_ShouldReturnError(t *testing.T) {
	prev := defaultKeySource
	defer func() { defaultKeySource = prev }()
	defaultKeySource = func() ([]byte, error) {
		return nil, fmt.Errorf("injected key source error")
	}
	_, err := NewFileManager(filepath.Join(t.TempDir(), ".secrets"))
	if err == nil {
		t.Fatal("NewFileManager when defaultKeySource fails: expected error")
	}
}

func TestFileManager_writeMap_WhenWriteFileFails_ShouldReturnError(t *testing.T) {
	prev := fileWriteFile
	defer func() { fileWriteFile = prev }()
	fileWriteFile = func(string, []byte, os.FileMode) error {
		return fmt.Errorf("injected write error")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, _ := NewFileManagerWithKey(path, key)
	err := m.Set("k", "v")
	if err == nil {
		t.Fatal("Set when writeFile fails: expected error")
	}
}

func TestFileManager_writeMap_WhenMarshalFails_ShouldReturnError(t *testing.T) {
	prev := fileMarshal
	defer func() { fileMarshal = prev }()
	fileMarshal = func(interface{}) ([]byte, error) {
		return nil, fmt.Errorf("injected marshal error")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, _ := NewFileManagerWithKey(path, key)
	err := m.Set("k", "v")
	if err == nil {
		t.Fatal("Set when marshal fails: expected error")
	}
}

func TestFileManager_writeMap_WhenRandReadFails_ShouldReturnError(t *testing.T) {
	prev := fileRandReader
	defer func() { fileRandReader = prev }()
	fileRandReader = &errReader{}
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, _ := NewFileManagerWithKey(path, key)
	err := m.Set("k", "v")
	if err == nil {
		t.Fatal("Set when rand read fails: expected error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("injected rand read error")
}

func TestFileManager_writeMap_WhenCipherFails_ShouldReturnError(t *testing.T) {
	// writeMap uses aes.NewCipher(f.key); invalid key length causes error.
	path := filepath.Join(t.TempDir(), ".secrets")
	fm := &fileManager{path: path, key: make([]byte, 33)}
	err := fm.Set("k", "v")
	if err == nil {
		t.Fatal("Set with invalid key length in writeMap: expected error")
	}
}

func TestFileManager_writeMap_WhenNewGCMFails_ShouldReturnError(t *testing.T) {
	prev := fileCipherNewGCM
	defer func() { fileCipherNewGCM = prev }()
	fileCipherNewGCM = func(cipher.Block) (cipher.AEAD, error) {
		return nil, fmt.Errorf("injected NewGCM error")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, _ := NewFileManagerWithKey(path, key)
	err := m.Set("k", "v")
	if err == nil {
		t.Fatal("Set when NewGCM fails in writeMap: expected error")
	}
	if !bytesContain([]byte(err.Error()), []byte("injected")) {
		t.Errorf("error should mention injected: %v", err)
	}
}

func TestFileManager_Delete_WhenKeyInvalidLength_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	// FileManager with key length 33 so aes.NewCipher fails inside Delete when file exists.
	fm := &fileManager{path: path, key: make([]byte, 33)}
	// Write file with at least nonceSizeGCM bytes so Delete tries to decrypt and hits NewCipher.
	if err := os.WriteFile(path, make([]byte, 20), 0600); err != nil {
		t.Fatal(err)
	}
	err := fm.Delete("k")
	if err == nil {
		t.Fatal("Delete with invalid key length: expected error")
	}
}

func TestFileManager_Delete_WhenCiphertextCorrupt_ShouldOverwriteWithEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, err := NewFileManagerWithKey(path, key)
	if err != nil {
		t.Fatalf("NewFileManagerWithKey: %v", err)
	}
	if err := m.Set("k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) > 13 {
		data[13] ^= 0xff
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Delete should treat corrupt file as decrypt failure and write empty map (no error)
	err = m.Delete("k")
	if err != nil {
		t.Errorf("Delete when decrypt fails should write empty map, not error: %v", err)
	}
}

func TestFileManager_writeMap_WhenDirIsReadOnly_WriteFileFails(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, ".secrets")
	key := DeriveKeyFromPassphrase("test-passphrase")
	m, _ := NewFileManagerWithKey(path, key)
	if err := os.Chmod(sub, 0555); err != nil {
		t.Skip("chmod 0555 not supported")
	}
	defer os.Chmod(sub, 0755)
	err := m.Set("k", "v")
	if err == nil {
		t.Fatal("Set when dir is read-only: expected error")
	}
}

func bytesContain(b, sub []byte) bool {
	for i := 0; i <= len(b)-len(sub); i++ {
		if string(b[i:i+len(sub)]) == string(sub) {
			return true
		}
	}
	return false
}
