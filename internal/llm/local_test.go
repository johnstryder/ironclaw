package llm

import (
	"context"
	"testing"
)

func TestLocalProvider_Generate_ShouldReturnEchoResponse(t *testing.T) {
	ctx := context.Background()
	p := NewLocalProvider("Local reply: ")

	got, err := p.Generate(ctx, "hello")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "Local reply: hello" {
		t.Errorf("want %q, got %q", "Local reply: hello", got)
	}
}

func TestLocalProvider_Generate_WhenPrefixEmpty_ShouldStillReturnPrompt(t *testing.T) {
	ctx := context.Background()
	p := NewLocalProvider("")

	got, err := p.Generate(ctx, "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "hi" {
		t.Errorf("want %q, got %q", "hi", got)
	}
}

func TestLocalProvider_Generate_WhenContextCanceled_ShouldReturnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewLocalProvider("prefix")

	_, err := p.Generate(ctx, "hi")
	if err == nil {
		t.Error("expected error when context canceled")
	}
}
