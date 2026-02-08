package router

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"ironclaw/internal/domain"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockGenerator implements Generator for tests.
type mockGenerator struct {
	mu        sync.Mutex
	response  string
	err       error
	calls     []generateCall // records every call
	perPrompt map[string]string // per-prompt overrides: prompt -> response
}

type generateCall struct {
	prompt string
}

func (m *mockGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, generateCall{prompt: prompt})
	if m.err != nil {
		return "", m.err
	}
	if resp, ok := m.perPrompt[prompt]; ok {
		return resp, nil
	}
	return m.response, nil
}

// mockHistoryStore implements domain.SessionHistoryStore for tests.
type mockHistoryStore struct {
	mu       sync.Mutex
	messages []domain.Message
	appendErr error
}

func (m *mockHistoryStore) Append(msg domain.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.appendErr != nil {
		return m.appendErr
	}
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockHistoryStore) LoadHistory(n int) ([]domain.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n <= 0 || len(m.messages) == 0 {
		return nil, nil
	}
	start := len(m.messages) - n
	if start < 0 {
		start = 0
	}
	result := make([]domain.Message, len(m.messages[start:]))
	copy(result, m.messages[start:])
	return result, nil
}

// trackingHistoryFactory creates mockHistoryStores and tracks them per channel.
type trackingHistoryFactory struct {
	mu     sync.Mutex
	stores map[string]*mockHistoryStore
}

func newTrackingHistoryFactory() *trackingHistoryFactory {
	return &trackingHistoryFactory{stores: make(map[string]*mockHistoryStore)}
}

func (f *trackingHistoryFactory) Create(channelID string) domain.SessionHistoryStore {
	f.mu.Lock()
	defer f.mu.Unlock()
	store := &mockHistoryStore{}
	f.stores[channelID] = store
	return store
}

func (f *trackingHistoryFactory) Get(channelID string) *mockHistoryStore {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stores[channelID]
}

// =============================================================================
// NewRouter tests
// =============================================================================

func TestNewRouter_WhenBrainIsNil_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewRouter(nil, ...) should panic")
		}
	}()
	NewRouter(nil, nil)
}

