package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	ironctx "ironclaw/internal/context"
	"ironclaw/internal/domain"
	"ironclaw/internal/tokenizer"
)

// =============================================================================
// main / runDemo
// =============================================================================

func TestMain_ShouldRunWithoutError(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	w.Close()
	os.Stdout = old

	var buf [16384]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "Adaptive Context Chunking Demo") {
		t.Errorf("expected demo header in output, got: %.100s...", output)
	}
	if !strings.Contains(output, "Demo Complete") {
		t.Errorf("expected 'Demo Complete' in output, got: %.100s...", output)
	}
}

func TestMain_WhenRunDemoFails_ShouldCallExitWithOne(t *testing.T) {
	oldExit := exitFunc
	oldTok := newTokenizerFn
	defer func() {
		exitFunc = oldExit
		newTokenizerFn = oldTok
	}()

	newTokenizerFn = func() (*tokenizer.TikToken, error) {
		return nil, errors.New("bad encoding")
	}

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	main()

	if exitCode != 1 {
		t.Errorf("want exit code 1, got %d", exitCode)
	}
}

func TestDefaultNewTokenizerFn_ShouldReturnTokenizer(t *testing.T) {
	tok, err := newTokenizerFn()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok == nil {
		t.Fatal("expected non-nil tokenizer")
	}
}

func TestRunDemo_WhenTokenizerFails_ShouldReturnError(t *testing.T) {
	oldTok := newTokenizerFn
	defer func() { newTokenizerFn = oldTok }()

	newTokenizerFn = func() (*tokenizer.TikToken, error) {
		return nil, errors.New("bad encoding")
	}

	err := runDemo()
	if err == nil {
		t.Fatal("expected error when tokenizer fails")
	}
	if !strings.Contains(err.Error(), "tokenizer") {
		t.Errorf("error should mention tokenizer, got: %v", err)
	}
}

func TestRunDemo_WhenFit200Fails_ShouldReturnError(t *testing.T) {
	oldFit := fitToWindowFn
	defer func() { fitToWindowFn = oldFit }()

	call := 0
	fitToWindowFn = func(mgr domain.ContextManager, msgs []domain.Message, sys string) ([]domain.Message, error) {
		call++
		if call == 1 { // first FitToWindow call = 200-token section
			return nil, errors.New("fit200 boom")
		}
		return oldFit(mgr, msgs, sys)
	}

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runDemo()

	w.Close()
	os.Stdout = old

	if err == nil || !strings.Contains(err.Error(), "fit 200") {
		t.Errorf("expected 'fit 200' error, got: %v", err)
	}
}

func TestRunDemo_WhenFit1000Fails_ShouldReturnError(t *testing.T) {
	oldFit := fitToWindowFn
	defer func() { fitToWindowFn = oldFit }()

	call := 0
	fitToWindowFn = func(mgr domain.ContextManager, msgs []domain.Message, sys string) ([]domain.Message, error) {
		call++
		if call == 2 { // second FitToWindow call = 1000-token section
			return nil, errors.New("fit1000 boom")
		}
		return oldFit(mgr, msgs, sys)
	}

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runDemo()

	w.Close()
	os.Stdout = old

	if err == nil || !strings.Contains(err.Error(), "fit 1000") {
		t.Errorf("expected 'fit 1000' error, got: %v", err)
	}
}

func TestRunDemo_WhenFitHugeFails_ShouldReturnError(t *testing.T) {
	oldFit := fitToWindowFn
	defer func() { fitToWindowFn = oldFit }()

	call := 0
	fitToWindowFn = func(mgr domain.ContextManager, msgs []domain.Message, sys string) ([]domain.Message, error) {
		call++
		if call == 3 { // third FitToWindow call = huge section
			return nil, errors.New("fithuge boom")
		}
		return oldFit(mgr, msgs, sys)
	}

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runDemo()

	w.Close()
	os.Stdout = old

	if err == nil || !strings.Contains(err.Error(), "fit huge") {
		t.Errorf("expected 'fit huge' error, got: %v", err)
	}
}

