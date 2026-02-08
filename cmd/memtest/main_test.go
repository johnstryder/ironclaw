package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

// =============================================================================
// echoProvider.Generate
// =============================================================================

func TestEchoProvider_Generate_ShouldReturnPromptAsIs(t *testing.T) {
	ep := &echoProvider{}
	result, err := ep.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("Expected 'hello', got '%s'", result)
	}
}

func TestEchoProvider_Generate_ShouldReturnEmptyStringForEmptyPrompt(t *testing.T) {
	ep := &echoProvider{}
	result, err := ep.Generate(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

// =============================================================================
// main
// =============================================================================

func TestMain_ShouldRunWithoutError(t *testing.T) {
	// Capture stdout to verify output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	w.Close()
	os.Stdout = old

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "Prompt sent to LLM") {
		t.Errorf("Expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "favorite color") {
		t.Errorf("Expected memory context in output, got: %s", output)
	}
}
