package tooling

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockSecretsManager is a test double for secrets.SecretsManager.
type mockSecretsManager struct {
	secrets map[string]string
	getErr  error
}

func newMockSecretsManager(secrets map[string]string) *mockSecretsManager {
	return &mockSecretsManager{secrets: secrets}
}

func (m *mockSecretsManager) Get(key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	val, ok := m.secrets[key]
	if !ok {
		return "", fmt.Errorf("secret not found: %s", key)
	}
	return val, nil
}

func (m *mockSecretsManager) Set(key, value string) error { return nil }
func (m *mockSecretsManager) Delete(key string) error     { return nil }

// mockGitRepo is a test double for GitRepo.
type mockGitRepo struct {
	cloneErr        error
	statusResult    string
	statusErr       error
	addErr          error
	commitErr       error
	pushErr         error
	pullErr         error
	logResult       []GitLogEntry
	logErr          error
	createBranchErr error
	checkoutErr     error

	// Spy fields
	clonedURL    string
	clonedPath   string
	clonedAuth   *GitAuth
	statusPath   string
	addedPath    string
	addedFiles   []string
	committedMsg string
	pushedRemote string
	pushedBranch string
	pulledRemote string
	pulledBranch string
	loggedLimit  int
	branchName   string
	checkedOut   string
}

func (m *mockGitRepo) Clone(url, path string, auth *GitAuth) error {
	m.clonedURL = url
	m.clonedPath = path
	m.clonedAuth = auth
	return m.cloneErr
}

func (m *mockGitRepo) Status(path string) (string, error) {
	m.statusPath = path
	return m.statusResult, m.statusErr
}

func (m *mockGitRepo) Add(path string, files []string) error {
	m.addedPath = path
	m.addedFiles = files
	return m.addErr
}

func (m *mockGitRepo) Commit(path, message, author string) error {
	m.committedMsg = message
	return m.commitErr
}

func (m *mockGitRepo) Push(path, remote, branch string, auth *GitAuth) error {
	m.pushedRemote = remote
	m.pushedBranch = branch
	return m.pushErr
}

func (m *mockGitRepo) Pull(path, remote, branch string, auth *GitAuth) error {
	m.pulledRemote = remote
	m.pulledBranch = branch
	return m.pullErr
}

func (m *mockGitRepo) Log(path string, limit int) ([]GitLogEntry, error) {
	m.loggedLimit = limit
	return m.logResult, m.logErr
}

func (m *mockGitRepo) CreateBranch(path, branch string) error {
	m.branchName = branch
	return m.createBranchErr
}

func (m *mockGitRepo) Checkout(path, branch string) error {
	m.checkedOut = branch
	return m.checkoutErr
}

// mockGitRemoteProvider is a test double for GitRemoteProvider.
type mockGitRemoteProvider struct {
	listIssuesResult []GitIssue
	listIssuesErr    error
	createIssueResult *GitIssue
	createIssueErr   error
	listPRsResult    []GitPullRequest
	listPRsErr       error
	createPRResult   *GitPullRequest
	createPRErr      error
	commentPRErr     error

	// Spy fields
	calledOwner      string
	calledRepo       string
	calledTitle      string
	calledBody       string
	calledBase       string
	calledHead       string
	calledNumber     int
	calledComment    string
}

func (m *mockGitRemoteProvider) ListIssues(owner, repo string) ([]GitIssue, error) {
	m.calledOwner = owner
	m.calledRepo = repo
	return m.listIssuesResult, m.listIssuesErr
}

func (m *mockGitRemoteProvider) CreateIssue(owner, repo, title, body string) (*GitIssue, error) {
	m.calledOwner = owner
	m.calledRepo = repo
	m.calledTitle = title
	m.calledBody = body
	return m.createIssueResult, m.createIssueErr
}

func (m *mockGitRemoteProvider) ListPullRequests(owner, repo string) ([]GitPullRequest, error) {
	m.calledOwner = owner
	m.calledRepo = repo
	return m.listPRsResult, m.listPRsErr
}

func (m *mockGitRemoteProvider) CreatePullRequest(owner, repo, title, body, base, head string) (*GitPullRequest, error) {
	m.calledOwner = owner
	m.calledRepo = repo
	m.calledTitle = title
	m.calledBody = body
	m.calledBase = base
	m.calledHead = head
	return m.createPRResult, m.createPRErr
}

func (m *mockGitRemoteProvider) CommentOnPR(owner, repo string, number int, body string) error {
	m.calledOwner = owner
	m.calledRepo = repo
	m.calledNumber = number
	m.calledComment = body
	return m.commentPRErr
}

// stubProviderFactory returns a factory that always returns the given provider.
func stubProviderFactory(provider *mockGitRemoteProvider) GitProviderFactory {
	return func(providerType, token string) (GitRemoteProvider, error) {
		return provider, nil
	}
}

// failingProviderFactory returns a factory that always returns an error.
func failingProviderFactory(err error) GitProviderFactory {
	return func(providerType, token string) (GitRemoteProvider, error) {
		return nil, err
	}
}

// =============================================================================
// GitTool — Name, Description, Definition
// =============================================================================

func TestGitTool_Name_ShouldReturnGit(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	if tool.Name() != "git" {
		t.Errorf("Expected name 'git', got '%s'", tool.Name())
	}
}

func TestGitTool_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
	if !strings.Contains(desc, "Git") && !strings.Contains(desc, "git") {
		t.Errorf("Expected description to mention git, got: %s", desc)
	}
}

func TestGitTool_Definition_ShouldReturnValidJSONSchema(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", parsed["type"])
	}
}

func TestGitTool_Definition_ShouldContainOperationProperty(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'properties' in schema")
	}
	if _, exists := props["operation"]; !exists {
		t.Error("Expected 'operation' property in schema")
	}
}

