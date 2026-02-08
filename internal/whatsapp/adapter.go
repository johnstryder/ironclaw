package whatsapp

import (
	"context"
	"fmt"
	"sync"
)

// WAClient abstracts WhatsApp client operations for testing.
type WAClient interface {
	Connect() error
	Disconnect()
	IsLoggedIn() bool
	SendText(ctx context.Context, chatJID string, text string) error
	MessageChannel() <-chan IncomingMessage
	GetQRChannel(ctx context.Context) (<-chan QREvent, error)
}

// MessageRouter routes messages to the brain (implemented by router.Router).
type MessageRouter interface {
	Route(ctx context.Context, channelID, prompt string) (string, error)
}

// IncomingMessage represents a received WhatsApp text message.
type IncomingMessage struct {
	SenderJID string // e.g., "1234567890@s.whatsapp.net"
	ChatJID   string // Same as SenderJID for DMs, or "groupid@g.us" for groups
	Text      string
	MessageID string
}

// QREvent represents a QR code event during the WhatsApp login flow.
type QREvent struct {
	Code  string // The QR code data (for "code" events)
	Event string // "code", "timeout", "success", "error"
}

// QRHandler is called with QR code data during login.
// In production, this prints the QR code to the terminal.
type QRHandler func(code string)

// Adapter bridges WhatsApp to IronClaw's multi-channel routing system.
type Adapter struct {
	client    WAClient
	router    MessageRouter
	qrHandler QRHandler

	mu     sync.Mutex
	cancel context.CancelFunc
}

// JIDToChannelID converts a WhatsApp JID string to an IronClaw ChannelID.
func JIDToChannelID(jid string) string {
	return "whatsapp-" + jid
}

// HandleMessage processes a single incoming WhatsApp message.
// Ignores messages with empty text.
// Routes the message text through the brain and sends the reply back to WhatsApp.
func (a *Adapter) HandleMessage(ctx context.Context, msg IncomingMessage) {
	if msg.Text == "" {
		return
	}

	channelID := JIDToChannelID(msg.ChatJID)

	reply, err := a.router.Route(ctx, channelID, msg.Text)
	if err != nil {
		reply = "Error: " + err.Error()
	}

	_ = a.client.SendText(ctx, msg.ChatJID, reply)
}

// NewAdapter creates a new WhatsApp adapter. Both client and router must be non-nil.
func NewAdapter(client WAClient, router MessageRouter, qrHandler QRHandler) *Adapter {
	if client == nil {
		panic("whatsapp: client must not be nil")
	}
	if router == nil {
		panic("whatsapp: router must not be nil")
	}
	if qrHandler == nil {
		qrHandler = func(string) {} // no-op default
	}
	return &Adapter{
		client:    client,
		router:    router,
		qrHandler: qrHandler,
	}
}

// Start connects to WhatsApp and begins processing incoming messages.
// If the client is not logged in, it performs the QR code login flow first.
// Blocks until ctx is canceled. When ctx is done, Disconnect is called.
func (a *Adapter) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()

	// QR code login flow if not already logged in.
	if !a.client.IsLoggedIn() {
		qrCh, err := a.client.GetQRChannel(ctx)
		if err != nil {
			cancel()
			return fmt.Errorf("whatsapp: QR channel: %w", err)
		}
		if err := a.client.Connect(); err != nil {
			cancel()
			return fmt.Errorf("whatsapp: connect: %w", err)
		}
		// Process QR events until login completes or channel closes.
		for evt := range qrCh {
			if evt.Event == "code" {
				a.qrHandler(evt.Code)
			}
		}
	} else {
		if err := a.client.Connect(); err != nil {
			cancel()
			return fmt.Errorf("whatsapp: connect: %w", err)
		}
	}

	// Listen for incoming messages.
	msgCh := a.client.MessageChannel()
	for {
		select {
		case <-ctx.Done():
			a.client.Disconnect()
			return nil
		case msg := <-msgCh:
			a.HandleMessage(ctx, msg)
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
