package tooling

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

// =============================================================================
// Helper — build a RodBrowser with stub pageFuncs (no real browser needed)
// =============================================================================

func stubPageFuncs() pageFuncs {
	return pageFuncs{
		navigate:    func(url string) error { return nil },
		waitLoad:    func() error { return nil },
		title:       func() (string, error) { return "", nil },
		elementText: func(_ string, _ time.Duration) (string, error) { return "", nil },
		screenshot:  func() ([]byte, error) { return nil, nil },
		closePage:   func() {},
	}
}

// =============================================================================
// NewRodBrowser — launch/connect/page error paths
// =============================================================================

func TestNewRodBrowser_ShouldReturnErrorWhenLaunchFails(t *testing.T) {
	original := rodLaunchFunc
	rodLaunchFunc = func() (string, error) {
		return "", fmt.Errorf("no chrome found")
	}
	defer func() { rodLaunchFunc = original }()

	_, err := NewRodBrowser()
	if err == nil {
		t.Fatal("Expected error when launch fails")
	}
	if !strings.Contains(err.Error(), "failed to launch browser") {
		t.Errorf("Expected 'failed to launch browser' in error, got: %v", err)
	}
}

func TestNewRodBrowser_ShouldReturnErrorWhenConnectFails(t *testing.T) {
	originalLaunch := rodLaunchFunc
	originalConnect := rodConnectFunc
	rodLaunchFunc = func() (string, error) {
		return "ws://fake:1234", nil
	}
	rodConnectFunc = func(url string) (*rod.Browser, error) {
		return nil, fmt.Errorf("connection refused")
	}
	defer func() {
		rodLaunchFunc = originalLaunch
		rodConnectFunc = originalConnect
	}()

	_, err := NewRodBrowser()
	if err == nil {
		t.Fatal("Expected error when connect fails")
	}
	if !strings.Contains(err.Error(), "failed to connect to browser") {
		t.Errorf("Expected 'failed to connect to browser' in error, got: %v", err)
	}
}

func TestNewRodBrowser_ShouldReturnErrorWhenPageCreationFails(t *testing.T) {
	originalLaunch := rodLaunchFunc
	originalConnect := rodConnectFunc
	originalCreate := rodCreatePageFunc
	rodLaunchFunc = func() (string, error) {
		return "ws://fake:1234", nil
	}
	rodConnectFunc = func(url string) (*rod.Browser, error) {
		return rod.New(), nil
	}
	rodCreatePageFunc = func(b *rod.Browser) (*rod.Page, error) {
		return nil, fmt.Errorf("page creation failed")
	}
	defer func() {
		rodLaunchFunc = originalLaunch
		rodConnectFunc = originalConnect
		rodCreatePageFunc = originalCreate
	}()

	_, err := NewRodBrowser()
	if err == nil {
		t.Fatal("Expected error when page creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create page") {
		t.Errorf("Expected 'failed to create page' in error, got: %v", err)
	}
}

// =============================================================================
// RodBrowser.Navigate — error paths
// =============================================================================

func TestRodBrowser_Navigate_ShouldReturnErrorWhenNavigateFails(t *testing.T) {
	fns := stubPageFuncs()
	fns.navigate = func(url string) error { return fmt.Errorf("network error") }
	rb := &RodBrowser{fns: fns}

	_, err := rb.Navigate("https://example.com")
	if err == nil {
		t.Fatal("Expected error when navigate fails")
	}
	if !strings.Contains(err.Error(), "navigation failed") {
		t.Errorf("Expected 'navigation failed' in error, got: %v", err)
	}
}

func TestRodBrowser_Navigate_ShouldReturnErrorWhenWaitLoadFails(t *testing.T) {
	fns := stubPageFuncs()
	fns.waitLoad = func() error { return fmt.Errorf("timeout") }
	rb := &RodBrowser{fns: fns}

	_, err := rb.Navigate("https://example.com")
	if err == nil {
		t.Fatal("Expected error when WaitLoad fails")
	}
	if !strings.Contains(err.Error(), "page load timed out") {
		t.Errorf("Expected 'page load timed out' in error, got: %v", err)
	}
}

func TestRodBrowser_Navigate_ShouldReturnErrorWhenTitleFails(t *testing.T) {
	fns := stubPageFuncs()
	fns.title = func() (string, error) { return "", fmt.Errorf("info unavailable") }
	rb := &RodBrowser{fns: fns}

	_, err := rb.Navigate("https://example.com")
	if err == nil {
		t.Fatal("Expected error when title() fails")
	}
	if !strings.Contains(err.Error(), "failed to get page info") {
		t.Errorf("Expected 'failed to get page info' in error, got: %v", err)
	}
}

func TestRodBrowser_Navigate_ShouldReturnTitleOnSuccess(t *testing.T) {
	fns := stubPageFuncs()
	fns.title = func() (string, error) { return "Test Title", nil }
	rb := &RodBrowser{fns: fns}

	title, err := rb.Navigate("https://example.com")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if title != "Test Title" {
		t.Errorf("Expected 'Test Title', got '%s'", title)
	}
}

// =============================================================================
// RodBrowser.GetText — error paths
// =============================================================================

func TestRodBrowser_GetText_ShouldReturnErrorWhenElementTextFails(t *testing.T) {
	fns := stubPageFuncs()
	fns.elementText = func(sel string, _ time.Duration) (string, error) {
		return "", fmt.Errorf("element not found for selector %q", sel)
	}
	rb := &RodBrowser{fns: fns}

	_, err := rb.GetText("h1")
	if err == nil {
		t.Fatal("Expected error when elementText fails")
	}
}

func TestRodBrowser_GetText_ShouldReturnTextOnSuccess(t *testing.T) {
	fns := stubPageFuncs()
	fns.elementText = func(_ string, _ time.Duration) (string, error) {
		return "Hello", nil
	}
	rb := &RodBrowser{fns: fns}

	text, err := rb.GetText("h1")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if text != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", text)
	}
}