func TestGitTool_Definition_ShouldRequireOperation(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("Expected 'required' array in schema")
	}
	found := false
	for _, r := range required {
		if r.(string) == "operation" {
			found = true
		}
	}
	if !found {
		t.Error("Expected 'operation' in required fields")
	}
}

// =============================================================================
// GitTool.Call — Input Validation
// =============================================================================

func TestGitTool_Call_ShouldRejectInvalidJSON(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestGitTool_Call_ShouldRejectMissingOperation(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error for missing operation")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestGitTool_Call_ShouldRejectInvalidOperation(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"destroy"}`))
	if err == nil {
		t.Fatal("Expected error for invalid operation")
	}
}

func TestGitTool_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := gitUnmarshalFunc
	gitUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { gitUnmarshalFunc = original }()

	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"status","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Clone Operation
// =============================================================================

func TestGitTool_Call_Clone_ShouldCloneRepoSuccessfully(t *testing.T) {
	repo := &mockGitRepo{}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test123"})
	tool := NewGitTool(sm, repo, nil)

	result, err := tool.Call(json.RawMessage(`{"operation":"clone","url":"https://github.com/user/repo.git","path":"/tmp/repo"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "Successfully cloned") {
		t.Errorf("Expected success message, got: %s", result.Data)
	}
	if result.Metadata["operation"] != "clone" {
		t.Errorf("Expected metadata operation='clone', got '%s'", result.Metadata["operation"])
	}
}

func TestGitTool_Call_Clone_ShouldPassURLAndPathToRepo(t *testing.T) {
	repo := &mockGitRepo{}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test123"})
	tool := NewGitTool(sm, repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"clone","url":"https://github.com/user/repo.git","path":"/tmp/repo"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if repo.clonedURL != "https://github.com/user/repo.git" {
		t.Errorf("Expected cloned URL 'https://github.com/user/repo.git', got '%s'", repo.clonedURL)
	}
	if repo.clonedPath != "/tmp/repo" {
		t.Errorf("Expected cloned path '/tmp/repo', got '%s'", repo.clonedPath)
	}
}

func TestGitTool_Call_Clone_ShouldPassAuthWhenTokenAvailable(t *testing.T) {
	repo := &mockGitRepo{}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_secret"})
	tool := NewGitTool(sm, repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"clone","url":"https://github.com/user/repo.git","path":"/tmp/repo"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if repo.clonedAuth == nil {
		t.Fatal("Expected auth to be passed")
	}
	if repo.clonedAuth.Token != "ghp_secret" {
		t.Errorf("Expected token 'ghp_secret', got '%s'", repo.clonedAuth.Token)
	}
}

func TestGitTool_Call_Clone_ShouldWorkWithoutToken(t *testing.T) {
	repo := &mockGitRepo{}
	sm := newMockSecretsManager(nil) // no tokens
	tool := NewGitTool(sm, repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"clone","url":"https://github.com/user/public-repo.git","path":"/tmp/repo"}`))
	if err != nil {
		t.Fatalf("Expected no error for public clone, got: %v", err)
	}
	if repo.clonedAuth != nil {
		t.Error("Expected nil auth for public clone")
	}
}

func TestGitTool_Call_Clone_ShouldReturnErrorWhenURLMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"clone","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error for missing URL")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("Expected 'url is required' in error, got: %v", err)
	}
}

func TestGitTool_Call_Clone_ShouldReturnErrorWhenPathMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"clone","url":"https://github.com/user/repo.git"}`))
	if err == nil {
		t.Fatal("Expected error for missing path")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Errorf("Expected 'path is required' in error, got: %v", err)
	}
}

func TestGitTool_Call_Clone_ShouldReturnErrorWhenCloneFails(t *testing.T) {
	repo := &mockGitRepo{cloneErr: fmt.Errorf("network error")}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"clone","url":"https://github.com/user/repo.git","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error when clone fails")
	}
	if !strings.Contains(err.Error(), "clone failed") {
		t.Errorf("Expected 'clone failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Status Operation
// =============================================================================

func TestGitTool_Call_Status_ShouldReturnRepoStatus(t *testing.T) {
	repo := &mockGitRepo{statusResult: "M  file.txt\n?? new.txt"}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	result, err := tool.Call(json.RawMessage(`{"operation":"status","path":"/tmp/repo"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data != "M  file.txt\n?? new.txt" {
		t.Errorf("Expected status output, got: %s", result.Data)
	}
	if result.Metadata["operation"] != "status" {
		t.Errorf("Expected metadata operation='status', got '%s'", result.Metadata["operation"])
	}
}

func TestGitTool_Call_Status_ShouldReturnErrorWhenPathMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"status"}`))
	if err == nil {
		t.Fatal("Expected error for missing path")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Errorf("Expected 'path is required' in error, got: %v", err)
	}
}

func TestGitTool_Call_Status_ShouldReturnErrorWhenStatusFails(t *testing.T) {
	repo := &mockGitRepo{statusErr: fmt.Errorf("not a git repo")}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"status","path":"/tmp/notrepo"}`))
	if err == nil {
		t.Fatal("Expected error when status fails")
	}
	if !strings.Contains(err.Error(), "status failed") {
		t.Errorf("Expected 'status failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Add Operation
// =============================================================================

func TestGitTool_Call_Add_ShouldAddFilesToStaging(t *testing.T) {
	repo := &mockGitRepo{}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	result, err := tool.Call(json.RawMessage(`{"operation":"add","path":"/tmp/repo","files":["file1.txt","file2.txt"]}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "2 file(s)") {
		t.Errorf("Expected '2 file(s)' in output, got: %s", result.Data)
	}
}

func TestGitTool_Call_Add_ShouldPassFilesToRepo(t *testing.T) {
	repo := &mockGitRepo{}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"add","path":"/tmp/repo","files":["a.go","b.go"]}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(repo.addedFiles) != 2 || repo.addedFiles[0] != "a.go" || repo.addedFiles[1] != "b.go" {
		t.Errorf("Expected files [a.go, b.go], got %v", repo.addedFiles)
	}
}

func TestGitTool_Call_Add_ShouldReturnErrorWhenPathMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"add","files":["a.go"]}`))
	if err == nil {
		t.Fatal("Expected error for missing path")
	}
}

func TestGitTool_Call_Add_ShouldReturnErrorWhenFilesMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"add","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error for missing files")
	}
	if !strings.Contains(err.Error(), "files are required") {
		t.Errorf("Expected 'files are required' in error, got: %v", err)
	}
}

