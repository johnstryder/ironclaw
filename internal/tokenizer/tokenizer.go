package tokenizer

import (
	"fmt"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

// TikToken wraps tiktoken-go to implement domain.Tokenizer.
type TikToken struct {
	encoding *tiktoken.Tiktoken
}

// NewTikToken creates a new TikToken tokenizer with the given encoding name.
// Common encodings: "cl100k_base" (GPT-4/3.5), "o200k_base" (GPT-4o).
// Returns an error if the encoding is not recognized.
func NewTikToken(encodingName string) (*TikToken, error) {
	enc, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		return nil, fmt.Errorf("tokenizer: unknown encoding %q: %w", encodingName, err)
	}
	return &TikToken{encoding: enc}, nil
}

// CountTokens returns the number of tokens in the given text.
func (t *TikToken) CountTokens(text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	tokens := t.encoding.Encode(text, nil, nil)
	return len(tokens), nil
}
