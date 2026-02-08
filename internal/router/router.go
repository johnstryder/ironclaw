package router

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"ironclaw/internal/domain"
	"ironclaw/internal/queue"
)

// Generator generates responses from prompts (implemented by brain.Brain).
type Generator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// HistoryFactory creates a SessionHistoryStore for a given channel ID.
type HistoryFactory func(channelID string) domain.SessionHistoryStore

// Channel represents an active channel with its own session and history.
type Channel struct {
	ID      string
	Session domain.Session
	History domain.SessionHistoryStore
}

// ErrEmptyChannelID is returned when Route is called with an empty channel ID.
var ErrEmptyChannelID = errors.New("router: channel ID must not be empty")

// Router manages active channels and routes messages to the brain.
// Each channel maintains its own session state and message history.
// Route calls for the same channel are serialized in FIFO order via a LaneQueue.
type Router struct {
	mu             sync.RWMutex
	channels       map[string]*Channel
	brain          Generator
	historyFactory HistoryFactory
	laneQueue      *queue.LaneQueue

	// afterReadMiss is a test hook called after a read-lock miss and before acquiring
	// the write lock in getOrCreateChannel. Allows tests to deterministically exercise
	// the double-check path. Nil in production.
	afterReadMiss func()
}

// NewRouter creates a new Router. brain must not be nil.
// historyFactory may be nil; if so, messages are not persisted to history.
func NewRouter(brain Generator, factory HistoryFactory) *Router {
	if brain == nil {
		panic("router: brain must not be nil")
	}
	return &Router{
		channels:       make(map[string]*Channel),
		brain:          brain,
		historyFactory: factory,
		laneQueue:      queue.NewLaneQueue(),
	}
}

// Route sends a prompt to the brain in the context of the specified channel.
// Creates the channel if it doesn't exist. Records user and assistant messages
// in the channel's history (if a HistoryFactory was provided).
// Route calls for the same channel are serialized in FIFO order.
func (r *Router) Route(ctx context.Context, channelID, prompt string) (string, error) {
	if channelID == "" {
		return "", ErrEmptyChannelID
	}

	var response string
	err := r.laneQueue.Do(ctx, channelID, func() error {
		ch := r.getOrCreateChannel(channelID)

		// Record user message in history.
		if ch.History != nil {
			userMsg := newTextMessage(domain.RoleUser, prompt)
			_ = ch.History.Append(userMsg)
		}

		// Generate response via the brain.
		resp, genErr := r.brain.Generate(ctx, prompt)
		if genErr != nil {
			return genErr
		}

		// Record assistant response in history.
		if ch.History != nil {
			assistantMsg := newTextMessage(domain.RoleAssistant, resp)
			_ = ch.History.Append(assistantMsg)
		}

		response = resp
		return nil
	})

	return response, err
}

// ActiveChannels returns a sorted list of active channel IDs.
func (r *Router) ActiveChannels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.channels))
	for id := range r.channels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// GetChannel returns a copy of the channel, or false if not found.
func (r *Router) GetChannel(channelID string) (Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.channels[channelID]
	if !ok {
		return Channel{}, false
	}
	return *ch, true
}

// ChannelCount returns the number of active channels.
func (r *Router) ChannelCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.channels)
}

// getOrCreateChannel returns the channel for the given ID, creating it if needed.
func (r *Router) getOrCreateChannel(channelID string) *Channel {
	// Fast path: read lock.
	r.mu.RLock()
	ch, ok := r.channels[channelID]
	r.mu.RUnlock()
	if ok {
		return ch
	}

	if r.afterReadMiss != nil {
		r.afterReadMiss()
	}

	// Slow path: write lock, double-check.
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok = r.channels[channelID]
	if ok {
		return ch
	}

	var hist domain.SessionHistoryStore
	if r.historyFactory != nil {
		hist = r.historyFactory(channelID)
	}

	now := time.Now()
	ch = &Channel{
		ID: channelID,
		Session: domain.Session{
			ID:        "session-" + channelID,
			ChannelID: channelID,
			Status:    domain.StatusIdle,
			CreatedAt: now,
			UpdatedAt: now,
		},
		History: hist,
	}
	r.channels[channelID] = ch
	return ch
}

// newTextMessage creates a Message with a text content block.
func newTextMessage(role domain.MessageRole, text string) domain.Message {
	raw, _ := json.Marshal(text)
	return domain.Message{
		Role:       role,
		Timestamp:  time.Now(),
		RawContent: json.RawMessage(raw),
		ContentBlocks: []domain.ContentBlock{
			domain.TextBlock{Text: text},
		},
	}
}
