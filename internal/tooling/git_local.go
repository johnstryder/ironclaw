package tooling

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// GoGitRepo implements GitRepo using go-git for local operations.
type GoGitRepo struct{}

// NewGoGitRepo returns a new GoGitRepo instance.
func NewGoGitRepo() *GoGitRepo {
	return &GoGitRepo{}
}

func goGitAuth(auth *GitAuth) *http.BasicAuth {
	if auth == nil || auth.Token == "" {
		return nil
	}
	username := auth.Username
	if username == "" {
		username = "git"
	}
	return &http.BasicAuth{
		Username: username,
		Password: auth.Token,
	}
}

// Clone clones a remote repository to the given path.
func (g *GoGitRepo) Clone(url, path string, auth *GitAuth) error {
	opts := &git.CloneOptions{
		URL:      url,
		Progress: nil,
	}
	if a := goGitAuth(auth); a != nil {
		opts.Auth = a
	}
	_, err := git.PlainClone(path, false, opts)
	if err != nil {
		return fmt.Errorf("go-git clone: %w", err)
	}
	return nil
}

// Status returns the working tree status as a formatted string.
func (g *GoGitRepo) Status(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("go-git open: %w", err)
	}
	w, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("go-git worktree: %w", err)
	}
	status, err := w.Status()
	if err != nil {
		return "", fmt.Errorf("go-git status: %w", err)
	}
	return status.String(), nil
}

// Add stages the given files in the working tree.
func (g *GoGitRepo) Add(path string, files []string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("go-git open: %w", err)
	}
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("go-git worktree: %w", err)
	}
	for _, f := range files {
		if _, err := w.Add(f); err != nil {
			return fmt.Errorf("go-git add %s: %w", f, err)
		}
	}
	return nil
}

// Commit creates a new commit with the staged changes.
func (g *GoGitRepo) Commit(path, message, author string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("go-git open: %w", err)
	}
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("go-git worktree: %w", err)
	}

	authorName := "Ironclaw"
	authorEmail := "ironclaw@local"
	if author != "" {
		parts := strings.SplitN(author, " <", 2)
		authorName = parts[0]
		if len(parts) == 2 {
			authorEmail = strings.TrimSuffix(parts[1], ">")
		}
	}

	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("go-git commit: %w", err)
	}
	return nil
}

// Push pushes commits to the given remote and branch.
func (g *GoGitRepo) Push(path, remote, branch string, auth *GitAuth) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("go-git open: %w", err)
	}

	refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))
	opts := &git.PushOptions{
		RemoteName: remote,
		RefSpecs:   []config.RefSpec{refSpec},
	}
	if a := goGitAuth(auth); a != nil {
		opts.Auth = a
	}

	if err := repo.Push(opts); err != nil {
		return fmt.Errorf("go-git push: %w", err)
	}
	return nil
}

// Pull fetches and merges changes from the given remote and branch.
func (g *GoGitRepo) Pull(path, remote, branch string, auth *GitAuth) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("go-git open: %w", err)
	}
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("go-git worktree: %w", err)
	}

	opts := &git.PullOptions{
		RemoteName:    remote,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	}
	if a := goGitAuth(auth); a != nil {
		opts.Auth = a
	}

	if err := w.Pull(opts); err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("go-git pull: %w", err)
	}
	return nil
}

// Log returns the most recent commit entries.
func (g *GoGitRepo) Log(path string, limit int) ([]GitLogEntry, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("go-git open: %w", err)
	}

	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, fmt.Errorf("go-git log: %w", err)
	}
	defer iter.Close()

	var entries []GitLogEntry
	count := 0
	err = iter.ForEach(func(c *object.Commit) error {
		if count >= limit {
			return fmt.Errorf("limit reached")
		}
		entries = append(entries, GitLogEntry{
			Hash:    c.Hash.String(),
			Author:  c.Author.Name,
			Date:    c.Author.When.Format(time.RFC3339),
			Message: strings.TrimSpace(c.Message),
		})
		count++
		return nil
	})
	// "limit reached" is our own sentinel, not a real error
	if err != nil && err.Error() != "limit reached" {
		return nil, fmt.Errorf("go-git log iterate: %w", err)
	}

	return entries, nil
}

// CreateBranch creates a new branch at the current HEAD.
func (g *GoGitRepo) CreateBranch(path, branch string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("go-git open: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("go-git head: %w", err)
	}

	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), head.Hash())
	if err := repo.Storer.SetReference(ref); err != nil {
		return fmt.Errorf("go-git create branch: %w", err)
	}
	return nil
}

// Checkout switches the working tree to the given branch.
func (g *GoGitRepo) Checkout(path, branch string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("go-git open: %w", err)
	}
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("go-git worktree: %w", err)
	}

	if err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
	}); err != nil {
		return fmt.Errorf("go-git checkout: %w", err)
	}
	return nil
}