func TestGitTool_Call_Add_ShouldReturnErrorWhenAddFails(t *testing.T) {
	repo := &mockGitRepo{addErr: fmt.Errorf("file not found")}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"add","path":"/tmp/repo","files":["missing.txt"]}`))
	if err == nil {
		t.Fatal("Expected error when add fails")
	}
	if !strings.Contains(err.Error(), "add failed") {
		t.Errorf("Expected 'add failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Commit Operation
// =============================================================================

func TestGitTool_Call_Commit_ShouldCommitSuccessfully(t *testing.T) {
	repo := &mockGitRepo{}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	result, err := tool.Call(json.RawMessage(`{"operation":"commit","path":"/tmp/repo","message":"initial commit"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "initial commit") {
		t.Errorf("Expected commit message in output, got: %s", result.Data)
	}
}

func TestGitTool_Call_Commit_ShouldPassMessageToRepo(t *testing.T) {
	repo := &mockGitRepo{}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"commit","path":"/tmp/repo","message":"fix: bug"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if repo.committedMsg != "fix: bug" {
		t.Errorf("Expected committed message 'fix: bug', got '%s'", repo.committedMsg)
	}
}

func TestGitTool_Call_Commit_ShouldReturnErrorWhenPathMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"commit","message":"test"}`))
	if err == nil {
		t.Fatal("Expected error for missing path")
	}
}

func TestGitTool_Call_Commit_ShouldReturnErrorWhenMessageMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"commit","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error for missing message")
	}
	if !strings.Contains(err.Error(), "message is required") {
		t.Errorf("Expected 'message is required' in error, got: %v", err)
	}
}

func TestGitTool_Call_Commit_ShouldReturnErrorWhenCommitFails(t *testing.T) {
	repo := &mockGitRepo{commitErr: fmt.Errorf("nothing to commit")}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"commit","path":"/tmp/repo","message":"test"}`))
	if err == nil {
		t.Fatal("Expected error when commit fails")
	}
	if !strings.Contains(err.Error(), "commit failed") {
		t.Errorf("Expected 'commit failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Push Operation
// =============================================================================

func TestGitTool_Call_Push_ShouldPushSuccessfully(t *testing.T) {
	repo := &mockGitRepo{}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, repo, nil)

	result, err := tool.Call(json.RawMessage(`{"operation":"push","path":"/tmp/repo","remote":"origin","branch":"main"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "origin/main") {
		t.Errorf("Expected 'origin/main' in output, got: %s", result.Data)
	}
}

func TestGitTool_Call_Push_ShouldDefaultToOriginMain(t *testing.T) {
	repo := &mockGitRepo{}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"push","path":"/tmp/repo"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if repo.pushedRemote != "origin" {
		t.Errorf("Expected default remote 'origin', got '%s'", repo.pushedRemote)
	}
	if repo.pushedBranch != "main" {
		t.Errorf("Expected default branch 'main', got '%s'", repo.pushedBranch)
	}
}

func TestGitTool_Call_Push_ShouldReturnErrorWhenPathMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"push"}`))
	if err == nil {
		t.Fatal("Expected error for missing path")
	}
}

func TestGitTool_Call_Push_ShouldReturnErrorWhenPushFails(t *testing.T) {
	repo := &mockGitRepo{pushErr: fmt.Errorf("rejected")}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"push","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error when push fails")
	}
	if !strings.Contains(err.Error(), "push failed") {
		t.Errorf("Expected 'push failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Pull Operation
// =============================================================================

func TestGitTool_Call_Pull_ShouldPullSuccessfully(t *testing.T) {
	repo := &mockGitRepo{}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, repo, nil)

	result, err := tool.Call(json.RawMessage(`{"operation":"pull","path":"/tmp/repo","remote":"upstream","branch":"develop"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "upstream/develop") {
		t.Errorf("Expected 'upstream/develop' in output, got: %s", result.Data)
	}
}

func TestGitTool_Call_Pull_ShouldDefaultToOriginMain(t *testing.T) {
	repo := &mockGitRepo{}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"pull","path":"/tmp/repo"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if repo.pulledRemote != "origin" {
		t.Errorf("Expected default remote 'origin', got '%s'", repo.pulledRemote)
	}
	if repo.pulledBranch != "main" {
		t.Errorf("Expected default branch 'main', got '%s'", repo.pulledBranch)
	}
}

func TestGitTool_Call_Pull_ShouldReturnErrorWhenPathMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"pull"}`))
	if err == nil {
		t.Fatal("Expected error for missing path")
	}
}

func TestGitTool_Call_Pull_ShouldReturnErrorWhenPullFails(t *testing.T) {
	repo := &mockGitRepo{pullErr: fmt.Errorf("merge conflict")}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"pull","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error when pull fails")
	}
	if !strings.Contains(err.Error(), "pull failed") {
		t.Errorf("Expected 'pull failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Log Operation
// =============================================================================

func TestGitTool_Call_Log_ShouldReturnLogEntries(t *testing.T) {
	repo := &mockGitRepo{
		logResult: []GitLogEntry{
			{Hash: "abc123", Author: "John", Date: "2025-01-01", Message: "init"},
			{Hash: "def456", Author: "Jane", Date: "2025-01-02", Message: "fix"},
		},
	}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	result, err := tool.Call(json.RawMessage(`{"operation":"log","path":"/tmp/repo","limit":5}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "abc123") {
		t.Errorf("Expected hash 'abc123' in output, got: %s", result.Data)
	}
	if result.Metadata["count"] != "2" {
		t.Errorf("Expected count '2', got '%s'", result.Metadata["count"])
	}
}

func TestGitTool_Call_Log_ShouldDefaultToLimit10(t *testing.T) {
	repo := &mockGitRepo{logResult: []GitLogEntry{}}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"log","path":"/tmp/repo"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if repo.loggedLimit != 10 {
		t.Errorf("Expected default limit 10, got %d", repo.loggedLimit)
	}
}

func TestGitTool_Call_Log_ShouldReturnErrorWhenPathMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"log"}`))
	if err == nil {
		t.Fatal("Expected error for missing path")
	}
}

func TestGitTool_Call_Log_ShouldReturnErrorWhenLogFails(t *testing.T) {
	repo := &mockGitRepo{logErr: fmt.Errorf("not a repository")}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"log","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error when log fails")
	}
	if !strings.Contains(err.Error(), "log failed") {
		t.Errorf("Expected 'log failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Branch Operation
// =============================================================================

func TestGitTool_Call_Branch_ShouldCreateBranchSuccessfully(t *testing.T) {
	repo := &mockGitRepo{}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	result, err := tool.Call(json.RawMessage(`{"operation":"branch","path":"/tmp/repo","branch":"feature/new"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "feature/new") {
		t.Errorf("Expected branch name in output, got: %s", result.Data)
	}
}

func TestGitTool_Call_Branch_ShouldPassBranchNameToRepo(t *testing.T) {
	repo := &mockGitRepo{}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"branch","path":"/tmp/repo","branch":"hotfix/urgent"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if repo.branchName != "hotfix/urgent" {
		t.Errorf("Expected branch 'hotfix/urgent', got '%s'", repo.branchName)
	}
}

func TestGitTool_Call_Branch_ShouldReturnErrorWhenPathMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"branch","branch":"test"}`))
	if err == nil {
		t.Fatal("Expected error for missing path")
	}
}

func TestGitTool_Call_Branch_ShouldReturnErrorWhenBranchMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"branch","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error for missing branch")
	}
	if !strings.Contains(err.Error(), "branch name is required") {
		t.Errorf("Expected 'branch name is required' in error, got: %v", err)
	}
}

