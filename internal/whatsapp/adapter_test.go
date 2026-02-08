package whatsapp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockWAClient implements WAClient for tests.
type mockWAClient struct {
	mu           sync.Mutex
	connected    bool
	loggedIn     bool
	sentMessages []sentMsg
	sendErr      error
	connectErr   error
	msgCh        chan IncomingMessage
	qrCh         chan QREvent
	qrErr        error
	disconnected bool
}

type sentMsg struct {
	chatJID string
	text    string
}

func newMockWAClient(loggedIn bool) *mockWAClient {
	return &mockWAClient{
		loggedIn: loggedIn,
		msgCh:    make(chan IncomingMessage, 100),
		qrCh:     make(chan QREvent, 100),
	}
}

func (m *mockWAClient) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}

func (m *mockWAClient) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	m.disconnected = true
}

func (m *mockWAClient) IsLoggedIn() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loggedIn
}

func (m *mockWAClient) SendText(ctx context.Context, chatJID string, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentMessages = append(m.sentMessages, sentMsg{chatJID: chatJID, text: text})
	return nil
}

func (m *mockWAClient) MessageChannel() <-chan IncomingMessage {
	return m.msgCh
}

func (m *mockWAClient) GetQRChannel(_ context.Context) (<-chan QREvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.qrErr != nil {
		return nil, m.qrErr
	}
	return m.qrCh, nil
}

func (m *mockWAClient) getSentMessages() []sentMsg {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]sentMsg, len(m.sentMessages))
	copy(result, m.sentMessages)
	return result
}

func (m *mockWAClient) wasDisconnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.disconnected
}

func (m *mockWAClient) isConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

// mockRouter implements MessageRouter for tests.
type mockRouter struct {
	mu       sync.Mutex
	calls    []routeCall
	response string
	err      error
}

type routeCall struct {
	channelID string
	prompt    string
}

func (m *mockRouter) Route(ctx context.Context, channelID, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, routeCall{channelID: channelID, prompt: prompt})
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockRouter) getCalls() []routeCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]routeCall, len(m.calls))
	copy(result, m.calls)
	return result
}

var (
	errBrainDown   = errors.New("brain is down")
	errSendFailed  = errors.New("send failed")
	errConnFailed  = errors.New("connection failed")
	errQRFailed    = errors.New("qr channel failed")
)

// =============================================================================
// JIDToChannelID tests
// =============================================================================

func TestJIDToChannelID_WhenPersonalJID_ShouldPrefixWithWhatsApp(t *testing.T) {
	result := JIDToChannelID("1234567890@s.whatsapp.net")
	expected := "whatsapp-1234567890@s.whatsapp.net"
	if result != expected {
		t.Errorf("want %q, got %q", expected, result)
	}
}

func TestJIDToChannelID_WhenGroupJID_ShouldPreserveFullJID(t *testing.T) {
	result := JIDToChannelID("120363012345@g.us")
	expected := "whatsapp-120363012345@g.us"
	if result != expected {
		t.Errorf("want %q, got %q", expected, result)
	}
}

func TestJIDToChannelID_WhenEmptyJID_ShouldReturnJustPrefix(t *testing.T) {
	result := JIDToChannelID("")
	expected := "whatsapp-"
	if result != expected {
		t.Errorf("want %q, got %q", expected, result)
	}
}

// =============================================================================
// NewAdapter tests
// =============================================================================

func TestNewAdapter_WhenClientIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewAdapter(nil, router, nil) should panic")
		}
	}()
	rtr := &mockRouter{response: "ok"}
	NewAdapter(nil, rtr, nil)
}

func TestNewAdapter_WhenRouterIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewAdapter(client, nil, nil) should panic")
		}
	}()
	client := newMockWAClient(true)
	NewAdapter(client, nil, nil)
}

func TestNewAdapter_WhenValidDependencies_ShouldReturnAdapter(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
}

func TestNewAdapter_WhenQRHandlerIsNil_ShouldUseNoOpDefault(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)
	// Should not panic when QR handler is called internally
	adapter.qrHandler("test-code")
}

// =============================================================================
// HandleMessage tests
// =============================================================================

func TestHandleMessage_WhenTextMessage_ShouldRouteAndReply(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "Hello from brain"}
	adapter := NewAdapter(client, rtr, nil)

	msg := IncomingMessage{
		SenderJID: "1234567890@s.whatsapp.net",
		ChatJID:   "1234567890@s.whatsapp.net",
		Text:      "hi there",
		MessageID: "msg-001",
	}
	adapter.HandleMessage(context.Background(), msg)

	// Verify router was called with correct channel ID and prompt
	calls := rtr.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 route call, got %d", len(calls))
	}
	if calls[0].channelID != "whatsapp-1234567890@s.whatsapp.net" {
		t.Errorf("channelID: want 'whatsapp-1234567890@s.whatsapp.net', got %q", calls[0].channelID)
	}
	if calls[0].prompt != "hi there" {
		t.Errorf("prompt: want 'hi there', got %q", calls[0].prompt)
	}

	// Verify client sent a reply
	sent := client.getSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].chatJID != "1234567890@s.whatsapp.net" {
		t.Errorf("reply chatJID: want '1234567890@s.whatsapp.net', got %q", sent[0].chatJID)
	}
	if sent[0].text != "Hello from brain" {
		t.Errorf("reply text: want 'Hello from brain', got %q", sent[0].text)
	}
}