func TestNewRouter_WhenBrainProvided_ShouldReturnRouter(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	r := NewRouter(brain, nil)
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestNewRouter_WhenHistoryFactoryNil_ShouldCreateRouterWithoutHistory(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	r := NewRouter(brain, nil)
	// Should be able to route without history
	resp, err := r.Route(context.Background(), "ch1", "hello")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if resp != "ok" {
		t.Errorf("want 'ok', got %q", resp)
	}
}

// =============================================================================
// Route tests
// =============================================================================

func TestRoute_WhenEmptyChannelID_ShouldReturnError(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	r := NewRouter(brain, nil)

	_, err := r.Route(context.Background(), "", "hello")
	if err == nil {
		t.Fatal("expected error for empty channel ID")
	}
	if !errors.Is(err, ErrEmptyChannelID) {
		t.Errorf("want ErrEmptyChannelID, got %v", err)
	}
}

func TestRoute_WhenValidChannelID_ShouldReturnBrainResponse(t *testing.T) {
	brain := &mockGenerator{response: "Hello from brain"}
	r := NewRouter(brain, nil)

	resp, err := r.Route(context.Background(), "general", "hi")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if resp != "Hello from brain" {
		t.Errorf("want 'Hello from brain', got %q", resp)
	}
}

func TestRoute_WhenBrainReturnsError_ShouldReturnError(t *testing.T) {
	brainErr := errors.New("LLM unavailable")
	brain := &mockGenerator{err: brainErr}
	r := NewRouter(brain, nil)

	_, err := r.Route(context.Background(), "general", "hi")
	if err == nil {
		t.Fatal("expected error when brain fails")
	}
	if !errors.Is(err, brainErr) {
		t.Errorf("want %v, got %v", brainErr, err)
	}
}

func TestRoute_WhenContextCanceled_ShouldPassContextToBrain(t *testing.T) {
	brain := &mockGenerator{err: context.Canceled}
	r := NewRouter(brain, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Route(ctx, "general", "hi")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestRoute_WhenCalledTwice_ShouldReuseChannel(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	_, _ = r.Route(context.Background(), "general", "msg1")
	_, _ = r.Route(context.Background(), "general", "msg2")

	channels := r.ActiveChannels()
	if len(channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(channels))
	}
	// History should have 4 entries (2 user + 2 assistant)
	store := factory.Get("general")
	if store == nil {
		t.Fatal("expected history store for 'general'")
	}
	store.mu.Lock()
	count := len(store.messages)
	store.mu.Unlock()
	if count != 4 {
		t.Errorf("expected 4 history entries (2 user + 2 assistant), got %d", count)
	}
}

func TestRoute_ShouldDelegatePromptToBrain(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	r := NewRouter(brain, nil)

	_, _ = r.Route(context.Background(), "general", "What is 2+2?")

	brain.mu.Lock()
	defer brain.mu.Unlock()
	if len(brain.calls) != 1 {
		t.Fatalf("expected 1 brain call, got %d", len(brain.calls))
	}
	if brain.calls[0].prompt != "What is 2+2?" {
		t.Errorf("want prompt 'What is 2+2?', got %q", brain.calls[0].prompt)
	}
}

// =============================================================================
// Channel isolation tests
// =============================================================================

func TestRoute_WhenDifferentChannels_ShouldIsolateHistory(t *testing.T) {
	brain := &mockGenerator{
		perPrompt: map[string]string{
			"general-msg": "general-reply",
			"support-msg": "support-reply",
		},
	}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	// Send to #general
	resp1, err := r.Route(context.Background(), "general", "general-msg")
	if err != nil {
		t.Fatalf("Route general: %v", err)
	}
	if resp1 != "general-reply" {
		t.Errorf("general: want 'general-reply', got %q", resp1)
	}

	// Send to #support
	resp2, err := r.Route(context.Background(), "support", "support-msg")
	if err != nil {
		t.Fatalf("Route support: %v", err)
	}
	if resp2 != "support-reply" {
		t.Errorf("support: want 'support-reply', got %q", resp2)
	}

	// Verify isolation: each channel should have exactly 2 messages (1 user + 1 assistant)
	generalStore := factory.Get("general")
	supportStore := factory.Get("support")

	if generalStore == nil || supportStore == nil {
		t.Fatal("expected stores for both channels")
	}

	generalStore.mu.Lock()
	generalCount := len(generalStore.messages)
	generalStore.mu.Unlock()

	supportStore.mu.Lock()
	supportCount := len(supportStore.messages)
	supportStore.mu.Unlock()

	if generalCount != 2 {
		t.Errorf("general: expected 2 messages, got %d", generalCount)
	}
	if supportCount != 2 {
		t.Errorf("support: expected 2 messages, got %d", supportCount)
	}

	// Verify message content isolation
	generalStore.mu.Lock()
	for _, msg := range generalStore.messages {
		for _, block := range msg.ContentBlocks {
			if tb, ok := block.(domain.TextBlock); ok {
				if tb.Text == "support-msg" || tb.Text == "support-reply" {
					t.Error("general channel contains support channel message")
				}
			}
		}
	}
	generalStore.mu.Unlock()

	supportStore.mu.Lock()
	for _, msg := range supportStore.messages {
		for _, block := range msg.ContentBlocks {
			if tb, ok := block.(domain.TextBlock); ok {
				if tb.Text == "general-msg" || tb.Text == "general-reply" {
					t.Error("support channel contains general channel message")
				}
			}
		}
	}
	supportStore.mu.Unlock()
}

func TestRoute_WhenMultipleChannels_ShouldCreateSeparateSessions(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	_, _ = r.Route(context.Background(), "general", "msg1")
	_, _ = r.Route(context.Background(), "support", "msg2")
	_, _ = r.Route(context.Background(), "random", "msg3")

	channels := r.ActiveChannels()
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d: %v", len(channels), channels)
	}

	// Verify each channel has its own session
	for _, chID := range []string{"general", "support", "random"} {
		ch, ok := r.GetChannel(chID)
		if !ok {
			t.Errorf("channel %q not found", chID)
			continue
		}
		if ch.Session.ChannelID != chID {
			t.Errorf("channel %q: session.ChannelID = %q", chID, ch.Session.ChannelID)
		}
	}
}

// =============================================================================
// ActiveChannels tests
// =============================================================================

func TestActiveChannels_WhenNoRouting_ShouldReturnEmpty(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	r := NewRouter(brain, nil)
	channels := r.ActiveChannels()
	if len(channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(channels))
	}
}

