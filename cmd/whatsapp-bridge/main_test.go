package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/store/sqlstore"

	"ironclaw/internal/domain"
	"ironclaw/internal/secrets"
	wa "ironclaw/internal/whatsapp"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockWAClient implements wa.WAClient for tests.
type mockWAClient struct {
	msgCh chan wa.IncomingMessage
	qrCh  chan wa.QREvent
}

func newMockWAClient() *mockWAClient {
	return &mockWAClient{
		msgCh: make(chan wa.IncomingMessage, 100),
		qrCh:  make(chan wa.QREvent, 100),
	}
}

func (m *mockWAClient) Connect() error                                            { return nil }
func (m *mockWAClient) Disconnect()                                               {}
func (m *mockWAClient) IsLoggedIn() bool                                          { return true }
func (m *mockWAClient) SendText(_ context.Context, _ string, _ string) error      { return nil }
func (m *mockWAClient) MessageChannel() <-chan wa.IncomingMessage                  { return m.msgCh }
func (m *mockWAClient) GetQRChannel(_ context.Context) (<-chan wa.QREvent, error) { return m.qrCh, nil }

// mockSecretsManager implements secrets.SecretsManager for tests.
type mockSecretsManager struct {
	store map[string]string
	err   error
}

func (m *mockSecretsManager) Get(key string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	v, ok := m.store[key]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return v, nil
}

func (m *mockSecretsManager) Set(key, value string) error { return nil }
func (m *mockSecretsManager) Delete(key string) error     { return nil }

// mockRouter implements wa.MessageRouter for the startAdapterFn test.
type mockRouter struct {
	response string
}

func (m *mockRouter) Route(_ context.Context, _, _ string) (string, error) {
	return m.response, nil
}

// =============================================================================
// Default function variable tests
// =============================================================================

func TestNewWAClientFn_ShouldBeSetByInit(t *testing.T) {
	// The init() in driver.go sets newWAClientFn to createWhatsmeowClient.
	// Verify it's non-nil after package init.
	if newWAClientFn == nil {
		t.Fatal("expected newWAClientFn to be set by init(), got nil")
	}
}

func TestCreateWhatsmeowClient_WhenValidTempDB_ShouldReturnClient(t *testing.T) {
	// Given: a temp directory for the database
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-whatsapp.db")

	// When: calling the real createWhatsmeowClient
	client, err := createWhatsmeowClient(dbPath)

	// Then: should succeed and return a valid client
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestCreateWhatsmeowClient_WhenInvalidDBPath_ShouldReturnError(t *testing.T) {
	// Given: an impossible path
	dbPath := "/dev/null/impossible/dir/test.db"

	// When: calling createWhatsmeowClient
	_, err := createWhatsmeowClient(dbPath)

	// Then: should return an error
	if err == nil {
		t.Fatal("expected error for invalid DB path")
	}
	if !strings.Contains(err.Error(), "db open") {
		t.Errorf("error should mention 'db open', got: %v", err)
	}
}

func TestCreateWhatsmeowClient_WhenPragmaFails_ShouldReturnError(t *testing.T) {
	// Given: a pragma function that always fails
	oldPragma := execPragmaFn
	defer func() { execPragmaFn = oldPragma }()

	execPragmaFn = func(db *sql.DB) error {
		return errors.New("pragma failed")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "pragma-fail.db")

	// When: calling createWhatsmeowClient
	_, err := createWhatsmeowClient(dbPath)

	// Then: should return an error about foreign keys
	if err == nil {
		t.Fatal("expected error when pragma fails")
	}
	if !strings.Contains(err.Error(), "enable foreign keys") {
		t.Errorf("error should mention 'enable foreign keys', got: %v", err)
	}
}

func TestCreateWhatsmeowClient_WhenUpgradeFails_ShouldReturnError(t *testing.T) {
	// Given: a container function that always fails upgrade
	oldContainer := newContainerFn
	defer func() { newContainerFn = oldContainer }()

	newContainerFn = func(db *sql.DB) (*sqlstore.Container, error) {
		return nil, errors.New("upgrade failed")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "upgrade-fail.db")

	// When: calling createWhatsmeowClient
	_, err := createWhatsmeowClient(dbPath)

	// Then: should return an error about sqlstore upgrade
	if err == nil {
		t.Fatal("expected error when upgrade fails")
	}
	if !strings.Contains(err.Error(), "sqlstore upgrade") {
		t.Errorf("error should mention 'sqlstore upgrade', got: %v", err)
	}
}

func TestDefaultNewContainerFn_WhenDBClosed_ShouldReturnError(t *testing.T) {
	// Given: a closed *sql.DB passed to the DEFAULT newContainerFn
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "closed.db")

	conn, err := connectFn(fmt.Sprintf("file:%s", dbPath))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	conn.Close() // close so Upgrade will fail

	// When: calling the default newContainerFn
	_, err = newContainerFn(conn)

	// Then: should return an error
	if err == nil {
		t.Fatal("expected error when DB is closed")
	}
}

