package tooling

import (
	"encoding/json"
	"fmt"

	"ironclaw/internal/domain"
	"ironclaw/internal/secrets"
)

// =============================================================================
// Types
// =============================================================================

// GitAuth holds authentication credentials for git operations.
type GitAuth struct {
	Token    string
	Username string
}

// GitLogEntry represents a single git log entry.
type GitLogEntry struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// GitIssue represents an issue on a remote git platform.
type GitIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	URL    string `json:"url"`
}

// GitPullRequest represents a pull request on a remote git platform.
type GitPullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	URL    string `json:"url"`
	Base   string `json:"base"`
	Head   string `json:"head"`
}

// =============================================================================
// Interfaces
// =============================================================================

// GitRepo abstracts local git repository operations for testability.
type GitRepo interface {
	Clone(url, path string, auth *GitAuth) error
	Status(path string) (string, error)
	Add(path string, files []string) error
	Commit(path, message, author string) error
	Push(path, remote, branch string, auth *GitAuth) error
	Pull(path, remote, branch string, auth *GitAuth) error
	Log(path string, limit int) ([]GitLogEntry, error)
	CreateBranch(path, branch string) error
	Checkout(path, branch string) error
}

// GitRemoteProvider abstracts remote git platform API operations for testability.
type GitRemoteProvider interface {
	ListIssues(owner, repo string) ([]GitIssue, error)
	CreateIssue(owner, repo, title, body string) (*GitIssue, error)
	ListPullRequests(owner, repo string) ([]GitPullRequest, error)
	CreatePullRequest(owner, repo, title, body, base, head string) (*GitPullRequest, error)
	CommentOnPR(owner, repo string, number int, body string) error
}

// GitProviderFactory creates GitRemoteProvider instances based on provider type and token.
type GitProviderFactory func(provider, token string) (GitRemoteProvider, error)

// =============================================================================
// Input Schema
// =============================================================================

// GitInput represents the input structure for git operations.
type GitInput struct {
	Operation  string   `json:"operation" jsonschema:"enum=clone,enum=status,enum=add,enum=commit,enum=push,enum=pull,enum=log,enum=branch,enum=checkout,enum=list_issues,enum=create_issue,enum=list_prs,enum=create_pr,enum=comment_pr"`
	URL        string   `json:"url,omitempty"`
	Path       string   `json:"path,omitempty"`
	Message    string   `json:"message,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Remote     string   `json:"remote,omitempty"`
	Files      []string `json:"files,omitempty"`
	Author     string   `json:"author,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Provider   string   `json:"provider,omitempty" jsonschema:"enum=github,enum=gitlab"`
	Owner      string   `json:"owner,omitempty"`
	Repo       string   `json:"repo,omitempty"`
	Title      string   `json:"title,omitempty"`
	Body       string   `json:"body,omitempty"`
	Number     int      `json:"number,omitempty"`
	Comment    string   `json:"comment,omitempty"`
	BaseBranch string   `json:"base_branch,omitempty"`
	HeadBranch string   `json:"head_branch,omitempty"`
}

// gitUnmarshalFunc is the JSON unmarshaler used by Call. Package-level so
// tests can inject a failing unmarshaler to cover the defense-in-depth error path.
var gitUnmarshalFunc = json.Unmarshal

// gitMarshalFunc is the JSON marshaler used by result serialization. Package-level
// so tests can inject a failing marshaler to cover defense-in-depth error paths.
var gitMarshalFunc = json.Marshal

// =============================================================================
// GitTool
// =============================================================================

// GitTool provides integrated GitHub and GitLab management.
// It supports local git operations (clone, commit, push) via go-git and
// remote operations (issues, PRs) via platform-specific API clients.
type GitTool struct {
	secrets         secrets.SecretsManager
	repo            GitRepo
	providerFactory GitProviderFactory
}

// NewGitTool creates a GitTool with the given dependencies.
func NewGitTool(sm secrets.SecretsManager, repo GitRepo, factory GitProviderFactory) *GitTool {
	return &GitTool{
		secrets:         sm,
		repo:            repo,
		providerFactory: factory,
	}
}

// newGitLabProviderFunc is the constructor for GitLabProvider. Package-level so
// tests can inject a failing constructor to cover the error path.
var newGitLabProviderFunc = func(token string) (GitRemoteProvider, error) {
	return NewGitLabProvider(token)
}