func TestActiveChannels_ShouldReturnSortedChannelIDs(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	r := NewRouter(brain, nil)

	_, _ = r.Route(context.Background(), "zebra", "z")
	_, _ = r.Route(context.Background(), "alpha", "a")
	_, _ = r.Route(context.Background(), "middle", "m")

	channels := r.ActiveChannels()
	expected := []string{"alpha", "middle", "zebra"}
	if len(channels) != len(expected) {
		t.Fatalf("expected %d channels, got %d", len(expected), len(channels))
	}
	for i, ch := range channels {
		if ch != expected[i] {
			t.Errorf("channel[%d]: want %q, got %q", i, expected[i], ch)
		}
	}
}

// =============================================================================
// GetChannel tests
// =============================================================================

func TestGetChannel_WhenChannelExists_ShouldReturnChannel(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	_, _ = r.Route(context.Background(), "general", "hello")

	ch, ok := r.GetChannel("general")
	if !ok {
		t.Fatal("expected channel to exist")
	}
	if ch.ID != "general" {
		t.Errorf("channel ID: want 'general', got %q", ch.ID)
	}
	if ch.Session.ChannelID != "general" {
		t.Errorf("session channel: want 'general', got %q", ch.Session.ChannelID)
	}
}

func TestGetChannel_WhenChannelNotFound_ShouldReturnFalse(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	r := NewRouter(brain, nil)

	_, ok := r.GetChannel("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent channel")
	}
}

// =============================================================================
// History recording tests
// =============================================================================

func TestRoute_WhenHistoryFactoryProvided_ShouldRecordUserMessage(t *testing.T) {
	brain := &mockGenerator{response: "reply"}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	_, _ = r.Route(context.Background(), "general", "user prompt")

	store := factory.Get("general")
	if store == nil {
		t.Fatal("expected history store")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.messages) < 1 {
		t.Fatal("expected at least 1 message in history")
	}

	// First message should be the user message
	userMsg := store.messages[0]
	if userMsg.Role != domain.RoleUser {
		t.Errorf("first message role: want user, got %s", userMsg.Role)
	}
}

func TestRoute_WhenHistoryFactoryProvided_ShouldRecordAssistantMessage(t *testing.T) {
	brain := &mockGenerator{response: "brain reply"}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	_, _ = r.Route(context.Background(), "general", "user prompt")

	store := factory.Get("general")
	if store == nil {
		t.Fatal("expected history store")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(store.messages))
	}

	// Second message should be the assistant message
	assistantMsg := store.messages[1]
	if assistantMsg.Role != domain.RoleAssistant {
		t.Errorf("second message role: want assistant, got %s", assistantMsg.Role)
	}
	// Verify the assistant message contains the brain's response
	if len(assistantMsg.ContentBlocks) == 0 {
		t.Fatal("expected content blocks in assistant message")
	}
	tb, ok := assistantMsg.ContentBlocks[0].(domain.TextBlock)
	if !ok {
		t.Fatalf("expected TextBlock, got %T", assistantMsg.ContentBlocks[0])
	}
	if tb.Text != "brain reply" {
		t.Errorf("assistant text: want 'brain reply', got %q", tb.Text)
	}
}