func TestGitTool_Call_Branch_ShouldReturnErrorWhenCreateBranchFails(t *testing.T) {
	repo := &mockGitRepo{createBranchErr: fmt.Errorf("branch exists")}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"branch","path":"/tmp/repo","branch":"main"}`))
	if err == nil {
		t.Fatal("Expected error when branch creation fails")
	}
	if !strings.Contains(err.Error(), "branch creation failed") {
		t.Errorf("Expected 'branch creation failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Checkout Operation
// =============================================================================

func TestGitTool_Call_Checkout_ShouldCheckoutSuccessfully(t *testing.T) {
	repo := &mockGitRepo{}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	result, err := tool.Call(json.RawMessage(`{"operation":"checkout","path":"/tmp/repo","branch":"develop"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "develop") {
		t.Errorf("Expected branch name in output, got: %s", result.Data)
	}
}

func TestGitTool_Call_Checkout_ShouldPassBranchToRepo(t *testing.T) {
	repo := &mockGitRepo{}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)

	_, err := tool.Call(json.RawMessage(`{"operation":"checkout","path":"/tmp/repo","branch":"feature/x"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if repo.checkedOut != "feature/x" {
		t.Errorf("Expected checked out 'feature/x', got '%s'", repo.checkedOut)
	}
}

func TestGitTool_Call_Checkout_ShouldReturnErrorWhenPathMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"checkout","branch":"test"}`))
	if err == nil {
		t.Fatal("Expected error for missing path")
	}
}

func TestGitTool_Call_Checkout_ShouldReturnErrorWhenBranchMissing(t *testing.T) {
	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"checkout","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error for missing branch")
	}
}

func TestGitTool_Call_Checkout_ShouldReturnErrorWhenCheckoutFails(t *testing.T) {
	repo := &mockGitRepo{checkoutErr: fmt.Errorf("branch not found")}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"checkout","path":"/tmp/repo","branch":"nonexistent"}`))
	if err == nil {
		t.Fatal("Expected error when checkout fails")
	}
	if !strings.Contains(err.Error(), "checkout failed") {
		t.Errorf("Expected 'checkout failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — List Issues Operation
// =============================================================================

func TestGitTool_Call_ListIssues_ShouldReturnIssues(t *testing.T) {
	provider := &mockGitRemoteProvider{
		listIssuesResult: []GitIssue{
			{Number: 1, Title: "Bug", State: "open", URL: "https://github.com/user/repo/issues/1"},
			{Number: 2, Title: "Feature", State: "open", URL: "https://github.com/user/repo/issues/2"},
		},
	}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	result, err := tool.Call(json.RawMessage(`{"operation":"list_issues","provider":"github","owner":"user","repo":"repo"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "Bug") {
		t.Errorf("Expected 'Bug' in output, got: %s", result.Data)
	}
	if result.Metadata["count"] != "2" {
		t.Errorf("Expected count '2', got '%s'", result.Metadata["count"])
	}
}

