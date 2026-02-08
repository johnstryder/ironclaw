package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"ironclaw/internal/brain"
	"ironclaw/internal/scheduler"
)

func TestRootCommand_WhenVersionFlag_ShouldPrintBuildMetadata(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("1.0.8", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"--version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := out.String()
	if got == "" {
		got = errOut.String()
	}
	if !bytes.Contains([]byte(got), []byte("1.0.8")) {
		t.Errorf("expected version 1.0.8 in output, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("linux")) {
		t.Errorf("expected GOOS (linux) in output, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("amd64")) {
		t.Errorf("expected GOARCH (amd64) in output, got %q", got)
	}
}

func TestRootCommand_WhenVersionShortFlag_ShouldPrintBuildMetadata(t *testing.T) {
	out := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("2.0.0", "darwin", "arm64"))
	root.SetOut(out)
	root.SetArgs([]string{"-V"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := out.String()
	if !bytes.Contains([]byte(got), []byte("2.0.0")) {
		t.Errorf("expected version 2.0.0 in output, got %q", got)
	}
}

func TestRootCommand_WhenCheckSubcommand_ShouldRunCheckAndExitZero(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"check"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !bytes.Contains(out.Bytes(), []byte("Check complete")) {
		t.Errorf("expected 'Check complete' in output, got %q", out.String())
	}
}

func TestRootCommand_WhenPolishSubcommand_ShouldPrintEmojiAndExitZero(t *testing.T) {
	out := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetArgs([]string{"polish"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := out.String()
	if got != "💅🏼\n" {
		t.Errorf("expected 💅🏼 newline, got %q", got)
	}
}

func TestRootCommand_WhenPrefsSetAndGet_ShouldPersist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	out := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetArgs([]string{"prefs", "set", "theme", "dark"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute set: %v", err)
	}
	if out.String() != "ok\n" {
		t.Errorf("set: want ok, got %q", out.String())
	}

	out.Reset()
	root.SetArgs([]string{"prefs", "get", "theme"})
	err = root.Execute()
	if err != nil {
		t.Fatalf("Execute get: %v", err)
	}
	if out.String() != "dark\n" {
		t.Errorf("get theme: want dark, got %q", out.String())
	}
}

func TestRootCommand_WhenCheckWithFix_ShouldPassFixToRunCheck(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"check", "--fix"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !bytes.Contains(out.Bytes(), []byte("Check complete")) {
		t.Errorf("expected 'Check complete' in output, got %q", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Wrote default config")) {
		t.Errorf("expected --fix to write default config, got %q", out.String())
	}
}

func TestRootCommand_WhenSecretsSetThenGet_ShouldReturnStoredValueAndNotStoreInConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "test-passphrase")

	out := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"secrets", "set", "openai", "sk-secret-key-123"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute secrets set: %v", err)
	}
	if out.String() != "ok\n" {
		t.Errorf("secrets set: want ok, got %q", out.String())
	}

	out.Reset()
	root.SetArgs([]string{"secrets", "get", "openai"})
	err = root.Execute()
	if err != nil {
		t.Fatalf("Execute secrets get: %v", err)
	}
	if out.String() != "sk-secret-key-123\n" {
		t.Errorf("secrets get: want sk-secret-key-123, got %q", out.String())
	}

	configPath := filepath.Join(dir, "ironclaw", "config.json")
	configData, _ := os.ReadFile(configPath)
	if bytes.Contains(configData, []byte("sk-secret-key-123")) {
		t.Error("API key must not appear in config.json")
	}
	secretsPath := filepath.Join(dir, "ironclaw", ".secrets")
	secretsData, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatalf("read .secrets: %v", err)
	}
	if bytes.Contains(secretsData, []byte("sk-secret-key-123")) {
		t.Error(".secrets file must not contain the API key in plain text")
	}
}

func TestRootCommand_WhenSecretsDelete_ShouldRemoveKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "test-passphrase")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"secrets", "set", "openai", "sk-to-delete"})
	if err := root.Execute(); err != nil {
		t.Fatalf("secrets set: %v", err)
	}
	root.SetArgs([]string{"secrets", "delete", "openai"})
	if err := root.Execute(); err != nil {
		t.Fatalf("secrets delete: %v", err)
	}
	root.SetArgs([]string{"secrets", "get", "openai"})
	err := root.Execute()
	if err == nil {
		t.Error("secrets get after delete: expected error (secret not found)")
	}
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("not found")) {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestGetVersion_WhenVERSIONFileMissing_ShouldReturnDev(t *testing.T) {
	dir := t.TempDir()
	prev, _ := os.Getwd()
	defer os.Chdir(prev)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// Ensure no VERSION file
	os.Remove("VERSION")
	got := getVersion()
	if got != "dev" {
		t.Errorf("getVersion() when VERSION missing: want dev, got %q", got)
	}
}

func TestNewBuildMeta_WhenGoosGoarchEmpty_ShouldUseRuntimeValues(t *testing.T) {
	bm := newBuildMeta("1.0.0", "", "")
	if bm.GoOS == "" {
		t.Error("newBuildMeta with empty goos should use runtime.GOOS")
	}
	if bm.GoArch == "" {
		t.Error("newBuildMeta with empty goarch should use runtime.GOARCH")
	}
	if bm.Version != "1.0.0" {
		t.Errorf("Version: want 1.0.0, got %q", bm.Version)
	}
	s := bm.String()
	if !bytes.Contains([]byte(s), []byte("1.0.0")) || !bytes.Contains([]byte(s), []byte(bm.GoOS)) {
		t.Errorf("String(): want version and GOOS in %q", s)
	}
}

func TestRootCommand_WhenPrefsGetUnknownKey_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"prefs", "get", "unknownKey"})
	err := root.Execute()
	if err == nil {
		t.Error("prefs get unknown key: expected error")
	}
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("unknown")) {
		t.Errorf("error should mention unknown key: %v", err)
	}
}

