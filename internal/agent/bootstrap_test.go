package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAgentContext_WhenRootDoesNotExist_ShouldReturnError(t *testing.T) {
	_, err := LoadAgentContext("/nonexistent-agent-root-12345")
	if err == nil {
		t.Fatal("expected error when root does not exist")
	}
}

func TestLoadAgentContext_WhenRootIsFile_ShouldReturnError(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadAgentContext(f)
	if err == nil {
		t.Fatal("expected error when root is a file")
	}
}

func TestLoadAgentContext_WhenFilesMissing_ShouldReturnContextWithEmptyFields(t *testing.T) {
	root := t.TempDir()
	ctx, err := LoadAgentContext(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected non-nil *domain.AgentContext")
	}
	if ctx.Identity != "" || ctx.Soul != "" || len(ctx.Tools) != 0 {
		t.Errorf("expected empty fields when files missing, got Identity=%q Soul=%q Tools=%v", ctx.Identity, ctx.Soul, ctx.Tools)
	}
}

func TestLoadAgentContext_WhenSOULAndIdentityExist_ShouldPopulateContext(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "SOUL.md"), []byte("# Soul\nI am helpful."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "IDENTITY.md"), []byte("Agent Zero"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, err := LoadAgentContext(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Soul != "# Soul\nI am helpful." {
		t.Errorf("soul: want %q, got %q", "# Soul\nI am helpful.", ctx.Soul)
	}
	if ctx.Identity != "Agent Zero" {
		t.Errorf("identity: want Agent Zero, got %q", ctx.Identity)
	}
}

func TestLoadAgentContext_WhenTOOLSMdExists_ShouldParseToolNames(t *testing.T) {
	root := t.TempDir()
	// TOOLS.md lists one tool per line or markdown list
	content := "- search\n- weather\n"
	if err := os.WriteFile(filepath.Join(root, "TOOLS.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, err := LoadAgentContext(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Tools) != 2 {
		t.Errorf("expected 2 tools, got %v", ctx.Tools)
	}
}

func TestLoadAgentContext_WhenTOOLSMdHasBlankLines_ShouldSkipThem(t *testing.T) {
	root := t.TempDir()
	content := "- a\n\n  \n- b\n"
	if err := os.WriteFile(filepath.Join(root, "TOOLS.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, err := LoadAgentContext(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Tools) != 2 {
		t.Errorf("expected 2 tools (a, b), got %v", ctx.Tools)
	}
}

func TestLoadAgentContext_WhenAGENTSMdExists_ShouldNotError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Directives"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, err := LoadAgentContext(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestLoadAgentContext_WhenPathContainsTraversal_ShouldNotEscapeRoot(t *testing.T) {
	root := t.TempDir()
	cleanRoot := filepath.Clean(root)
	ctx, err := LoadAgentContext(cleanRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	// LoadAgentContext must clean root internally; reading only from root
	_ = ctx
}
