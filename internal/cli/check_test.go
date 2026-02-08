package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCheck_WhenConfigMissing_ShouldNoteAndCompleteWithZero(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.json")
	os.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	var out, errOut bytes.Buffer
	code := RunCheck([]string{"ironclaw", "check"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if out.Len() == 0 {
		t.Error("expected some stdout")
	}
	if !bytes.Contains(out.Bytes(), []byte("No config")) {
		t.Errorf("expected 'No config' in output: %s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Check complete.")) {
		t.Errorf("expected 'Check complete.' in output: %s", out.String())
	}
}

func TestRunCheck_WhenConfigMissingAndFix_ShouldWriteDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	os.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	var out, errOut bytes.Buffer
	code := RunCheck([]string{"ironclaw", "check", "--fix"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config file should exist after --fix: %v", err)
	}
	if !bytes.Contains(data, []byte("gateway")) || !bytes.Contains(data, []byte("8080")) {
		t.Errorf("expected default config content: %s", data)
	}
}

func TestRunCheck_WhenConfigExists_ShouldReportGatewayAndPaths(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	os.WriteFile(cfgPath, []byte(`{"gateway":{"port":9000,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"defaultModel":"gpt-4o","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`), 0644)
	os.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	var out, errOut bytes.Buffer
	code := RunCheck([]string{"ironclaw", "check"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	s := out.String()
	if !bytes.Contains([]byte(s), []byte("Loaded")) {
		t.Errorf("expected 'Loaded' in output: %s", s)
	}
	if !bytes.Contains([]byte(s), []byte("port=9000")) {
		t.Errorf("expected 'port=9000' in output: %s", s)
	}
	if !bytes.Contains([]byte(s), []byte("Check complete.")) {
		t.Errorf("expected 'Check complete.' in output: %s", s)
	}
}

func TestRun_WhenFirstArgIsCheck_ShouldRunCheckAndReturn(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	os.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	var out, errOut bytes.Buffer
	code := Run([]string{"ironclaw", "check"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0 from check, got %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("Check complete.")) {
		t.Errorf("expected Check complete in output: %s", out.String())
	}
}

func TestRunCheck_WhenConfigHasPathsThatDoNotExist_ShouldCreateThem(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	cfg := `{"gateway":{"port":8080,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"defaultModel":"","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	var out, errOut bytes.Buffer
	code := RunCheck([]string{"ironclaw", "check"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d: %s", code, errOut.String())
	}
	for _, name := range []string{"agents", "memory"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("path %q should exist after check: %v", name, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("path %q should be a directory", name)
		}
	}
}

func TestRunCheck_WhenConfigHasPathThatIsFile_ShouldReportNotDirectory(t *testing.T) {
	dir := t.TempDir()
	agentsFile := filepath.Join(dir, "agents")
	if err := os.WriteFile(agentsFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ironclaw.json")
	cfg := `{"gateway":{"port":8080,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"defaultModel":"","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	var out, errOut bytes.Buffer
	code := RunCheck([]string{"ironclaw", "check"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0 (check still completes), got %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("not a directory")) {
		t.Errorf("expected 'not a directory' in output when path is file: %s", out.String())
	}
}

func TestRunCheck_WhenConfigInvalidJSON_ShouldReturnOneAndNoteError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ironclaw.json")
	if err := os.WriteFile(cfgPath, []byte(`{invalid`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	var out, errOut bytes.Buffer
	code := RunCheck([]string{"ironclaw", "check"}, &out, &errOut)
	if code != 1 {
		t.Errorf("expected exit code 1 for invalid config, got %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("[Config]")) {
		t.Errorf("expected [Config] in output: %s", out.String())
	}
}

func TestRunCheck_WhenFixAndWriteDefaultFails_ShouldReturnOne(t *testing.T) {
	dir := t.TempDir()
	// Use a read-only directory so WriteDefault cannot create the config file
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0555); err != nil {
		t.Skip("chmod 0555 not supported")
	}
	defer os.Chmod(sub, 0755)
	cfgPath := filepath.Join(sub, "ironclaw.json")
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	defer os.Unsetenv("IRONCLAW_CONFIG")

	var out, errOut bytes.Buffer
	code := RunCheck([]string{"ironclaw", "check", "--fix"}, &out, &errOut)
	if code != 1 {
		t.Errorf("expected exit code 1 when write default fails, got %d (stderr: %q)", code, errOut.String())
	}
}

func TestEnsureDir_WhenPathUnderFile_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	fileAsParent := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	pathUnderFile := filepath.Join(fileAsParent, "sub")
	err := ensureDir(pathUnderFile, "label")
	if err == nil {
		t.Fatal("ensureDir when parent is file: expected error")
	}
	// Stat may return "not a directory" or we may hit MkdirAll "mkdir failed"
	if !bytes.Contains([]byte(err.Error()), []byte("mkdir")) && !bytes.Contains([]byte(err.Error()), []byte("not a directory")) {
		t.Errorf("error should mention mkdir or not a directory: %v", err)
	}
}

func TestEnsureDir_WhenPathExistsAsFile_ShouldReturnNotADirectory(t *testing.T) {
	dir := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)
	filePath := filepath.Join(dir, "agents")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := ensureDir("agents", "agents.root")
	if err == nil {
		t.Fatal("ensureDir when path is file: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("not a directory")) {
		t.Errorf("error should mention not a directory: %v", err)
	}
}

func TestEnsureDir_WhenAbsolutePathIsFile_ShouldReturnNotADirectory(t *testing.T) {
	dir := t.TempDir()
	absFile := filepath.Join(dir, "agents")
	if err := os.WriteFile(absFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := ensureDir(absFile, "agents.root")
	if err == nil {
		t.Fatal("ensureDir when absolute path is file: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("not a directory")) {
		t.Errorf("error should mention not a directory: %v", err)
	}
}

func TestRunCheck_WhenMemoryPathIsFile_ShouldNotePathsError(t *testing.T) {
	dir := t.TempDir()
	memFile := filepath.Join(dir, "memory")
	if err := os.WriteFile(memFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ironclaw.json")
	cfg := `{"gateway":{"port":8080,"auth":{"mode":"none"},"allowedHosts":[]},"agents":{"defaultModel":"","modelAliases":{},"paths":{"root":"agents","memory":"memory"}},"infra":{"logFormat":"text","logLevel":"info"}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IRONCLAW_CONFIG", cfgPath)
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	var out, errOut bytes.Buffer
	code := RunCheck([]string{"ironclaw", "check"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("[Paths]")) {
		t.Errorf("expected [Paths] in output: %s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("not a directory")) {
		t.Errorf("expected 'not a directory' when memory path is file: %s", out.String())
	}
}

func TestEnsureDir_WhenCurrentDirRemoved_AbsFailsAndReturnsError(t *testing.T) {
	dir := t.TempDir()
	prev, _ := os.Getwd()
	defer os.Chdir(prev)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// Remove current directory so Getwd (used by Abs) fails
	os.RemoveAll(dir)
	err := ensureDir("x", "label")
	if err == nil {
		t.Fatal("ensureDir when cwd removed: expected error")
	}
}

func TestEnsureDir_WhenStatFailsWithNonNotExist_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0000); err != nil {
		t.Skip("chmod 0000 not supported")
	}
	defer os.Chmod(sub, 0755)
	// Stat on path inside unreadable dir fails with permission denied (not IsNotExist)
	path := filepath.Join(sub, "f")
	err := ensureDir(path, "label")
	if err == nil {
		t.Fatal("ensureDir when stat fails (e.g. permission denied): expected error")
	}
}

func TestEnsureDir_WhenPathNotExistButMkdirAllFails_ShouldReturnError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0555); err != nil {
		t.Skip("chmod 0555 not supported")
	}
	defer os.Chmod(sub, 0755)
	// Path sub/new does not exist (IsNotExist), but MkdirAll fails (no write in sub)
	path := filepath.Join(sub, "new")
	err := ensureDir(path, "label")
	if err == nil {
		t.Fatal("ensureDir when mkdir fails: expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("mkdir")) {
		t.Errorf("error should mention mkdir: %v", err)
	}
}