func TestHandleMessage_WhenGroupMessage_ShouldUseChatJIDForChannel(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "group reply"}
	adapter := NewAdapter(client, rtr, nil)

	msg := IncomingMessage{
		SenderJID: "1234567890@s.whatsapp.net",
		ChatJID:   "120363012345@g.us",
		Text:      "group message",
		MessageID: "msg-002",
	}
	adapter.HandleMessage(context.Background(), msg)

	calls := rtr.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 route call, got %d", len(calls))
	}
	// Channel ID should be based on ChatJID (the group), not SenderJID
	if calls[0].channelID != "whatsapp-120363012345@g.us" {
		t.Errorf("channelID: want 'whatsapp-120363012345@g.us', got %q", calls[0].channelID)
	}

	// Reply should go to the group chat
	sent := client.getSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].chatJID != "120363012345@g.us" {
		t.Errorf("reply chatJID: want '120363012345@g.us', got %q", sent[0].chatJID)
	}
}

func TestHandleMessage_WhenEmptyText_ShouldDoNothing(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)

	msg := IncomingMessage{
		SenderJID: "1234567890@s.whatsapp.net",
		ChatJID:   "1234567890@s.whatsapp.net",
		Text:      "",
		MessageID: "msg-003",
	}
	adapter.HandleMessage(context.Background(), msg)

	calls := rtr.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected 0 route calls for empty text, got %d", len(calls))
	}
	sent := client.getSentMessages()
	if len(sent) != 0 {
		t.Errorf("expected 0 sent messages for empty text, got %d", len(sent))
	}
}

func TestHandleMessage_WhenRouterReturnsError_ShouldSendErrorMessage(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{err: errBrainDown}
	adapter := NewAdapter(client, rtr, nil)

	msg := IncomingMessage{
		SenderJID: "1234567890@s.whatsapp.net",
		ChatJID:   "1234567890@s.whatsapp.net",
		Text:      "hello",
		MessageID: "msg-004",
	}
	adapter.HandleMessage(context.Background(), msg)

	sent := client.getSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].text != "Error: brain is down" {
		t.Errorf("error reply: want 'Error: brain is down', got %q", sent[0].text)
	}
}

func TestHandleMessage_WhenSendFails_ShouldNotPanic(t *testing.T) {
	client := newMockWAClient(true)
	client.sendErr = errSendFailed
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)

	msg := IncomingMessage{
		SenderJID: "1234567890@s.whatsapp.net",
		ChatJID:   "1234567890@s.whatsapp.net",
		Text:      "hello",
		MessageID: "msg-005",
	}
	// Should not panic even when SendText fails
	adapter.HandleMessage(context.Background(), msg)

	// Router should still have been called
	calls := rtr.getCalls()
	if len(calls) != 1 {
		t.Errorf("expected 1 route call even when send fails, got %d", len(calls))
	}
}

func TestHandleMessage_WhenConcurrentMessages_ShouldBeSafe(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "concurrent reply"}
	adapter := NewAdapter(client, rtr, nil)

	const goroutines = 20
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := IncomingMessage{
				SenderJID: "user@s.whatsapp.net",
				ChatJID:   "user@s.whatsapp.net",
				Text:      "concurrent msg",
				MessageID: "msg-concurrent",
			}
			adapter.HandleMessage(context.Background(), msg)
		}(i)
	}
	wg.Wait()

	calls := rtr.getCalls()
	if len(calls) != goroutines {
		t.Errorf("expected %d route calls, got %d", goroutines, len(calls))
	}

	sent := client.getSentMessages()
	if len(sent) != goroutines {
		t.Errorf("expected %d sent messages, got %d", goroutines, len(sent))
	}
}

// =============================================================================
// Start / Stop lifecycle tests
// =============================================================================

func TestStart_WhenLoggedIn_ShouldConnectAndProcessMessages(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "pong"}
	adapter := NewAdapter(client, rtr, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		_ = adapter.Start(ctx)
		close(done)
	}()

	// Send a message through the mock channel
	client.msgCh <- IncomingMessage{
		SenderJID: "user@s.whatsapp.net",
		ChatJID:   "user@s.whatsapp.net",
		Text:      "ping",
		MessageID: "msg-start-001",
	}

	// Wait for it to be processed
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for message to be processed")
		default:
		}
		if len(client.getSentMessages()) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	sent := client.getSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].text != "pong" {
		t.Errorf("reply: want 'pong', got %q", sent[0].text)
	}

	// Verify client was connected
	if !client.isConnected() {
		t.Error("expected client to be connected")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}
}

