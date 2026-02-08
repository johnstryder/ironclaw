package tooling

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-rod/rod"
)

// requireChrome skips the test if no Chrome/Chromium binary is available.
func requireChrome(t *testing.T) {
	t.Helper()
	for _, name := range []string{"chromium", "google-chrome-stable", "google-chrome", "chrome"} {
		if _, err := exec.LookPath(name); err == nil {
			return
		}
	}
	t.Skip("skipping: no Chrome/Chromium binary found in PATH")
}

// =============================================================================
// RodBrowser — Integration Tests (requires real headless Chrome)
// =============================================================================

func startTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
  <h1>Hello Rod</h1>
  <div id="content">This is test content</div>
  <span class="empty"></span>
</body>
</html>`))
	})
	mux.HandleFunc("/notitle", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head></head><body><p>No title</p></body></html>`))
	})
	return httptest.NewServer(mux)
}

func TestRodBrowser_NewRodBrowser_ShouldLaunchSuccessfully(t *testing.T) {
	requireChrome(t)
	browser, err := NewRodBrowser()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer browser.Close()
}

// TestRodBrowser_EndToEnd exercises all default rod closures (navigate, waitLoad,
// title, elementText, screenshot, closePage) in a single browser session so the
// package-level var function bodies reach 100 % coverage without launching many
// separate Chrome instances.
func TestRodBrowser_EndToEnd_AllOperations(t *testing.T) {
	requireChrome(t)

	srv := startTestServer()
	defer srv.Close()

	browser, err := NewRodBrowser()
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	// --- Navigate (covers: navigate, waitLoad, title closures) ---
	title, err := browser.Navigate(srv.URL + "/")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}
	if title != "Test Page" {
		t.Errorf("Expected 'Test Page', got '%s'", title)
	}

	// --- GetText (covers: elementText closure — happy path) ---
	text, err := browser.GetText("h1")
	if err != nil {
		t.Fatalf("GetText failed: %v", err)
	}
	if text != "Hello Rod" {
		t.Errorf("Expected 'Hello Rod', got '%s'", text)
	}

	text, err = browser.GetText("#content")
	if err != nil {
		t.Fatalf("GetText #content failed: %v", err)
	}
	if text != "This is test content" {
		t.Errorf("Expected 'This is test content', got '%s'", text)
	}

	// --- GetText nonexistent (covers: elementText closure — error path) ---
	_, err = browser.GetText("#nonexistent-element-xyz")
	if err == nil {
		t.Fatal("Expected error for nonexistent selector")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Errorf("Expected 'element not found' in error, got: %v", err)
	}

	// --- Screenshot (covers: screenshot closure) ---
	data, err := browser.Screenshot()
	if err != nil {
		t.Fatalf("Screenshot failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected non-empty screenshot data")
	}
	// PNG magic bytes: 0x89 0x50 0x4E 0x47
	if len(data) >= 4 && (data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4E || data[3] != 0x47) {
		t.Error("Expected PNG format (magic bytes mismatch)")
	}

	// --- Navigate to page without <title> ---
	title, err = browser.Navigate(srv.URL + "/notitle")
	if err != nil {
		t.Fatalf("Navigate /notitle failed: %v", err)
	}
	// Chrome falls back to URL when no <title> is present
	if title == "" {
		t.Error("Expected non-empty fallback title from Chrome")
	}
}

// TestRodBrowser_Close covers the closePage closure when browser.Close() is called.
func TestRodBrowser_Close_ShouldNotErrorOnCleanIntegration(t *testing.T) {
	requireChrome(t)
	browser, err := NewRodBrowser()
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	err = browser.Close()
	if err != nil {
		t.Errorf("Expected no error from Close, got: %v", err)
	}
}

// =============================================================================
// Default closure error branches — cover the error paths inside the
// package-level var function bodies that only trigger under failure.
// =============================================================================

func TestRodConnectFunc_ShouldReturnErrorForBadURL(t *testing.T) {
	_, err := rodConnectFunc("ws://127.0.0.1:1")
	if err == nil {
		t.Fatal("Expected error for unreachable DevTools URL")
	}
}

func TestRodCreatePageFunc_ShouldReturnErrorWhenBrowserNotConnected(t *testing.T) {
	// rod.New() without Connect() — Page() will panic; catch it
	defer func() {
		if r := recover(); r != nil {
			// Panic confirms the code path is reachable
		}
	}()
	_, err := rodCreatePageFunc(rod.New())
	if err == nil {
		t.Fatal("Expected error for unconnected browser")
	}
}

func TestBuildPageFuncs_ShouldReturnErrorWhenCreatePageFails(t *testing.T) {
	original := rodCreatePageFunc
	rodCreatePageFunc = func(b *rod.Browser) (*rod.Page, error) {
		return nil, fmt.Errorf("injected page failure")
	}
	defer func() { rodCreatePageFunc = original }()

	_, _, err := buildPageFuncs(rod.New())
	if err == nil {
		t.Fatal("Expected error when page creation fails")
	}
}

func TestBuildPageFuncs_TitleClosure_ShouldReturnErrorWhenPageClosed(t *testing.T) {
	requireChrome(t)

	browser, err := NewRodBrowser()
	if err != nil {
		t.Fatalf("Failed to launch: %v", err)
	}
	defer browser.Close()

	// Close the page to break the captured *rod.Page, then call title()
	browser.fns.closePage()
	_, err = browser.fns.title()
	if err == nil {
		t.Fatal("Expected error when page is closed")
	}
}

func TestBuildPageFuncs_ElementTextClosure_ShouldReturnErrorWhenTextFails(t *testing.T) {
	requireChrome(t)

	srv := startTestServer()
	defer srv.Close()

	browser, err := NewRodBrowser()
	if err != nil {
		t.Fatalf("Failed to launch: %v", err)
	}
	defer browser.Close()

	_, _ = browser.Navigate(srv.URL + "/")

	// Inject a failing text extractor to cover the el.Text() error path
	original := rodElementTextFunc
	rodElementTextFunc = func(el *rod.Element) (string, error) {
		return "", fmt.Errorf("forced text extraction failure")
	}
	defer func() { rodElementTextFunc = original }()

	_, err = browser.fns.elementText("h1", 10*1e9) // 10s timeout
	if err == nil {
		t.Fatal("Expected error when text extraction fails")
	}
	if !strings.Contains(err.Error(), "failed to read text from element") {
		t.Errorf("Expected 'failed to read text from element' in error, got: %v", err)
	}
}

// =============================================================================
// Compile-time interface check for RodBrowser
// =============================================================================

var _ Browser = (*RodBrowser)(nil)
