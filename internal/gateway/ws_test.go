package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"ironclaw/internal/domain"
)

func TestHandleWS_WhenValidMessageSent_ShouldEchoResponse(t *testing.T) {
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hello"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var out WSMessage
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.Type != "chat" || out.Content != "echo: hello" {
		t.Errorf("want type=chat content=echo: hello, got type=%q content=%q", out.Type, out.Content)
	}
}

func TestHandleWS_WhenInvalidJSONSent_ShouldReturnErrorType(t *testing.T) {
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("not json")); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	var out WSMessage
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.Type != "error" || out.Content != "invalid JSON" {
		t.Errorf("want type=error content=invalid JSON, got type=%q content=%q", out.Type, out.Content)
	}
}

func TestHandleWS_WhenMethodNotGet_ShouldReturn405(t *testing.T) {
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/ws", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /ws: want 405, got %d", rec.Code)
	}
}

func TestHandleWS_WhenNotWebSocketRequest_ShouldReturnBadRequest(t *testing.T) {
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	// No Upgrade or Connection headers — not a WebSocket handshake.
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("GET /ws without upgrade headers: want 400, got %d", rec.Code)
	}
}

func TestHandleWS_WhenAuthTokenSet_ShouldRequireBearer(t *testing.T) {
	cfg := &domain.GatewayConfig{
		Port: 0,
		Auth: domain.AuthConfig{AuthToken: "my-secret"},
	}
	srv, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /ws without token: want 401, got %d", rec.Code)
	}
}

func TestWriteWSMessage_WhenMarshalFails_ShouldNotSend(t *testing.T) {
	jsonMarshalMu.Lock()
	oldMarshal := jsonMarshal
	jsonMarshal = func(v any) ([]byte, error) { return nil, errors.New("marshal fail") }
	jsonMarshalMu.Unlock()
	defer func() {
		jsonMarshalMu.Lock()
		jsonMarshal = oldMarshal
		jsonMarshalMu.Unlock()
	}()

	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Send invalid JSON so server tries to write error reply; marshal fails so nothing is sent.
	_ = conn.WriteMessage(websocket.TextMessage, []byte("not json"))
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var out WSMessage
	err = conn.ReadJSON(&out)
	if err == nil {
		t.Error("expected no reply when marshal fails")
	}
}

// mockChatBrain returns a fixed response for tests.
type mockChatBrain struct {
	response string
	err      error
}

func (m *mockChatBrain) Generate(_ context.Context, _ string) (string, error) {
	return m.response, m.err
}

func TestHandleWS_WhenBrainProvidedAndTypeChat_ShouldReturnBrainResponse(t *testing.T) {
	brain := &mockChatBrain{response: "Brain says hi"}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hello"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Skip typing_start
	var typing WSMessage
	if err := conn.ReadJSON(&typing); err != nil {
		t.Fatalf("ReadJSON typing_start: %v", err)
	}

	var out WSMessage
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.Type != "chat" || out.Content != "Brain says hi" {
		t.Errorf("want type=chat content=Brain says hi, got type=%q content=%q", out.Type, out.Content)
	}
}

func TestHandleWS_WhenBrainReturnsError_ShouldReturnErrorContent(t *testing.T) {
	brain := &mockChatBrain{err: errors.New("provider failed")}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hi"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Skip typing_start
	var typing WSMessage
	if err := conn.ReadJSON(&typing); err != nil {
		t.Fatalf("ReadJSON typing_start: %v", err)
	}

	var out WSMessage
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.Type != "chat" || !strings.HasPrefix(out.Content, "error: ") {
		t.Errorf("want type=chat content prefix error: , got type=%q content=%q", out.Type, out.Content)
	}
}

// =============================================================================
// ChannelID tests (multi-channel routing)
// =============================================================================