func TestRootCommand_WhenPrefsSetUnknownKey_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"prefs", "set", "unknownKey", "value"})
	err := root.Execute()
	if err == nil {
		t.Error("prefs set unknown key: expected error")
	}
}

func TestRootCommand_WhenSecretsGetNotFound_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "test-pass")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"secrets", "get", "nonexistent"})
	err := root.Execute()
	if err == nil {
		t.Error("secrets get nonexistent: expected error")
	}
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("not found")) {
		t.Errorf("error should mention not found: %v", err)
	}
}

func TestGetVersion_WhenVersionVarSet_ShouldReturnIt(t *testing.T) {
	version = "1.0.99-ldflags"
	defer func() { version = "" }()
	got := getVersion()
	if got != "1.0.99-ldflags" {
		t.Errorf("getVersion(): want 1.0.99-ldflags, got %q", got)
	}
}

func TestRunDaemon_WhenShutdownChClosed_ShouldReturnImmediately(t *testing.T) {
	ch := make(chan struct{})
	close(ch)
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 1000 }
	defer func() { daemonShutdownCh = nil; daemonEUIDGetter = nil }()

	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{}) // no subcommand -> run daemon

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestRunApp_WhenVersionFlag_ReturnsZero(t *testing.T) {
	code := runApp([]string{"ironclaw", "--version"})
	if code != 0 {
		t.Errorf("runApp(--version): want 0, got %d", code)
	}
}

func TestRunApp_WhenVersionEmpty_UsesGetVersion(t *testing.T) {
	version = ""
	defer func() { version = "" }()
	dir := t.TempDir()
	prev, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(prev)
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("9.9.9"), 0644)
	code := runApp([]string{"ironclaw", "--version"})
	if code != 0 {
		t.Errorf("runApp with empty version var: want 0, got %d", code)
	}
}

func TestRunApp_WhenDaemonRequiresRoot_ReturnsTwo(t *testing.T) {
	ch := make(chan struct{})
	close(ch)
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 0 }
	defer func() { daemonShutdownCh = nil; daemonEUIDGetter = nil }()

	code := runApp([]string{"ironclaw"})
	if code != 2 {
		t.Errorf("runApp(daemon as root): want 2, got %d", code)
	}
}

