package tooling

import (
	"context"
	"fmt"

	"github.com/google/go-github/v68/github"
)

// GitHubProvider implements GitRemoteProvider using the GitHub API.
type GitHubProvider struct {
	client *github.Client
}

// NewGitHubProvider creates a GitHubProvider authenticated with the given token.
func NewGitHubProvider(token string) *GitHubProvider {
	client := github.NewClient(nil).WithAuthToken(token)
	return &GitHubProvider{client: client}
}

// ListIssues returns open issues for the given repository.
func (g *GitHubProvider) ListIssues(owner, repo string) ([]GitIssue, error) {
	ctx := context.Background()
	issues, _, err := g.client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
		State: "open",
	})
	if err != nil {
		return nil, fmt.Errorf("github list issues: %w", err)
	}

	var result []GitIssue
	for _, issue := range issues {
		// Skip pull requests (GitHub returns PRs in the issues endpoint)
		if issue.PullRequestLinks != nil {
			continue
		}
		result = append(result, GitIssue{
			Number: issue.GetNumber(),
			Title:  issue.GetTitle(),
			Body:   issue.GetBody(),
			State:  issue.GetState(),
			URL:    issue.GetHTMLURL(),
		})
	}
	return result, nil
}

// CreateIssue creates a new issue in the given repository.
func (g *GitHubProvider) CreateIssue(owner, repo, title, body string) (*GitIssue, error) {
	ctx := context.Background()
	issue, _, err := g.client.Issues.Create(ctx, owner, repo, &github.IssueRequest{
		Title: &title,
		Body:  &body,
	})
	if err != nil {
		return nil, fmt.Errorf("github create issue: %w", err)
	}

	return &GitIssue{
		Number: issue.GetNumber(),
		Title:  issue.GetTitle(),
		Body:   issue.GetBody(),
		State:  issue.GetState(),
		URL:    issue.GetHTMLURL(),
	}, nil
}

// ListPullRequests returns open pull requests for the given repository.
func (g *GitHubProvider) ListPullRequests(owner, repo string) ([]GitPullRequest, error) {
	ctx := context.Background()
	prs, _, err := g.client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		State: "open",
	})
	if err != nil {
		return nil, fmt.Errorf("github list PRs: %w", err)
	}

	var result []GitPullRequest
	for _, pr := range prs {
		result = append(result, GitPullRequest{
			Number: pr.GetNumber(),
			Title:  pr.GetTitle(),
			Body:   pr.GetBody(),
			State:  pr.GetState(),
			URL:    pr.GetHTMLURL(),
			Base:   pr.GetBase().GetRef(),
			Head:   pr.GetHead().GetRef(),
		})
	}
	return result, nil
}

// CreatePullRequest creates a new pull request in the given repository.
func (g *GitHubProvider) CreatePullRequest(owner, repo, title, body, base, head string) (*GitPullRequest, error) {
	ctx := context.Background()
	pr, _, err := g.client.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Base:  &base,
		Head:  &head,
	})
	if err != nil {
		return nil, fmt.Errorf("github create PR: %w", err)
	}

	return &GitPullRequest{
		Number: pr.GetNumber(),
		Title:  pr.GetTitle(),
		Body:   pr.GetBody(),
		State:  pr.GetState(),
		URL:    pr.GetHTMLURL(),
		Base:   pr.GetBase().GetRef(),
		Head:   pr.GetHead().GetRef(),
	}, nil
}

// CommentOnPR adds a comment to a pull request.
func (g *GitHubProvider) CommentOnPR(owner, repo string, number int, body string) error {
	ctx := context.Background()
	_, _, err := g.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("github comment on PR: %w", err)
	}
	return nil
}
