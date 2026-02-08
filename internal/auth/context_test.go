package auth

import (
	"sync"
	"testing"
	"time"

	"ironclaw/internal/domain"
)

func TestAuthState_WhenNew_ShouldNotBeAuthenticated(t *testing.T) {
	s := NewAuthState(domain.SessionAuthState{})
	if s.IsAuthenticated() {
		t.Error("expected new auth state to not be authenticated")
	}
}

func TestAuthState_WhenSetAuthenticated_ShouldReportAuthenticated(t *testing.T) {
	s := NewAuthState(domain.SessionAuthState{})
	s.SetAuthenticated(true)
	if !s.IsAuthenticated() {
		t.Error("expected authenticated after SetAuthenticated(true)")
	}
	s.SetAuthenticated(false)
	if s.IsAuthenticated() {
		t.Error("expected not authenticated after SetAuthenticated(false)")
	}
}

func TestAuthState_RecordAttempt_ShouldIncrementAttemptsAndUpdateLastAttempt(t *testing.T) {
	s := NewAuthState(domain.SessionAuthState{})
	before := time.Now()
	s.RecordAttempt()
	snapshot := s.Snapshot()
	if snapshot.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", snapshot.Attempts)
	}
	if snapshot.LastAttempt.Before(before) {
		t.Error("expected LastAttempt to be >= before")
	}
	s.RecordAttempt()
	snapshot = s.Snapshot()
	if snapshot.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", snapshot.Attempts)
	}
}

func TestAuthState_WhenConcurrentAccess_ShouldRemainConsistent(t *testing.T) {
	s := NewAuthState(domain.SessionAuthState{})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.RecordAttempt()
			s.IsAuthenticated()
			s.Snapshot()
		}()
	}
	wg.Wait()
	snapshot := s.Snapshot()
	if snapshot.Attempts != 100 {
		t.Errorf("expected 100 attempts under concurrency, got %d", snapshot.Attempts)
	}
}

func TestAuthState_ResetAttempts_ShouldZeroAttempts(t *testing.T) {
	s := NewAuthState(domain.SessionAuthState{})
	s.RecordAttempt()
	s.RecordAttempt()
	s.ResetAttempts()
	snapshot := s.Snapshot()
	if snapshot.Attempts != 0 {
		t.Errorf("expected 0 attempts after reset, got %d", snapshot.Attempts)
	}
}

func TestAuthState_Snapshot_ShouldReturnCopyOfState(t *testing.T) {
	s := NewAuthState(domain.SessionAuthState{})
	s.SetAuthenticated(true)
	s.RecordAttempt()
	snap := s.Snapshot()
	if !snap.IsAuthenticated {
		t.Error("snapshot should reflect authenticated")
	}
	if snap.Attempts != 1 {
		t.Errorf("snapshot attempts: want 1, got %d", snap.Attempts)
	}
	// Mutate snapshot; original should be unchanged
	snap.Attempts = 99
	snap2 := s.Snapshot()
	if snap2.Attempts != 1 {
		t.Errorf("original state should be unchanged, got %d", snap2.Attempts)
	}
}

func TestRequirePINForChannel_WhenChannelInList_ShouldReturnTrue(t *testing.T) {
	cfg := domain.AuthConfig{
		ExternalChannels:      []string{"telegram", "whatsapp"},
		RequirePINForExternal: true,
	}
	if !RequirePINForChannel(cfg, "telegram") {
		t.Error("expected true for telegram")
	}
	if !RequirePINForChannel(cfg, "whatsapp") {
		t.Error("expected true for whatsapp")
	}
}

func TestRequirePINForChannel_WhenChannelNotInList_ShouldReturnFalse(t *testing.T) {
	cfg := domain.AuthConfig{
		ExternalChannels:      []string{"telegram"},
		RequirePINForExternal: true,
	}
	if RequirePINForChannel(cfg, "slack") {
		t.Error("expected false for slack")
	}
}

func TestRequirePINForChannel_WhenRequirePINForExternalFalse_ShouldReturnFalse(t *testing.T) {
	cfg := domain.AuthConfig{
		ExternalChannels:      []string{"telegram"},
		RequirePINForExternal: false,
	}
	if RequirePINForChannel(cfg, "telegram") {
		t.Error("expected false when RequirePINForExternal is false")
	}
}