func TestCreateWhatsmeowClient_WhenGetFirstDeviceFails_ShouldReturnError(t *testing.T) {
	// Given: a container whose underlying DB is closed after upgrade,
	// so GetFirstDevice will fail.
	oldContainer := newContainerFn
	defer func() { newContainerFn = oldContainer }()

	newContainerFn = func(db *sql.DB) (*sqlstore.Container, error) {
		// Perform real upgrade so schema is created.
		container := sqlstore.NewWithDB(db, "sqlite3", nil)
		if err := container.Upgrade(context.Background()); err != nil {
			return nil, err
		}
		// Close the DB so GetFirstDevice will fail.
		db.Close()
		return container, nil
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "device-fail.db")

	// When: calling createWhatsmeowClient
	_, err := createWhatsmeowClient(dbPath)

	// Then: should return an error about device store
	if err == nil {
		t.Fatal("expected error when GetFirstDevice fails")
	}
	if !strings.Contains(err.Error(), "device store") {
		t.Errorf("error should mention 'device store', got: %v", err)
	}
}

func TestDefaultStartAdapterFn_ShouldCallStart(t *testing.T) {
	mock := newMockWAClient()
	rtr := &mockRouter{response: "ok"}
	adapter := wa.NewAdapter(mock, rtr, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so Start returns right away

	// The default startAdapterFn just calls adapter.Start(ctx).
	err := startAdapterFn(adapter, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultSignalContextFn_ShouldReturnCancelableContext(t *testing.T) {
	ctx, cancel := signalContextFn()
	defer cancel()

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	cancel()
}

func TestDefaultQRHandlerFn_ShouldReturnNonNilHandler(t *testing.T) {
	handler := qrHandlerFn()
	if handler == nil {
		t.Fatal("expected non-nil QR handler")
	}
}

func TestDefaultQRHandlerFn_ShouldPrintQRCode(t *testing.T) {
	handler := qrHandlerFn()
	// Should not panic when called with a QR code string.
	// Output goes to stdout, which is fine in tests.
	handler("test-qr-code-data")
}

// =============================================================================
// main tests
// =============================================================================

func TestMain_WhenRunFails_ShouldCallExitWithOne(t *testing.T) {
	oldExit := exitFunc
	oldNewClient := newWAClientFn
	defer func() {
		exitFunc = oldExit
		newWAClientFn = oldNewClient
	}()

	// Make newWAClientFn fail.
	newWAClientFn = func(dbPath string) (wa.WAClient, error) {
		return nil, errors.New("client init failed")
	}

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	main()

	if exitCode != 1 {
		t.Errorf("want exit code 1, got %d", exitCode)
	}
}

// =============================================================================
// buildBrain tests
// =============================================================================

func TestBuildBrain_WhenConfigMissing_ShouldReturnError(t *testing.T) {
	t.Setenv("IRONCLAW_CONFIG", "/nonexistent/ironclaw.json")

	_, err := buildBrain()
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "config load") {
		t.Errorf("error should mention config load, got: %v", err)
	}
}

func TestBuildBrain_WhenLocalProvider_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test-config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
		Retry: domain.RetryConfig{
			MaxRetries:     0,
			InitialBackoff: 500,
			MaxBackoff:     5000,
			Multiplier:     2,
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)

	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	b, err := buildBrain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil brain")
	}
}

