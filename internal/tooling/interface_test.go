package tooling

import (
	"context"
	"encoding/json"
	"testing"

	"ironclaw/internal/domain"
)

type stubTool struct {
	def domain.ToolDefinition
}

func (s stubTool) Definition() domain.ToolDefinition { return s.def }
func (s stubTool) Execute(ctx domain.ToolContext, input json.RawMessage) (*domain.ToolResult, error) {
	return &domain.ToolResult{Data: "ok"}, nil
}

func TestExecuteTool_WhenToolSucceeds_ShouldReturnResult(t *testing.T) {
	tool := stubTool{def: domain.ToolDefinition{Name: "test", Description: "test", InputSchema: nil}}
	ctx := context.Background()
	result, err := ExecuteTool(tool, ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Data != "ok" {
		t.Errorf("expected result Data=ok, got %v", result)
	}
}

func TestExecuteTool_WhenContextIsUsed_ShouldPassThrough(t *testing.T) {
	tool := stubTool{def: domain.ToolDefinition{Name: "test"}}
	ctx := context.Background()
	_, err := ExecuteTool(tool, ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
