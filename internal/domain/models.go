package domain

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Core Configuration
// =============================================================================

type Config struct {
	Gateway         GatewayConfig `json:"gateway"`
	Agents          AgentsConfig  `json:"agents"`
	Infra           InfraConfig   `json:"infra"`
	Retry           RetryConfig   `json:"retry"`
	AllowedCommands []string      `json:"allowedCommands"` // If non-empty, only these command binaries may be executed
	Mode            string        `json:"mode,omitempty"`  // Setup mode: "local", "server", "remote"
	RemoteURL       string        `json:"remoteUrl,omitempty"`
	RemoteToken     string        `json:"remoteToken,omitempty"`
	Channels        []string      `json:"channels,omitempty"` // Enabled channels (e.g., telegram, discord)
}

// RetryConfig controls retry behaviour for external API calls (LLM, webhooks).
type RetryConfig struct {
	MaxRetries     int `json:"maxRetries"`     // Maximum retry attempts (0 = no retries)
	InitialBackoff int `json:"initialBackoff"` // Initial backoff in milliseconds
	MaxBackoff     int `json:"maxBackoff"`     // Maximum backoff in milliseconds
	Multiplier     int `json:"multiplier"`     // Backoff multiplier (e.g. 2 for exponential doubling)
}

type GatewayConfig struct {
	Port         int        `json:"port"`
	Auth         AuthConfig `json:"auth"`
	AllowedHosts []string   `json:"allowedHosts"`
}

type AuthConfig struct {
	Mode                  string   `json:"mode"`                  // "password" | "token" | "none"
	AuthToken             string   `json:"authToken,omitempty"`   // When set, gateway requires Authorization: Bearer <authToken>
	PromptPIN             string   `json:"promptPin,omitempty"`   // The 4-digit secret
	RequirePINForExternal bool     `json:"requirePinForExternal"` // Enforce PIN on public channels
	ExternalChannels      []string `json:"externalChannels"`      // e.g., ["telegram", "whatsapp"]
	RateLimitMaxAttempts  int      `json:"rateLimitMaxAttempts"`
}

type AgentsConfig struct {
	Provider     string            `json:"provider"` // "openai" | "anthropic" | "local"
	DefaultModel string            `json:"defaultModel"`
	ModelAliases map[string]string `json:"modelAliases"`
	Paths        AgentPaths        `json:"paths"`
	Fallbacks    []FallbackConfig  `json:"fallbacks,omitempty"` // optional failover providers
}

// FallbackConfig describes an alternative LLM provider for failover.
type FallbackConfig struct {
	Provider     string `json:"provider"` // "openai" | "anthropic" | "local" | "ollama" | "gemini" | "openrouter"
	DefaultModel string `json:"defaultModel"`
}

type AgentPaths struct {
	Root   string `json:"root"`   // Path to agent workspace (contains AGENTS.md)
	Memory string `json:"memory"` // Path to durable memory logs
}

type InfraConfig struct {
	LogFormat string `json:"logFormat"` // "json" | "text"
	LogLevel  string `json:"logLevel"`
}

// =============================================================================
// Agent & Session Domain
// =============================================================================

type AgentStatus string

const (
	StatusIdle     AgentStatus = "idle"
	StatusThinking AgentStatus = "thinking"
	StatusTyping   AgentStatus = "typing"
	StatusFailed   AgentStatus = "failed"
)

type Session struct {
	ID        string            `json:"id"`
	ChannelID string            `json:"channelId"`
	Platform  string            `json:"platform"` // telegram, slack, etc.
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	Status    AgentStatus       `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`

	// AuthState tracks if the user has authenticated via PIN.
	AuthState SessionAuthState `json:"authState"`

	// Context window management
	History     []Message `json:"history"`
	TokenCount  int       `json:"tokenCount"`
	HistoryPath string    `json:"historyPath"`
}

type SessionAuthState struct {
	IsAuthenticated bool      `json:"isAuthenticated"`
	Attempts        int       `json:"attempts"`
	LastAttempt     time.Time `json:"lastAttempt"`
}

type AgentContext struct {
	Identity string   `json:"identity"` // Content of IDENTITY.md
	Soul     string   `json:"soul"`     // Content of SOUL.md
	Tools    []string `json:"tools"`    // List of enabled tools from TOOLS.md
}