func TestRoute_WhenBrainFails_ShouldNotRecordAssistantMessage(t *testing.T) {
	brain := &mockGenerator{err: errors.New("boom")}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	_, _ = r.Route(context.Background(), "general", "hello")

	store := factory.Get("general")
	if store == nil {
		t.Fatal("expected history store")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	// Should only have the user message, not an assistant message
	if len(store.messages) != 1 {
		t.Errorf("expected 1 message (user only), got %d", len(store.messages))
	}
	if store.messages[0].Role != domain.RoleUser {
		t.Errorf("expected user role, got %s", store.messages[0].Role)
	}
}

func TestRoute_WhenNoHistoryFactory_ShouldStillWork(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	r := NewRouter(brain, nil)

	resp, err := r.Route(context.Background(), "general", "hello")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if resp != "ok" {
		t.Errorf("want 'ok', got %q", resp)
	}
}

// =============================================================================
// Concurrency tests
// =============================================================================

func TestRoute_WhenConcurrentDifferentChannels_ShouldBeSafe(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	const goroutines = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			chID := "ch-" + string(rune('A'+idx%26))
			_, err := r.Route(context.Background(), chID, "msg")
			if err != nil {
				t.Errorf("goroutine %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	channels := r.ActiveChannels()
	if len(channels) == 0 {
		t.Error("expected at least 1 channel")
	}
	// Verify all channels are properly sorted
	if !sort.StringsAreSorted(channels) {
		t.Error("channels should be sorted")
	}
}

func TestRoute_WhenConcurrentSameChannel_ShouldBeSafe(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	const goroutines = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := r.Route(context.Background(), "shared", "msg")
			if err != nil {
				t.Errorf("goroutine %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	channels := r.ActiveChannels()
	if len(channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(channels))
	}

	store := factory.Get("shared")
	if store == nil {
		t.Fatal("expected history store for 'shared'")
	}
	store.mu.Lock()
	count := len(store.messages)
	store.mu.Unlock()
	// Each goroutine produces 2 messages (user + assistant)
	if count != goroutines*2 {
		t.Errorf("expected %d messages, got %d", goroutines*2, count)
	}
}

// =============================================================================
// ChannelCount test
// =============================================================================

func TestGetOrCreateChannel_WhenChannelCreatedBetweenReadAndWriteLock_ShouldReturnExisting(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	factory := newTrackingHistoryFactory()
	r := NewRouter(brain, factory.Create)

	// Set up: goroutine A will pause after the read-lock miss, allowing
	// goroutine B to create the channel first. When A resumes, it hits the
	// double-check path and finds the channel already exists.
	// We call getOrCreateChannel directly to test the double-check locking,
	// since Route now serializes per channel via the LaneQueue.
	hookReached := make(chan struct{})
	proceed := make(chan struct{})
	r.afterReadMiss = func() {
		hookReached <- struct{}{}
		<-proceed
	}

	var wg sync.WaitGroup

	// Goroutine A: pauses between read-miss and write-lock.
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.getOrCreateChannel("contested")
	}()

	// Wait for goroutine A to reach the hook (read-lock missed, channel doesn't exist yet).
	<-hookReached

	// Disable hook so goroutine B proceeds normally.
	r.afterReadMiss = nil

	// Goroutine B: creates the channel via the normal path.
	r.getOrCreateChannel("contested")

	// Let goroutine A continue — it acquires write-lock and hits the double-check.
	close(proceed)
	wg.Wait()

	// Should still only have one channel.
	if r.ChannelCount() != 1 {
		t.Errorf("expected 1 channel, got %d", r.ChannelCount())
	}
}

// =============================================================================
// Lane-based serial queue tests
// =============================================================================

func TestRoute_WhenSameChannel_ShouldSerializeProcessing(t *testing.T) {
	// A slow brain that records execution order and detects concurrent execution.
	var mu sync.Mutex
	var order []string
	var concurrent int
	var maxConcurrent int

	brain := &mockGenerator{response: "ok"}
	// Override Generate to track concurrent execution
	slowBrain := &serialCheckBrain{
		mu:             &mu,
		order:          &order,
		concurrent:     &concurrent,
		maxConcurrent:  &maxConcurrent,
	}

	factory := newTrackingHistoryFactory()
	r := NewRouter(slowBrain, factory.Create)

	gate := make(chan struct{})
	var wg sync.WaitGroup

	_ = brain // suppress unused

	// Submit 5 messages to the same channel concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-gate
			_, _ = r.Route(context.Background(), "serial-ch", "msg")
		}(i)
	}

	close(gate) // Release all goroutines at once
	wg.Wait()

	mu.Lock()
	mc := maxConcurrent
	mu.Unlock()

	if mc > 1 {
		t.Errorf("expected max concurrent Generate calls on same channel to be 1, got %d", mc)
	}
}

func TestRoute_WhenSameChannel_ShouldPreserveFIFOOrder(t *testing.T) {
	var mu sync.Mutex
	var prompts []string

	gate := make(chan struct{})
	fifoBrain := &fifoTrackBrain{
		mu:      &mu,
		prompts: &prompts,
		gate:    gate, // set before any goroutine starts to avoid data race
	}

	factory := newTrackingHistoryFactory()
	r := NewRouter(fifoBrain, factory.Create)

	// Block the lane by sending a blocking first message
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = r.Route(context.Background(), "fifo-ch", "block")
	}()

	time.Sleep(30 * time.Millisecond) // ensure blocker is running

	// Queue up ordered messages
	for i := 0; i < 5; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			prompt := "msg-" + string(rune('A'+i))
			_, _ = r.Route(context.Background(), "fifo-ch", prompt)
		}()
		time.Sleep(10 * time.Millisecond) // stagger to encourage FIFO
	}

	close(gate) // release the blocker
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	// First prompt should be "block", then msg-A through msg-E in order
	if len(prompts) != 6 {
		t.Fatalf("expected 6 prompts, got %d: %v", len(prompts), prompts)
	}
	if prompts[0] != "block" {
		t.Errorf("first prompt should be 'block', got %q", prompts[0])
	}
	expected := []string{"msg-A", "msg-B", "msg-C", "msg-D", "msg-E"}
	for i, want := range expected {
		if prompts[i+1] != want {
			t.Errorf("prompt[%d]: want %q, got %q (all: %v)", i+1, want, prompts[i+1], prompts)
			break
		}
	}
}