// DefaultGitProviderFactory creates the appropriate real GitRemoteProvider
// based on the provider type string ("github" or "gitlab").
func DefaultGitProviderFactory(providerType, token string) (GitRemoteProvider, error) {
	switch providerType {
	case "github":
		return NewGitHubProvider(token), nil
	case "gitlab":
		p, err := newGitLabProviderFunc(token)
		if err != nil {
			return nil, err
		}
		return p, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerType)
	}
}

// Name returns the tool name used in function-calling.
func (g *GitTool) Name() string { return "git" }

// Description returns a human-readable description for the LLM.
func (g *GitTool) Description() string {
	return "Integrated Git, GitHub, and GitLab management: local operations (clone, commit, push) and remote operations (issues, PRs)"
}

// Definition returns the JSON Schema for git input.
func (g *GitTool) Definition() string {
	return GenerateSchema(GitInput{})
}

// Call validates the input and dispatches to the appropriate git operation.
func (g *GitTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	schema := g.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	var input GitInput
	if err := gitUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	switch input.Operation {
	case "clone":
		return g.clone(input)
	case "status":
		return g.status(input)
	case "add":
		return g.add(input)
	case "commit":
		return g.commit(input)
	case "push":
		return g.push(input)
	case "pull":
		return g.pull(input)
	case "log":
		return g.gitLog(input)
	case "branch":
		return g.createBranch(input)
	case "checkout":
		return g.checkout(input)
	case "list_issues":
		return g.listIssues(input)
	case "create_issue":
		return g.createIssue(input)
	case "list_prs":
		return g.listPRs(input)
	case "create_pr":
		return g.createPR(input)
	case "comment_pr":
		return g.commentPR(input)
	default:
		return nil, fmt.Errorf("unknown operation: %s", input.Operation)
	}
}

// =============================================================================
// Local Git Operations
// =============================================================================

func (g *GitTool) clone(input GitInput) (*domain.ToolResult, error) {
	if input.URL == "" {
		return nil, fmt.Errorf("url is required for clone")
	}
	if input.Path == "" {
		return nil, fmt.Errorf("path is required for clone")
	}

	var auth *GitAuth
	token, err := g.secrets.Get("github_token")
	if err == nil && token != "" {
		auth = &GitAuth{Token: token, Username: "git"}
	}

	if err := g.repo.Clone(input.URL, input.Path, auth); err != nil {
		return nil, fmt.Errorf("clone failed: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Successfully cloned %s to %s", input.URL, input.Path),
		Metadata: map[string]string{
			"operation": "clone",
			"url":       input.URL,
			"path":      input.Path,
		},
	}, nil
}

func (g *GitTool) status(input GitInput) (*domain.ToolResult, error) {
	if input.Path == "" {
		return nil, fmt.Errorf("path is required for status")
	}

	result, err := g.repo.Status(input.Path)
	if err != nil {
		return nil, fmt.Errorf("status failed: %w", err)
	}

	return &domain.ToolResult{
		Data: result,
		Metadata: map[string]string{
			"operation": "status",
			"path":      input.Path,
		},
	}, nil
}

func (g *GitTool) add(input GitInput) (*domain.ToolResult, error) {
	if input.Path == "" {
		return nil, fmt.Errorf("path is required for add")
	}
	if len(input.Files) == 0 {
		return nil, fmt.Errorf("files are required for add")
	}

	if err := g.repo.Add(input.Path, input.Files); err != nil {
		return nil, fmt.Errorf("add failed: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Added %d file(s) to staging", len(input.Files)),
		Metadata: map[string]string{
			"operation": "add",
			"path":      input.Path,
		},
	}, nil
}

func (g *GitTool) commit(input GitInput) (*domain.ToolResult, error) {
	if input.Path == "" {
		return nil, fmt.Errorf("path is required for commit")
	}
	if input.Message == "" {
		return nil, fmt.Errorf("message is required for commit")
	}

	if err := g.repo.Commit(input.Path, input.Message, input.Author); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Committed with message: %s", input.Message),
		Metadata: map[string]string{
			"operation": "commit",
			"path":      input.Path,
			"message":   input.Message,
		},
	}, nil
}