// MemoryStore persists agent memory entries. Implementations must append only (no overwrite).
type MemoryStore interface {
	// Append writes content to the log for the given date (YYYY-MM-DD). Must append, not overwrite.
	Append(date string, content string) error

	// Remember appends a fact or note to the persistent long-term memory file (memory.md).
	Remember(content string) error

	// LoadMemory reads the entire persistent long-term memory file and returns its content.
	// Returns empty string (not error) when the file does not exist yet.
	LoadMemory() (string, error)
}

// =============================================================================
// Messaging Protocol
// =============================================================================

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

// Message is the canonical message type. RawContent holds JSON; ContentBlocks
// is populated after UnmarshalJSON for polymorphic content (text, image, tool_use, tool_result).
type Message struct {
	ID        string      `json:"id"`
	Role      MessageRole `json:"role"`
	Timestamp time.Time   `json:"timestamp"`

	// Polymorphic content: string or []ContentBlock (stored as raw JSON)
	RawContent json.RawMessage `json:"content"`
	// Parsed blocks (populated after Unmarshal)
	ContentBlocks []ContentBlock `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling for polymorphic content.
// If content is a string, it becomes a single TextBlock; if an array, each element
// is decoded by its "type" field into the appropriate ContentBlock implementation.
func (m *Message) UnmarshalJSON(data []byte) error {
	type messageMessage Message
	type alias struct {
		Content json.RawMessage `json:"content"`
		messageMessage
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	m.ID = a.ID
	m.Role = a.Role
	m.Timestamp = a.Timestamp
	m.RawContent = a.Content
	m.ContentBlocks = nil

	if len(a.Content) == 0 {
		return nil
	}
	blocks, err := parseMessageContent(a.Content)
	if err != nil {
		return err
	}
	m.ContentBlocks = blocks
	return nil
}

// parseMessageContent decodes content (string or array of blocks) into ContentBlocks.
// Used by Message.UnmarshalJSON and by tests to cover the array-unmarshal error path.
func parseMessageContent(content json.RawMessage) ([]ContentBlock, error) {
	var s string
	if err := json.Unmarshal(content, &s); err == nil {
		return []ContentBlock{TextBlock{Text: s}}, nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, err
	}
	blocks := make([]ContentBlock, 0, len(raw))
	for _, r := range raw {
		var typeOnly struct {
			Type BlockType `json:"type"`
		}
		if err := json.Unmarshal(r, &typeOnly); err != nil {
			continue
		}
		switch typeOnly.Type {
		case BlockText:
			var b TextBlock
			if err := json.Unmarshal(r, &b); err == nil {
				blocks = append(blocks, b)
			}
		case BlockImage:
			var b ImageBlock
			if err := json.Unmarshal(r, &b); err == nil {
				blocks = append(blocks, b)
			}
		case BlockToolUse:
			var b ToolUseBlock
			if err := json.Unmarshal(r, &b); err == nil {
				blocks = append(blocks, b)
			}
		case BlockToolResult:
			var b ToolResultBlock
			if err := json.Unmarshal(r, &b); err == nil {
				blocks = append(blocks, b)
			}
		}
	}
	return blocks, nil
}

type BlockType string

const (
	BlockText       BlockType = "text"
	BlockImage      BlockType = "image"
	BlockToolUse    BlockType = "tool_use"
	BlockToolResult BlockType = "tool_result"
)

type ContentBlock interface {
	Type() BlockType
}

type TextBlock struct {
	Text string `json:"text"`
}

func (TextBlock) Type() BlockType { return BlockText }

type ImageBlock struct {
	Source MediaType `json:"source"`
}

type MediaType struct {
	Type      string `json:"type"`       // e.g., "base64"
	MediaType string `json:"media_type"` // e.g., "image/jpeg"
	Data      string `json:"data"`
}

func (ImageBlock) Type() BlockType { return BlockImage }

type ToolUseBlock struct {
	ToolUseID string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
}

func (ToolUseBlock) Type() BlockType { return BlockToolUse }

type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

func (ToolResultBlock) Type() BlockType { return BlockToolResult }

// =============================================================================
// Tooling & Skills
// =============================================================================

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type Tool interface {
	Definition() ToolDefinition
	Execute(ctx ToolContext, input json.RawMessage) (*ToolResult, error)
}

type ToolResult struct {
	Data      string            `json:"data"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Artifacts []string          `json:"artifacts,omitempty"`
}

// ToolContext mirrors context.Context for tool execution (Deadline, Done, Err, Value).
type ToolContext interface {
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

// SemanticMemory represents a stored memory retrieved by vector similarity search.
type SemanticMemory struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	Score     float64   `json:"score"` // cosine similarity (0-1)
	CreatedAt time.Time `json:"createdAt"`
}
