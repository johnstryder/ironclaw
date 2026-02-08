package tooling

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gitlab "github.com/xanzy/go-gitlab"
)

// newTestGitLabProvider creates a GitLabProvider backed by a test HTTP server.
// The handler receives all requests; callers inspect r.URL to route.
func newTestGitLabProvider(t *testing.T, handler http.HandlerFunc) *GitLabProvider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	if err != nil {
		t.Fatalf("failed to create gitlab client: %v", err)
	}

	return &GitLabProvider{client: client}
}

// gitlabRawPath returns the raw path from a request, handling %2F encoding.
func gitlabRawPath(r *http.Request) string {
	if r.URL.RawPath != "" {
		return r.URL.RawPath
	}
	return r.URL.Path
}

// =============================================================================
// projectPath
// =============================================================================

func TestProjectPath_ShouldReturnOwnerSlashRepo(t *testing.T) {
	result := projectPath("mygroup", "myproject")
	if result != "mygroup/myproject" {
		t.Errorf("Expected 'mygroup/myproject', got '%s'", result)
	}
}

// =============================================================================
// NewGitLabProvider
// =============================================================================

func TestNewGitLabProvider_ShouldReturnProviderWithoutBaseURL(t *testing.T) {
	provider, err := NewGitLabProvider("glpat-test")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}
}

func TestNewGitLabProvider_ShouldReturnProviderWithBaseURL(t *testing.T) {
	provider, err := NewGitLabProvider("glpat-test", "https://gitlab.example.com/api/v4")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}
}

func TestNewGitLabProvider_ShouldIgnoreEmptyBaseURL(t *testing.T) {
	provider, err := NewGitLabProvider("glpat-test", "")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}
}

func TestNewGitLabProvider_ShouldReturnErrorForInvalidBaseURL(t *testing.T) {
	_, err := NewGitLabProvider("glpat-test", "://invalid")
	if err == nil {
		t.Fatal("Expected error for invalid base URL")
	}
	if !strings.Contains(err.Error(), "gitlab client init") {
		t.Errorf("Expected 'gitlab client init' in error, got: %v", err)
	}
}

// =============================================================================
// GitLabProvider — ListIssues
// =============================================================================

func TestGitLabProvider_ListIssues_ShouldReturnIssues(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		raw := gitlabRawPath(r)
		if strings.Contains(raw, "owner%2Frepo/issues") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{"id":101,"iid":1,"title":"Bug","description":"desc","state":"opened","web_url":"https://gitlab.com/owner/repo/-/issues/1","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z","author":{"id":1,"username":"test","name":"Test"}},
				{"id":102,"iid":2,"title":"Feature","description":"need","state":"opened","web_url":"https://gitlab.com/owner/repo/-/issues/2","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z","author":{"id":1,"username":"test","name":"Test"}}
			]`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	issues, err := provider.ListIssues("owner", "repo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("Expected 2 issues, got %d", len(issues))
	}
	if issues[0].Number != 1 || issues[0].Title != "Bug" {
		t.Errorf("Expected issue #1 'Bug', got #%d '%s'", issues[0].Number, issues[0].Title)
	}
	if issues[0].Body != "desc" {
		t.Errorf("Expected body 'desc', got '%s'", issues[0].Body)
	}
	if issues[0].State != "opened" {
		t.Errorf("Expected state 'opened', got '%s'", issues[0].State)
	}
}

func TestGitLabProvider_ListIssues_ShouldReturnEmptyForNoIssues(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})

	issues, err := provider.ListIssues("owner", "repo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues, got %d", len(issues))
	}
}

func TestGitLabProvider_ListIssues_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := provider.ListIssues("owner", "repo")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "gitlab list issues") {
		t.Errorf("Expected 'gitlab list issues' in error, got: %v", err)
	}
}

// =============================================================================
// GitLabProvider — CreateIssue
// =============================================================================

func TestGitLabProvider_CreateIssue_ShouldCreateIssue(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		raw := gitlabRawPath(r)
		if strings.Contains(raw, "owner%2Frepo/issues") && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":142,"iid":42,"title":"Bug Report","description":"broke","state":"opened","web_url":"https://gitlab.com/owner/repo/-/issues/42","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z","author":{"id":1,"username":"test","name":"Test"}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	issue, err := provider.CreateIssue("owner", "repo", "Bug Report", "broke")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if issue.Number != 42 {
		t.Errorf("Expected issue #42, got #%d", issue.Number)
	}
	if issue.Title != "Bug Report" {
		t.Errorf("Expected title 'Bug Report', got '%s'", issue.Title)
	}
}

func TestGitLabProvider_CreateIssue_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := provider.CreateIssue("owner", "repo", "title", "body")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "gitlab create issue") {
		t.Errorf("Expected 'gitlab create issue' in error, got: %v", err)
	}
}

// =============================================================================
// GitLabProvider — ListPullRequests (Merge Requests)
// =============================================================================

func TestGitLabProvider_ListPullRequests_ShouldReturnMergeRequests(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		raw := gitlabRawPath(r)
		if strings.Contains(raw, "owner%2Frepo/merge_requests") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{
					"id":110,"iid":10,"title":"Fix","description":"fixing","state":"opened",
					"web_url":"https://gitlab.com/owner/repo/-/merge_requests/10",
					"target_branch":"main","source_branch":"fix-branch",
					"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z",
					"author":{"id":1,"username":"test","name":"Test"}
				}
			]`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	prs, err := provider.ListPullRequests("owner", "repo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("Expected 1 MR, got %d", len(prs))
	}
	if prs[0].Number != 10 || prs[0].Title != "Fix" {
		t.Errorf("Expected MR #10 'Fix', got #%d '%s'", prs[0].Number, prs[0].Title)
	}
	if prs[0].Base != "main" || prs[0].Head != "fix-branch" {
		t.Errorf("Expected base=main head=fix-branch, got base=%s head=%s", prs[0].Base, prs[0].Head)
	}
}