func TestStart_WhenContextCanceled_ShouldDisconnect(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		_ = adapter.Start(ctx)
		close(done)
	}()

	// Give Start time to set up
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}

	if !client.wasDisconnected() {
		t.Error("expected Disconnect to be called")
	}
}

func TestStart_ShouldProcessMultipleMessages(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "reply"}
	adapter := NewAdapter(client, rtr, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = adapter.Start(ctx)
		close(done)
	}()

	// Send 3 messages
	for i := 0; i < 3; i++ {
		client.msgCh <- IncomingMessage{
			SenderJID: "user@s.whatsapp.net",
			ChatJID:   "user@s.whatsapp.net",
			Text:      "msg",
			MessageID: "msg-multi",
		}
	}

	// Wait for all to be processed
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out: expected 3 sent messages, got %d", len(client.getSentMessages()))
		default:
		}
		if len(client.getSentMessages()) >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	calls := rtr.getCalls()
	if len(calls) != 3 {
		t.Errorf("expected 3 route calls, got %d", len(calls))
	}

	cancel()
	<-done
}

func TestStart_WhenConnectFails_ShouldReturnError(t *testing.T) {
	client := newMockWAClient(true)
	client.connectErr = errConnFailed
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)

	err := adapter.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when Connect fails")
	}
	if !errors.Is(err, errConnFailed) {
		t.Errorf("expected errConnFailed, got %v", err)
	}
}

func TestStop_WhenCalledBeforeStart_ShouldNotPanic(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)

	// Should not panic
	adapter.Stop()
}

func TestStop_WhenCalledAfterStart_ShouldCancelContext(t *testing.T) {
	client := newMockWAClient(true)
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		_ = adapter.Start(ctx)
		close(done)
	}()

	// Give Start time to set up
	time.Sleep(50 * time.Millisecond)

	// Stop should trigger cancel which causes Start to return
	adapter.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop was called")
	}

	if !client.wasDisconnected() {
		t.Error("expected Disconnect to be called after Stop")
	}
}

// =============================================================================
// QR code flow tests
// =============================================================================

func TestStart_WhenNotLoggedIn_ShouldCallQRHandler(t *testing.T) {
	client := newMockWAClient(false) // not logged in
	rtr := &mockRouter{response: "ok"}

	var qrCodes []string
	var qrMu sync.Mutex
	qrHandler := func(code string) {
		qrMu.Lock()
		qrCodes = append(qrCodes, code)
		qrMu.Unlock()
	}
	adapter := NewAdapter(client, rtr, qrHandler)

	ctx, cancel := context.WithCancel(context.Background())

	// Pre-load QR events before start
	client.qrCh <- QREvent{Event: "code", Code: "qr-data-1"}
	client.qrCh <- QREvent{Event: "code", Code: "qr-data-2"}
	client.qrCh <- QREvent{Event: "success"}
	close(client.qrCh)

	done := make(chan struct{})
	go func() {
		_ = adapter.Start(ctx)
		close(done)
	}()

	// Give Start time to process QR events
	time.Sleep(200 * time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}

	qrMu.Lock()
	defer qrMu.Unlock()
	if len(qrCodes) != 2 {
		t.Fatalf("expected 2 QR codes, got %d: %v", len(qrCodes), qrCodes)
	}
	if qrCodes[0] != "qr-data-1" {
		t.Errorf("qrCodes[0]: want 'qr-data-1', got %q", qrCodes[0])
	}
	if qrCodes[1] != "qr-data-2" {
		t.Errorf("qrCodes[1]: want 'qr-data-2', got %q", qrCodes[1])
	}
}

func TestStart_WhenLoggedIn_ShouldSkipQRFlow(t *testing.T) {
	client := newMockWAClient(true) // already logged in
	rtr := &mockRouter{response: "ok"}

	qrCalled := false
	qrHandler := func(code string) {
		qrCalled = true
	}
	adapter := NewAdapter(client, rtr, qrHandler)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		_ = adapter.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}

	if qrCalled {
		t.Error("QR handler should not be called when already logged in")
	}
}

func TestStart_WhenNotLoggedInAndConnectFails_ShouldReturnError(t *testing.T) {
	client := newMockWAClient(false) // not logged in
	client.connectErr = errConnFailed
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)

	err := adapter.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when Connect fails during QR flow")
	}
	if !errors.Is(err, errConnFailed) {
		t.Errorf("expected errConnFailed, got %v", err)
	}
}

func TestStart_WhenQRChannelFails_ShouldReturnError(t *testing.T) {
	client := newMockWAClient(false) // not logged in
	client.qrErr = errQRFailed
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(client, rtr, nil)

	err := adapter.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when GetQRChannel fails")
	}
	if !errors.Is(err, errQRFailed) {
		t.Errorf("expected errQRFailed, got %v", err)
	}
}
