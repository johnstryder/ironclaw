package tooling

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	readability "github.com/go-shiori/go-readability"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockHTTPFetcher is a test double for HTTPFetcher.
type mockHTTPFetcher struct {
	response []byte
	err      error
}

func (m *mockHTTPFetcher) Fetch(url string) ([]byte, error) {
	return m.response, m.err
}

// sampleArticleHTML is a realistic HTML page with scripts, styles, and
// substantial article content for readability extraction.
const sampleArticleHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Test Article</title>
    <style>body { color: red; font-size: 14px; }</style>
    <script>console.log('tracking');</script>
</head>
<body>
    <nav><a href="/">Home</a> | <a href="/about">About</a></nav>
    <article>
        <h1>Main Article Title</h1>
        <p>This is the first paragraph of the main article content. It contains
        enough text to be recognized as substantial content by the readability
        algorithm for extraction purposes.</p>
        <p>This is the second paragraph with more detailed information about the
        topic. The readability algorithm identifies this section as the primary
        content area based on text density analysis and heuristics.</p>
        <p>Here is a third paragraph providing additional context and details
        about the subject matter. This ensures proper readability extraction by
        providing sufficient content density in the article area.</p>
        <p>A fourth paragraph adds even more substance to the article body.
        Multiple paragraphs help the readability algorithm correctly identify
        the main content region of the page and separate it from navigation.</p>
        <p>The fifth paragraph rounds out the article with concluding thoughts.
        This level of content is typical for a real-world news article or blog
        post that would be scraped and summarized by an LLM agent.</p>
    </article>
    <aside>Sidebar advertisements and related links</aside>
    <footer>Copyright 2025 Test Inc. All rights reserved.</footer>
    <script>trackPageView();</script>
</body>
</html>`

// =============================================================================
// ScrapeTool Metadata
// =============================================================================

func TestScrapeTool_Name_ShouldReturnScrape(t *testing.T) {
	tool := NewScrapeTool(&mockHTTPFetcher{})
	if tool.Name() != "scrape" {
		t.Errorf("Expected name 'scrape', got '%s'", tool.Name())
	}
}

func TestScrapeTool_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	tool := NewScrapeTool(&mockHTTPFetcher{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestScrapeTool_Definition_ShouldContainURLProperty(t *testing.T) {
	tool := NewScrapeTool(&mockHTTPFetcher{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}

	properties, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'properties' in schema")
	}

	if _, exists := properties["url"]; !exists {
		t.Error("Expected 'url' property in schema")
	}
}

func TestScrapeTool_Definition_ShouldBeValidJSONSchema(t *testing.T) {
	tool := NewScrapeTool(&mockHTTPFetcher{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", parsed["type"])
	}
}

func TestScrapeTool_ShouldSatisfySchemaToolInterface(t *testing.T) {
	var _ SchemaTool = (*ScrapeTool)(nil)
}

// =============================================================================
// ScrapeTool.Call — Validation
// =============================================================================

func TestScrapeTool_Call_ShouldRejectMissingURL(t *testing.T) {
	tool := NewScrapeTool(&mockHTTPFetcher{})
	_, err := tool.Call(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("Expected validation error for missing URL")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestScrapeTool_Call_ShouldRejectInvalidJSON(t *testing.T) {
	tool := NewScrapeTool(&mockHTTPFetcher{})
	_, err := tool.Call(json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestScrapeTool_Call_ShouldRejectEmptyURL(t *testing.T) {
	tool := NewScrapeTool(&mockHTTPFetcher{})
	_, err := tool.Call(json.RawMessage(`{"url":""}`))
	if err == nil {
		t.Fatal("Expected validation error for empty URL")
	}
}

func TestScrapeTool_Call_ShouldRejectURLWithoutHTTPScheme(t *testing.T) {
	tool := NewScrapeTool(&mockHTTPFetcher{})
	_, err := tool.Call(json.RawMessage(`{"url":"ftp://example.com"}`))
	if err == nil {
		t.Fatal("Expected error for non-HTTP URL")
	}
	if !strings.Contains(err.Error(), "http:// or https://") {
		t.Errorf("Expected URL scheme error, got: %v", err)
	}
}

// =============================================================================
// ScrapeTool.Call — Happy Path
// =============================================================================

func TestScrapeTool_Call_ShouldReturnCleanTextFromHTML(t *testing.T) {
	fetcher := &mockHTTPFetcher{response: []byte(sampleArticleHTML)}
	tool := NewScrapeTool(fetcher)

	result, err := tool.Call(json.RawMessage(`{"url":"https://example.com/article"}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Data == "" {
		t.Fatal("Expected non-empty content")
	}
}

