package auth

import (
	"sync"
	"time"

	"ironclaw/internal/domain"
)

// AuthState provides thread-safe access to SessionAuthState (PIN attempts, authenticated flag).
type AuthState struct {
	mu   sync.RWMutex
	state domain.SessionAuthState
}

// NewAuthState returns an AuthState backed by the given initial state.
func NewAuthState(initial domain.SessionAuthState) *AuthState {
	return &AuthState{state: initial}
}

// IsAuthenticated returns whether the session has authenticated via PIN.
func (a *AuthState) IsAuthenticated() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state.IsAuthenticated
}

// SetAuthenticated sets the authenticated flag.
func (a *AuthState) SetAuthenticated(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.IsAuthenticated = v
}

// RecordAttempt increments attempt count and sets LastAttempt to now.
func (a *AuthState) RecordAttempt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Attempts++
	a.state.LastAttempt = time.Now()
}

// ResetAttempts sets Attempts to 0.
func (a *AuthState) ResetAttempts() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Attempts = 0
}

// Snapshot returns a copy of the current SessionAuthState.
func (a *AuthState) Snapshot() domain.SessionAuthState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return domain.SessionAuthState{
		IsAuthenticated: a.state.IsAuthenticated,
		Attempts:        a.state.Attempts,
		LastAttempt:     a.state.LastAttempt,
	}
}

// RequirePINForChannel returns true if the channel is in AuthConfig.ExternalChannels
// and RequirePINForExternal is true (security gate for external channels).
func RequirePINForChannel(cfg domain.AuthConfig, channel string) bool {
	if !cfg.RequirePINForExternal {
		return false
	}
	for _, c := range cfg.ExternalChannels {
		if c == channel {
			return true
		}
	}
	return false
}
