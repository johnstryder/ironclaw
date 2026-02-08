package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestConfig_JSONRoundtrip_ShouldPreserveData(t *testing.T) {
	want := Config{
		Gateway: GatewayConfig{
			Port: 8080,
			Auth: AuthConfig{
				Mode:                  "password",
				AuthToken:             "bearer-secret",
				PromptPIN:             "9999",
				RequirePINForExternal: true,
				ExternalChannels:      []string{"telegram"},
				RateLimitMaxAttempts:  5,
			},
			AllowedHosts: []string{"localhost"},
		},
		Agents: AgentsConfig{
			DefaultModel: "gpt-4",
			ModelAliases: map[string]string{"fast": "gpt-4o-mini"},
			Paths:        AgentPaths{Root: "/agents", Memory: "/memory"},
		},
		Infra: InfraConfig{LogFormat: "json", LogLevel: "info"},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Gateway.Port != want.Gateway.Port {
		t.Errorf("gateway.port: want %d, got %d", want.Gateway.Port, got.Gateway.Port)
	}
	if got.Gateway.Auth.Mode != want.Gateway.Auth.Mode {
		t.Errorf("gateway.auth.mode: want %q, got %q", want.Gateway.Auth.Mode, got.Gateway.Auth.Mode)
	}
	if got.Gateway.Auth.AuthToken != want.Gateway.Auth.AuthToken {
		t.Errorf("gateway.auth.authToken: want %q, got %q", want.Gateway.Auth.AuthToken, got.Gateway.Auth.AuthToken)
	}
	if got.Agents.Paths.Root != want.Agents.Paths.Root {
		t.Errorf("agents.paths.root: want %q, got %q", want.Agents.Paths.Root, got.Agents.Paths.Root)
	}
}

func TestMessage_UnmarshalJSON_WhenContentIsString_ShouldProduceSingleTextBlock(t *testing.T) {
	raw := `{"id":"m1","role":"user","timestamp":"2024-01-01T12:00:00Z","content":"hello"}`
	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(m.ContentBlocks) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(m.ContentBlocks))
	}
	tb, ok := m.ContentBlocks[0].(TextBlock)
	if !ok {
		t.Fatalf("expected TextBlock, got %T", m.ContentBlocks[0])
	}
	if tb.Text != "hello" {
		t.Errorf("text: want hello, got %q", tb.Text)
	}
}

func TestMessage_UnmarshalJSON_WhenContentIsArrayOfBlocks_ShouldParseByType(t *testing.T) {
	raw := `{
		"id":"m2","role":"assistant","timestamp":"2024-01-01T12:00:00Z",
		"content":[
			{"type":"text","text":"here is an image"},
			{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"/9j/4AAQ"}},
			{"type":"tool_use","id":"call_1","name":"search","input":{}},
			{"type":"tool_result","tool_use_id":"call_1","content":"found it","is_error":false}
		]
	}`
	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(m.ContentBlocks) != 4 {
		t.Fatalf("expected 4 content blocks, got %d", len(m.ContentBlocks))
	}
	if m.ContentBlocks[0].Type() != BlockText {
		t.Errorf("block 0: want text, got %s", m.ContentBlocks[0].Type())
	}
	if m.ContentBlocks[1].Type() != BlockImage {
		t.Errorf("block 1: want image, got %s", m.ContentBlocks[1].Type())
	}
	if m.ContentBlocks[2].Type() != BlockToolUse {
		t.Errorf("block 2: want tool_use, got %s", m.ContentBlocks[2].Type())
	}
	tu := m.ContentBlocks[2].(ToolUseBlock)
	if tu.Name != "search" || tu.ToolUseID != "call_1" {
		t.Errorf("tool_use: id=%q name=%q", tu.ToolUseID, tu.Name)
	}
	if m.ContentBlocks[3].Type() != BlockToolResult {
		t.Errorf("block 3: want tool_result, got %s", m.ContentBlocks[3].Type())
	}
	tr := m.ContentBlocks[3].(ToolResultBlock)
	if tr.ToolUseID != "call_1" || tr.Content != "found it" || tr.IsError {
		t.Errorf("tool_result: id=%q content=%q is_error=%v", tr.ToolUseID, tr.Content, tr.IsError)
	}
}

func TestMessage_UnmarshalJSON_WhenContentIsEmpty_ShouldNotPanic(t *testing.T) {
	raw := `{"id":"m3","role":"system","timestamp":"2024-01-01T12:00:00Z","content":""}`
	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Empty string is decoded as one TextBlock with empty text; no panic.
	if len(m.ContentBlocks) > 1 {
		t.Errorf("expected at most 1 content block for empty content, got len=%d", len(m.ContentBlocks))
	}
}

func TestMessage_UnmarshalJSON_WhenContentIsOmitted_ShouldLeaveContentBlocksNil(t *testing.T) {
	raw := `{"id":"m4","role":"user","timestamp":"2024-01-01T12:00:00Z"}`
	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.ContentBlocks != nil {
		t.Errorf("expected nil ContentBlocks when content omitted, got len=%d", len(m.ContentBlocks))
	}
}

