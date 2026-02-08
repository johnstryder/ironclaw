package context

import (
	"encoding/json"
	"strings"

	"ironclaw/internal/domain"
)

// MessageText extracts a text representation of a Message for token counting.
// It inspects ContentBlocks first; falls back to RawContent if blocks are nil.
func MessageText(msg domain.Message) string {
	if len(msg.ContentBlocks) > 0 {
		return blocksToText(msg.ContentBlocks)
	}
	return rawContentToText(msg.RawContent)
}

// blocksToText converts parsed ContentBlocks to a single string.
func blocksToText(blocks []domain.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch b := block.(type) {
		case domain.TextBlock:
			parts = append(parts, b.Text)
		case domain.ToolUseBlock:
			parts = append(parts, b.Name+" "+string(b.Input))
		case domain.ToolResultBlock:
			parts = append(parts, b.Content)
		case domain.ImageBlock:
			parts = append(parts, "[image]")
		}
	}
	return strings.Join(parts, "\n")
}

// rawContentToText extracts text from RawContent JSON (string or opaque JSON).
func rawContentToText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try to unmarshal as a plain string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Fall back to the raw JSON text itself.
	return string(raw)
}
