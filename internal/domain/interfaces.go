package domain

import "context"

// LLMProvider is the model-agnostic interface for text generation.
// Implementations may be OpenAI, Anthropic, local models, or mocks.
type LLMProvider interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// SessionHistoryStore persists session messages to a JSONL file and supports
// loading the last N messages to restore context on restart.
type SessionHistoryStore interface {
	// Append serializes a Message to JSON and appends it as a single line to the history file.
	Append(msg Message) error

	// LoadHistory reads the last n messages from the history file.
	// Returns empty slice when the file does not exist or n <= 0.
	LoadHistory(n int) ([]Message, error)
}

// Tokenizer counts tokens in a string for LLM context window management.
type Tokenizer interface {
	// CountTokens returns the number of tokens in the given text.
	CountTokens(text string) (int, error)
}

// ContextManager fits messages into a model's context window.
type ContextManager interface {
	// FitToWindow takes messages and a system prompt, and returns messages
	// that fit within the configured token limit. The system prompt tokens
	// are always reserved. Older messages are dropped first (sliding window).
	FitToWindow(messages []Message, systemPrompt string) ([]Message, error)
}

// HistorySyncer watches a history JSONL file for external changes (e.g. from
// Syncthing, Dropbox, or another device) and delivers newly appended messages
// via a callback so they can be merged into runtime memory.
type HistorySyncer interface {
	// Start begins watching the history file for changes. Calls the provided
	// callback whenever new messages are detected. Must not block.
	Start(callback func([]Message)) error

	// Stop ceases watching and releases all resources.
	Stop() error
}

// Embedder generates vector embeddings from text using a local or remote model.
// Implementations may use Ollama, OpenAI embeddings, or other providers.
type Embedder interface {
	// Embed returns a dense float64 vector for the given text.
	Embed(ctx context.Context, text string) ([]float64, error)
}

// SubAgentRunner runs a specialist sub-agent in isolation with a custom system
// prompt (role) and a task. Implementations create a secondary LLM loop that
// does not share the parent's memory, history, or context.
type SubAgentRunner interface {
	// RunSubAgent executes a task with a specialist system prompt and returns
	// the LLM response. The sub-agent runs in isolation from the parent.
	RunSubAgent(ctx context.Context, systemPrompt string, task string) (string, error)
}
