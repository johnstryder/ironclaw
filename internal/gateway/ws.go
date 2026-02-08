package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"ironclaw/internal/router"
)

// ChatBrain is the interface used by the WS handler to generate replies.
// Implementations (e.g. brain.Brain) are provider-agnostic.
type ChatBrain interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// DefaultChannelID is used when a message arrives without a ChannelID.
const DefaultChannelID = "default"

// WSMessage is the JSON message protocol for the WebSocket gateway.
// Example: {"type": "chat", "content": "hello", "channelId": "general"}
type WSMessage struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	ChannelID string `json:"channelId,omitempty"`
}

// jsonMarshal is used when encoding WSMessage; tests may replace it to force Marshal errors.
// Access is protected by jsonMarshalMu for race-safe test swaps.
var (
	jsonMarshalMu sync.RWMutex
	jsonMarshal   = json.Marshal
)

// Default upgrader for WebSocket connections.
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// HandleWS upgrades the request to WebSocket and runs a read loop, responding on the same connection.
// If brain is non-nil and message type is "chat", the reply is routed through a per-channel Router;
// otherwise echo. ChannelID from the incoming message is preserved in the response.
// Messages without a ChannelID are assigned to the "default" channel.
// Writes are serialized with a mutex so multiple goroutines could write safely.
// Only GET is accepted for the WebSocket handshake.
func HandleWS(w http.ResponseWriter, r *http.Request, brain ChatBrain) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Create a per-connection router that wraps the brain.
	// Each connection gets its own router so channel state is per-connection.
	var rt *router.Router
	if brain != nil {
		rt = router.NewRouter(brain, nil)
	}

	var writeMu sync.Mutex
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var in WSMessage
		if err := json.Unmarshal(raw, &in); err != nil {
			reply := WSMessage{Type: "error", Content: "invalid JSON"}
			writeWSMessage(conn, &writeMu, &reply)
			continue
		}

		// Resolve channel ID.
		channelID := in.ChannelID
		if channelID == "" {
			channelID = DefaultChannelID
		}

		isBrainChat := rt != nil && in.Type == "chat"

		// Send typing_start before brain generation.
		if isBrainChat {
			typingStart := WSMessage{Type: "typing_start", ChannelID: channelID}
			writeWSMessage(conn, &writeMu, &typingStart)
		}

		content := "echo: " + in.Content
		if isBrainChat {
			reply, err := rt.Route(r.Context(), channelID, in.Content)
			if err != nil {
				content = "error: " + err.Error()
			} else {
				content = reply
			}
		}
		out := WSMessage{Type: in.Type, Content: content, ChannelID: channelID}
		writeWSMessage(conn, &writeMu, &out)

		// Send typing_stop after brain response is delivered.
		if isBrainChat {
			typingStop := WSMessage{Type: "typing_stop", ChannelID: channelID}
			writeWSMessage(conn, &writeMu, &typingStop)
		}
	}
}

func writeWSMessage(conn *websocket.Conn, mu *sync.Mutex, msg *WSMessage) {
	jsonMarshalMu.RLock()
	marshal := jsonMarshal
	jsonMarshalMu.RUnlock()
	data, err := marshal(msg)
	if err != nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	_ = conn.WriteMessage(websocket.TextMessage, data)
}
