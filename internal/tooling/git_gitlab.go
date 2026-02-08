package tooling

import (
	"fmt"

	gitlab "github.com/xanzy/go-gitlab"
)

// GitLabProvider implements GitRemoteProvider using the GitLab API.
type GitLabProvider struct {
	client *gitlab.Client
}

// NewGitLabProvider creates a GitLabProvider authenticated with the given token.
// The optional baseURL parameter allows connecting to self-hosted GitLab instances.
func NewGitLabProvider(token string, baseURL ...string) (*GitLabProvider, error) {
	opts := []gitlab.ClientOptionFunc{}
	if len(baseURL) > 0 && baseURL[0] != "" {
		opts = append(opts, gitlab.WithBaseURL(baseURL[0]))
	}
	client, err := gitlab.NewClient(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("gitlab client init: %w", err)
	}
	return &GitLabProvider{client: client}, nil
}

// projectPath returns the GitLab project path "owner/repo".
func projectPath(owner, repo string) string {
	return owner + "/" + repo
}

// ListIssues returns open issues for the given project.
func (g *GitLabProvider) ListIssues(owner, repo string) ([]GitIssue, error) {
	state := "opened"
	issues, _, err := g.client.Issues.ListProjectIssues(projectPath(owner, repo), &gitlab.ListProjectIssuesOptions{
		State: &state,
	})
	if err != nil {
		return nil, fmt.Errorf("gitlab list issues: %w", err)
	}

	var result []GitIssue
	for _, issue := range issues {
		result = append(result, GitIssue{
			Number: issue.IID,
			Title:  issue.Title,
			Body:   issue.Description,
			State:  issue.State,
			URL:    issue.WebURL,
		})
	}
	return result, nil
}

// CreateIssue creates a new issue in the given project.
func (g *GitLabProvider) CreateIssue(owner, repo, title, body string) (*GitIssue, error) {
	issue, _, err := g.client.Issues.CreateIssue(projectPath(owner, repo), &gitlab.CreateIssueOptions{
		Title:       &title,
		Description: &body,
	})
	if err != nil {
		return nil, fmt.Errorf("gitlab create issue: %w", err)
	}

	return &GitIssue{
		Number: issue.IID,
		Title:  issue.Title,
		Body:   issue.Description,
		State:  issue.State,
		URL:    issue.WebURL,
	}, nil
}

// ListPullRequests returns open merge requests for the given project.
func (g *GitLabProvider) ListPullRequests(owner, repo string) ([]GitPullRequest, error) {
	state := "opened"
	mrs, _, err := g.client.MergeRequests.ListProjectMergeRequests(projectPath(owner, repo), &gitlab.ListProjectMergeRequestsOptions{
		State: &state,
	})
	if err != nil {
		return nil, fmt.Errorf("gitlab list merge requests: %w", err)
	}

	var result []GitPullRequest
	for _, mr := range mrs {
		result = append(result, GitPullRequest{
			Number: mr.IID,
			Title:  mr.Title,
			Body:   mr.Description,
			State:  mr.State,
			URL:    mr.WebURL,
			Base:   mr.TargetBranch,
			Head:   mr.SourceBranch,
		})
	}
	return result, nil
}

// CreatePullRequest creates a new merge request in the given project.
func (g *GitLabProvider) CreatePullRequest(owner, repo, title, body, base, head string) (*GitPullRequest, error) {
	mr, _, err := g.client.MergeRequests.CreateMergeRequest(projectPath(owner, repo), &gitlab.CreateMergeRequestOptions{
		Title:        &title,
		Description:  &body,
		TargetBranch: &base,
		SourceBranch: &head,
	})
	if err != nil {
		return nil, fmt.Errorf("gitlab create merge request: %w", err)
	}

	return &GitPullRequest{
		Number: mr.IID,
		Title:  mr.Title,
		Body:   mr.Description,
		State:  mr.State,
		URL:    mr.WebURL,
		Base:   mr.TargetBranch,
		Head:   mr.SourceBranch,
	}, nil
}

// CommentOnPR adds a comment (note) to a merge request.
func (g *GitLabProvider) CommentOnPR(owner, repo string, number int, body string) error {
	_, _, err := g.client.Notes.CreateMergeRequestNote(projectPath(owner, repo), number, &gitlab.CreateMergeRequestNoteOptions{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("gitlab comment on MR: %w", err)
	}
	return nil
}