func TestGitTool_Call_ListIssues_ShouldPassOwnerAndRepoToProvider(t *testing.T) {
	provider := &mockGitRemoteProvider{listIssuesResult: []GitIssue{}}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	_, err := tool.Call(json.RawMessage(`{"operation":"list_issues","provider":"github","owner":"myorg","repo":"myrepo"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if provider.calledOwner != "myorg" {
		t.Errorf("Expected owner 'myorg', got '%s'", provider.calledOwner)
	}
	if provider.calledRepo != "myrepo" {
		t.Errorf("Expected repo 'myrepo', got '%s'", provider.calledRepo)
	}
}

func TestGitTool_Call_ListIssues_ShouldReturnErrorWhenOwnerMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_issues","provider":"github","repo":"repo"}`))
	if err == nil {
		t.Fatal("Expected error for missing owner")
	}
	if !strings.Contains(err.Error(), "owner and repo are required") {
		t.Errorf("Expected 'owner and repo are required' in error, got: %v", err)
	}
}

func TestGitTool_Call_ListIssues_ShouldReturnErrorWhenRepoMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_issues","provider":"github","owner":"user"}`))
	if err == nil {
		t.Fatal("Expected error for missing repo")
	}
}

func TestGitTool_Call_ListIssues_ShouldReturnErrorWhenProviderMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_issues","owner":"user","repo":"repo"}`))
	if err == nil {
		t.Fatal("Expected error for missing provider")
	}
	if !strings.Contains(err.Error(), "provider is required") {
		t.Errorf("Expected 'provider is required' in error, got: %v", err)
	}
}

func TestGitTool_Call_ListIssues_ShouldReturnErrorWhenTokenMissing(t *testing.T) {
	sm := newMockSecretsManager(nil) // no tokens
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_issues","provider":"github","owner":"user","repo":"repo"}`))
	if err == nil {
		t.Fatal("Expected error when token is missing")
	}
	if !strings.Contains(err.Error(), "failed to get github token") {
		t.Errorf("Expected 'failed to get github token' in error, got: %v", err)
	}
}

func TestGitTool_Call_ListIssues_ShouldReturnErrorWhenProviderFactoryFails(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, failingProviderFactory(fmt.Errorf("unsupported")))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_issues","provider":"github","owner":"user","repo":"repo"}`))
	if err == nil {
		t.Fatal("Expected error when factory fails")
	}
	if !strings.Contains(err.Error(), "failed to create github provider") {
		t.Errorf("Expected 'failed to create github provider' in error, got: %v", err)
	}
}