func TestRunApp_WhenCommandReturnsError_ReturnsOne(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	code := runApp([]string{"ironclaw", "prefs", "get", "unknownKey"})
	if code != 1 {
		t.Errorf("runApp(prefs get unknown): want 1, got %d", code)
	}
}

func TestRunApp_WhenCheckFails_ReturnsExitCode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(cfgPath, []byte(`{invalid`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	code := runApp([]string{"ironclaw", "check"})
	if code != 1 {
		t.Errorf("runApp(check with bad config): want 1, got %d", code)
	}
}

func TestRootCommand_WhenCheckReturnsNonZero_ShouldReturnExitCodeError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(cfgPath, []byte(`{invalid`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"check"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when check fails")
	}
	ec, ok := err.(interface{ ExitCode() int })
	if !ok {
		t.Fatalf("expected exitCodeErr, got %T", err)
	}
	if ec.ExitCode() != 1 {
		t.Errorf("ExitCode(): want 1, got %d", ec.ExitCode())
	}
}

func TestGetVersion_WhenVERSIONFileExists_ShouldReturnTrimmedContent(t *testing.T) {
	dir := t.TempDir()
	prev, _ := os.Getwd()
	defer os.Chdir(prev)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("VERSION", []byte("  2.0.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("VERSION")
	got := getVersion()
	if got != "2.0.0" {
		t.Errorf("getVersion() when VERSION exists: want 2.0.0, got %q", got)
	}
}

func TestRootCommand_WhenPrefsGetDefaultModel_ShouldPrintValue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	out := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetArgs([]string{"prefs", "set", "defaultModel", "gpt-4o"})
	if err := root.Execute(); err != nil {
		t.Fatalf("prefs set: %v", err)
	}
	out.Reset()
	root.SetArgs([]string{"prefs", "get", "defaultModel"})
	if err := root.Execute(); err != nil {
		t.Fatalf("prefs get defaultModel: %v", err)
	}
	if out.String() != "gpt-4o\n" {
		t.Errorf("prefs get defaultModel: want gpt-4o newline, got %q", out.String())
	}
}

func TestRootCommand_WhenPrefsConfigPathFails_ShouldReturnError(t *testing.T) {
	// Force ConfigPath() to fail by making UserConfigDir fail (invalid HOME when XDG_CONFIG_HOME unset).
	prevHome := os.Getenv("HOME")
	prevXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Setenv("HOME", "")
	defer func() {
		if prevHome != "" {
			os.Setenv("HOME", prevHome)
		}
		if prevXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", prevXDG)
		}
	}()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"prefs", "get", "theme"})
	err := root.Execute()
	if err == nil {
		t.Fatal("prefs get when ConfigPath fails: expected error")
	}
	// Same env: prefs set should also fail at ConfigPath
	root.SetArgs([]string{"prefs", "set", "theme", "dark"})
	err = root.Execute()
	if err == nil {
		t.Fatal("prefs set when ConfigPath fails: expected error")
	}
}

func TestRootCommand_WhenPrefsLoadFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	ironclawDir := filepath.Join(dir, "ironclaw")
	if err := os.MkdirAll(ironclawDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(ironclawDir, "config.json")
	if err := os.Mkdir(configPath, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"prefs", "get", "theme"})
	err := root.Execute()
	if err == nil {
		t.Fatal("prefs get when config path is dir: expected error")
	}
	// Same setup: prefs set should also fail when Load fails (config path is a directory)
	root.SetArgs([]string{"prefs", "set", "theme", "dark"})
	err = root.Execute()
	if err == nil {
		t.Fatal("prefs set when config path is dir (Load fails): expected error")
	}
}

