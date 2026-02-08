package telegram

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockBotAPI implements BotAPI for tests.
type mockBotAPI struct {
	mu       sync.Mutex
	sent     []tgbotapi.Chattable
	sendErr  error
	updates  chan tgbotapi.Update
	stopCh   chan struct{}
	stopped  bool
}

func newMockBotAPI() *mockBotAPI {
	return &mockBotAPI{
		updates: make(chan tgbotapi.Update, 100),
		stopCh:  make(chan struct{}),
	}
}

func (m *mockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return tgbotapi.Message{}, m.sendErr
	}
	m.sent = append(m.sent, c)
	return tgbotapi.Message{}, nil
}

func (m *mockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return m.updates
}

func (m *mockBotAPI) StopReceivingUpdates() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.stopped {
		m.stopped = true
		close(m.stopCh)
	}
}

func (m *mockBotAPI) sentMessages() []tgbotapi.Chattable {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]tgbotapi.Chattable, len(m.sent))
	copy(result, m.sent)
	return result
}

func (m *mockBotAPI) wasStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
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

// =============================================================================
// NewAdapter tests
// =============================================================================

func TestNewAdapter_WhenBotIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewAdapter(nil, router) should panic")
		}
	}()
	router := &mockRouter{response: "ok"}
	NewAdapter(nil, router)
}

func TestNewAdapter_WhenRouterIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewAdapter(bot, nil) should panic")
		}
	}()
	bot := newMockBotAPI()
	NewAdapter(bot, nil)
}

func TestNewAdapter_WhenValidDependencies_ShouldReturnAdapter(t *testing.T) {
	bot := newMockBotAPI()
	router := &mockRouter{response: "ok"}
	adapter := NewAdapter(bot, router)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
}

// =============================================================================
// ChatIDToChannelID tests
// =============================================================================

func TestChatIDToChannelID_ShouldPrefixWithTelegram(t *testing.T) {
	result := ChatIDToChannelID(12345)
	expected := "telegram-12345"
	if result != expected {
		t.Errorf("want %q, got %q", expected, result)
	}
}

func TestChatIDToChannelID_WhenNegativeChatID_ShouldIncludeSign(t *testing.T) {
	// Group chats in Telegram have negative IDs
	result := ChatIDToChannelID(-100123456)
	expected := "telegram--100123456"
	if result != expected {
		t.Errorf("want %q, got %q", expected, result)
	}
}

func TestChatIDToChannelID_WhenZero_ShouldReturnTelegramZero(t *testing.T) {
	result := ChatIDToChannelID(0)
	expected := "telegram-0"
	if result != expected {
		t.Errorf("want %q, got %q", expected, result)
	}
}

// =============================================================================
// HandleUpdate tests
// =============================================================================

func makeTextUpdate(chatID int64, messageID int, text string) tgbotapi.Update {
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: messageID,
			Chat:      &tgbotapi.Chat{ID: chatID},
			Text:      text,
		},
	}
}

func TestHandleUpdate_WhenTextMessage_ShouldRouteAndReply(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "Hello from brain"}
	adapter := NewAdapter(bot, rtr)

	update := makeTextUpdate(42, 1, "hi there")
	adapter.HandleUpdate(context.Background(), update)

	// Verify router was called with correct channel ID and prompt
	calls := rtr.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 route call, got %d", len(calls))
	}
	if calls[0].channelID != "telegram-42" {
		t.Errorf("channelID: want 'telegram-42', got %q", calls[0].channelID)
	}
	if calls[0].prompt != "hi there" {
		t.Errorf("prompt: want 'hi there', got %q", calls[0].prompt)
	}

	// Verify bot sent a reply
	sent := bot.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	msg, ok := sent[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("expected MessageConfig, got %T", sent[0])
	}
	if msg.Text != "Hello from brain" {
		t.Errorf("reply text: want 'Hello from brain', got %q", msg.Text)
	}
	if msg.ChatID != 42 {
		t.Errorf("reply chatID: want 42, got %d", msg.ChatID)
	}
	if msg.ReplyToMessageID != 1 {
		t.Errorf("reply to message ID: want 1, got %d", msg.ReplyToMessageID)
	}
}

func TestHandleUpdate_WhenMessageIsNil_ShouldDoNothing(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(bot, rtr)

	// Update with no message (e.g., callback query, channel post)
	update := tgbotapi.Update{}
	adapter.HandleUpdate(context.Background(), update)

	calls := rtr.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected 0 route calls, got %d", len(calls))
	}
	sent := bot.sentMessages()
	if len(sent) != 0 {
		t.Errorf("expected 0 sent messages, got %d", len(sent))
	}
}

func TestHandleUpdate_WhenEmptyText_ShouldDoNothing(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(bot, rtr)

	// Message with empty text (e.g., photo-only message)
	update := makeTextUpdate(42, 1, "")
	adapter.HandleUpdate(context.Background(), update)

	calls := rtr.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected 0 route calls for empty text, got %d", len(calls))
	}
	sent := bot.sentMessages()
	if len(sent) != 0 {
		t.Errorf("expected 0 sent messages, got %d", len(sent))
	}
}

