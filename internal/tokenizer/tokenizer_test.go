package tokenizer

import (
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// TikToken Tokenizer Tests
// =============================================================================

func TestNewTikToken_WhenValidEncoding_ShouldReturnTokenizer(t *testing.T) {
	tok, err := NewTikToken("cl100k_base")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tok == nil {
		t.Fatal("expected non-nil tokenizer")
	}
}

func TestNewTikToken_WhenInvalidEncoding_ShouldReturnError(t *testing.T) {
	tok, err := NewTikToken("totally_invalid_encoding_xyz")
	if err == nil {
		t.Fatal("expected error for invalid encoding")
	}
	if tok != nil {
		t.Fatal("expected nil tokenizer on error")
	}
}

func TestTikToken_CountTokens_WhenEmptyString_ShouldReturnZero(t *testing.T) {
	tok, err := NewTikToken("cl100k_base")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	count, err := tok.CountTokens("")
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", count)
	}
}

func TestTikToken_CountTokens_WhenSimpleText_ShouldReturnPositiveCount(t *testing.T) {
	tok, err := NewTikToken("cl100k_base")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	count, err := tok.CountTokens("Hello, world!")
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if count <= 0 {
		t.Errorf("expected positive token count for 'Hello, world!', got %d", count)
	}
}

func TestTikToken_CountTokens_WhenLongerText_ShouldReturnMoreTokens(t *testing.T) {
	tok, err := NewTikToken("cl100k_base")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	shortCount, err := tok.CountTokens("Hi")
	if err != nil {
		t.Fatalf("CountTokens short: %v", err)
	}

	longCount, err := tok.CountTokens("This is a significantly longer sentence with many more words in it")
	if err != nil {
		t.Fatalf("CountTokens long: %v", err)
	}

	if longCount <= shortCount {
		t.Errorf("expected longer text (%d tokens) > shorter text (%d tokens)", longCount, shortCount)
	}
}

func TestTikToken_CountTokens_WhenLargeDocument_ShouldCountAccurately(t *testing.T) {
	tok, err := NewTikToken("cl100k_base")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Build a ~1000-word document
	words := strings.Repeat("the quick brown fox jumps over the lazy dog ", 111)
	count, err := tok.CountTokens(words)
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	// 999 words should produce at least 500 tokens and less than 2000
	if count < 500 || count > 2000 {
		t.Errorf("expected token count in [500, 2000] for ~999 words, got %d", count)
	}
}

// Verify the tokenizer satisfies domain.Tokenizer interface
func TestTikToken_ShouldImplementTokenizerInterface(t *testing.T) {
	tok, err := NewTikToken("cl100k_base")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	var _ domain.Tokenizer = tok
}
