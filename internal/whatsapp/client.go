package whatsapp

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// rawClient abstracts the whatsmeow.Client methods used by WhatsmeowClient.
// This enables unit testing without a real WhatsApp connection.
// *whatsmeow.Client satisfies this interface implicitly.
type rawClient interface {
	Connect() error
	Disconnect()
	SendMessage(ctx context.Context, to types.JID, message *waE2E.Message, extra ...whatsmeow.SendRequestExtra) (whatsmeow.SendResponse, error)
	GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error)
	AddEventHandler(handler whatsmeow.EventHandler) uint32
}

// WhatsmeowClient wraps a whatsmeow.Client to implement the WAClient interface.
// This is the production implementation used in cmd/whatsapp-bridge.
type WhatsmeowClient struct {
	client rawClient
	store  *store.Device
	msgCh  chan IncomingMessage
}

// NewWhatsmeowClient creates a new WhatsmeowClient wrapping the given whatsmeow client.
func NewWhatsmeowClient(client *whatsmeow.Client) *WhatsmeowClient {
	wc := &WhatsmeowClient{
		client: client,
		store:  client.Store,
		msgCh:  make(chan IncomingMessage, 100),
	}
	client.AddEventHandler(wc.eventHandler)
	return wc
}

// Connect establishes the WhatsApp connection.
func (w *WhatsmeowClient) Connect() error {
	return w.client.Connect()
}

// Disconnect closes the WhatsApp connection.
func (w *WhatsmeowClient) Disconnect() {
	w.client.Disconnect()
}

// IsLoggedIn returns true if the client has a stored session (device ID).
func (w *WhatsmeowClient) IsLoggedIn() bool {
	return w.store.ID != nil
}

// SendText sends a text message to the given JID.
func (w *WhatsmeowClient) SendText(ctx context.Context, chatJID string, text string) error {
	jid, err := types.ParseJID(chatJID)
	if err != nil {
		return fmt.Errorf("whatsapp: parse JID %q: %w", chatJID, err)
	}
	_, err = w.client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(text),
	})
	return err
}

// MessageChannel returns a channel that receives incoming text messages.
func (w *WhatsmeowClient) MessageChannel() <-chan IncomingMessage {
	return w.msgCh
}

// GetQRChannel returns a channel of QR events for the login flow.
func (w *WhatsmeowClient) GetQRChannel(ctx context.Context) (<-chan QREvent, error) {
	qrChan, err := w.client.GetQRChannel(ctx)
	if err != nil {
		return nil, err
	}
	out := make(chan QREvent, 10)
	go func() {
		defer close(out)
		for item := range qrChan {
			out <- QREvent{
				Code:  item.Code,
				Event: item.Event,
			}
		}
	}()
	return out, nil
}

// eventHandler processes whatsmeow events and forwards text messages to msgCh.
func (w *WhatsmeowClient) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		text := extractText(v.Message)
		if text == "" {
			return
		}
		w.msgCh <- IncomingMessage{
			SenderJID: v.Info.Sender.String(),
			ChatJID:   v.Info.Chat.String(),
			Text:      text,
			MessageID: string(v.Info.ID),
		}
	}
}

// extractText gets the text content from a WhatsApp message proto.
func extractText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	// Simple text message
	if msg.Conversation != nil {
		return *msg.Conversation
	}
	// Extended text message (quoted replies, links, etc.)
	if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil {
		return *msg.ExtendedTextMessage.Text
	}
	return ""
}