func TestGitTool_Call_ListIssues_ShouldReturnErrorWhenListFails(t *testing.T) {
	provider := &mockGitRemoteProvider{listIssuesErr: fmt.Errorf("API rate limit")}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_issues","provider":"github","owner":"user","repo":"repo"}`))
	if err == nil {
		t.Fatal("Expected error when list issues fails")
	}
	if !strings.Contains(err.Error(), "list issues failed") {
		t.Errorf("Expected 'list issues failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Create Issue Operation
// =============================================================================

func TestGitTool_Call_CreateIssue_ShouldCreateIssueSuccessfully(t *testing.T) {
	provider := &mockGitRemoteProvider{
		createIssueResult: &GitIssue{Number: 42, Title: "New Bug", URL: "https://github.com/user/repo/issues/42"},
	}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	result, err := tool.Call(json.RawMessage(`{"operation":"create_issue","provider":"github","owner":"user","repo":"repo","title":"New Bug","body":"Description"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "42") {
		t.Errorf("Expected issue number in output, got: %s", result.Data)
	}
	if result.Metadata["url"] != "https://github.com/user/repo/issues/42" {
		t.Errorf("Expected URL in metadata, got: %s", result.Metadata["url"])
	}
}

func TestGitTool_Call_CreateIssue_ShouldPassTitleAndBodyToProvider(t *testing.T) {
	provider := &mockGitRemoteProvider{
		createIssueResult: &GitIssue{Number: 1, URL: "https://example.com"},
	}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	_, err := tool.Call(json.RawMessage(`{"operation":"create_issue","provider":"github","owner":"o","repo":"r","title":"T","body":"B"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if provider.calledTitle != "T" {
		t.Errorf("Expected title 'T', got '%s'", provider.calledTitle)
	}
	if provider.calledBody != "B" {
		t.Errorf("Expected body 'B', got '%s'", provider.calledBody)
	}
}

func TestGitTool_Call_CreateIssue_ShouldReturnErrorWhenTitleMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_issue","provider":"github","owner":"u","repo":"r"}`))
	if err == nil {
		t.Fatal("Expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("Expected 'title is required' in error, got: %v", err)
	}
}

func TestGitTool_Call_CreateIssue_ShouldReturnErrorWhenOwnerOrRepoMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_issue","provider":"github","title":"Bug"}`))
	if err == nil {
		t.Fatal("Expected error for missing owner/repo")
	}
}

func TestGitTool_Call_CreateIssue_ShouldReturnErrorWhenCreateFails(t *testing.T) {
	provider := &mockGitRemoteProvider{createIssueErr: fmt.Errorf("forbidden")}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_issue","provider":"github","owner":"u","repo":"r","title":"Bug"}`))
	if err == nil {
		t.Fatal("Expected error when create issue fails")
	}
	if !strings.Contains(err.Error(), "create issue failed") {
		t.Errorf("Expected 'create issue failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — List PRs Operation
// =============================================================================

func TestGitTool_Call_ListPRs_ShouldReturnPullRequests(t *testing.T) {
	provider := &mockGitRemoteProvider{
		listPRsResult: []GitPullRequest{
			{Number: 10, Title: "Fix bug", State: "open", URL: "https://github.com/u/r/pulls/10"},
		},
	}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	result, err := tool.Call(json.RawMessage(`{"operation":"list_prs","provider":"github","owner":"u","repo":"r"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "Fix bug") {
		t.Errorf("Expected 'Fix bug' in output, got: %s", result.Data)
	}
	if result.Metadata["count"] != "1" {
		t.Errorf("Expected count '1', got '%s'", result.Metadata["count"])
	}
}

func TestGitTool_Call_ListPRs_ShouldReturnErrorWhenOwnerOrRepoMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_prs","provider":"github","owner":"u"}`))
	if err == nil {
		t.Fatal("Expected error for missing repo")
	}
}

func TestGitTool_Call_ListPRs_ShouldReturnErrorWhenListFails(t *testing.T) {
	provider := &mockGitRemoteProvider{listPRsErr: fmt.Errorf("not found")}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_prs","provider":"github","owner":"u","repo":"r"}`))
	if err == nil {
		t.Fatal("Expected error when list PRs fails")
	}
	if !strings.Contains(err.Error(), "list pull requests failed") {
		t.Errorf("Expected 'list pull requests failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Create PR Operation
// =============================================================================

func TestGitTool_Call_CreatePR_ShouldCreatePRSuccessfully(t *testing.T) {
	provider := &mockGitRemoteProvider{
		createPRResult: &GitPullRequest{Number: 5, Title: "New Feature", URL: "https://github.com/u/r/pulls/5"},
	}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	result, err := tool.Call(json.RawMessage(`{"operation":"create_pr","provider":"github","owner":"u","repo":"r","title":"New Feature","body":"Desc","base_branch":"main","head_branch":"feature"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "New Feature") {
		t.Errorf("Expected 'New Feature' in output, got: %s", result.Data)
	}
	if result.Metadata["url"] != "https://github.com/u/r/pulls/5" {
		t.Errorf("Expected URL in metadata, got: %s", result.Metadata["url"])
	}
}

func TestGitTool_Call_CreatePR_ShouldPassBranchesToProvider(t *testing.T) {
	provider := &mockGitRemoteProvider{
		createPRResult: &GitPullRequest{Number: 1, URL: "https://example.com"},
	}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	_, err := tool.Call(json.RawMessage(`{"operation":"create_pr","provider":"github","owner":"o","repo":"r","title":"T","base_branch":"main","head_branch":"dev"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if provider.calledBase != "main" {
		t.Errorf("Expected base 'main', got '%s'", provider.calledBase)
	}
	if provider.calledHead != "dev" {
		t.Errorf("Expected head 'dev', got '%s'", provider.calledHead)
	}
}

func TestGitTool_Call_CreatePR_ShouldReturnErrorWhenTitleMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_pr","provider":"github","owner":"u","repo":"r","base_branch":"main","head_branch":"dev"}`))
	if err == nil {
		t.Fatal("Expected error for missing title")
	}
}

func TestGitTool_Call_CreatePR_ShouldReturnErrorWhenBranchesMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_pr","provider":"github","owner":"u","repo":"r","title":"T"}`))
	if err == nil {
		t.Fatal("Expected error for missing branches")
	}
	if !strings.Contains(err.Error(), "base_branch and head_branch are required") {
		t.Errorf("Expected 'base_branch and head_branch are required' in error, got: %v", err)
	}
}

func TestGitTool_Call_CreatePR_ShouldReturnErrorWhenCreateFails(t *testing.T) {
	provider := &mockGitRemoteProvider{createPRErr: fmt.Errorf("validation error")}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_pr","provider":"github","owner":"u","repo":"r","title":"T","base_branch":"main","head_branch":"dev"}`))
	if err == nil {
		t.Fatal("Expected error when create PR fails")
	}
	if !strings.Contains(err.Error(), "create pull request failed") {
		t.Errorf("Expected 'create pull request failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — Comment on PR Operation
// =============================================================================

func TestGitTool_Call_CommentPR_ShouldCommentSuccessfully(t *testing.T) {
	provider := &mockGitRemoteProvider{}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	result, err := tool.Call(json.RawMessage(`{"operation":"comment_pr","provider":"github","owner":"u","repo":"r","number":42,"comment":"LGTM!"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "#42") {
		t.Errorf("Expected '#42' in output, got: %s", result.Data)
	}
}

func TestGitTool_Call_CommentPR_ShouldPassCommentToProvider(t *testing.T) {
	provider := &mockGitRemoteProvider{}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	_, err := tool.Call(json.RawMessage(`{"operation":"comment_pr","provider":"github","owner":"o","repo":"r","number":7,"comment":"Nice work!"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if provider.calledNumber != 7 {
		t.Errorf("Expected number 7, got %d", provider.calledNumber)
	}
	if provider.calledComment != "Nice work!" {
		t.Errorf("Expected comment 'Nice work!', got '%s'", provider.calledComment)
	}
}

func TestGitTool_Call_CommentPR_ShouldReturnErrorWhenNumberMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"comment_pr","provider":"github","owner":"u","repo":"r","comment":"text"}`))
	if err == nil {
		t.Fatal("Expected error for missing number")
	}
	if !strings.Contains(err.Error(), "number is required") {
		t.Errorf("Expected 'number is required' in error, got: %v", err)
	}
}

func TestGitTool_Call_CommentPR_ShouldReturnErrorWhenCommentMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"comment_pr","provider":"github","owner":"u","repo":"r","number":1}`))
	if err == nil {
		t.Fatal("Expected error for missing comment")
	}
	if !strings.Contains(err.Error(), "comment is required") {
		t.Errorf("Expected 'comment is required' in error, got: %v", err)
	}
}

func TestGitTool_Call_CommentPR_ShouldReturnErrorWhenCommentFails(t *testing.T) {
	provider := &mockGitRemoteProvider{commentPRErr: fmt.Errorf("permission denied")}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))
	_, err := tool.Call(json.RawMessage(`{"operation":"comment_pr","provider":"github","owner":"u","repo":"r","number":1,"comment":"test"}`))
	if err == nil {
		t.Fatal("Expected error when comment fails")
	}
	if !strings.Contains(err.Error(), "comment on PR failed") {
		t.Errorf("Expected 'comment on PR failed' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool.Call — GitLab Provider Tests
// =============================================================================

func TestGitTool_Call_ListIssues_ShouldWorkWithGitLabProvider(t *testing.T) {
	provider := &mockGitRemoteProvider{
		listIssuesResult: []GitIssue{
			{Number: 1, Title: "GitLab Issue", State: "opened"},
		},
	}
	sm := newMockSecretsManager(map[string]string{"gitlab_token": "glpat-test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))

	result, err := tool.Call(json.RawMessage(`{"operation":"list_issues","provider":"gitlab","owner":"mygroup","repo":"myproject"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "GitLab Issue") {
		t.Errorf("Expected 'GitLab Issue' in output, got: %s", result.Data)
	}
}

// =============================================================================
// GitTool.Call — Default switch branch (defense-in-depth)
// =============================================================================

func TestGitTool_Call_ShouldReturnErrorForUnknownOperationDefenseInDepth(t *testing.T) {
	original := gitUnmarshalFunc
	gitUnmarshalFunc = func(data []byte, v interface{}) error {
		input, ok := v.(*GitInput)
		if !ok {
			return fmt.Errorf("unexpected type")
		}
		input.Operation = "unknown_op"
		return nil
	}
	defer func() { gitUnmarshalFunc = original }()

	tool := NewGitTool(newMockSecretsManager(nil), &mockGitRepo{}, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"status","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error for unknown operation (defense-in-depth)")
	}
	if !strings.Contains(err.Error(), "unknown operation") {
		t.Errorf("Expected 'unknown operation' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool — Marshal error paths (defense-in-depth)
// =============================================================================

func TestGitTool_Call_Log_ShouldReturnErrorWhenMarshalFails(t *testing.T) {
	original := gitMarshalFunc
	gitMarshalFunc = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("forced marshal failure")
	}
	defer func() { gitMarshalFunc = original }()

	repo := &mockGitRepo{logResult: []GitLogEntry{{Hash: "abc"}}}
	tool := NewGitTool(newMockSecretsManager(nil), repo, nil)
	_, err := tool.Call(json.RawMessage(`{"operation":"log","path":"/tmp/repo"}`))
	if err == nil {
		t.Fatal("Expected error when marshal fails")
	}
	if !strings.Contains(err.Error(), "failed to marshal log entries") {
		t.Errorf("Expected 'failed to marshal log entries' in error, got: %v", err)
	}
}

func TestGitTool_Call_ListIssues_ShouldReturnErrorWhenMarshalFails(t *testing.T) {
	original := gitMarshalFunc
	gitMarshalFunc = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("forced marshal failure")
	}
	defer func() { gitMarshalFunc = original }()

	provider := &mockGitRemoteProvider{listIssuesResult: []GitIssue{{Number: 1}}}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_issues","provider":"github","owner":"u","repo":"r"}`))
	if err == nil {
		t.Fatal("Expected error when marshal fails")
	}
	if !strings.Contains(err.Error(), "failed to marshal issues") {
		t.Errorf("Expected 'failed to marshal issues' in error, got: %v", err)
	}
}

func TestGitTool_Call_CreateIssue_ShouldReturnErrorWhenMarshalFails(t *testing.T) {
	original := gitMarshalFunc
	gitMarshalFunc = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("forced marshal failure")
	}
	defer func() { gitMarshalFunc = original }()

	provider := &mockGitRemoteProvider{
		createIssueResult: &GitIssue{Number: 1, URL: "https://example.com"},
	}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_issue","provider":"github","owner":"u","repo":"r","title":"T"}`))
	if err == nil {
		t.Fatal("Expected error when marshal fails")
	}
	if !strings.Contains(err.Error(), "failed to marshal issue") {
		t.Errorf("Expected 'failed to marshal issue' in error, got: %v", err)
	}
}

func TestGitTool_Call_ListPRs_ShouldReturnErrorWhenMarshalFails(t *testing.T) {
	original := gitMarshalFunc
	gitMarshalFunc = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("forced marshal failure")
	}
	defer func() { gitMarshalFunc = original }()

	provider := &mockGitRemoteProvider{listPRsResult: []GitPullRequest{{Number: 1}}}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_prs","provider":"github","owner":"u","repo":"r"}`))
	if err == nil {
		t.Fatal("Expected error when marshal fails")
	}
	if !strings.Contains(err.Error(), "failed to marshal pull requests") {
		t.Errorf("Expected 'failed to marshal pull requests' in error, got: %v", err)
	}
}

func TestGitTool_Call_CreatePR_ShouldReturnErrorWhenMarshalFails(t *testing.T) {
	original := gitMarshalFunc
	gitMarshalFunc = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("forced marshal failure")
	}
	defer func() { gitMarshalFunc = original }()

	provider := &mockGitRemoteProvider{
		createPRResult: &GitPullRequest{Number: 1, URL: "https://example.com"},
	}
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(provider))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_pr","provider":"github","owner":"u","repo":"r","title":"T","base_branch":"main","head_branch":"dev"}`))
	if err == nil {
		t.Fatal("Expected error when marshal fails")
	}
	if !strings.Contains(err.Error(), "failed to marshal pull request") {
		t.Errorf("Expected 'failed to marshal pull request' in error, got: %v", err)
	}
}

// =============================================================================
// GitTool — Missing owner/repo validation for all remote ops
// =============================================================================

func TestGitTool_Call_CommentPR_ShouldReturnErrorWhenOwnerOrRepoMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"comment_pr","provider":"github","number":1,"comment":"text"}`))
	if err == nil {
		t.Fatal("Expected error for missing owner/repo")
	}
	if !strings.Contains(err.Error(), "owner and repo are required") {
		t.Errorf("Expected 'owner and repo are required' in error, got: %v", err)
	}
}

func TestGitTool_Call_CreatePR_ShouldReturnErrorWhenOwnerOrRepoMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_pr","provider":"github","title":"T","base_branch":"main","head_branch":"dev"}`))
	if err == nil {
		t.Fatal("Expected error for missing owner/repo")
	}
}

func TestGitTool_Call_ListPRs_ShouldReturnErrorWhenOwnerMissing(t *testing.T) {
	sm := newMockSecretsManager(map[string]string{"github_token": "ghp_test"})
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_prs","provider":"github","repo":"r"}`))
	if err == nil {
		t.Fatal("Expected error for missing owner")
	}
}

// =============================================================================
// GitTool — getRemoteProvider error path per calling function
// =============================================================================

func TestGitTool_Call_CreateIssue_ShouldReturnErrorWhenTokenMissing(t *testing.T) {
	sm := newMockSecretsManager(nil)
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_issue","provider":"github","owner":"u","repo":"r","title":"T"}`))
	if err == nil {
		t.Fatal("Expected error when token is missing")
	}
	if !strings.Contains(err.Error(), "failed to get github token") {
		t.Errorf("Expected 'failed to get github token' in error, got: %v", err)
	}
}

func TestGitTool_Call_ListPRs_ShouldReturnErrorWhenTokenMissing(t *testing.T) {
	sm := newMockSecretsManager(nil)
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"list_prs","provider":"github","owner":"u","repo":"r"}`))
	if err == nil {
		t.Fatal("Expected error when token is missing")
	}
	if !strings.Contains(err.Error(), "failed to get github token") {
		t.Errorf("Expected 'failed to get github token' in error, got: %v", err)
	}
}

func TestGitTool_Call_CreatePR_ShouldReturnErrorWhenTokenMissing(t *testing.T) {
	sm := newMockSecretsManager(nil)
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"create_pr","provider":"github","owner":"u","repo":"r","title":"T","base_branch":"main","head_branch":"dev"}`))
	if err == nil {
		t.Fatal("Expected error when token is missing")
	}
	if !strings.Contains(err.Error(), "failed to get github token") {
		t.Errorf("Expected 'failed to get github token' in error, got: %v", err)
	}
}

func TestGitTool_Call_CommentPR_ShouldReturnErrorWhenTokenMissing(t *testing.T) {
	sm := newMockSecretsManager(nil)
	tool := NewGitTool(sm, &mockGitRepo{}, stubProviderFactory(&mockGitRemoteProvider{}))
	_, err := tool.Call(json.RawMessage(`{"operation":"comment_pr","provider":"github","owner":"u","repo":"r","number":1,"comment":"text"}`))
	if err == nil {
		t.Fatal("Expected error when token is missing")
	}
	if !strings.Contains(err.Error(), "failed to get github token") {
		t.Errorf("Expected 'failed to get github token' in error, got: %v", err)
	}
}

// =============================================================================
// DefaultGitProviderFactory Tests
// =============================================================================

func TestDefaultGitProviderFactory_ShouldReturnGitHubProviderForGitHub(t *testing.T) {
	provider, err := DefaultGitProviderFactory("github", "ghp_test")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if _, ok := provider.(*GitHubProvider); !ok {
		t.Errorf("Expected *GitHubProvider, got %T", provider)
	}
}

func TestDefaultGitProviderFactory_ShouldReturnGitLabProviderForGitLab(t *testing.T) {
	provider, err := DefaultGitProviderFactory("gitlab", "glpat-test")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if _, ok := provider.(*GitLabProvider); !ok {
		t.Errorf("Expected *GitLabProvider, got %T", provider)
	}
}

func TestDefaultGitProviderFactory_ShouldReturnErrorForUnsupportedProvider(t *testing.T) {
	_, err := DefaultGitProviderFactory("bitbucket", "token")
	if err == nil {
		t.Fatal("Expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("Expected 'unsupported provider' in error, got: %v", err)
	}
}

func TestDefaultGitProviderFactory_ShouldReturnErrorWhenGitLabConstructorFails(t *testing.T) {
	original := newGitLabProviderFunc
	newGitLabProviderFunc = func(token string) (GitRemoteProvider, error) {
		return nil, fmt.Errorf("forced gitlab init failure")
	}
	defer func() { newGitLabProviderFunc = original }()

	_, err := DefaultGitProviderFactory("gitlab", "token")
	if err == nil {
		t.Fatal("Expected error when GitLab constructor fails")
	}
	if !strings.Contains(err.Error(), "forced gitlab init failure") {
		t.Errorf("Expected 'forced gitlab init failure' in error, got: %v", err)
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*GitTool)(nil)
var _ GitRepo = (*mockGitRepo)(nil)
var _ GitRemoteProvider = (*mockGitRemoteProvider)(nil)
var _ GitRepo = (*GoGitRepo)(nil)
var _ GitRemoteProvider = (*GitHubProvider)(nil)
var _ GitRemoteProvider = (*GitLabProvider)(nil)