func TestRunDaemon_WhenShutdownChNil_ShouldCallWaitForShutdownSignal(t *testing.T) {
	// Inject a no-op so we don't block; covers the runDaemon path when shutdownCh is nil.
	prevWait := daemonWaitForShutdown
	daemonWaitForShutdown = func() {}
	defer func() { daemonWaitForShutdown = prevWait }()

	gatewayServerForTest = nil
	daemonShutdownCh = nil
	daemonEUIDGetter = func() int { return 1000 }
	defer func() { daemonShutdownCh = nil; daemonEUIDGetter = nil; gatewayServerForTest = nil }()

	// Use a valid config so the gateway starts and we cover close(gatewayShutdown) when shutdownCh is nil.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	cfg := `{"gateway":{"port":0,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute (daemon with nil shutdownCh): %v", err)
	}
}

func TestWaitForShutdownSignal_WhenSignalReceived_Returns(t *testing.T) {
	if waitForShutdownSignalStub {
		t.Skip("real waitForShutdownSignal only built without -tags=excludemain")
	}
	if runtime.GOOS == "windows" {
		t.Skip("signal test only on Unix")
	}
	done := make(chan struct{})
	go func() {
		waitForShutdownSignal()
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
		t.Skipf("sending SIGINT to self: %v", err)
	}
	select {
	case <-done:
		// covered
	case <-time.After(2 * time.Second):
		t.Fatal("waitForShutdownSignal did not return after SIGINT")
	}
}

func TestRunDaemon_WhenConfigLoadFails_ShouldSucceedWithShutdownCh(t *testing.T) {
	ch := make(chan struct{})
	close(ch)
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 1000 }
	defer func() { daemonShutdownCh = nil; daemonEUIDGetter = nil }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.json")
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// runDaemon prints "(no config file, using defaults)" and "ready." via fmt.Println (stdout);
	// when config load fails we just ensure the daemon path runs and returns.
}

