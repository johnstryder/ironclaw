package injection

import (
	"fmt"
	"io"
	"os"
	"strings"

	"ironclaw/internal/domain"
)

// Default high-risk phrases (case-insensitive).
var defaultPatterns = []string{
	"ignore previous",
	"system prompt",
	"simulated mode",
}

// ScanResult holds the result of a prompt-injection scan.
type ScanResult struct {
	Detected bool     // true if any high-risk pattern was found
	Patterns []string // matched phrases
}

// Scan checks text for high-risk prompt-injection keywords and returns a ScanResult.
func Scan(text string) ScanResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return ScanResult{}
	}
	lower := strings.ToLower(text)
	var matched []string
	for _, p := range defaultPatterns {
		if strings.Contains(lower, p) {
			matched = append(matched, p)
		}
	}
	if len(matched) == 0 {
		return ScanResult{}
	}
	return ScanResult{Detected: true, Patterns: matched}
}

// ScanMessage extracts text from msg's ContentBlocks (TextBlock only) and runs Scan.
func ScanMessage(msg *domain.Message) ScanResult {
	if msg == nil {
		return ScanResult{}
	}
	var combined strings.Builder
	for _, b := range msg.ContentBlocks {
		if tb, ok := b.(domain.TextBlock); ok {
			combined.WriteString(tb.Text)
			combined.WriteString("\n")
		}
	}
	return Scan(combined.String())
}

// LogIfDetected scans text and, if injection is detected, writes a security alert to w (or stdout if w is nil), then returns the result.
func LogIfDetected(text string, w io.Writer) ScanResult {
	r := Scan(text)
	if !r.Detected {
		return r
	}
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, "[security] prompt injection may be present (matched: %v)\n", r.Patterns)
	return r
}