func (g *GitTool) push(input GitInput) (*domain.ToolResult, error) {
	if input.Path == "" {
		return nil, fmt.Errorf("path is required for push")
	}

	remote := input.Remote
	if remote == "" {
		remote = "origin"
	}
	branch := input.Branch
	if branch == "" {
		branch = "main"
	}

	var auth *GitAuth
	token, err := g.secrets.Get("github_token")
	if err == nil && token != "" {
		auth = &GitAuth{Token: token, Username: "git"}
	}

	if err := g.repo.Push(input.Path, remote, branch, auth); err != nil {
		return nil, fmt.Errorf("push failed: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Pushed to %s/%s", remote, branch),
		Metadata: map[string]string{
			"operation": "push",
			"path":      input.Path,
			"remote":    remote,
			"branch":    branch,
		},
	}, nil
}

func (g *GitTool) pull(input GitInput) (*domain.ToolResult, error) {
	if input.Path == "" {
		return nil, fmt.Errorf("path is required for pull")
	}

	remote := input.Remote
	if remote == "" {
		remote = "origin"
	}
	branch := input.Branch
	if branch == "" {
		branch = "main"
	}

	var auth *GitAuth
	token, err := g.secrets.Get("github_token")
	if err == nil && token != "" {
		auth = &GitAuth{Token: token, Username: "git"}
	}

	if err := g.repo.Pull(input.Path, remote, branch, auth); err != nil {
		return nil, fmt.Errorf("pull failed: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Pulled from %s/%s", remote, branch),
		Metadata: map[string]string{
			"operation": "pull",
			"path":      input.Path,
			"remote":    remote,
			"branch":    branch,
		},
	}, nil
}

func (g *GitTool) gitLog(input GitInput) (*domain.ToolResult, error) {
	if input.Path == "" {
		return nil, fmt.Errorf("path is required for log")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}

	entries, err := g.repo.Log(input.Path, limit)
	if err != nil {
		return nil, fmt.Errorf("log failed: %w", err)
	}

	data, err := gitMarshalFunc(entries)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log entries: %w", err)
	}

	return &domain.ToolResult{
		Data: string(data),
		Metadata: map[string]string{
			"operation": "log",
			"path":      input.Path,
			"count":     fmt.Sprintf("%d", len(entries)),
		},
	}, nil
}

func (g *GitTool) createBranch(input GitInput) (*domain.ToolResult, error) {
	if input.Path == "" {
		return nil, fmt.Errorf("path is required for branch")
	}
	if input.Branch == "" {
		return nil, fmt.Errorf("branch name is required for branch")
	}

	if err := g.repo.CreateBranch(input.Path, input.Branch); err != nil {
		return nil, fmt.Errorf("branch creation failed: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Created branch: %s", input.Branch),
		Metadata: map[string]string{
			"operation": "branch",
			"path":      input.Path,
			"branch":    input.Branch,
		},
	}, nil
}

func (g *GitTool) checkout(input GitInput) (*domain.ToolResult, error) {
	if input.Path == "" {
		return nil, fmt.Errorf("path is required for checkout")
	}
	if input.Branch == "" {
		return nil, fmt.Errorf("branch name is required for checkout")
	}

	if err := g.repo.Checkout(input.Path, input.Branch); err != nil {
		return nil, fmt.Errorf("checkout failed: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Checked out branch: %s", input.Branch),
		Metadata: map[string]string{
			"operation": "checkout",
			"path":      input.Path,
			"branch":    input.Branch,
		},
	}, nil
}

// =============================================================================
// Remote Git Operations
// =============================================================================

// getRemoteProvider resolves the token from SecretsManager and creates a remote provider.
func (g *GitTool) getRemoteProvider(input GitInput) (GitRemoteProvider, error) {
	if input.Provider == "" {
		return nil, fmt.Errorf("provider is required for remote operations")
	}

	secretKey := input.Provider + "_token"
	token, err := g.secrets.Get(secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s token: %w", input.Provider, err)
	}

	provider, err := g.providerFactory(input.Provider, token)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s provider: %w", input.Provider, err)
	}

	return provider, nil
}

