package whatsapp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// newWhatsmeowClientRaw creates a minimal whatsmeow.Client for testing.
// Uses a noop device (no database required).
func newWhatsmeowClientRaw() *whatsmeow.Client {
	device := &store.Device{}
	return whatsmeow.NewClient(device, nil)
}

// =============================================================================
// mockRawClient implements rawClient for testing WhatsmeowClient internals
// =============================================================================

type mockRawClient struct {
	mu         sync.Mutex
	connectErr error
	sendErr    error
	qrErr      error
	qrCh       chan whatsmeow.QRChannelItem

	connected    bool
	disconnected bool
	sentMsgs     []rawSentMsg
	handlers     []whatsmeow.EventHandler
}

type rawSentMsg struct {
	to   types.JID
	text string
}

func newMockRawClient() *mockRawClient {
	return &mockRawClient{
		qrCh: make(chan whatsmeow.QRChannelItem, 10),
	}
}

func (m *mockRawClient) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}

func (m *mockRawClient) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnected = true
	m.connected = false
}

func (m *mockRawClient) SendMessage(_ context.Context, to types.JID, message *waE2E.Message, _ ...whatsmeow.SendRequestExtra) (whatsmeow.SendResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return whatsmeow.SendResponse{}, m.sendErr
	}
	text := ""
	if message.Conversation != nil {
		text = *message.Conversation
	}
	m.sentMsgs = append(m.sentMsgs, rawSentMsg{to: to, text: text})
	return whatsmeow.SendResponse{}, nil
}

func (m *mockRawClient) GetQRChannel(_ context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.qrErr != nil {
		return nil, m.qrErr
	}
	return m.qrCh, nil
}

func (m *mockRawClient) AddEventHandler(handler whatsmeow.EventHandler) uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
	return uint32(len(m.handlers))
}

func (m *mockRawClient) getSentMsgs() []rawSentMsg {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]rawSentMsg, len(m.sentMsgs))
	copy(result, m.sentMsgs)
	return result
}

var errMockConnect = errors.New("mock connect failed")
var errMockSend = errors.New("mock send failed")
var errMockQR = errors.New("mock qr failed")

// =============================================================================
// extractText tests
// =============================================================================

func TestExtractText_WhenConversationSet_ShouldReturnText(t *testing.T) {
	msg := &waE2E.Message{
		Conversation: proto.String("hello world"),
	}
	result := extractText(msg)
	if result != "hello world" {
		t.Errorf("want 'hello world', got %q", result)
	}
}

func TestExtractText_WhenExtendedTextMessage_ShouldReturnText(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("extended text"),
		},
	}
	result := extractText(msg)
	if result != "extended text" {
		t.Errorf("want 'extended text', got %q", result)
	}
}

func TestExtractText_WhenNilMessage_ShouldReturnEmpty(t *testing.T) {
	result := extractText(nil)
	if result != "" {
		t.Errorf("want empty string, got %q", result)
	}
}

func TestExtractText_WhenNoTextContent_ShouldReturnEmpty(t *testing.T) {
	msg := &waE2E.Message{
		// No conversation or extended text (e.g., image-only message)
	}
	result := extractText(msg)
	if result != "" {
		t.Errorf("want empty string, got %q", result)
	}
}

func TestExtractText_WhenConversationPreferredOverExtended_ShouldReturnConversation(t *testing.T) {
	msg := &waE2E.Message{
		Conversation: proto.String("simple"),
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("extended"),
		},
	}
	// Conversation is checked first
	result := extractText(msg)
	if result != "simple" {
		t.Errorf("want 'simple', got %q", result)
	}
}

// =============================================================================
// eventHandler tests
// =============================================================================