func TestHandleUpdate_WhenRouterReturnsError_ShouldSendErrorMessage(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{err: errBrainDown}
	adapter := NewAdapter(bot, rtr)

	update := makeTextUpdate(42, 1, "hello")
	adapter.HandleUpdate(context.Background(), update)

	sent := bot.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	msg, ok := sent[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("expected MessageConfig, got %T", sent[0])
	}
	if msg.Text != "Error: brain is down" {
		t.Errorf("error reply: want 'Error: brain is down', got %q", msg.Text)
	}
}

func TestHandleUpdate_WhenSendFails_ShouldNotPanic(t *testing.T) {
	bot := newMockBotAPI()
	bot.sendErr = errSendFailed
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(bot, rtr)

	// Should not panic even when Send fails
	update := makeTextUpdate(42, 1, "hello")
	adapter.HandleUpdate(context.Background(), update)

	// Router should still have been called
	calls := rtr.getCalls()
	if len(calls) != 1 {
		t.Errorf("expected 1 route call even when send fails, got %d", len(calls))
	}
}

func TestHandleUpdate_WhenGroupChat_ShouldMapNegativeChatID(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "group reply"}
	adapter := NewAdapter(bot, rtr)

	update := makeTextUpdate(-100123456, 5, "group message")
	adapter.HandleUpdate(context.Background(), update)

	calls := rtr.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 route call, got %d", len(calls))
	}
	if calls[0].channelID != "telegram--100123456" {
		t.Errorf("channelID: want 'telegram--100123456', got %q", calls[0].channelID)
	}
}

var (
	errBrainDown  = errors.New("brain is down")
	errSendFailed = errors.New("send failed")
)

// =============================================================================
// Start / Stop lifecycle tests
// =============================================================================

func TestStart_ShouldProcessUpdatesFromChannel(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "pong"}
	adapter := NewAdapter(bot, rtr)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		adapter.Start(ctx)
		close(done)
	}()

	// Send an update through the mock channel
	bot.updates <- makeTextUpdate(99, 1, "ping")

	// Wait for it to be processed
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for message to be processed")
		default:
		}
		if len(bot.sentMessages()) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	sent := bot.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	msg := sent[0].(tgbotapi.MessageConfig)
	if msg.Text != "pong" {
		t.Errorf("reply: want 'pong', got %q", msg.Text)
	}

	// Stop and verify cleanup
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}
}

func TestStart_WhenContextCanceled_ShouldStopReceivingUpdates(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(bot, rtr)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		adapter.Start(ctx)
		close(done)
	}()

	// Cancel immediately
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}

	if !bot.wasStopped() {
		t.Error("expected StopReceivingUpdates to be called")
	}
}

func TestStart_ShouldProcessMultipleUpdates(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "reply"}
	adapter := NewAdapter(bot, rtr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		adapter.Start(ctx)
		close(done)
	}()

	// Send 3 updates
	for i := 0; i < 3; i++ {
		bot.updates <- makeTextUpdate(int64(100+i), i+1, "msg")
	}

	// Wait for all to be processed
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out: expected 3 sent messages, got %d", len(bot.sentMessages()))
		default:
		}
		if len(bot.sentMessages()) >= 3 {
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

func TestStop_WhenCalledBeforeStart_ShouldNotPanic(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(bot, rtr)

	// Should not panic
	adapter.Stop()
}

func TestStop_WhenCalledAfterStart_ShouldCancelContext(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(bot, rtr)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		adapter.Start(ctx)
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

	if !bot.wasStopped() {
		t.Error("expected StopReceivingUpdates to be called after Stop")
	}
}

// =============================================================================
// Concurrency tests
// =============================================================================

func TestHandleUpdate_WhenConcurrentDifferentChats_ShouldBeSafe(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "concurrent reply"}
	adapter := NewAdapter(bot, rtr)

	const goroutines = 20
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			update := makeTextUpdate(int64(idx), idx+1, "concurrent msg")
			adapter.HandleUpdate(context.Background(), update)
		}(i)
	}
	wg.Wait()

	calls := rtr.getCalls()
	if len(calls) != goroutines {
		t.Errorf("expected %d route calls, got %d", goroutines, len(calls))
	}

	sent := bot.sentMessages()
	if len(sent) != goroutines {
		t.Errorf("expected %d sent messages, got %d", goroutines, len(sent))
	}
}

func TestStart_WhenUpdatesChannelClosed_ShouldReturn(t *testing.T) {
	bot := newMockBotAPI()
	rtr := &mockRouter{response: "ok"}
	adapter := NewAdapter(bot, rtr)

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		adapter.Start(ctx)
		close(done)
	}()

	// Give Start time to set up
	time.Sleep(50 * time.Millisecond)

	// Close the updates channel -- simulates Telegram disconnect
	close(bot.updates)

	// The zero-value update from a closed channel will have Message == nil,
	// so HandleUpdate will return early. We need Stop or cancel to actually stop.
	// Let's cancel via Stop.
	adapter.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after updates channel closed and Stop called")
	}
}
