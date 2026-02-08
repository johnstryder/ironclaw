package session

import (
	"sync"

	"ironclaw/internal/domain"
)

// Manager provides thread-safe access to a Session, notably Status.
type Manager struct {
	mu      sync.RWMutex
	session *domain.Session
}

// NewManager returns a Manager wrapping the given session (may be nil).
func NewManager(session *domain.Session) *Manager {
	return &Manager{session: session}
}

// Status returns the current agent status (idle, thinking, typing, failed).
func (m *Manager) Status() domain.AgentStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.session == nil {
		return domain.StatusIdle
	}
	return m.session.Status
}

// SetStatus updates the session status.
func (m *Manager) SetStatus(s domain.AgentStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		m.session.Status = s
	}
}

// Snapshot returns a copy of the current Session.
func (m *Manager) Snapshot() domain.Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.session == nil {
		return domain.Session{}
	}
	return *m.session
}