func TestEventHandler_WhenTextMessage_ShouldSendToChannel(t *testing.T) {
	wc := &WhatsmeowClient{
		msgCh: make(chan IncomingMessage, 10),
	}

	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   types.NewJID("1234567890", types.DefaultUserServer),
				Sender: types.NewJID("1234567890", types.DefaultUserServer),
			},
			ID: "msg-evt-001",
		},
		Message: &waE2E.Message{
			Conversation: proto.String("hello from handler"),
		},
	}

	wc.eventHandler(evt)

	select {
	case msg := <-wc.msgCh:
		if msg.Text != "hello from handler" {
			t.Errorf("text: want 'hello from handler', got %q", msg.Text)
		}
		if msg.MessageID != "msg-evt-001" {
			t.Errorf("messageID: want 'msg-evt-001', got %q", msg.MessageID)
		}
		expectedChat := types.NewJID("1234567890", types.DefaultUserServer).String()
		if msg.ChatJID != expectedChat {
			t.Errorf("chatJID: want %q, got %q", expectedChat, msg.ChatJID)
		}
	default:
		t.Fatal("expected message on channel")
	}
}

func TestEventHandler_WhenEmptyText_ShouldNotSendToChannel(t *testing.T) {
	wc := &WhatsmeowClient{
		msgCh: make(chan IncomingMessage, 10),
	}

	// Image message with no text
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   types.NewJID("1234567890", types.DefaultUserServer),
				Sender: types.NewJID("1234567890", types.DefaultUserServer),
			},
		},
		Message: &waE2E.Message{},
	}

	wc.eventHandler(evt)

	select {
	case msg := <-wc.msgCh:
		t.Fatalf("expected no message, got: %+v", msg)
	default:
		// Expected: no message
	}
}

func TestEventHandler_WhenNonMessageEvent_ShouldIgnore(t *testing.T) {
	wc := &WhatsmeowClient{
		msgCh: make(chan IncomingMessage, 10),
	}

	// Pass a string event (not *events.Message)
	wc.eventHandler("some other event")

	select {
	case msg := <-wc.msgCh:
		t.Fatalf("expected no message for non-Message event, got: %+v", msg)
	default:
		// Expected: no message
	}
}

// =============================================================================
// WhatsmeowClient wrapper tests (using real whatsmeow.Client with NoopDevice)
// =============================================================================

