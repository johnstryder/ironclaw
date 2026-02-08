package tooling

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// =============================================================================
// pageFuncs — function fields replace the rodPage interface+adapter
// =============================================================================

// pageFuncs holds the browser-page operations as function fields.
// In production, NewRodBrowser wires them to a real rod.Page.
// In tests, they are set directly to stubs or spies.
type pageFuncs struct {
	navigate    func(url string) error
	waitLoad    func() error
	title       func() (string, error)
	elementText func(selector string, timeout time.Duration) (string, error)
	screenshot  func() ([]byte, error)
	closePage   func()
}

// =============================================================================
// Rod launch/connect/page creation — injectable for testability
// =============================================================================

// rodLaunchFunc launches a headless browser and returns a DevTools URL.
var rodLaunchFunc = func() (string, error) {
	return launcher.New().Headless(true).Launch()
}

// rodConnectFunc creates and connects a rod.Browser to the given DevTools URL.
var rodConnectFunc = func(controlURL string) (*rod.Browser, error) {
	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		return nil, err
	}
	return b, nil
}

// rodCreatePageFunc creates a blank page on the browser. Package-level so
// tests can inject a failing function to cover the error path.
var rodCreatePageFunc = func(b *rod.Browser) (*rod.Page, error) {
	return b.Page(proto.TargetCreateTarget{URL: "about:blank"})
}

// rodElementTextFunc extracts text from a rod element. Package-level so
// tests can inject a failing function to cover the defense-in-depth error path.
var rodElementTextFunc = func(el *rod.Element) (string, error) {
	return el.Text()
}

// buildPageFuncs wires a real rod.Page into a pageFuncs struct.
func buildPageFuncs(b *rod.Browser) (*rod.Page, pageFuncs, error) {
	p, err := rodCreatePageFunc(b)
	if err != nil {
		return nil, pageFuncs{}, err
	}
	return p, pageFuncs{
		navigate: func(url string) error { return p.Navigate(url) },
		waitLoad: func() error { return p.WaitLoad() },
		title: func() (string, error) {
			info, err := p.Info()
			if err != nil {
				return "", err
			}
			return info.Title, nil
		},
		elementText: func(selector string, timeout time.Duration) (string, error) {
			el, err := p.Timeout(timeout).Element(selector)
			if err != nil {
				return "", fmt.Errorf("element not found for selector %q: %w", selector, err)
			}
			text, err := rodElementTextFunc(el)
			if err != nil {
				return "", fmt.Errorf("failed to read text from element: %w", err)
			}
			return text, nil
		},
		screenshot: func() ([]byte, error) { return p.Screenshot(true, nil) },
		closePage:  func() { p.Close() },
	}, nil
}

// =============================================================================
// RodBrowser — real Browser implementation backed by go-rod
// =============================================================================

// RodBrowser implements Browser using the go-rod headless Chrome library.
// It manages a single browser instance with one page. Call Close() when done
// to release resources.
type RodBrowser struct {
	browser *rod.Browser
	fns     pageFuncs
}

// NewRodBrowser launches a headless Chrome instance and returns a ready-to-use
// RodBrowser. The caller must call Close() to clean up.
func NewRodBrowser() (*RodBrowser, error) {
	url, err := rodLaunchFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser, err := rodConnectFunc(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}

	_, fns, err := buildPageFuncs(browser)
	if err != nil {
		safeCloseBrowser(browser)
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	return &RodBrowser{
		browser: browser,
		fns:     fns,
	}, nil
}

// Navigate opens the given URL and returns the page title after the page loads.
func (r *RodBrowser) Navigate(url string) (string, error) {
	if err := r.fns.navigate(url); err != nil {
		return "", fmt.Errorf("navigation failed: %w", err)
	}

	if err := r.fns.waitLoad(); err != nil {
		return "", fmt.Errorf("page load timed out: %w", err)
	}

	title, err := r.fns.title()
	if err != nil {
		return "", fmt.Errorf("failed to get page info: %w", err)
	}

	return title, nil
}

// GetText returns the text content of the first element matching the CSS selector.
func (r *RodBrowser) GetText(selector string) (string, error) {
	text, err := r.fns.elementText(selector, 10*time.Second)
	if err != nil {
		return "", err
	}
	return text, nil
}

// Screenshot captures the current page as a PNG byte slice.
func (r *RodBrowser) Screenshot() ([]byte, error) {
	data, err := r.fns.screenshot()
	if err != nil {
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}
	return data, nil
}

// safeCloseBrowser attempts to close the browser, recovering from any panic
// that may occur if the browser was never fully connected.
func safeCloseBrowser(b *rod.Browser) {
	defer func() { recover() }()
	b.Close()
}

// Close shuts down the browser and releases all resources.
func (r *RodBrowser) Close() error {
	if r.fns.closePage != nil {
		r.fns.closePage()
	}
	if r.browser != nil {
		safeCloseBrowser(r.browser)
	}
	return nil
}