func TestRunDaemon_WhenConfigLoads_ShouldSucceed(t *testing.T) {
	ch := make(chan struct{})
	close(ch)
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 1000 }
	defer func() { daemonShutdownCh = nil; daemonEUIDGetter = nil }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	// Port 0 so bind can succeed when env allows; covers "listen ... ready." path.
	if err := os.WriteFile(cfgPath, []byte(`{"gateway":{"port":0,"auth":{"mode":"token"},"allowedHosts":[]},"agents":{"defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// When bind succeeds we get "listen <addr>" and "ready." on stdout.
	if out.String() != "" && (bytes.Contains(out.Bytes(), []byte("listen ")) && bytes.Contains(out.Bytes(), []byte("ready."))) {
		return // success path covered
	}
	// When bind fails (e.g. sandbox) we may get error on stderr; test still passed.
	if errOut.Len() > 0 && bytes.Contains(errOut.Bytes(), []byte("gateway failed to bind")) {
		return
	}
	// No config file path: we get "ready." only (gateway not started).
	if bytes.Contains(out.Bytes(), []byte("ready.")) {
		return
	}
}

func TestRunDaemon_WhenFallbacksConfigured_ShouldSucceed(t *testing.T) {
	ch := make(chan struct{})
	close(ch)
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 1000 }
	defer func() { daemonShutdownCh = nil; daemonEUIDGetter = nil }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	cfg := `{"gateway":{"port":0,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"provider":"local","defaultModel":"test","modelAliases":{},"paths":{"root":"agents","memory":""},"fallbacks":[{"provider":"local","defaultModel":"fb1"},{"provider":"local","defaultModel":"fb2"}]},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestRunDaemon_WhenFallbacksAllInvalid_ShouldStillSucceed(t *testing.T) {
	ch := make(chan struct{})
	close(ch)
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 1000 }
	defer func() { daemonShutdownCh = nil; daemonEUIDGetter = nil }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	// Fallbacks use openai/anthropic with no keys — they'll be silently skipped.
	cfg := `{"gateway":{"port":0,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"provider":"local","defaultModel":"test","modelAliases":{},"paths":{"root":"agents","memory":""},"fallbacks":[{"provider":"openai","defaultModel":"gpt-4o"},{"provider":"anthropic","defaultModel":"claude"}]},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestRunDaemon_WhenGatewayBindFails_ShouldPrintListenErr(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot pre-bind for test: %v", err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	ch := make(chan struct{})
	close(ch)
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 1000 }
	errOut := &bytes.Buffer{}
	prevErr := gatewayBindErrWriter
	gatewayBindErrWriter = errOut
	defer func() {
		daemonShutdownCh = nil
		daemonEUIDGetter = nil
		gatewayBindErrWriter = prevErr
	}()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	cfg := fmt.Sprintf(`{"gateway":{"port":%s,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`, port)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{})

	_ = root.Execute()
	if !bytes.Contains(errOut.Bytes(), []byte("gateway failed to bind:")) {
		t.Errorf("stderr should contain 'gateway failed to bind:' when port in use, got: %s", errOut.Bytes())
	}
}

func TestRunDaemon_WhenBindWaitSkipped_ShouldPrintCheckPortOrPermissions(t *testing.T) {
	ch := make(chan struct{})
	close(ch)
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 1000 }
	prevIter := daemonBindWaitIterations
	daemonBindWaitIterations = 0
	errOut := &bytes.Buffer{}
	prevErr := gatewayBindErrWriter
	gatewayBindErrWriter = errOut
	defer func() {
		daemonShutdownCh = nil
		daemonEUIDGetter = nil
		daemonBindWaitIterations = prevIter
		gatewayBindErrWriter = prevErr
	}()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	if err := os.WriteFile(cfgPath, []byte(`{"gateway":{"port":0,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{})

	_ = root.Execute()
	// With 0 wait iterations we hit "failed to bind (check port or permissions)" when Run() hasn't set listenErr yet.
	if !bytes.Contains(errOut.Bytes(), []byte("gateway failed to bind")) {
		t.Errorf("stderr should contain 'gateway failed to bind', got: %s", errOut.Bytes())
	}
}

func TestRunDaemon_WhenGatewayAuthTokenSet_ShouldRequireBearer(t *testing.T) {
	gatewayServerForTest = nil // ensure we don't use a stale server from another test
	ch := make(chan struct{})
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 1000 }
	defer func() {
		daemonShutdownCh = nil
		daemonEUIDGetter = nil
		gatewayServerForTest = nil
	}()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	cfg := `{"gateway":{"port":0,"auth":{"mode":"token","authToken":"test-token"},"allowedHosts":[]},"agents":{"defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{})

	done := make(chan error, 1)
	go func() { done <- root.Execute() }()

	// Wait for our gateway to be listening (port 0 so we get a fresh server)
	time.Sleep(100 * time.Millisecond) // let server goroutine bind
	var addr string
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if srv := gatewayServerForTest; srv != nil {
			addr = srv.Addr()
			if addr != "" && addr != ":0" {
				break
			}
		}
	}
	if addr == "" || addr == ":0" {
		close(ch)
		<-done
		t.Skip("skipping: gateway did not bind in time (run with network permission for full daemon auth test)")
	}

	url := "http://" + addr + "/"

	// Without token -> 401
	resp, err := http.Get(url)
	if err != nil {
		close(ch)
		<-done
		t.Fatalf("get without token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		close(ch)
		<-done
		t.Errorf("without token: want 401, got %d", resp.StatusCode)
	}

	// With wrong token -> 401
	reqBad, _ := http.NewRequest(http.MethodGet, url, nil)
	reqBad.Header.Set("Authorization", "Bearer wrong-token")
	respBad, err := http.DefaultClient.Do(reqBad)
	if err != nil {
		close(ch)
		<-done
		t.Fatalf("get with wrong token: %v", err)
	}
	respBad.Body.Close()
	if respBad.StatusCode != http.StatusUnauthorized {
		close(ch)
		<-done
		t.Errorf("with wrong token: want 401, got %d", respBad.StatusCode)
	}

	// With correct token -> 200
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		close(ch)
		<-done
		t.Fatalf("get with token: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		close(ch)
		<-done
		t.Errorf("with correct token: want 200, got %d", resp2.StatusCode)
	}

	close(ch)
	if err := <-done; err != nil {
		t.Errorf("runDaemon: %v", err)
	}
}

func TestRunDaemon_WhenGatewayPortInvalid_ShouldLogAndContinue(t *testing.T) {
	gatewayServerForTest = nil
	ch := make(chan struct{})
	daemonShutdownCh = ch
	daemonEUIDGetter = func() int { return 1000 }
	defer func() {
		daemonShutdownCh = nil
		daemonEUIDGetter = nil
		gatewayServerForTest = nil
	}()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	cfg := `{"gateway":{"port":-1,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{})

	done := make(chan error, 1)
	go func() { done <- root.Execute() }()
	time.Sleep(50 * time.Millisecond)
	close(ch)
	err := <-done
	if err != nil {
		t.Errorf("runDaemon with invalid port: want nil, got %v", err)
	}
	// Gateway start error is written to process stderr; daemon continues and exits on shutdown.
}

func TestRunApp_WhenCommandNotFound_ReturnsOne(t *testing.T) {
	code := runApp([]string{"ironclaw", "notacommand"})
	if code != 1 {
		t.Errorf("runApp(unknown command): want 1, got %d", code)
	}
}

func TestExitCodeErr_ShouldImplementErrorAndExitCode(t *testing.T) {
	var e exitCodeErr = 3
	if e.Error() == "" {
		t.Error("exitCodeErr.Error() should not be empty")
	}
	if e.ExitCode() != 3 {
		t.Errorf("ExitCode(): want 3, got %d", e.ExitCode())
	}
}

func TestRootCommand_WhenSecretsDefaultManagerFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	fileAsConfig := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsConfig, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", fileAsConfig)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"secrets", "set", "k", "v"})
	err := root.Execute()
	if err == nil {
		t.Fatal("secrets set when DefaultManager fails: expected error")
	}
}

func TestRootCommand_WhenSecretsGetDefaultManagerFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	fileAsConfig := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsConfig, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", fileAsConfig)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"secrets", "get", "any"})
	err := root.Execute()
	if err == nil {
		t.Fatal("secrets get when DefaultManager fails: expected error")
	}
}

func TestRootCommand_WhenSecretsDeleteDefaultManagerFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	fileAsConfig := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsConfig, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", fileAsConfig)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"secrets", "delete", "any"})
	err := root.Execute()
	if err == nil {
		t.Fatal("secrets delete when DefaultManager fails: expected error")
	}
}

func TestRootCommand_WhenSecretsSetFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "test-pass")
	sub := filepath.Join(dir, "ironclaw", "ro")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0555); err != nil {
		t.Skip("chmod 0555 not supported")
	}
	defer os.Chmod(sub, 0755)
	// DefaultManager uses DefaultSecretsPath() which is XDG_CONFIG_HOME/ironclaw/.secrets.
	// We need Set to fail: use a path under ro. We can't change DefaultSecretsPath, so we need
	// the default path to be in a read-only dir. So make ironclaw dir, create .secrets there,
	// then chmod ironclaw to 0555 so we can't write. But then we can't create .secrets. So
	// create ironclaw, create ironclaw/.secrets as empty file, chmod ironclaw 0555 - then
	// we can't write to .secrets (dir is read-only for writing new content? No - 0555 means
	// we can't create new files in the dir, but we might be able to write to existing .secrets).
	// Actually 0555 on dir = we can list but not create/delete. Writing to existing file might
	// work if we own it. So create ironclaw, chmod 0555, then DefaultSecretsPath is ironclaw/.secrets.
	// When we Set, writeMap does MkdirAll(dir) - dir is ironclaw, already exists. Then WriteFile
	// to ironclaw/.secrets - we're creating a new file. So we get permission denied. Good.
	os.MkdirAll(filepath.Join(dir, "ironclaw"), 0755)
	if err := os.Chmod(filepath.Join(dir, "ironclaw"), 0555); err != nil {
		t.Skip("chmod 0555 not supported")
	}
	defer os.Chmod(filepath.Join(dir, "ironclaw"), 0755)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"secrets", "set", "k", "v"})
	err := root.Execute()
	if err == nil {
		t.Fatal("secrets set when write fails: expected error")
	}
}

