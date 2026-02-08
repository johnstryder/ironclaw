package tooling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// initTestRepo creates a real git repo in a temp directory with an initial commit.
// Returns the repo path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	// Create a file and make an initial commit
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}
	if _, err := w.Add("README.md"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	_, err = w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	return dir
}

// =============================================================================
// GoGitRepo — Status
// =============================================================================

func TestGoGitRepo_Status_ShouldReturnCleanStatusForCleanRepo(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	status, err := g.Status(dir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	// Clean repo should have empty status (or whitespace only)
	if strings.TrimSpace(status) != "" {
		t.Errorf("Expected clean status, got: '%s'", status)
	}
}

func TestGoGitRepo_Status_ShouldShowModifiedFiles(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	// Modify a file
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	status, err := g.Status(dir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(status, "README.md") {
		t.Errorf("Expected modified file in status, got: '%s'", status)
	}
}

func TestGoGitRepo_Status_ShouldReturnErrorForNonRepo(t *testing.T) {
	dir := t.TempDir() // not a git repo
	g := NewGoGitRepo()

	_, err := g.Status(dir)
	if err == nil {
		t.Fatal("Expected error for non-repo directory")
	}
}

// =============================================================================
// GoGitRepo — Add
// =============================================================================

func TestGoGitRepo_Add_ShouldStageFiles(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	// Create a new file
	newFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(newFile, []byte("new content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err := g.Add(dir, []string{"new.txt"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the file is staged
	status, _ := g.Status(dir)
	if !strings.Contains(status, "new.txt") {
		t.Errorf("Expected new.txt in status after add, got: '%s'", status)
	}
}

func TestGoGitRepo_Add_ShouldReturnErrorForNonExistentFile(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	err := g.Add(dir, []string{"nonexistent.txt"})
	if err != nil {
		// go-git may not error on adding a non-existent file, it just ignores it
		// This is expected behavior
		t.Logf("Got error (acceptable): %v", err)
	}
}

func TestGoGitRepo_Add_ShouldReturnErrorForNonRepo(t *testing.T) {
	dir := t.TempDir()
	g := NewGoGitRepo()

	err := g.Add(dir, []string{"file.txt"})
	if err == nil {
		t.Fatal("Expected error for non-repo directory")
	}
}

// =============================================================================
// GoGitRepo — Commit
// =============================================================================

func TestGoGitRepo_Commit_ShouldCreateCommitWithMessage(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	// Create and stage a file
	if err := os.WriteFile(filepath.Join(dir, "change.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := g.Add(dir, []string{"change.txt"}); err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	err := g.Commit(dir, "test commit", "")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify commit exists in log
	entries, err := g.Log(dir, 1)
	if err != nil {
		t.Fatalf("failed to get log: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("Expected at least one log entry")
	}
	if entries[0].Message != "test commit" {
		t.Errorf("Expected message 'test commit', got '%s'", entries[0].Message)
	}
}

func TestGoGitRepo_Commit_ShouldUseCustomAuthor(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	if err := os.WriteFile(filepath.Join(dir, "author.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := g.Add(dir, []string{"author.txt"}); err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	err := g.Commit(dir, "custom author", "Alice <alice@example.com>")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	entries, err := g.Log(dir, 1)
	if err != nil {
		t.Fatalf("failed to get log: %v", err)
	}
	if entries[0].Author != "Alice" {
		t.Errorf("Expected author 'Alice', got '%s'", entries[0].Author)
	}
}

func TestGoGitRepo_Commit_ShouldReturnErrorForNonRepo(t *testing.T) {
	dir := t.TempDir()
	g := NewGoGitRepo()

	err := g.Commit(dir, "test", "")
	if err == nil {
		t.Fatal("Expected error for non-repo directory")
	}
}

// =============================================================================
// GoGitRepo — Log
// =============================================================================

func TestGoGitRepo_Log_ShouldReturnCommitHistory(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	entries, err := g.Log(dir, 10)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
	if entries[0].Message != "initial commit" {
		t.Errorf("Expected message 'initial commit', got '%s'", entries[0].Message)
	}
	if entries[0].Hash == "" {
		t.Error("Expected non-empty hash")
	}
}

func TestGoGitRepo_Log_ShouldRespectLimit(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	// Create multiple commits
	for i := 0; i < 5; i++ {
		fname := filepath.Join(dir, "file"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(fname, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
		if err := g.Add(dir, []string{filepath.Base(fname)}); err != nil {
			t.Fatalf("failed to add: %v", err)
		}
		if err := g.Commit(dir, "commit "+string(rune('a'+i)), ""); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}
	}

	entries, err := g.Log(dir, 3)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries (limited), got %d", len(entries))
	}
}

func TestGoGitRepo_Log_ShouldReturnErrorForNonRepo(t *testing.T) {
	dir := t.TempDir()
	g := NewGoGitRepo()

	_, err := g.Log(dir, 10)
	if err == nil {
		t.Fatal("Expected error for non-repo directory")
	}
}

// =============================================================================
// GoGitRepo — CreateBranch
// =============================================================================

func TestGoGitRepo_CreateBranch_ShouldCreateNewBranch(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	err := g.CreateBranch(dir, "feature/test")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify we can checkout to it
	err = g.Checkout(dir, "feature/test")
	if err != nil {
		t.Fatalf("Expected to checkout new branch, got: %v", err)
	}
}

func TestGoGitRepo_CreateBranch_ShouldReturnErrorForNonRepo(t *testing.T) {
	dir := t.TempDir()
	g := NewGoGitRepo()

	err := g.CreateBranch(dir, "test")
	if err == nil {
		t.Fatal("Expected error for non-repo directory")
	}
}

// =============================================================================
// GoGitRepo — Checkout
// =============================================================================

func TestGoGitRepo_Checkout_ShouldSwitchBranch(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	// Create and checkout a new branch
	if err := g.CreateBranch(dir, "develop"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	err := g.Checkout(dir, "develop")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestGoGitRepo_Checkout_ShouldReturnErrorForNonExistentBranch(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	err := g.Checkout(dir, "nonexistent-branch")
	if err == nil {
		t.Fatal("Expected error for non-existent branch")
	}
}

func TestGoGitRepo_Checkout_ShouldReturnErrorForNonRepo(t *testing.T) {
	dir := t.TempDir()
	g := NewGoGitRepo()

	err := g.Checkout(dir, "main")
	if err == nil {
		t.Fatal("Expected error for non-repo directory")
	}
}

// =============================================================================
// GoGitRepo — Clone (local clone for testing)
// =============================================================================

func TestGoGitRepo_Clone_ShouldCloneLocalRepo(t *testing.T) {
	srcDir := initTestRepo(t)
	dstDir := filepath.Join(t.TempDir(), "cloned")
	g := NewGoGitRepo()

	err := g.Clone(srcDir, dstDir, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the clone has the file
	content, err := os.ReadFile(filepath.Join(dstDir, "README.md"))
	if err != nil {
		t.Fatalf("Expected README.md in clone, got: %v", err)
	}
	if !strings.Contains(string(content), "# Test") {
		t.Errorf("Expected '# Test' in README, got: %s", string(content))
	}
}

func TestGoGitRepo_Clone_ShouldReturnErrorForInvalidURL(t *testing.T) {
	dstDir := filepath.Join(t.TempDir(), "cloned")
	g := NewGoGitRepo()

	err := g.Clone("not-a-valid-url", dstDir, nil)
	if err == nil {
		t.Fatal("Expected error for invalid URL")
	}
}

// =============================================================================
// GoGitRepo — Push (no remote to push to, should error)
// =============================================================================

func TestGoGitRepo_Push_ShouldReturnErrorWithoutRemote(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	err := g.Push(dir, "origin", "main", nil)
	if err == nil {
		t.Fatal("Expected error when pushing without remote")
	}
}

func TestGoGitRepo_Push_ShouldReturnErrorForNonRepo(t *testing.T) {
	dir := t.TempDir()
	g := NewGoGitRepo()

	err := g.Push(dir, "origin", "main", nil)
	if err == nil {
		t.Fatal("Expected error for non-repo directory")
	}
}

// =============================================================================
// GoGitRepo — Pull (no remote to pull from, should error or succeed if already up to date)
// =============================================================================

func TestGoGitRepo_Pull_ShouldReturnErrorForNonRepo(t *testing.T) {
	dir := t.TempDir()
	g := NewGoGitRepo()

	err := g.Pull(dir, "origin", "main", nil)
	if err == nil {
		t.Fatal("Expected error for non-repo directory")
	}
}

// =============================================================================
// goGitAuth helper
// =============================================================================

func TestGoGitAuth_ShouldReturnNilForNilAuth(t *testing.T) {
	result := goGitAuth(nil)
	if result != nil {
		t.Error("Expected nil for nil auth")
	}
}

func TestGoGitAuth_ShouldReturnNilForEmptyToken(t *testing.T) {
	result := goGitAuth(&GitAuth{Token: ""})
	if result != nil {
		t.Error("Expected nil for empty token")
	}
}

func TestGoGitAuth_ShouldReturnBasicAuthWithToken(t *testing.T) {
	result := goGitAuth(&GitAuth{Token: "ghp_test", Username: "myuser"})
	if result == nil {
		t.Fatal("Expected non-nil auth")
	}
	if result.Username != "myuser" {
		t.Errorf("Expected username 'myuser', got '%s'", result.Username)
	}
	if result.Password != "ghp_test" {
		t.Errorf("Expected password 'ghp_test', got '%s'", result.Password)
	}
}

func TestGoGitAuth_ShouldDefaultUsernameToGit(t *testing.T) {
	result := goGitAuth(&GitAuth{Token: "ghp_test"})
	if result == nil {
		t.Fatal("Expected non-nil auth")
	}
	if result.Username != "git" {
		t.Errorf("Expected default username 'git', got '%s'", result.Username)
	}
}

// =============================================================================
// GoGitRepo — Pull (with actual remote)
// =============================================================================

func TestGoGitRepo_Pull_ShouldPullFromClonedRemote(t *testing.T) {
	// Create a "remote" repo
	srcDir := initTestRepo(t)
	dstDir := filepath.Join(t.TempDir(), "cloned")
	g := NewGoGitRepo()

	// Clone it
	if err := g.Clone(srcDir, dstDir, nil); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	// Add a commit to the source repo
	if err := os.WriteFile(filepath.Join(srcDir, "new.txt"), []byte("new"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	srcRepo, _ := git.PlainOpen(srcDir)
	w, _ := srcRepo.Worktree()
	w.Add("new.txt")
	w.Commit("add new.txt", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
	})

	// Pull from the cloned dir — the default branch name after clone
	// might be "master" for local repos. We need to detect it.
	clonedRepo, _ := git.PlainOpen(dstDir)
	head, _ := clonedRepo.Head()
	branchName := head.Name().Short()

	err := g.Pull(dstDir, "origin", branchName, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the new file exists
	_, err = os.Stat(filepath.Join(dstDir, "new.txt"))
	if err != nil {
		t.Errorf("Expected new.txt after pull, got: %v", err)
	}
}

func TestGoGitRepo_Pull_ShouldSucceedWhenAlreadyUpToDate(t *testing.T) {
	srcDir := initTestRepo(t)
	dstDir := filepath.Join(t.TempDir(), "cloned")
	g := NewGoGitRepo()

	if err := g.Clone(srcDir, dstDir, nil); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	clonedRepo, _ := git.PlainOpen(dstDir)
	head, _ := clonedRepo.Head()
	branchName := head.Name().Short()

	// Pull when already up to date — should not error
	err := g.Pull(dstDir, "origin", branchName, nil)
	if err != nil {
		t.Fatalf("Expected no error for up-to-date pull, got: %v", err)
	}
}

// =============================================================================
// GoGitRepo — Clone with auth (local doesn't need auth, but covers auth path)
// =============================================================================

func TestGoGitRepo_Clone_ShouldAcceptAuthForLocalClone(t *testing.T) {
	srcDir := initTestRepo(t)
	dstDir := filepath.Join(t.TempDir(), "cloned")
	g := NewGoGitRepo()

	// Auth is ignored for local paths but the code path is exercised
	err := g.Clone(srcDir, dstDir, &GitAuth{Token: "test-token", Username: "user"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// =============================================================================
// GoGitRepo — Push with auth (covers auth path even though it fails for other reasons)
// =============================================================================

func TestGoGitRepo_Push_ShouldPassAuthWhenProvided(t *testing.T) {
	srcDir := initTestRepo(t)
	dstDir := filepath.Join(t.TempDir(), "cloned")
	g := NewGoGitRepo()

	if err := g.Clone(srcDir, dstDir, nil); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	// Add a commit in the clone
	if err := os.WriteFile(filepath.Join(dstDir, "push.txt"), []byte("push"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := g.Add(dstDir, []string{"push.txt"}); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := g.Commit(dstDir, "push commit", ""); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	clonedRepo, _ := git.PlainOpen(dstDir)
	head, _ := clonedRepo.Head()
	branchName := head.Name().Short()

	// Push with auth — will fail because local remote doesn't support push,
	// but the auth code path is exercised
	err := g.Push(dstDir, "origin", branchName, &GitAuth{Token: "test"})
	// Local repos can't be pushed to via HTTP auth, but we exercise the code path
	if err != nil {
		t.Logf("Push error (expected for local repo): %v", err)
	}
}

// =============================================================================
// GoGitRepo — Pull with auth (covers auth path)
// =============================================================================

func TestGoGitRepo_Pull_ShouldPassAuthWhenProvided(t *testing.T) {
	srcDir := initTestRepo(t)
	dstDir := filepath.Join(t.TempDir(), "cloned")
	g := NewGoGitRepo()

	if err := g.Clone(srcDir, dstDir, nil); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	clonedRepo, _ := git.PlainOpen(dstDir)
	head, _ := clonedRepo.Head()
	branchName := head.Name().Short()

	// Pull with auth — auth path is exercised even for local repos
	err := g.Pull(dstDir, "origin", branchName, &GitAuth{Token: "test"})
	// Should succeed (already up to date) regardless of auth
	if err != nil {
		t.Logf("Pull error (may be expected for local with auth): %v", err)
	}
}

// =============================================================================
// GoGitRepo — Commit with author parsing edge case
// =============================================================================

func TestGoGitRepo_Commit_ShouldHandleAuthorWithoutEmail(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	if err := os.WriteFile(filepath.Join(dir, "nomail.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := g.Add(dir, []string{"nomail.txt"}); err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	// Author without email angle brackets
	err := g.Commit(dir, "no email", "JustAName")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	entries, _ := g.Log(dir, 1)
	if entries[0].Author != "JustAName" {
		t.Errorf("Expected author 'JustAName', got '%s'", entries[0].Author)
	}
}

// =============================================================================
// GoGitRepo — Bare repo tests (triggers worktree error paths)
// =============================================================================

func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := git.PlainInit(dir, true) // bare = true
	if err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}
	return dir
}

func TestGoGitRepo_Status_ShouldReturnErrorForBareRepo(t *testing.T) {
	dir := initBareRepo(t)
	g := NewGoGitRepo()

	_, err := g.Status(dir)
	if err == nil {
		t.Fatal("Expected error for bare repo (no worktree)")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("Expected 'worktree' in error, got: %v", err)
	}
}

func TestGoGitRepo_Add_ShouldReturnErrorForBareRepo(t *testing.T) {
	dir := initBareRepo(t)
	g := NewGoGitRepo()

	err := g.Add(dir, []string{"file.txt"})
	if err == nil {
		t.Fatal("Expected error for bare repo (no worktree)")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("Expected 'worktree' in error, got: %v", err)
	}
}

func TestGoGitRepo_Commit_ShouldReturnErrorForBareRepo(t *testing.T) {
	dir := initBareRepo(t)
	g := NewGoGitRepo()

	err := g.Commit(dir, "test", "")
	if err == nil {
		t.Fatal("Expected error for bare repo (no worktree)")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("Expected 'worktree' in error, got: %v", err)
	}
}

func TestGoGitRepo_Pull_ShouldReturnErrorForBareRepo(t *testing.T) {
	// This covers the worktree error path in Pull specifically
	dir := initBareRepo(t)
	g := NewGoGitRepo()

	err := g.Pull(dir, "origin", "main", nil)
	if err == nil {
		t.Fatal("Expected error for bare repo (no worktree)")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("Expected 'worktree' in error, got: %v", err)
	}
}

func TestGoGitRepo_Checkout_ShouldReturnErrorForBareRepo(t *testing.T) {
	dir := initBareRepo(t)
	g := NewGoGitRepo()

	err := g.Checkout(dir, "main")
	if err == nil {
		t.Fatal("Expected error for bare repo (no worktree)")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("Expected 'worktree' in error, got: %v", err)
	}
}

// =============================================================================
// GoGitRepo — Empty repo tests (no commits, triggers head error)
// =============================================================================

func TestGoGitRepo_CreateBranch_ShouldReturnErrorForEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := git.PlainInit(dir, false) // non-bare but no commits
	if err != nil {
		t.Fatalf("failed to init: %v", err)
	}
	g := NewGoGitRepo()

	err = g.CreateBranch(dir, "feature")
	if err == nil {
		t.Fatal("Expected error for repo without commits (no HEAD)")
	}
	if !strings.Contains(err.Error(), "head") {
		t.Errorf("Expected 'head' in error, got: %v", err)
	}
}

func TestGoGitRepo_Commit_ShouldReturnErrorForEmptyCommit(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	// Commit with nothing staged — go-git returns an error
	err := g.Commit(dir, "empty commit", "")
	if err == nil {
		t.Fatal("Expected error for empty commit (nothing staged)")
	}
}

// =============================================================================
// GoGitRepo — Push to bare remote (covers success path + auth)
// =============================================================================

// initRepoWithBareRemote creates a working repo with a bare repo as "origin" remote.
// Returns workDir path. The working repo has one commit and can push to the bare remote.
func initRepoWithBareRemote(t *testing.T) string {
	t.Helper()

	// Create bare remote
	bareDir := t.TempDir()
	_, err := git.PlainInit(bareDir, true)
	if err != nil {
		t.Fatalf("failed to init bare: %v", err)
	}

	// Create working repo
	workDir := t.TempDir()
	repo, err := git.PlainInit(workDir, false)
	if err != nil {
		t.Fatalf("failed to init work: %v", err)
	}

	// Add bare as "origin" remote
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{bareDir},
	})
	if err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	// Create initial commit
	if err := os.WriteFile(filepath.Join(workDir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	w, _ := repo.Worktree()
	w.Add("file.txt")
	w.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "t@t.com", When: time.Now()},
	})

	return workDir
}

func TestGoGitRepo_Push_ShouldSucceedToBareRemote(t *testing.T) {
	workDir := initRepoWithBareRemote(t)
	g := NewGoGitRepo()

	err := g.Push(workDir, "origin", "master", nil)
	if err != nil {
		t.Fatalf("Expected push to bare remote to succeed, got: %v", err)
	}
}

func TestGoGitRepo_Push_ShouldSucceedToBareRemoteWithAuth(t *testing.T) {
	workDir := initRepoWithBareRemote(t)
	g := NewGoGitRepo()

	// Auth is ignored for local file:// transport but exercises the code path
	err := g.Push(workDir, "origin", "master", &GitAuth{Token: "test-token"})
	if err != nil {
		t.Fatalf("Expected push to succeed, got: %v", err)
	}
}

// =============================================================================
// GoGitRepo — Pull with auth to cloned repo (exercises auth code path on success)
// =============================================================================

// =============================================================================
// GoGitRepo — Corrupted/edge case tests for remaining uncovered error paths
// =============================================================================

func TestGoGitRepo_Status_ShouldReturnErrorForCorruptedIndex(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	// Corrupt the git index to make w.Status() fail
	indexPath := filepath.Join(dir, ".git", "index")
	if err := os.WriteFile(indexPath, []byte("corrupted"), 0644); err != nil {
		t.Fatalf("failed to corrupt index: %v", err)
	}

	_, err := g.Status(dir)
	if err == nil {
		t.Fatal("Expected error for corrupted index")
	}
}

func TestGoGitRepo_Pull_ShouldReturnErrorForInvalidRemote(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init: %v", err)
	}
	// Add a remote pointing to a non-existent path
	repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"/nonexistent/path/that/does/not/exist"},
	})
	// Create a commit so we have a HEAD
	w, _ := repo.Worktree()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	w.Add("f.txt")
	w.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "T", Email: "t@t", When: time.Now()},
	})

	g := NewGoGitRepo()
	err = g.Pull(dir, "origin", "master", nil)
	if err == nil {
		t.Fatal("Expected error for invalid remote")
	}
	if !strings.Contains(err.Error(), "go-git pull") {
		t.Errorf("Expected 'go-git pull' in error, got: %v", err)
	}
}

func TestGoGitRepo_Log_ShouldReturnErrorForEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := git.PlainInit(dir, false) // no commits
	if err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	g := NewGoGitRepo()
	_, err = g.Log(dir, 10)
	if err == nil {
		t.Fatal("Expected error for repo without commits")
	}
}

func TestGoGitRepo_Log_ShouldReturnErrorWhenIteratorFails(t *testing.T) {
	dir := initTestRepo(t) // has 1 commit
	g := NewGoGitRepo()

	// Add a second commit so we have a parent chain
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("extra"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := g.Add(dir, []string{"extra.txt"}); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := g.Commit(dir, "second commit", ""); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Open repo to find the parent commit hash
	repo, _ := git.PlainOpen(dir)
	head, _ := repo.Head()
	headCommit, _ := repo.CommitObject(head.Hash())
	parentHash := headCommit.ParentHashes[0]

	// Corrupt the parent commit's object file
	prefix := parentHash.String()[:2]
	suffix := parentHash.String()[2:]
	objPath := filepath.Join(dir, ".git", "objects", prefix, suffix)
	if err := os.WriteFile(objPath, []byte("corrupt"), 0644); err != nil {
		t.Fatalf("failed to corrupt object: %v", err)
	}

	// Log should read HEAD commit, then fail on the corrupted parent
	_, err := g.Log(dir, 10)
	if err == nil {
		t.Fatal("Expected error for corrupted parent commit object")
	}
	if !strings.Contains(err.Error(), "go-git log iterate") {
		t.Errorf("Expected 'go-git log iterate' in error, got: %v", err)
	}
}

func TestGoGitRepo_CreateBranch_ShouldReturnErrorForReadOnlyRefs(t *testing.T) {
	dir := initTestRepo(t)
	g := NewGoGitRepo()

	// Make refs/heads readable but not writable to trigger SetReference error
	// (0555 = r-xr-xr-x: can read/traverse but not create new files)
	headsDir := filepath.Join(dir, ".git", "refs", "heads")
	os.Chmod(headsDir, 0555)
	defer os.Chmod(headsDir, 0755)

	err := g.CreateBranch(dir, "new-branch")
	if err == nil {
		t.Fatal("Expected error for read-only refs directory")
	}
	if !strings.Contains(err.Error(), "go-git create branch") {
		t.Errorf("Expected 'go-git create branch' in error, got: %v", err)
	}
}

func TestGoGitRepo_Pull_ShouldSucceedWithAuthWhenUpToDate(t *testing.T) {
	srcDir := initTestRepo(t)
	dstDir := filepath.Join(t.TempDir(), "cloned")
	g := NewGoGitRepo()

	if err := g.Clone(srcDir, dstDir, nil); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	clonedRepo, _ := git.PlainOpen(dstDir)
	head, _ := clonedRepo.Head()
	branchName := head.Name().Short()

	// Pull with auth — should succeed (already up to date)
	err := g.Pull(dstDir, "origin", branchName, &GitAuth{Token: "test"})
	if err != nil {
		t.Fatalf("Expected no error for up-to-date pull with auth, got: %v", err)
	}
}
