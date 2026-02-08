package tooling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	readability "github.com/go-shiori/go-readability"

	"ironclaw/internal/domain"
)

// HTTPFetcher abstracts HTTP GET requests for testability.
type HTTPFetcher interface {
	Fetch(url string) ([]byte, error)
}

// ScrapeInput represents the input structure for the scrape tool.
type ScrapeInput struct {
	URL string `json:"url" jsonschema:"minLength=1"`
}

// ScrapeTool implements SchemaTool for web scraping with content extraction.
// It fetches a URL, strips script/style tags with goquery, and extracts the
// main article content using go-readability to reduce LLM token usage.
type ScrapeTool struct {
	fetcher HTTPFetcher
}

// NewScrapeTool creates a ScrapeTool with the given HTTP fetcher.
func NewScrapeTool(fetcher HTTPFetcher) *ScrapeTool {
	return &ScrapeTool{fetcher: fetcher}
}

// Package-level injectable function vars. Tests override these to cover
// defense-in-depth error paths that are unreachable with natural inputs.
var (
	scrapeUnmarshalFunc      = json.Unmarshal
	scrapeStripFunc          = stripScriptsAndStyles
	scrapeExtractReadableFunc = extractReadableContent
	scrapeExtractTextFunc    = extractPlainText
	scrapeGoQueryParseFunc   = goquery.NewDocumentFromReader
	scrapeURLParseFunc       = url.Parse
	scrapeHTTPNewRequestFunc = http.NewRequest
	scrapeRenderHTMLFunc     = func(doc *goquery.Document) (string, error) { return doc.Html() }
	scrapeReadabilityFunc    = func(input io.Reader, pageURL *url.URL) (readability.Article, error) {
		return readability.FromReader(input, pageURL)
	}
	scrapeReadAllFunc = io.ReadAll
)

// Name returns the tool name used in function-calling.
func (s *ScrapeTool) Name() string { return "scrape" }

// Description returns a human-readable description for the LLM.
func (s *ScrapeTool) Description() string {
	return "Scrapes a web page URL, strips scripts and styles, and extracts the main article content as clean text"
}

// Definition returns the JSON Schema for scrape input.
func (s *ScrapeTool) Definition() string {
	return GenerateSchema(ScrapeInput{})
}

// Call validates the JSON arguments against the schema and executes the
// scrape operation: fetch → strip scripts/styles → extract readable content.
func (s *ScrapeTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	// 1. Validate input against JSON schema
	schema := s.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 2. Unmarshal input
	var input ScrapeInput
	if err := scrapeUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// 3. Validate URL scheme
	if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
		return nil, fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	// 4. Fetch the page
	rawHTML, err := s.fetcher.Fetch(input.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}

	// 5. Process HTML (strip scripts/styles + extract readable content)
	content, err := processHTML(rawHTML, input.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to process HTML: %w", err)
	}

	return &domain.ToolResult{
		Data: content,
		Metadata: map[string]string{
			"url":    input.URL,
			"source": "scrape",
		},
	}, nil
}

// processHTML strips scripts/styles and extracts readable content.
// Falls back to plain text extraction when readability cannot identify an article.
func processHTML(rawHTML []byte, sourceURL string) (string, error) {
	// Strip scripts and styles
	cleanedHTML, err := scrapeStripFunc(rawHTML)
	if err != nil {
		return "", fmt.Errorf("failed to clean HTML: %w", err)
	}

	// Try readability extraction
	content, err := scrapeExtractReadableFunc(cleanedHTML, sourceURL)
	if err == nil && strings.TrimSpace(content) != "" {
		return strings.TrimSpace(content), nil
	}

	// Fallback: extract plain text from cleaned HTML
	text, err := scrapeExtractTextFunc(cleanedHTML)
	if err != nil {
		return "", fmt.Errorf("failed to extract text: %w", err)
	}

	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("no content found at URL")
	}

	return strings.TrimSpace(text), nil
}

// stripScriptsAndStyles removes script, style, and noscript tags from HTML
// using goquery.
func stripScriptsAndStyles(rawHTML []byte) (string, error) {
	doc, err := scrapeGoQueryParseFunc(bytes.NewReader(rawHTML))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	doc.Find("script, style, noscript").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})

	html, err := scrapeRenderHTMLFunc(doc)
	if err != nil {
		return "", fmt.Errorf("failed to render HTML: %w", err)
	}

	return html, nil
}

// extractReadableContent extracts the main article text from HTML using
// go-readability.
func extractReadableContent(htmlContent string, sourceURL string) (string, error) {
	parsedURL, err := scrapeURLParseFunc(sourceURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	article, err := scrapeReadabilityFunc(strings.NewReader(htmlContent), parsedURL)
	if err != nil {
		return "", fmt.Errorf("readability extraction failed: %w", err)
	}

	return article.TextContent, nil
}

// extractPlainText extracts all visible text from HTML using goquery.
func extractPlainText(htmlContent string) (string, error) {
	doc, err := scrapeGoQueryParseFunc(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	return doc.Text(), nil
}

// maxResponseSize limits the maximum HTTP response body to 10 MB.
const maxResponseSize = 10 * 1024 * 1024

// DefaultHTTPFetcher implements HTTPFetcher using net/http.
type DefaultHTTPFetcher struct {
	client *http.Client
}

// NewDefaultHTTPFetcher creates a DefaultHTTPFetcher with sensible defaults.
func NewDefaultHTTPFetcher() *DefaultHTTPFetcher {
	return &DefaultHTTPFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Fetch retrieves the content at the given URL with a User-Agent header.
func (f *DefaultHTTPFetcher) Fetch(fetchURL string) ([]byte, error) {
	req, err := scrapeHTTPNewRequestFunc(http.MethodGet, fetchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Ironclaw/1.0 (Web Scraper)")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	limitedReader := io.LimitReader(resp.Body, maxResponseSize)
	body, err := scrapeReadAllFunc(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}