func TestScrapeTool_Call_ShouldStripScriptContent(t *testing.T) {
	fetcher := &mockHTTPFetcher{response: []byte(sampleArticleHTML)}
	tool := NewScrapeTool(fetcher)

	result, err := tool.Call(json.RawMessage(`{"url":"https://example.com/article"}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strings.Contains(result.Data, "console.log") {
		t.Error("Result should not contain script content")
	}
	if strings.Contains(result.Data, "trackPageView") {
		t.Error("Result should not contain tracking script content")
	}
}

func TestScrapeTool_Call_ShouldStripStyleContent(t *testing.T) {
	fetcher := &mockHTTPFetcher{response: []byte(sampleArticleHTML)}
	tool := NewScrapeTool(fetcher)

	result, err := tool.Call(json.RawMessage(`{"url":"https://example.com/article"}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strings.Contains(result.Data, "font-size") {
		t.Error("Result should not contain style content")
	}
}

func TestScrapeTool_Call_ShouldReturnMetadataWithURL(t *testing.T) {
	fetcher := &mockHTTPFetcher{response: []byte(sampleArticleHTML)}
	tool := NewScrapeTool(fetcher)

	result, err := tool.Call(json.RawMessage(`{"url":"https://example.com/article"}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Metadata["url"] != "https://example.com/article" {
		t.Errorf("Expected URL in metadata, got: %v", result.Metadata)
	}
	if result.Metadata["source"] != "scrape" {
		t.Errorf("Expected source 'scrape' in metadata, got: %v", result.Metadata["source"])
	}
}

func TestScrapeTool_Call_ShouldContainArticleText(t *testing.T) {
	fetcher := &mockHTTPFetcher{response: []byte(sampleArticleHTML)}
	tool := NewScrapeTool(fetcher)

	result, err := tool.Call(json.RawMessage(`{"url":"https://example.com/article"}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(result.Data, "first paragraph") {
		t.Error("Expected article content in result")
	}
}

// =============================================================================
// ScrapeTool.Call — Error Paths
// =============================================================================

func TestScrapeTool_Call_ShouldReturnErrorWhenFetchFails(t *testing.T) {
	fetcher := &mockHTTPFetcher{err: fmt.Errorf("network error")}
	tool := NewScrapeTool(fetcher)

	_, err := tool.Call(json.RawMessage(`{"url":"https://example.com"}`))
	if err == nil {
		t.Fatal("Expected error when fetch fails")
	}
	if !strings.Contains(err.Error(), "failed to fetch") {
		t.Errorf("Expected 'failed to fetch' in error, got: %v", err)
	}
}

func TestScrapeTool_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := scrapeUnmarshalFunc
	scrapeUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { scrapeUnmarshalFunc = original }()

	tool := NewScrapeTool(&mockHTTPFetcher{})
	_, err := tool.Call(json.RawMessage(`{"url":"https://example.com"}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

func TestScrapeTool_Call_ShouldReturnErrorWhenProcessingFails(t *testing.T) {
	// Override strip function to simulate processing failure
	original := scrapeStripFunc
	scrapeStripFunc = func(rawHTML []byte) (string, error) {
		return "", fmt.Errorf("forced processing failure")
	}
	defer func() { scrapeStripFunc = original }()

	fetcher := &mockHTTPFetcher{response: []byte("<html></html>")}
	tool := NewScrapeTool(fetcher)

	_, err := tool.Call(json.RawMessage(`{"url":"https://example.com"}`))
	if err == nil {
		t.Fatal("Expected error when processing fails")
	}
	if !strings.Contains(err.Error(), "failed to process") {
		t.Errorf("Expected 'failed to process' in error, got: %v", err)
	}
}

// =============================================================================
// processHTML — Internal Function Tests
// =============================================================================

func TestProcessHTML_ShouldReturnReadableContentForArticle(t *testing.T) {
	result, err := processHTML([]byte(sampleArticleHTML), "https://example.com/article")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("Expected non-empty result")
	}
	if !strings.Contains(result, "first paragraph") {
		t.Error("Expected article content in result")
	}
}

func TestProcessHTML_ShouldFallbackToPlainTextWhenReadabilityFails(t *testing.T) {
	// Override readability to fail
	origExtract := scrapeExtractReadableFunc
	scrapeExtractReadableFunc = func(htmlContent string, sourceURL string) (string, error) {
		return "", fmt.Errorf("readability failed")
	}
	defer func() { scrapeExtractReadableFunc = origExtract }()

	simpleHTML := `<html><body><p>Simple plain text content here</p></body></html>`
	result, err := processHTML([]byte(simpleHTML), "https://example.com")
	if err != nil {
		t.Fatalf("Expected fallback to plain text, got error: %v", err)
	}
	if !strings.Contains(result, "Simple plain text content") {
		t.Error("Expected plain text fallback content in result")
	}
}

func TestProcessHTML_ShouldFallbackWhenReadabilityReturnsEmpty(t *testing.T) {
	// Override readability to return empty
	origExtract := scrapeExtractReadableFunc
	scrapeExtractReadableFunc = func(htmlContent string, sourceURL string) (string, error) {
		return "", nil
	}
	defer func() { scrapeExtractReadableFunc = origExtract }()

	simpleHTML := `<html><body><p>Fallback text content</p></body></html>`
	result, err := processHTML([]byte(simpleHTML), "https://example.com")
	if err != nil {
		t.Fatalf("Expected fallback, got error: %v", err)
	}
	if !strings.Contains(result, "Fallback text content") {
		t.Error("Expected plain text fallback content")
	}
}

func TestProcessHTML_ShouldReturnErrorWhenStripFails(t *testing.T) {
	original := scrapeStripFunc
	scrapeStripFunc = func(rawHTML []byte) (string, error) {
		return "", fmt.Errorf("forced strip error")
	}
	defer func() { scrapeStripFunc = original }()

	_, err := processHTML([]byte("<html></html>"), "https://example.com")
	if err == nil {
		t.Fatal("Expected error when strip fails")
	}
	if !strings.Contains(err.Error(), "failed to clean HTML") {
		t.Errorf("Expected 'failed to clean HTML' in error, got: %v", err)
	}
}

func TestProcessHTML_ShouldReturnErrorWhenTextExtractionFails(t *testing.T) {
	// Override readability to fail, AND text extraction to fail
	origExtract := scrapeExtractReadableFunc
	scrapeExtractReadableFunc = func(htmlContent string, sourceURL string) (string, error) {
		return "", fmt.Errorf("readability failed")
	}
	origText := scrapeExtractTextFunc
	scrapeExtractTextFunc = func(htmlContent string) (string, error) {
		return "", fmt.Errorf("forced text extraction error")
	}
	defer func() {
		scrapeExtractReadableFunc = origExtract
		scrapeExtractTextFunc = origText
	}()

	_, err := processHTML([]byte("<html></html>"), "https://example.com")
	if err == nil {
		t.Fatal("Expected error when text extraction fails")
	}
	if !strings.Contains(err.Error(), "failed to extract text") {
		t.Errorf("Expected 'failed to extract text' in error, got: %v", err)
	}
}

func TestProcessHTML_ShouldReturnErrorWhenNoContentFound(t *testing.T) {
	// Override readability to return empty, AND text extraction to return empty
	origExtract := scrapeExtractReadableFunc
	scrapeExtractReadableFunc = func(htmlContent string, sourceURL string) (string, error) {
		return "", fmt.Errorf("no article")
	}
	origText := scrapeExtractTextFunc
	scrapeExtractTextFunc = func(htmlContent string) (string, error) {
		return "   ", nil // whitespace only
	}
	defer func() {
		scrapeExtractReadableFunc = origExtract
		scrapeExtractTextFunc = origText
	}()

	_, err := processHTML([]byte("<html></html>"), "https://example.com")
	if err == nil {
		t.Fatal("Expected error when no content found")
	}
	if !strings.Contains(err.Error(), "no content found") {
		t.Errorf("Expected 'no content found' in error, got: %v", err)
	}
}

// =============================================================================
// stripScriptsAndStyles — Internal Function Tests
// =============================================================================

func TestStripScriptsAndStyles_ShouldRemoveScriptTags(t *testing.T) {
	html := `<html><body><script>alert('xss')</script><p>Hello</p></body></html>`
	result, err := stripScriptsAndStyles([]byte(html))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strings.Contains(result, "alert") {
		t.Error("Script content should be removed")
	}
	if !strings.Contains(result, "Hello") {
		t.Error("Body content should be preserved")
	}
}

func TestStripScriptsAndStyles_ShouldRemoveStyleTags(t *testing.T) {
	html := `<html><head><style>.red{color:red}</style></head><body><p>Content</p></body></html>`
	result, err := stripScriptsAndStyles([]byte(html))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strings.Contains(result, "color:red") {
		t.Error("Style content should be removed")
	}
	if !strings.Contains(result, "Content") {
		t.Error("Body content should be preserved")
	}
}

func TestStripScriptsAndStyles_ShouldRemoveNoscriptTags(t *testing.T) {
	html := `<html><body><noscript>Enable JS</noscript><p>Main</p></body></html>`
	result, err := stripScriptsAndStyles([]byte(html))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strings.Contains(result, "Enable JS") {
		t.Error("Noscript content should be removed")
	}
}

func TestStripScriptsAndStyles_ShouldPreserveArticleContent(t *testing.T) {
	html := `<html><body><article><h1>Title</h1><p>Article text here</p></article></body></html>`
	result, err := stripScriptsAndStyles([]byte(html))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "Article text here") {
		t.Error("Article content should be preserved")
	}
}

func TestStripScriptsAndStyles_ShouldHandleEmptyHTML(t *testing.T) {
	result, err := stripScriptsAndStyles([]byte(""))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Empty HTML should produce some minimal output (goquery wraps it)
	if result == "" {
		t.Error("Expected some output even for empty input")
	}
}

func TestStripScriptsAndStyles_ShouldHandleMultipleScriptTags(t *testing.T) {
	html := `<html><body>
        <script>var a = 1;</script>
        <p>Content</p>
        <script>var b = 2;</script>
        <script src="external.js"></script>
    </body></html>`
	result, err := stripScriptsAndStyles([]byte(html))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strings.Contains(result, "var a") || strings.Contains(result, "var b") {
		t.Error("All script content should be removed")
	}
	if !strings.Contains(result, "Content") {
		t.Error("Body content should be preserved")
	}
}

func TestStripScriptsAndStyles_ShouldRemoveMixedTags(t *testing.T) {
	html := `<html><head>
        <style>body{margin:0}</style>
        <script>init();</script>
    </head><body>
        <script>track();</script>
        <p>Visible text</p>
        <noscript>Fallback</noscript>
    </body></html>`
	result, err := stripScriptsAndStyles([]byte(html))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strings.Contains(result, "margin:0") || strings.Contains(result, "init()") ||
		strings.Contains(result, "track()") || strings.Contains(result, "Fallback") {
		t.Error("All script, style, and noscript content should be removed")
	}
	if !strings.Contains(result, "Visible text") {
		t.Error("Visible text should be preserved")
	}
}

// =============================================================================
// extractReadableContent — Internal Function Tests
// =============================================================================

func TestExtractReadableContent_ShouldExtractArticleText(t *testing.T) {
	html := `<html><body>
        <nav>Menu items</nav>
        <article>
            <h1>Article Title</h1>
            <p>This is the main article content with enough text for readability
            to detect. It needs to be substantial enough for the algorithm.</p>
            <p>Another paragraph to increase content density for proper extraction
            by the readability algorithm heuristics.</p>
            <p>A third paragraph ensures the readability algorithm has enough
            content density to work with and identify the article region.</p>
        </article>
        <footer>Footer content</footer>
    </body></html>`

	result, err := extractReadableContent(html, "https://example.com/article")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("Expected non-empty result")
	}
	if !strings.Contains(result, "main article content") {
		t.Error("Expected article content in result")
	}
}

func TestExtractReadableContent_ShouldHandleMinimalHTML(t *testing.T) {
	html := `<html><body><p>Simple content</p></body></html>`
	// Readability might or might not extract from minimal HTML.
	// It should not panic, but may return an error.
	_, _ = extractReadableContent(html, "https://example.com")
	// No assertion on result — just ensure no panic
}

// =============================================================================
// extractPlainText — Internal Function Tests
// =============================================================================

func TestExtractPlainText_ShouldReturnTextFromHTML(t *testing.T) {
	html := `<html><body><h1>Title</h1><p>Paragraph text</p></body></html>`
	result, err := extractPlainText(html)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "Title") {
		t.Error("Expected title text in result")
	}
	if !strings.Contains(result, "Paragraph text") {
		t.Error("Expected paragraph text in result")
	}
}

func TestExtractPlainText_ShouldHandleEmptyHTML(t *testing.T) {
	result, err := extractPlainText("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Empty HTML should return minimal text (from goquery's wrapping)
	_ = result
}

func TestExtractPlainText_ShouldStripHTMLTags(t *testing.T) {
	html := `<html><body><div><strong>Bold</strong> and <em>italic</em></div></body></html>`
	result, err := extractPlainText(html)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strings.Contains(result, "<strong>") || strings.Contains(result, "<em>") {
		t.Error("HTML tags should be stripped from result")
	}
	if !strings.Contains(result, "Bold") || !strings.Contains(result, "italic") {
		t.Error("Text content should be preserved")
	}
}

// =============================================================================
// DefaultHTTPFetcher Tests
// =============================================================================

func TestDefaultHTTPFetcher_Fetch_ShouldReturnHTMLContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>Hello World</body></html>"))
	}))
	defer server.Close()

	fetcher := NewDefaultHTTPFetcher()
	body, err := fetcher.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(string(body), "Hello World") {
		t.Error("Expected HTML content in response")
	}
}

func TestDefaultHTTPFetcher_Fetch_ShouldReturnErrorForNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	fetcher := NewDefaultHTTPFetcher()
	_, err := fetcher.Fetch(server.URL)
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected '404' in error, got: %v", err)
	}
}

func TestDefaultHTTPFetcher_Fetch_ShouldReturnErrorForServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewDefaultHTTPFetcher()
	_, err := fetcher.Fetch(server.URL)
	if err == nil {
		t.Fatal("Expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected '500' in error, got: %v", err)
	}
}

func TestDefaultHTTPFetcher_Fetch_ShouldReturnErrorForNetworkFailure(t *testing.T) {
	fetcher := NewDefaultHTTPFetcher()
	_, err := fetcher.Fetch("http://localhost:1/nonexistent")
	if err == nil {
		t.Fatal("Expected error for network failure")
	}
}

func TestDefaultHTTPFetcher_Fetch_ShouldSetUserAgent(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.Write([]byte("<html></html>"))
	}))
	defer server.Close()

	fetcher := NewDefaultHTTPFetcher()
	_, err := fetcher.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if receivedUA == "" {
		t.Error("Expected User-Agent header to be set")
	}
}

// =============================================================================
// stripScriptsAndStyles — Error Path Tests (injectable)
// =============================================================================

func TestStripScriptsAndStyles_ShouldReturnErrorWhenGoQueryParseFails(t *testing.T) {
	original := scrapeGoQueryParseFunc
	scrapeGoQueryParseFunc = func(r io.Reader) (*goquery.Document, error) {
		return nil, fmt.Errorf("forced parse error")
	}
	defer func() { scrapeGoQueryParseFunc = original }()

	_, err := stripScriptsAndStyles([]byte("<html></html>"))
	if err == nil {
		t.Fatal("Expected error when goquery parse fails")
	}
	if !strings.Contains(err.Error(), "failed to parse HTML") {
		t.Errorf("Expected 'failed to parse HTML' in error, got: %v", err)
	}
}

func TestStripScriptsAndStyles_ShouldReturnErrorWhenRenderFails(t *testing.T) {
	original := scrapeRenderHTMLFunc
	scrapeRenderHTMLFunc = func(doc *goquery.Document) (string, error) {
		return "", fmt.Errorf("forced render error")
	}
	defer func() { scrapeRenderHTMLFunc = original }()

	_, err := stripScriptsAndStyles([]byte("<html><body>Test</body></html>"))
	if err == nil {
		t.Fatal("Expected error when render fails")
	}
	if !strings.Contains(err.Error(), "failed to render HTML") {
		t.Errorf("Expected 'failed to render HTML' in error, got: %v", err)
	}
}

// =============================================================================
// extractReadableContent — Error Path Tests (injectable)
// =============================================================================

func TestExtractReadableContent_ShouldReturnErrorWhenURLParseFails(t *testing.T) {
	original := scrapeURLParseFunc
	scrapeURLParseFunc = func(rawURL string) (*url.URL, error) {
		return nil, fmt.Errorf("forced URL parse error")
	}
	defer func() { scrapeURLParseFunc = original }()

	_, err := extractReadableContent("<html></html>", "https://example.com")
	if err == nil {
		t.Fatal("Expected error when URL parse fails")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("Expected 'invalid URL' in error, got: %v", err)
	}
}

func TestExtractReadableContent_ShouldReturnErrorWhenReadabilityFails(t *testing.T) {
	original := scrapeReadabilityFunc
	scrapeReadabilityFunc = func(input io.Reader, pageURL *url.URL) (readability.Article, error) {
		return readability.Article{}, fmt.Errorf("forced readability failure")
	}
	defer func() { scrapeReadabilityFunc = original }()

	_, err := extractReadableContent("<html></html>", "https://example.com")
	if err == nil {
		t.Fatal("Expected error when readability fails")
	}
	if !strings.Contains(err.Error(), "readability extraction failed") {
		t.Errorf("Expected 'readability extraction failed' in error, got: %v", err)
	}
}

// =============================================================================
// extractPlainText — Error Path Tests (injectable)
// =============================================================================

func TestExtractPlainText_ShouldReturnErrorWhenGoQueryParseFails(t *testing.T) {
	original := scrapeGoQueryParseFunc
	scrapeGoQueryParseFunc = func(r io.Reader) (*goquery.Document, error) {
		return nil, fmt.Errorf("forced parse error")
	}
	defer func() { scrapeGoQueryParseFunc = original }()

	_, err := extractPlainText("<html></html>")
	if err == nil {
		t.Fatal("Expected error when goquery parse fails")
	}
	if !strings.Contains(err.Error(), "failed to parse HTML") {
		t.Errorf("Expected 'failed to parse HTML' in error, got: %v", err)
	}
}

// =============================================================================
// DefaultHTTPFetcher.Fetch — Error Path Tests (injectable)
// =============================================================================

func TestDefaultHTTPFetcher_Fetch_ShouldReturnErrorWhenReadAllFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html></html>"))
	}))
	defer server.Close()

	original := scrapeReadAllFunc
	scrapeReadAllFunc = func(r io.Reader) ([]byte, error) {
		return nil, fmt.Errorf("forced read error")
	}
	defer func() { scrapeReadAllFunc = original }()

	fetcher := NewDefaultHTTPFetcher()
	_, err := fetcher.Fetch(server.URL)
	if err == nil {
		t.Fatal("Expected error when ReadAll fails")
	}
	if !strings.Contains(err.Error(), "failed to read response") {
		t.Errorf("Expected 'failed to read response' in error, got: %v", err)
	}
}

func TestDefaultHTTPFetcher_Fetch_ShouldReturnErrorWhenRequestCreationFails(t *testing.T) {
	original := scrapeHTTPNewRequestFunc
	scrapeHTTPNewRequestFunc = func(method, url string, body io.Reader) (*http.Request, error) {
		return nil, fmt.Errorf("forced request creation error")
	}
	defer func() { scrapeHTTPNewRequestFunc = original }()

	fetcher := NewDefaultHTTPFetcher()
	_, err := fetcher.Fetch("https://example.com")
	if err == nil {
		t.Fatal("Expected error when request creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create request") {
		t.Errorf("Expected 'failed to create request' in error, got: %v", err)
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*ScrapeTool)(nil)
var _ HTTPFetcher = (*DefaultHTTPFetcher)(nil)
