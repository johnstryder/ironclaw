package telegram

import (
	"context"
	"strconv"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BotAPI abstracts the Telegram Bot API for testing.
type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

// MessageRouter routes messages to the brain (implemented by router.Router).
type MessageRouter interface {
	Route(ctx context.Context, channelID, prompt string) (string, error)
}

// Adapter bridges Telegram to IronClaw's multi-channel routing system.
type Adapter struct {
	bot    BotAPI
	router MessageRouter

	mu     sync.Mutex
	cancel context.CancelFunc
}

// NewAdapter creates a new Telegram adapter. Both bot and router must be non-nil.
func NewAdapter(bot BotAPI, router MessageRouter) *Adapter {
	if bot == nil {
		panic("telegram: bot must not be nil")
	}
	if router == nil {
		panic("telegram: router must not be nil")
	}
	return &Adapter{
		bot:    bot,
		router: router,
	}
}

// ChatIDToChannelID converts a Telegram ChatID to an IronClaw ChannelID.
func ChatIDToChannelID(chatID int64) string {
	return "telegram-" + strconv.FormatInt(chatID, 10)
}

// HandleUpdate processes a single Telegram update.
// Ignores updates without a message or with empty text.
// Routes the message text through the brain and sends the reply back to Telegram.
func (a *Adapter) HandleUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}
	text := update.Message.Text
	if text == "" {
		return
	}

	chatID := update.Message.Chat.ID
	channelID := ChatIDToChannelID(chatID)

	reply, err := a.router.Route(ctx, channelID, text)
	if err != nil {
		reply = "Error: " + err.Error()
	}

	msg := tgbotapi.NewMessage(chatID, reply)
	msg.ReplyToMessageID = update.Message.MessageID
	_, _ = a.bot.Send(msg)
}

// Start begins polling for Telegram updates and processing them.
// Blocks until ctx is canceled. When ctx is done, StopReceivingUpdates is called.
func (a *Adapter) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := a.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			a.bot.StopReceivingUpdates()
			return
		case update := <-updates:
			a.HandleUpdate(ctx, update)
		}
	}
}

// Stop gracefully shuts down the adapter.
func (a *Adapter) Stop() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