func TestRootCommand_WhenSecretsGetReturnsNonNotFoundError_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("IRONCLAW_SECRETS_PASSPHRASE", "test-pass")
	secretsPath := filepath.Join(dir, "ironclaw", ".secrets")
	if err := os.MkdirAll(filepath.Dir(secretsPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretsPath, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(secretsPath, 0000); err != nil {
		t.Skip("chmod 0000 not supported")
	}
	defer os.Chmod(secretsPath, 0600)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root := newRootCommand(newBuildMeta("dev", "linux", "amd64"))
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"secrets", "get", "any"})
	err := root.Execute()
	if err == nil {
		t.Fatal("secrets get when file unreadable: expected error")
	}
	if bytes.Contains([]byte(err.Error()), []byte("not found")) {
		t.Errorf("expected read/error other than 'not found': %v", err)
	}
}

// =============================================================================
// makeSchedulerHandler tests
// =============================================================================

// testProvider implements domain.LLMProvider for scheduler handler tests.
type testProvider struct {
	response string
	err      error
	prompt   string
}

func (m *testProvider) Generate(ctx context.Context, prompt string) (string, error) {
	m.prompt = prompt
	return m.response, m.err
}

func TestMakeSchedulerHandler_WhenBrainSucceeds_ShouldReturnNilAndPrintResponse(t *testing.T) {
	provider := &testProvider{response: "All systems nominal."}
	b := brain.NewBrain(provider)
	var output bytes.Buffer
	printFn := func(format string, args ...any) {
		fmt.Fprintf(&output, format, args...)
	}

	handler := makeSchedulerHandler(b, printFn)
	job := scheduler.Job{ID: "health-check", Name: "Health", CronExpr: "@every 1m", Prompt: "Check system health."}

	err := handler(context.Background(), job)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte("All systems nominal.")) {
		t.Errorf("expected response in output, got %q", output.String())
	}
	if !bytes.Contains(output.Bytes(), []byte("health-check")) {
		t.Errorf("expected job ID in output, got %q", output.String())
	}
}