func TestNewWhatsmeowClient_ShouldReturnNonNil(t *testing.T) {
	client := newTestWhatsmeowClient(t)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestWhatsmeowClient_MessageChannel_ShouldReturnChannel(t *testing.T) {
	client := newTestWhatsmeowClient(t)
	ch := client.MessageChannel()
	if ch == nil {
		t.Fatal("expected non-nil message channel")
	}
}

func TestWhatsmeowClient_IsLoggedIn_WhenNoStoredSession_ShouldReturnFalse(t *testing.T) {
	client := newTestWhatsmeowClient(t)
	if client.IsLoggedIn() {
		t.Error("expected IsLoggedIn to return false for new device")
	}
}

func TestWhatsmeowClient_Disconnect_WhenNotConnected_ShouldNotPanic(t *testing.T) {
	client := newTestWhatsmeowClient(t)
	// Should not panic even when not connected
	client.Disconnect()
}

func TestWhatsmeowClient_SendText_WhenInvalidJID_ShouldReturnError(t *testing.T) {
	mock := newMockRawClient()
	wc := newMockWhatsmeowClient(mock)
	// AD-JID with too many dots triggers ParseJID error
	err := wc.SendText(context.Background(), "a.b.c@s.whatsapp.net", "test")
	if err == nil {
		t.Fatal("expected error for malformed AD-JID")
	}
}

func newTestWhatsmeowClient(t *testing.T) *WhatsmeowClient {
	t.Helper()
	wmClient := newWhatsmeowClientRaw()
	return NewWhatsmeowClient(wmClient)
}

// =============================================================================
// WhatsmeowClient wrapper tests (using mockRawClient for full coverage)
// =============================================================================

func newMockWhatsmeowClient(mock *mockRawClient) *WhatsmeowClient {
	return &WhatsmeowClient{
		client: mock,
		store:  &store.Device{}, // no stored session (ID == nil)
		msgCh:  make(chan IncomingMessage, 100),
	}
}

func TestConnect_WhenSucceeds_ShouldReturnNil(t *testing.T) {
	mock := newMockRawClient()
	wc := newMockWhatsmeowClient(mock)

	err := wc.Connect()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestConnect_WhenFails_ShouldReturnError(t *testing.T) {
	mock := newMockRawClient()
	mock.connectErr = errMockConnect
	wc := newMockWhatsmeowClient(mock)

	err := wc.Connect()
	if !errors.Is(err, errMockConnect) {
		t.Fatalf("expected errMockConnect, got %v", err)
	}
}

func TestSendText_WhenValidJID_ShouldCallSendMessage(t *testing.T) {
	mock := newMockRawClient()
	wc := newMockWhatsmeowClient(mock)

	err := wc.SendText(context.Background(), "1234567890@s.whatsapp.net", "hello world")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	sent := mock.getSentMsgs()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].text != "hello world" {
		t.Errorf("text: want 'hello world', got %q", sent[0].text)
	}
	expectedJID := types.NewJID("1234567890", types.DefaultUserServer)
	if sent[0].to != expectedJID {
		t.Errorf("to JID: want %v, got %v", expectedJID, sent[0].to)
	}
}

func TestSendText_WhenSendMessageFails_ShouldReturnError(t *testing.T) {
	mock := newMockRawClient()
	mock.sendErr = errMockSend
	wc := newMockWhatsmeowClient(mock)

	err := wc.SendText(context.Background(), "1234567890@s.whatsapp.net", "hello")
	if !errors.Is(err, errMockSend) {
		t.Fatalf("expected errMockSend, got %v", err)
	}
}

func TestGetQRChannel_WhenSucceeds_ShouldForwardEvents(t *testing.T) {
	mock := newMockRawClient()
	wc := newMockWhatsmeowClient(mock)

	// Pre-load QR events
	mock.qrCh <- whatsmeow.QRChannelItem{Event: "code", Code: "qr-123"}
	mock.qrCh <- whatsmeow.QRChannelItem{Event: "success"}
	close(mock.qrCh)

	qrCh, err := wc.GetQRChannel(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var evts []QREvent
	for evt := range qrCh {
		evts = append(evts, evt)
	}

	if len(evts) != 2 {
		t.Fatalf("expected 2 QR events, got %d", len(evts))
	}
	if evts[0].Event != "code" || evts[0].Code != "qr-123" {
		t.Errorf("event[0]: want code/qr-123, got %s/%s", evts[0].Event, evts[0].Code)
	}
	if evts[1].Event != "success" {
		t.Errorf("event[1]: want success, got %s", evts[1].Event)
	}
}

func TestGetQRChannel_WhenFails_ShouldReturnError(t *testing.T) {
	mock := newMockRawClient()
	mock.qrErr = errMockQR
	wc := newMockWhatsmeowClient(mock)

	_, err := wc.GetQRChannel(context.Background())
	if !errors.Is(err, errMockQR) {
		t.Fatalf("expected errMockQR, got %v", err)
	}
}

func TestGetQRChannel_WhenChannelCloses_ShouldCloseOutputChannel(t *testing.T) {
	mock := newMockRawClient()
	wc := newMockWhatsmeowClient(mock)

	close(mock.qrCh) // close immediately

	qrCh, err := wc.GetQRChannel(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// Channel should close eventually
	select {
	case _, ok := <-qrCh:
		if ok {
			t.Error("expected channel to be closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for QR channel to close")
	}
}

func TestEventHandler_WhenGroupMessage_ShouldUseChatJID(t *testing.T) {
	wc := &WhatsmeowClient{
		msgCh: make(chan IncomingMessage, 10),
	}

	groupJID := types.NewJID("120363012345", types.GroupServer)
	senderJID := types.NewJID("1234567890", types.DefaultUserServer)

	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:    groupJID,
				Sender:  senderJID,
				IsGroup: true,
			},
			ID: "msg-group-001",
		},
		Message: &waE2E.Message{
			Conversation: proto.String("group message"),
		},
	}

	wc.eventHandler(evt)

	select {
	case msg := <-wc.msgCh:
		if msg.ChatJID != groupJID.String() {
			t.Errorf("chatJID: want %q, got %q", groupJID.String(), msg.ChatJID)
		}
		if msg.SenderJID != senderJID.String() {
			t.Errorf("senderJID: want %q, got %q", senderJID.String(), msg.SenderJID)
		}
	default:
		t.Fatal("expected message on channel")
	}
}