func TestGitLabProvider_ListPullRequests_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := provider.ListPullRequests("owner", "repo")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "gitlab list merge requests") {
		t.Errorf("Expected 'gitlab list merge requests' in error, got: %v", err)
	}
}

// =============================================================================
// GitLabProvider — CreatePullRequest (Merge Request)
// =============================================================================

func TestGitLabProvider_CreatePullRequest_ShouldCreateMergeRequest(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		raw := gitlabRawPath(r)
		if strings.Contains(raw, "owner%2Frepo/merge_requests") && r.Method == http.MethodPost {
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{
				"id":105,"iid":5,"title":"Feature","description":"new feature","state":"opened",
				"web_url":"https://gitlab.com/owner/repo/-/merge_requests/5",
				"target_branch":"main","source_branch":"feature",
				"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z",
				"author":{"id":1,"username":"test","name":"Test"}
			}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	pr, err := provider.CreatePullRequest("owner", "repo", "Feature", "new feature", "main", "feature")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if pr.Number != 5 {
		t.Errorf("Expected MR #5, got #%d", pr.Number)
	}
	if pr.Base != "main" || pr.Head != "feature" {
		t.Errorf("Expected base=main head=feature, got base=%s head=%s", pr.Base, pr.Head)
	}
}

func TestGitLabProvider_CreatePullRequest_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	_, err := provider.CreatePullRequest("owner", "repo", "T", "B", "main", "dev")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "gitlab create merge request") {
		t.Errorf("Expected 'gitlab create merge request' in error, got: %v", err)
	}
}

// =============================================================================
// GitLabProvider — CommentOnPR (Merge Request Note)
// =============================================================================

func TestGitLabProvider_CommentOnPR_ShouldAddNote(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		raw := gitlabRawPath(r)
		if strings.Contains(raw, "merge_requests/42/notes") && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":1,"body":"LGTM!"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	err := provider.CommentOnPR("owner", "repo", 42, "LGTM!")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestGitLabProvider_CommentOnPR_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider := newTestGitLabProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	err := provider.CommentOnPR("owner", "repo", 1, "test")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "gitlab comment on MR") {
		t.Errorf("Expected 'gitlab comment on MR' in error, got: %v", err)
	}
}
