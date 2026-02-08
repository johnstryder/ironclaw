package session

import (
	"sync"
	"testing"
	"time"

	"ironclaw/internal/domain"
)

func TestManager_Status_WhenNew_ShouldBeIdle(t *testing.T) {
	sess := domain.Session{ID: "s1", Status: domain.StatusIdle}
	m := NewManager(&sess)
	if m.Status() != domain.StatusIdle {
		t.Errorf("expected idle, got %s", m.Status())
	}
}

func TestManager_SetStatus_ShouldUpdateStatus(t *testing.T) {
	sess := domain.Session{ID: "s1", Status: domain.StatusIdle}
	m := NewManager(&sess)
	m.SetStatus(domain.StatusThinking)
	if m.Status() != domain.StatusThinking {
		t.Errorf("expected thinking, got %s", m.Status())
	}
	m.SetStatus(domain.StatusTyping)
	if m.Status() != domain.StatusTyping {
		t.Errorf("expected typing, got %s", m.Status())
	}
}

func TestManager_WhenConcurrentSetStatus_ShouldRemainConsistent(t *testing.T) {
	sess := domain.Session{ID: "s1", Status: domain.StatusIdle}
	m := NewManager(&sess)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.SetStatus(domain.StatusThinking)
			_ = m.Status()
			m.SetStatus(domain.StatusIdle)
		}()
	}
	wg.Wait()
	st := m.Status()
	if st != domain.StatusIdle && st != domain.StatusThinking {
		t.Errorf("expected idle or thinking after concurrent access, got %s", st)
	}
}

func TestManager_Snapshot_ShouldReturnCopyOfSession(t *testing.T) {
	ts := time.Now()
	sess := domain.Session{
		ID: "s1", ChannelID: "ch1", Platform: "telegram",
		Status: domain.StatusThinking, CreatedAt: ts, UpdatedAt: ts,
	}
	m := NewManager(&sess)
	m.SetStatus(domain.StatusTyping)
	snap := m.Snapshot()
	if snap.ID != "s1" || snap.ChannelID != "ch1" {
		t.Errorf("snapshot: id=%q channel=%q", snap.ID, snap.ChannelID)
	}
	if snap.Status != domain.StatusTyping {
		t.Errorf("snapshot status: want typing, got %s", snap.Status)
	}
}

func TestManager_Snapshot_WhenSessionNil_ShouldReturnZeroValue(t *testing.T) {
	m := NewManager(nil)
	if m.Status() != domain.StatusIdle {
		t.Errorf("Status when session nil: want idle, got %s", m.Status())
	}
	snap := m.Snapshot()
	if snap.ID != "" {
		t.Errorf("expected zero snapshot when session nil, got id=%q", snap.ID)
	}
}