func TestWSMessage_WhenChannelIDSet_ShouldMarshalAndUnmarshal(t *testing.T) {
	original := WSMessage{Type: "chat", Content: "hello", ChannelID: "general"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded WSMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.ChannelID != "general" {
		t.Errorf("ChannelID: want 'general', got %q", decoded.ChannelID)
	}
	if decoded.Type != "chat" || decoded.Content != "hello" {
		t.Errorf("Type/Content mismatch: %+v", decoded)
	}
}

func TestWSMessage_WhenChannelIDOmitted_ShouldDefaultToEmpty(t *testing.T) {
	raw := `{"type":"chat","content":"hello"}`
	var msg WSMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if msg.ChannelID != "" {
		t.Errorf("expected empty ChannelID, got %q", msg.ChannelID)
	}
}

func TestHandleWS_WhenChannelIDProvided_ShouldRouteToChannel(t *testing.T) {
	brain := &mockChatBrain{response: "reply from brain"}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Send message with ChannelID
	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hello", ChannelID: "general"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Skip typing_start
	var typing WSMessage
	if err := conn.ReadJSON(&typing); err != nil {
		t.Fatalf("ReadJSON typing_start: %v", err)
	}

	var out WSMessage
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.ChannelID != "general" {
		t.Errorf("response ChannelID: want 'general', got %q", out.ChannelID)
	}
	if out.Content != "reply from brain" {
		t.Errorf("response Content: want 'reply from brain', got %q", out.Content)
	}
}

func TestHandleWS_WhenNoChannelID_ShouldUseDefaultChannel(t *testing.T) {
	brain := &mockChatBrain{response: "default reply"}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Send message without ChannelID
	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hello"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Skip typing_start
	var typing WSMessage
	if err := conn.ReadJSON(&typing); err != nil {
		t.Fatalf("ReadJSON typing_start: %v", err)
	}

	var out WSMessage
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.ChannelID != "default" {
		t.Errorf("response ChannelID: want 'default', got %q", out.ChannelID)
	}
	if out.Content != "default reply" {
		t.Errorf("response Content: want 'default reply', got %q", out.Content)
	}
}

func TestHandleWS_WhenTwoChannels_ShouldIsolateResponses(t *testing.T) {
	// Brain that returns different responses based on prompt
	brain := &channelAwareBrain{
		responses: map[string]string{
			"general-msg": "general-reply",
			"support-msg": "support-reply",
		},
	}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Send to #general
	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "general-msg", ChannelID: "general"}); err != nil {
		t.Fatalf("WriteJSON general: %v", err)
	}
	// Read typing_start, response, typing_stop for #general
	var generalMsgs [3]WSMessage
	for i := range generalMsgs {
		if err := conn.ReadJSON(&generalMsgs[i]); err != nil {
			t.Fatalf("ReadJSON general[%d]: %v", i, err)
		}
	}
	if generalMsgs[1].ChannelID != "general" {
		t.Errorf("general ChannelID: want 'general', got %q", generalMsgs[1].ChannelID)
	}
	if generalMsgs[1].Content != "general-reply" {
		t.Errorf("general Content: want 'general-reply', got %q", generalMsgs[1].Content)
	}

	// Send to #support
	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "support-msg", ChannelID: "support"}); err != nil {
		t.Fatalf("WriteJSON support: %v", err)
	}
	// Read typing_start, response, typing_stop for #support
	var supportMsgs [3]WSMessage
	for i := range supportMsgs {
		if err := conn.ReadJSON(&supportMsgs[i]); err != nil {
			t.Fatalf("ReadJSON support[%d]: %v", i, err)
		}
	}
	if supportMsgs[1].ChannelID != "support" {
		t.Errorf("support ChannelID: want 'support', got %q", supportMsgs[1].ChannelID)
	}
	if supportMsgs[1].Content != "support-reply" {
		t.Errorf("support Content: want 'support-reply', got %q", supportMsgs[1].Content)
	}
}

// channelAwareBrain returns different responses based on prompt content.
type channelAwareBrain struct {
	responses map[string]string
}

func (b *channelAwareBrain) Generate(_ context.Context, prompt string) (string, error) {
	if resp, ok := b.responses[prompt]; ok {
		return resp, nil
	}
	return "unknown", nil
}

// =============================================================================
// Typing indicator tests
// =============================================================================

func TestHandleWS_WhenBrainAndChatMessage_ShouldSendTypingStartBeforeResponse(t *testing.T) {
	brain := &mockChatBrain{response: "hello back"}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hi"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// First message should be typing_start
	var msg1 WSMessage
	if err := conn.ReadJSON(&msg1); err != nil {
		t.Fatalf("ReadJSON typing_start: %v", err)
	}
	if msg1.Type != "typing_start" {
		t.Errorf("first message type: want 'typing_start', got %q", msg1.Type)
	}
}

func TestHandleWS_WhenBrainAndChatMessage_ShouldSendTypingStopAfterResponse(t *testing.T) {
	brain := &mockChatBrain{response: "hello back"}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hi"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Read all three messages: typing_start, chat response, typing_stop
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msgs []WSMessage
	for i := 0; i < 3; i++ {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ReadJSON[%d]: %v", i, err)
		}
		msgs = append(msgs, msg)
	}

	if msgs[0].Type != "typing_start" {
		t.Errorf("message[0] type: want 'typing_start', got %q", msgs[0].Type)
	}
	if msgs[1].Type != "chat" || msgs[1].Content != "hello back" {
		t.Errorf("message[1]: want type=chat content='hello back', got type=%q content=%q", msgs[1].Type, msgs[1].Content)
	}
	if msgs[2].Type != "typing_stop" {
		t.Errorf("message[2] type: want 'typing_stop', got %q", msgs[2].Type)
	}
}