func TestMessage_UnmarshalJSON_WhenContentIsNotStringOrArray_ShouldReturnError(t *testing.T) {
	// Content must be string or array of blocks; number/object/bool cause Unmarshal(a.Content, &raw) to fail.
	raw := `{"id":"m5","role":"user","timestamp":"2024-01-01T12:00:00Z","content":123}`
	var m Message
	err := json.Unmarshal([]byte(raw), &m)
	if err == nil {
		t.Fatal("expected error when content is number")
	}
	if m.ContentBlocks != nil {
		t.Error("ContentBlocks should be nil when unmarshal fails")
	}
}

func TestParseMessageContent_WhenContentIsNotStringOrArray_ShouldReturnError(t *testing.T) {
	_, err := parseMessageContent(json.RawMessage("123"))
	if err == nil {
		t.Fatal("expected error when content is number")
	}
}

func TestMessage_UnmarshalJSON_WhenTopLevelJSONIsInvalid_ShouldReturnError(t *testing.T) {
	// Valid JSON but invalid timestamp so first Unmarshal(data, &a) fails and we hit return err.
	raw := `{"id":"x","role":"user","timestamp":999,"content":"hi"}`
	var m Message
	err := json.Unmarshal([]byte(raw), &m)
	if err == nil {
		t.Fatal("expected error when timestamp is invalid type")
	}
}

func TestMessage_UnmarshalJSON_WhenArrayElementHasInvalidJSON_ShouldSkipElement(t *testing.T) {
	raw := `{"id":"m6","role":"assistant","timestamp":"2024-01-01T12:00:00Z","content":[{"type":"text","text":"ok"}, "not valid json", {"type":"text","text":"two"}]}`
	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// First and third elements parse; second is skipped (invalid JSON for typeOnly)
	if len(m.ContentBlocks) != 2 {
		t.Errorf("expected 2 blocks (invalid element skipped), got %d", len(m.ContentBlocks))
	}
}

func TestMessage_UnmarshalJSON_WhenArrayElementHasUnknownType_ShouldSkipElement(t *testing.T) {
	raw := `{"id":"m7","role":"assistant","timestamp":"2024-01-01T12:00:00Z","content":[{"type":"text","text":"ok"},{"type":"unknown_thing"},{"type":"text","text":"two"}]}`
	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(m.ContentBlocks) != 2 {
		t.Errorf("expected 2 blocks (unknown type skipped), got %d", len(m.ContentBlocks))
	}
}

func TestMessage_UnmarshalJSON_WhenTextBlockUnmarshalFails_ShouldSkipElement(t *testing.T) {
	// type is "text" but "text" field is object not string -> Unmarshal to TextBlock fails
	raw := `{"id":"m8","role":"assistant","timestamp":"2024-01-01T12:00:00Z","content":[{"type":"text","text":{}}]}`
	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(m.ContentBlocks) != 0 {
		t.Errorf("expected 0 blocks (invalid text block skipped), got %d", len(m.ContentBlocks))
	}
}

func TestAgentStatus_Constants(t *testing.T) {
	if StatusIdle != "idle" || StatusThinking != "thinking" || StatusTyping != "typing" || StatusFailed != "failed" {
		t.Error("AgentStatus constants mismatch")
	}
}

func TestMessageRole_Constants(t *testing.T) {
	if RoleUser != "user" || RoleAssistant != "assistant" || RoleSystem != "system" || RoleTool != "tool" {
		t.Error("MessageRole constants mismatch")
	}
}

func TestSessionAuthState_ZeroValue(t *testing.T) {
	var s SessionAuthState
	if s.IsAuthenticated || s.Attempts != 0 {
		t.Errorf("zero value: IsAuthenticated=%v Attempts=%d", s.IsAuthenticated, s.Attempts)
	}
}

func TestToolResult_JSONRoundtrip(t *testing.T) {
	want := ToolResult{
		Data:      "ok",
		Metadata:  map[string]string{"k": "v"},
		Artifacts: []string{"a.txt"},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ToolResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Data != want.Data || got.Metadata["k"] != "v" || len(got.Artifacts) != 1 || got.Artifacts[0] != "a.txt" {
		t.Errorf("roundtrip: got Data=%q Metadata=%v Artifacts=%v", got.Data, got.Metadata, got.Artifacts)
	}
}

func TestToolDefinition_JSONRoundtrip(t *testing.T) {
	want := ToolDefinition{
		Name:        "echo",
		Description: "Echo input",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ToolDefinition
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != want.Name || got.Description != want.Description || string(got.InputSchema) != string(want.InputSchema) {
		t.Errorf("roundtrip: got Name=%q Desc=%q Schema=%s", got.Name, got.Description, got.InputSchema)
	}
}

func TestSession_JSONRoundtrip(t *testing.T) {
	ts := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	want := Session{
		ID: "s1", ChannelID: "ch1", Platform: "telegram",
		CreatedAt: ts, UpdatedAt: ts,
		Status: StatusIdle, Metadata: map[string]string{"k": "v"},
		AuthState: SessionAuthState{IsAuthenticated: true, Attempts: 0, LastAttempt: ts},
		History: nil, TokenCount: 0, HistoryPath: "/hist",
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Session
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != want.ID || got.Platform != want.Platform || got.Status != want.Status {
		t.Errorf("roundtrip: got ID=%q Platform=%q Status=%s", got.ID, got.Platform, got.Status)
	}
	if !got.AuthState.IsAuthenticated {
		t.Error("auth state not preserved")
	}
}