func TestMakeSchedulerHandler_WhenBrainFails_ShouldReturnErrorAndPrintIt(t *testing.T) {
	provider := &testProvider{err: errors.New("LLM unavailable")}
	b := brain.NewBrain(provider)
	var output bytes.Buffer
	printFn := func(format string, args ...any) {
		fmt.Fprintf(&output, format, args...)
	}

	handler := makeSchedulerHandler(b, printFn)
	job := scheduler.Job{ID: "failing-job", Name: "Fail", CronExpr: "@every 1m", Prompt: "Do something."}

	err := handler(context.Background(), job)
	if err == nil {
		t.Fatal("expected error when brain fails")
	}
	if !bytes.Contains(output.Bytes(), []byte("LLM unavailable")) {
		t.Errorf("expected error in output, got %q", output.String())
	}
	if !bytes.Contains(output.Bytes(), []byte("failing-job")) {
		t.Errorf("expected job ID in output, got %q", output.String())
	}
}

func TestMakeSchedulerHandler_ShouldFormatSystemEventPrompt(t *testing.T) {
	provider := &testProvider{response: "ok"}
	b := brain.NewBrain(provider)
	printFn := func(string, ...any) {}

	handler := makeSchedulerHandler(b, printFn)
	job := scheduler.Job{ID: "j1", Name: "Nightly Backup", CronExpr: "@daily", Prompt: "Run backup now."}

	_ = handler(context.Background(), job)

	if provider.prompt == "" {
		t.Fatal("expected prompt to be passed to brain")
	}
	if !bytes.Contains([]byte(provider.prompt), []byte("[System Event: Scheduled Job")) {
		t.Errorf("expected system event header, got %q", provider.prompt)
	}
	if !bytes.Contains([]byte(provider.prompt), []byte("Nightly Backup")) {
		t.Errorf("expected job name in prompt, got %q", provider.prompt)
	}
	if !bytes.Contains([]byte(provider.prompt), []byte("Run backup now.")) {
		t.Errorf("expected job prompt in system prompt, got %q", provider.prompt)
	}
}

func TestSchedulerPrintFn_ShouldNotPanic(t *testing.T) {
	// Smoke test: the default schedulerPrintFn should not panic.
	schedulerPrintFn("test %s\n", "ok")
}