func TestHandleWS_WhenTypingIndicator_ShouldIncludeChannelID(t *testing.T) {
	brain := &mockChatBrain{response: "reply"}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hi", ChannelID: "general"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Read all three messages
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msgs []WSMessage
	for i := 0; i < 3; i++ {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ReadJSON[%d]: %v", i, err)
		}
		msgs = append(msgs, msg)
	}

	// typing_start should carry the channel ID
	if msgs[0].ChannelID != "general" {
		t.Errorf("typing_start channelId: want 'general', got %q", msgs[0].ChannelID)
	}
	// typing_stop should carry the channel ID
	if msgs[2].ChannelID != "general" {
		t.Errorf("typing_stop channelId: want 'general', got %q", msgs[2].ChannelID)
	}
}

func TestHandleWS_WhenBrainReturnsError_ShouldStillSendTypingStop(t *testing.T) {
	brain := &mockChatBrain{err: errors.New("provider failed")}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hi"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Read all three messages: typing_start, error response, typing_stop
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msgs []WSMessage
	for i := 0; i < 3; i++ {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ReadJSON[%d]: %v", i, err)
		}
		msgs = append(msgs, msg)
	}

	if msgs[0].Type != "typing_start" {
		t.Errorf("message[0] type: want 'typing_start', got %q", msgs[0].Type)
	}
	if msgs[1].Type != "chat" || !strings.HasPrefix(msgs[1].Content, "error: ") {
		t.Errorf("message[1]: want type=chat with error prefix, got type=%q content=%q", msgs[1].Type, msgs[1].Content)
	}
	if msgs[2].Type != "typing_stop" {
		t.Errorf("message[2] type: want 'typing_stop', got %q", msgs[2].Type)
	}
}

func TestHandleWS_WhenNoBrain_ShouldNotSendTypingIndicators(t *testing.T) {
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hello"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Should only get the echo response, no typing indicators
	var out WSMessage
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.Type != "chat" || out.Content != "echo: hello" {
		t.Errorf("want type=chat content='echo: hello', got type=%q content=%q", out.Type, out.Content)
	}

	// Verify no additional messages (typing indicators) are sent
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var extra WSMessage
	err = conn.ReadJSON(&extra)
	if err == nil {
		t.Errorf("expected no extra messages, got type=%q", extra.Type)
	}
}

func TestHandleWS_WhenNonChatTypeWithBrain_ShouldNotSendTypingIndicators(t *testing.T) {
	brain := &mockChatBrain{response: "should not matter"}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "ping", Content: "test"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Should only get the echo response
	var out WSMessage
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.Type != "ping" || out.Content != "echo: test" {
		t.Errorf("want type=ping content='echo: test', got type=%q content=%q", out.Type, out.Content)
	}

	// Verify no additional messages
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var extra WSMessage
	err = conn.ReadJSON(&extra)
	if err == nil {
		t.Errorf("expected no extra messages, got type=%q", extra.Type)
	}
}

func TestHandleWS_WhenNoChannelIDWithBrain_ShouldSendTypingWithDefaultChannel(t *testing.T) {
	brain := &mockChatBrain{response: "reply"}
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, brain)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Send without channelId
	if err := conn.WriteJSON(WSMessage{Type: "chat", Content: "hi"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Read all three messages
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msgs []WSMessage
	for i := 0; i < 3; i++ {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ReadJSON[%d]: %v", i, err)
		}
		msgs = append(msgs, msg)
	}

	// typing_start should use default channel
	if msgs[0].ChannelID != DefaultChannelID {
		t.Errorf("typing_start channelId: want %q, got %q", DefaultChannelID, msgs[0].ChannelID)
	}
	// typing_stop should use default channel
	if msgs[2].ChannelID != DefaultChannelID {
		t.Errorf("typing_stop channelId: want %q, got %q", DefaultChannelID, msgs[2].ChannelID)
	}
}

func TestHandleWS_WhenNonChatTypeWithChannelID_ShouldEchoWithChannelID(t *testing.T) {
	srv, err := NewServer(&domain.GatewayConfig{Port: 0, Auth: domain.AuthConfig{}}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(WSMessage{Type: "ping", Content: "test", ChannelID: "general"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var out WSMessage
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.ChannelID != "general" {
		t.Errorf("echo ChannelID: want 'general', got %q", out.ChannelID)
	}
	if out.Content != "echo: test" {
		t.Errorf("echo Content: want 'echo: test', got %q", out.Content)
	}
}