func (g *GitTool) listIssues(input GitInput) (*domain.ToolResult, error) {
	if input.Owner == "" || input.Repo == "" {
		return nil, fmt.Errorf("owner and repo are required for list_issues")
	}

	provider, err := g.getRemoteProvider(input)
	if err != nil {
		return nil, err
	}

	issues, err := provider.ListIssues(input.Owner, input.Repo)
	if err != nil {
		return nil, fmt.Errorf("list issues failed: %w", err)
	}

	data, err := gitMarshalFunc(issues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal issues: %w", err)
	}

	return &domain.ToolResult{
		Data: string(data),
		Metadata: map[string]string{
			"operation": "list_issues",
			"owner":     input.Owner,
			"repo":      input.Repo,
			"count":     fmt.Sprintf("%d", len(issues)),
		},
	}, nil
}

func (g *GitTool) createIssue(input GitInput) (*domain.ToolResult, error) {
	if input.Owner == "" || input.Repo == "" {
		return nil, fmt.Errorf("owner and repo are required for create_issue")
	}
	if input.Title == "" {
		return nil, fmt.Errorf("title is required for create_issue")
	}

	provider, err := g.getRemoteProvider(input)
	if err != nil {
		return nil, err
	}

	issue, err := provider.CreateIssue(input.Owner, input.Repo, input.Title, input.Body)
	if err != nil {
		return nil, fmt.Errorf("create issue failed: %w", err)
	}

	data, err := gitMarshalFunc(issue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal issue: %w", err)
	}

	return &domain.ToolResult{
		Data: string(data),
		Metadata: map[string]string{
			"operation": "create_issue",
			"owner":     input.Owner,
			"repo":      input.Repo,
			"url":       issue.URL,
		},
	}, nil
}

func (g *GitTool) listPRs(input GitInput) (*domain.ToolResult, error) {
	if input.Owner == "" || input.Repo == "" {
		return nil, fmt.Errorf("owner and repo are required for list_prs")
	}

	provider, err := g.getRemoteProvider(input)
	if err != nil {
		return nil, err
	}

	prs, err := provider.ListPullRequests(input.Owner, input.Repo)
	if err != nil {
		return nil, fmt.Errorf("list pull requests failed: %w", err)
	}

	data, err := gitMarshalFunc(prs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pull requests: %w", err)
	}

	return &domain.ToolResult{
		Data: string(data),
		Metadata: map[string]string{
			"operation": "list_prs",
			"owner":     input.Owner,
			"repo":      input.Repo,
			"count":     fmt.Sprintf("%d", len(prs)),
		},
	}, nil
}

func (g *GitTool) createPR(input GitInput) (*domain.ToolResult, error) {
	if input.Owner == "" || input.Repo == "" {
		return nil, fmt.Errorf("owner and repo are required for create_pr")
	}
	if input.Title == "" {
		return nil, fmt.Errorf("title is required for create_pr")
	}
	if input.BaseBranch == "" || input.HeadBranch == "" {
		return nil, fmt.Errorf("base_branch and head_branch are required for create_pr")
	}

	provider, err := g.getRemoteProvider(input)
	if err != nil {
		return nil, err
	}

	pr, err := provider.CreatePullRequest(input.Owner, input.Repo, input.Title, input.Body, input.BaseBranch, input.HeadBranch)
	if err != nil {
		return nil, fmt.Errorf("create pull request failed: %w", err)
	}

	data, err := gitMarshalFunc(pr)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pull request: %w", err)
	}

	return &domain.ToolResult{
		Data: string(data),
		Metadata: map[string]string{
			"operation": "create_pr",
			"owner":     input.Owner,
			"repo":      input.Repo,
			"url":       pr.URL,
		},
	}, nil
}

func (g *GitTool) commentPR(input GitInput) (*domain.ToolResult, error) {
	if input.Owner == "" || input.Repo == "" {
		return nil, fmt.Errorf("owner and repo are required for comment_pr")
	}
	if input.Number <= 0 {
		return nil, fmt.Errorf("number is required for comment_pr")
	}
	if input.Comment == "" {
		return nil, fmt.Errorf("comment is required for comment_pr")
	}

	provider, err := g.getRemoteProvider(input)
	if err != nil {
		return nil, err
	}

	if err := provider.CommentOnPR(input.Owner, input.Repo, input.Number, input.Comment); err != nil {
		return nil, fmt.Errorf("comment on PR failed: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Commented on PR #%d", input.Number),
		Metadata: map[string]string{
			"operation": "comment_pr",
			"owner":     input.Owner,
			"repo":      input.Repo,
			"number":    fmt.Sprintf("%d", input.Number),
		},
	}, nil
}