func TestBuildBrain_WhenMemoryPathSet_ShouldConfigureMemory(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0755)
	cfgPath := filepath.Join(dir, "test-config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: memDir},
		},
		Retry: domain.RetryConfig{
			MaxRetries:     0,
			InitialBackoff: 500,
			MaxBackoff:     5000,
			Multiplier:     2,
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)

	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	b, err := buildBrain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil brain")
	}
}

func TestBuildBrain_WhenSecretsManagerFails_ShouldReturnError(t *testing.T) {
	oldSM := secretsManagerFn
	defer func() { secretsManagerFn = oldSM }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	secretsManagerFn = func() (secrets.SecretsManager, error) {
		return nil, errors.New("keyring broken")
	}

	_, err := buildBrain()
	if err == nil {
		t.Fatal("expected error when secrets manager fails")
	}
	if !strings.Contains(err.Error(), "secrets manager") {
		t.Errorf("error should mention secrets manager, got: %v", err)
	}
}

func TestBuildBrain_WhenProviderNeedsApiKey_ShouldReturnError(t *testing.T) {
	oldSM := secretsManagerFn
	defer func() { secretsManagerFn = oldSM }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "anthropic",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	secretsManagerFn = func() (secrets.SecretsManager, error) {
		return &mockSecretsManager{store: map[string]string{}}, nil
	}

	_, err := buildBrain()
	if err == nil {
		t.Fatal("expected error when provider needs API key")
	}
	if !strings.Contains(err.Error(), "llm provider") {
		t.Errorf("error should mention llm provider, got: %v", err)
	}
}

func TestBuildBrain_WhenFallbacksConfigured_ShouldSucceed(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test-config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
			Fallbacks: []domain.FallbackConfig{
				{Provider: "local", DefaultModel: "fallback1"},
				{Provider: "local", DefaultModel: "fallback2"},
			},
		},
		Retry: domain.RetryConfig{
			MaxRetries:     0,
			InitialBackoff: 500,
			MaxBackoff:     5000,
			Multiplier:     2,
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	b, err := buildBrain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil brain with fallbacks")
	}
}

func TestBuildBrain_WhenFallbacksAllInvalid_ShouldStillSucceed(t *testing.T) {
	oldSM := secretsManagerFn
	defer func() { secretsManagerFn = oldSM }()

	secretsManagerFn = func() (secrets.SecretsManager, error) {
		return &mockSecretsManager{store: map[string]string{}}, nil
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test-config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
			Fallbacks: []domain.FallbackConfig{
				{Provider: "openai", DefaultModel: "gpt-4o"},       // no key -> skipped
				{Provider: "anthropic", DefaultModel: "claude-3.5"}, // no key -> skipped
			},
		},
		Retry: domain.RetryConfig{
			MaxRetries:     0,
			InitialBackoff: 500,
			MaxBackoff:     5000,
			Multiplier:     2,
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	b, err := buildBrain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil brain even with all invalid fallbacks")
	}
}

func TestBuildBrain_WhenDefaultConfigPath_ShouldUseIronclawJson(t *testing.T) {
	t.Setenv("IRONCLAW_CONFIG", "")

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(t.TempDir())

	_, err := buildBrain()
	if err == nil {
		t.Fatal("expected error for missing default config")
	}
}

// =============================================================================
// run tests
// =============================================================================