// =============================================================================
// RodBrowser.Screenshot — error paths
// =============================================================================

func TestRodBrowser_Screenshot_ShouldReturnErrorWhenScreenshotFails(t *testing.T) {
	fns := stubPageFuncs()
	fns.screenshot = func() ([]byte, error) { return nil, fmt.Errorf("render error") }
	rb := &RodBrowser{fns: fns}

	_, err := rb.Screenshot()
	if err == nil {
		t.Fatal("Expected error when screenshot fails")
	}
	if !strings.Contains(err.Error(), "screenshot failed") {
		t.Errorf("Expected 'screenshot failed' in error, got: %v", err)
	}
}

func TestRodBrowser_Screenshot_ShouldReturnDataOnSuccess(t *testing.T) {
	fns := stubPageFuncs()
	fns.screenshot = func() ([]byte, error) { return []byte{0x89, 0x50, 0x4E, 0x47}, nil }
	rb := &RodBrowser{fns: fns}

	data, err := rb.Screenshot()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(data) != 4 {
		t.Errorf("Expected 4 bytes, got %d", len(data))
	}
}

// =============================================================================
// RodBrowser.Close — nil page/browser paths
// =============================================================================

func TestRodBrowser_Close_ShouldHandleNilClosePageAndBrowser(t *testing.T) {
	rb := &RodBrowser{fns: pageFuncs{closePage: nil}, browser: nil}
	err := rb.Close()
	if err != nil {
		t.Errorf("Expected no error for nil close, got: %v", err)
	}
}

func TestRodBrowser_Close_ShouldCallClosePageWhenBrowserNil(t *testing.T) {
	called := false
	fns := stubPageFuncs()
	fns.closePage = func() { called = true }
	rb := &RodBrowser{fns: fns, browser: nil}

	err := rb.Close()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !called {
		t.Error("Expected closePage to be called")
	}
}

func TestRodBrowser_Close_ShouldSafelyCloseBrowserWhenNonNil(t *testing.T) {
	called := false
	fns := stubPageFuncs()
	fns.closePage = func() { called = true }
	// rod.New() returns an unconnected browser — safeCloseBrowser handles the panic
	rb := &RodBrowser{fns: fns, browser: rod.New()}

	err := rb.Close()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !called {
		t.Error("Expected closePage to be called")
	}
}

// =============================================================================
// Compile-time interface check
// =============================================================================

var _ Browser = (*RodBrowser)(nil)