func TestRoute_WhenDifferentChannels_ShouldAllowParallelProcessing(t *testing.T) {
	var mu sync.Mutex
	var concurrent int
	var maxConcurrent int

	parallelBrain := &serialCheckBrain{
		mu:            &mu,
		order:         &[]string{},
		concurrent:    &concurrent,
		maxConcurrent: &maxConcurrent,
	}

	factory := newTrackingHistoryFactory()
	r := NewRouter(parallelBrain, factory.Create)

	barrier := make(chan struct{})
	parallelBrain.barrier = barrier

	var wg sync.WaitGroup

	// Submit to 5 different channels
	for i := 0; i < 5; i++ {
		wg.Add(1)
		chID := "ch-" + string(rune('A'+i))
		go func() {
			defer wg.Done()
			_, _ = r.Route(context.Background(), chID, "msg")
		}()
	}

	// Give goroutines time to reach the barrier
	time.Sleep(100 * time.Millisecond)
	close(barrier)
	wg.Wait()

	mu.Lock()
	mc := maxConcurrent
	mu.Unlock()

	if mc < 2 {
		t.Errorf("expected parallel execution across channels (max concurrent >= 2), got %d", mc)
	}
}

// serialCheckBrain tracks concurrent Generate calls.
type serialCheckBrain struct {
	mu            *sync.Mutex
	order         *[]string
	concurrent    *int
	maxConcurrent *int
	barrier       chan struct{} // if set, blocks until closed
}

func (b *serialCheckBrain) Generate(_ context.Context, prompt string) (string, error) {
	b.mu.Lock()
	*b.concurrent++
	if *b.concurrent > *b.maxConcurrent {
		*b.maxConcurrent = *b.concurrent
	}
	*b.order = append(*b.order, prompt)
	b.mu.Unlock()

	if b.barrier != nil {
		<-b.barrier
	}

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	b.mu.Lock()
	*b.concurrent--
	b.mu.Unlock()

	return "reply-" + prompt, nil
}

// fifoTrackBrain records prompts in order and optionally blocks on "block" prompt.
type fifoTrackBrain struct {
	mu      *sync.Mutex
	prompts *[]string
	gate    chan struct{}
}

func (b *fifoTrackBrain) Generate(_ context.Context, prompt string) (string, error) {
	if prompt == "block" && b.gate != nil {
		<-b.gate
	}
	b.mu.Lock()
	*b.prompts = append(*b.prompts, prompt)
	b.mu.Unlock()
	return "reply-" + prompt, nil
}

func TestChannelCount_ShouldReturnNumberOfActiveChannels(t *testing.T) {
	brain := &mockGenerator{response: "ok"}
	r := NewRouter(brain, nil)

	if r.ChannelCount() != 0 {
		t.Errorf("expected 0, got %d", r.ChannelCount())
	}

	_, _ = r.Route(context.Background(), "ch1", "hi")
	if r.ChannelCount() != 1 {
		t.Errorf("expected 1, got %d", r.ChannelCount())
	}

	_, _ = r.Route(context.Background(), "ch2", "hi")
	if r.ChannelCount() != 2 {
		t.Errorf("expected 2, got %d", r.ChannelCount())
	}

	// Same channel again shouldn't increase count
	_, _ = r.Route(context.Background(), "ch1", "hi again")
	if r.ChannelCount() != 2 {
		t.Errorf("expected 2 after routing to existing channel, got %d", r.ChannelCount())
	}
}
