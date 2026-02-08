// Command context_demo exercises the adaptive context chunking feature manually.
// It creates a simulated conversation, counts tokens, and demonstrates the
// sliding-window strategy dropping older messages to fit the token budget.
//
// Usage:
//
//	go run ./cmd/context_demo/
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	ironctx "ironclaw/internal/context"
	"ironclaw/internal/domain"
	"ironclaw/internal/tokenizer"
)

// exitFunc is the function used by main to exit; tests replace it.
var exitFunc = os.Exit

func main() {
	if err := runDemo(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		exitFunc(1)
	}
}

// newTokenizerFn creates a tokenizer; tests replace to inject errors.
var newTokenizerFn = func() (*tokenizer.TikToken, error) {
	return tokenizer.NewTikToken("cl100k_base")
}

// fitToWindowFn wraps context manager FitToWindow; tests replace to inject errors.
var fitToWindowFn = func(mgr domain.ContextManager, msgs []domain.Message, sys string) ([]domain.Message, error) {
	return mgr.FitToWindow(msgs, sys)
}

func runDemo() error {
	fmt.Println("=== Adaptive Context Chunking Demo ===")
	fmt.Println()

	// -----------------------------------------------------------------------
	// 1. Tokenizer basics
	// -----------------------------------------------------------------------
	tok, err := newTokenizerFn()
	if err != nil {
		return fmt.Errorf("tokenizer: %w", err)
	}

	fmt.Println("--- 1. Tokenizer Basics ---")

	// Large document
	doc := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 1000)
	docTokens, _ := tok.CountTokens(doc)
	fmt.Printf("Large document : %d chars, %d tokens\n", len(doc), docTokens)

	// Short prompt
	short := "Hello, world!"
	shortTokens, _ := tok.CountTokens(short)
	fmt.Printf("Short prompt   : %q = %d tokens\n", short, shortTokens)

	// Typical system prompt
	sys := "You are a helpful Go programming assistant. Answer concisely."
	sysTokens, _ := tok.CountTokens(sys)
	fmt.Printf("System prompt  : %q = %d tokens\n", sys, sysTokens)
	fmt.Println()

	// -----------------------------------------------------------------------
	// 2. Sliding-window context manager with a small budget (200 tokens)
	// -----------------------------------------------------------------------
	fmt.Println("--- 2. Sliding Window (200-token budget) ---")

	mgr := ironctx.NewManager(tok, 200)

	msgs := buildConversation(50)
	fitted, err := fitToWindowFn(mgr, msgs, sys)
	if err != nil {
		return fmt.Errorf("fit 200: %w", err)
	}

	printResults(tok, msgs, fitted, sys, 200)
	fmt.Println()

	// -----------------------------------------------------------------------
	// 3. Sliding-window with a larger budget (1000 tokens)
	// -----------------------------------------------------------------------
	fmt.Println("--- 3. Sliding Window (1000-token budget) ---")

	mgr1k := ironctx.NewManager(tok, 1000)
	fitted1k, err := fitToWindowFn(mgr1k, msgs, sys)
	if err != nil {
		return fmt.Errorf("fit 1000: %w", err)
	}

	printResults(tok, msgs, fitted1k, sys, 1000)
	fmt.Println()

	// -----------------------------------------------------------------------
	// 4. Edge case: single huge message
	// -----------------------------------------------------------------------
	fmt.Println("--- 4. Edge Case: Single Huge Message ---")

	hugeText := strings.Repeat("word ", 500) // ~500 tokens
	hugeMsgs := []domain.Message{makeMsg(domain.RoleUser, hugeText)}
	mgrSmall := ironctx.NewManager(tok, 100) // only 100 token budget
	fittedHuge, err := fitToWindowFn(mgrSmall, hugeMsgs, "sys")
	if err != nil {
		return fmt.Errorf("fit huge: %w", err)
	}
	hugeTokens, _ := tok.CountTokens(hugeText)
	fmt.Printf("Message tokens: %d, Budget: 100\n", hugeTokens)
	fmt.Printf("Messages kept : %d (message too large, correctly dropped)\n", len(fittedHuge))
	fmt.Println()

	// -----------------------------------------------------------------------
	// 5. MessageText extraction from different block types
	// -----------------------------------------------------------------------
	fmt.Println("--- 5. MessageText Extraction ---")

	textMsg := domain.Message{
		ContentBlocks: []domain.ContentBlock{
			domain.TextBlock{Text: "Hello there!"},
		},
	}
	fmt.Printf("TextBlock      : %q\n", ironctx.MessageText(textMsg))

	toolMsg := domain.Message{
		ContentBlocks: []domain.ContentBlock{
			domain.ToolUseBlock{Name: "shell", Input: json.RawMessage(`{"cmd":"ls -la"}`)},
		},
	}
	fmt.Printf("ToolUseBlock   : %q\n", ironctx.MessageText(toolMsg))

	resultMsg := domain.Message{
		ContentBlocks: []domain.ContentBlock{
			domain.ToolResultBlock{Content: "file1.txt\nfile2.txt"},
		},
	}
	fmt.Printf("ToolResultBlock: %q\n", ironctx.MessageText(resultMsg))

	imgMsg := domain.Message{
		ContentBlocks: []domain.ContentBlock{
			domain.ImageBlock{},
		},
	}
	fmt.Printf("ImageBlock     : %q\n", ironctx.MessageText(imgMsg))

	fmt.Println()
	fmt.Println("=== Demo Complete ===")
	return nil
}

// buildConversation creates n alternating user/assistant messages.
func buildConversation(n int) []domain.Message {
	msgs := make([]domain.Message, 0, n)
	for i := 0; i < n; i++ {
		role := domain.RoleUser
		if i%2 == 1 {
			role = domain.RoleAssistant
		}
		text := fmt.Sprintf("This is message number %d in our conversation about Go programming", i)
		msgs = append(msgs, makeMsg(role, text))
	}
	return msgs
}

// makeMsg creates a Message with a single TextBlock.
func makeMsg(role domain.MessageRole, text string) domain.Message {
	raw, _ := json.Marshal(text)
	return domain.Message{
		Role:       role,
		RawContent: raw,
		ContentBlocks: []domain.ContentBlock{
			domain.TextBlock{Text: text},
		},
	}
}

// printResults prints the before/after of context fitting.
func printResults(tok *tokenizer.TikToken, original, fitted []domain.Message, sys string, budget int) {
	sysTokens, _ := tok.CountTokens(sys)
	totalOriginal := sysTokens
	for _, m := range original {
		c, _ := tok.CountTokens(ironctx.MessageText(m))
		totalOriginal += c
	}

	totalFitted := sysTokens
	for _, m := range fitted {
		c, _ := tok.CountTokens(ironctx.MessageText(m))
		totalFitted += c
	}

	fmt.Printf("Original messages: %d (%d tokens total incl. system)\n", len(original), totalOriginal)
	fmt.Printf("Fitted messages  : %d (%d tokens total incl. system)\n", len(fitted), totalFitted)
	fmt.Printf("Token budget     : %d\n", budget)
	fmt.Printf("Dropped          : %d oldest messages\n", len(original)-len(fitted))

	if len(fitted) > 0 {
		fmt.Println("Kept messages (most recent):")
		for _, m := range fitted {
			c, _ := tok.CountTokens(ironctx.MessageText(m))
			fmt.Printf("  [%s] (%d tok) %s\n", m.Role, c, ironctx.MessageText(m))
		}
	}
}