func TestDefaultFitToWindowFn_ShouldDelegateToManager(t *testing.T) {
	tok, err := tokenizer.NewTikToken("cl100k_base")
	if err != nil {
		t.Fatalf("tokenizer: %v", err)
	}
	mgr := ironctx.NewManager(tok, 1000)
	msgs := buildConversation(3)
	result, err := fitToWindowFn(mgr, msgs, "sys")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestRunDemo_ShouldReturnNil(t *testing.T) {
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runDemo()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runDemo: %v", err)
	}
}

func TestRunDemo_ShouldPrintAllSections(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = runDemo()

	w.Close()
	os.Stdout = old

	var buf [32768]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	sections := []string{
		"1. Tokenizer Basics",
		"2. Sliding Window (200-token budget)",
		"3. Sliding Window (1000-token budget)",
		"4. Edge Case: Single Huge Message",
		"5. MessageText Extraction",
	}
	for _, section := range sections {
		if !strings.Contains(output, section) {
			t.Errorf("missing section %q in output", section)
		}
	}
}

// =============================================================================
// buildConversation
// =============================================================================

func TestBuildConversation_ShouldReturnNMessages(t *testing.T) {
	msgs := buildConversation(10)
	if len(msgs) != 10 {
		t.Errorf("expected 10 messages, got %d", len(msgs))
	}
}

func TestBuildConversation_ShouldAlternateRoles(t *testing.T) {
	msgs := buildConversation(4)
	expected := []domain.MessageRole{
		domain.RoleUser, domain.RoleAssistant,
		domain.RoleUser, domain.RoleAssistant,
	}
	for i, want := range expected {
		if msgs[i].Role != want {
			t.Errorf("msg[%d].Role: want %s, got %s", i, want, msgs[i].Role)
		}
	}
}

func TestBuildConversation_WhenZero_ShouldReturnEmpty(t *testing.T) {
	msgs := buildConversation(0)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

// =============================================================================
// makeMsg
// =============================================================================

func TestMakeMsg_ShouldSetRole(t *testing.T) {
	msg := makeMsg(domain.RoleUser, "hello")
	if msg.Role != domain.RoleUser {
		t.Errorf("role: want user, got %s", msg.Role)
	}
}

func TestMakeMsg_ShouldSetTextBlock(t *testing.T) {
	msg := makeMsg(domain.RoleAssistant, "world")
	if len(msg.ContentBlocks) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msg.ContentBlocks))
	}
	tb, ok := msg.ContentBlocks[0].(domain.TextBlock)
	if !ok {
		t.Fatalf("expected TextBlock, got %T", msg.ContentBlocks[0])
	}
	if tb.Text != "world" {
		t.Errorf("text: want 'world', got %q", tb.Text)
	}
}

func TestMakeMsg_ShouldSetRawContent(t *testing.T) {
	msg := makeMsg(domain.RoleUser, "test")
	if msg.RawContent == nil {
		t.Fatal("expected non-nil RawContent")
	}
}

// =============================================================================
// printResults
// =============================================================================

func TestPrintResults_WhenFittedHasMessages_ShouldPrintKeptMessages(t *testing.T) {
	tok, err := tokenizer.NewTikToken("cl100k_base")
	if err != nil {
		t.Fatalf("tokenizer: %v", err)
	}

	original := buildConversation(10)
	fitted := buildConversation(3)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printResults(tok, original, fitted, "system", 200)

	w.Close()
	os.Stdout = old

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "Original messages: 10") {
		t.Errorf("expected original count in output, got: %s", output)
	}
	if !strings.Contains(output, "Fitted messages  : 3") {
		t.Errorf("expected fitted count in output, got: %s", output)
	}
	if !strings.Contains(output, "Dropped          : 7") {
		t.Errorf("expected dropped count in output, got: %s", output)
	}
	if !strings.Contains(output, "Kept messages") {
		t.Errorf("expected 'Kept messages' in output, got: %s", output)
	}
}

func TestPrintResults_WhenFittedEmpty_ShouldNotPrintKeptMessages(t *testing.T) {
	tok, err := tokenizer.NewTikToken("cl100k_base")
	if err != nil {
		t.Fatalf("tokenizer: %v", err)
	}

	original := buildConversation(5)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printResults(tok, original, nil, "system", 100)

	w.Close()
	os.Stdout = old

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if strings.Contains(output, "Kept messages") {
		t.Errorf("should not print 'Kept messages' when fitted is empty, got: %s", output)
	}
}