func TestRun_WhenClientInitFails_ShouldReturnError(t *testing.T) {
	oldNewClient := newWAClientFn
	defer func() { newWAClientFn = oldNewClient }()

	newWAClientFn = func(dbPath string) (wa.WAClient, error) {
		return nil, errors.New("sqlite init failed")
	}

	err := run()
	if err == nil {
		t.Fatal("expected error when client init fails")
	}
	if !strings.Contains(err.Error(), "whatsapp client init") {
		t.Errorf("error should mention whatsapp client init, got: %v", err)
	}
}

func TestRun_WhenBrainBuildFails_ShouldReturnError(t *testing.T) {
	oldNewClient := newWAClientFn
	defer func() { newWAClientFn = oldNewClient }()

	t.Setenv("IRONCLAW_CONFIG", "/nonexistent/config.json")

	newWAClientFn = func(dbPath string) (wa.WAClient, error) {
		return newMockWAClient(), nil
	}

	err := run()
	if err == nil {
		t.Fatal("expected error when brain build fails")
	}
	if !strings.Contains(err.Error(), "brain setup") {
		t.Errorf("error should mention brain setup, got: %v", err)
	}
}

func TestRun_WhenStartFails_ShouldReturnError(t *testing.T) {
	oldNewClient := newWAClientFn
	oldStart := startAdapterFn
	oldSignal := signalContextFn
	defer func() {
		newWAClientFn = oldNewClient
		startAdapterFn = oldStart
		signalContextFn = oldSignal
	}()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	newWAClientFn = func(dbPath string) (wa.WAClient, error) {
		return newMockWAClient(), nil
	}

	signalContextFn = func() (context.Context, context.CancelFunc) {
		return context.WithCancel(context.Background())
	}

	startAdapterFn = func(adapter *wa.Adapter, ctx context.Context) error {
		return errors.New("connect failed")
	}

	err := run()
	if err == nil {
		t.Fatal("expected error when start fails")
	}
	if !strings.Contains(err.Error(), "whatsapp bridge") {
		t.Errorf("error should mention whatsapp bridge, got: %v", err)
	}
}

func TestRun_WhenAllDepsValid_ShouldStartAndReturn(t *testing.T) {
	oldNewClient := newWAClientFn
	oldStart := startAdapterFn
	oldSignal := signalContextFn
	defer func() {
		newWAClientFn = oldNewClient
		startAdapterFn = oldStart
		signalContextFn = oldSignal
	}()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	newWAClientFn = func(dbPath string) (wa.WAClient, error) {
		return newMockWAClient(), nil
	}

	signalContextFn = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately
		return ctx, cancel
	}

	startAdapterFn = func(adapter *wa.Adapter, ctx context.Context) error {
		return adapter.Start(ctx)
	}

	err := run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_WhenDBPathEnvSet_ShouldUseCustomPath(t *testing.T) {
	oldNewClient := newWAClientFn
	oldStart := startAdapterFn
	oldSignal := signalContextFn
	defer func() {
		newWAClientFn = oldNewClient
		startAdapterFn = oldStart
		signalContextFn = oldSignal
	}()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := domain.Config{
		Agents: domain.AgentsConfig{
			Provider:     "local",
			DefaultModel: "test",
			Paths:        domain.AgentPaths{Root: ".", Memory: ""},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	customDBPath := filepath.Join(dir, "custom.db")
	t.Setenv("WHATSAPP_DB_PATH", customDBPath)

	var usedDBPath string
	newWAClientFn = func(dbPath string) (wa.WAClient, error) {
		usedDBPath = dbPath
		return newMockWAClient(), nil
	}

	signalContextFn = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, cancel
	}

	startAdapterFn = func(adapter *wa.Adapter, ctx context.Context) error {
		return adapter.Start(ctx)
	}

	err := run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usedDBPath != customDBPath {
		t.Errorf("want DB path %q, got %q", customDBPath, usedDBPath)
	}
}
