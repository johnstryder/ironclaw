package tooling

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-github/v68/github"
)

// newTestGitHubProvider creates a GitHubProvider backed by a test HTTP server.
// Returns the provider and a ServeMux you can add handlers to.
func newTestGitHubProvider(t *testing.T) (*GitHubProvider, *http.ServeMux) {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := github.NewClient(nil).WithAuthToken("test-token")
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	client.UploadURL = u

	return &GitHubProvider{client: client}, mux
}

// =============================================================================
// GitHubProvider — ListIssues
// =============================================================================

func TestGitHubProvider_ListIssues_ShouldReturnIssues(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{"number":1,"title":"Bug","body":"desc","state":"open","html_url":"https://github.com/owner/repo/issues/1"},
			{"number":2,"title":"Feature","body":"need","state":"open","html_url":"https://github.com/owner/repo/issues/2"}
		]`)
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
	if issues[0].URL != "https://github.com/owner/repo/issues/1" {
		t.Errorf("Expected URL, got '%s'", issues[0].URL)
	}
}

func TestGitHubProvider_ListIssues_ShouldFilterOutPullRequests(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{"number":1,"title":"Real Issue","state":"open","html_url":"https://github.com/owner/repo/issues/1"},
			{"number":2,"title":"PR Disguised","state":"open","html_url":"https://github.com/owner/repo/pulls/2","pull_request":{"url":"https://api.github.com/repos/owner/repo/pulls/2"}}
		]`)
	})

	issues, err := provider.ListIssues("owner", "repo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue (PR filtered out), got %d", len(issues))
	}
	if issues[0].Title != "Real Issue" {
		t.Errorf("Expected 'Real Issue', got '%s'", issues[0].Title)
	}
}

func TestGitHubProvider_ListIssues_ShouldReturnEmptyForNoIssues(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
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

func TestGitHubProvider_ListIssues_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := provider.ListIssues("owner", "repo")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "github list issues") {
		t.Errorf("Expected 'github list issues' in error, got: %v", err)
	}
}

// =============================================================================
// GitHubProvider — CreateIssue
// =============================================================================

func TestGitHubProvider_CreateIssue_ShouldCreateIssue(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"number":42,"title":"%s","body":"%s","state":"open","html_url":"https://github.com/owner/repo/issues/42"}`,
			req["title"], req["body"])
	})

	issue, err := provider.CreateIssue("owner", "repo", "Bug Report", "Something broke")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if issue.Number != 42 {
		t.Errorf("Expected issue #42, got #%d", issue.Number)
	}
	if issue.Title != "Bug Report" {
		t.Errorf("Expected title 'Bug Report', got '%s'", issue.Title)
	}
	if issue.URL != "https://github.com/owner/repo/issues/42" {
		t.Errorf("Expected URL, got '%s'", issue.URL)
	}
}

func TestGitHubProvider_CreateIssue_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := provider.CreateIssue("owner", "repo", "title", "body")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "github create issue") {
		t.Errorf("Expected 'github create issue' in error, got: %v", err)
	}
}

// =============================================================================
// GitHubProvider — ListPullRequests
// =============================================================================

func TestGitHubProvider_ListPullRequests_ShouldReturnPRs(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{
				"number":10,"title":"Fix","body":"fixing","state":"open",
				"html_url":"https://github.com/owner/repo/pull/10",
				"base":{"ref":"main"},"head":{"ref":"fix-branch"}
			}
		]`)
	})

	prs, err := provider.ListPullRequests("owner", "repo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("Expected 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 10 || prs[0].Title != "Fix" {
		t.Errorf("Expected PR #10 'Fix', got #%d '%s'", prs[0].Number, prs[0].Title)
	}
	if prs[0].Base != "main" || prs[0].Head != "fix-branch" {
		t.Errorf("Expected base=main head=fix-branch, got base=%s head=%s", prs[0].Base, prs[0].Head)
	}
}

func TestGitHubProvider_ListPullRequests_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := provider.ListPullRequests("owner", "repo")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "github list PRs") {
		t.Errorf("Expected 'github list PRs' in error, got: %v", err)
	}
}

// =============================================================================
// GitHubProvider — CreatePullRequest
// =============================================================================

func TestGitHubProvider_CreatePullRequest_ShouldCreatePR(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{
			"number":5,"title":"Feature","body":"New feature","state":"open",
			"html_url":"https://github.com/owner/repo/pull/5",
			"base":{"ref":"main"},"head":{"ref":"feature"}
		}`)
	})

	pr, err := provider.CreatePullRequest("owner", "repo", "Feature", "New feature", "main", "feature")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if pr.Number != 5 {
		t.Errorf("Expected PR #5, got #%d", pr.Number)
	}
	if pr.Base != "main" || pr.Head != "feature" {
		t.Errorf("Expected base=main head=feature, got base=%s head=%s", pr.Base, pr.Head)
	}
}

func TestGitHubProvider_CreatePullRequest_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	_, err := provider.CreatePullRequest("owner", "repo", "T", "B", "main", "dev")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "github create PR") {
		t.Errorf("Expected 'github create PR' in error, got: %v", err)
	}
}

// =============================================================================
// GitHubProvider — CommentOnPR
// =============================================================================

func TestGitHubProvider_CommentOnPR_ShouldAddComment(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	var receivedBody string
	mux.HandleFunc("/repos/owner/repo/issues/42/comments", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		receivedBody = req["body"]

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":1,"body":"LGTM!"}`)
	})

	err := provider.CommentOnPR("owner", "repo", 42, "LGTM!")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if receivedBody != "LGTM!" {
		t.Errorf("Expected body 'LGTM!', got '%s'", receivedBody)
	}
}

func TestGitHubProvider_CommentOnPR_ShouldReturnErrorOnAPIFailure(t *testing.T) {
	provider, mux := newTestGitHubProvider(t)

	mux.HandleFunc("/repos/owner/repo/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	err := provider.CommentOnPR("owner", "repo", 1, "test")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
	if !strings.Contains(err.Error(), "github comment on PR") {
		t.Errorf("Expected 'github comment on PR' in error, got: %v", err)
	}
}

// =============================================================================
// NewGitHubProvider
// =============================================================================

func TestNewGitHubProvider_ShouldReturnProvider(t *testing.T) {
	provider := NewGitHubProvider("ghp_test")
	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}
	if provider.client == nil {
		t.Fatal("Expected non-nil client")
	}
}
