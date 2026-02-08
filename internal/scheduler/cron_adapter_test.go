package scheduler

import (
	"sync"
	"testing"
	"time"
)

// =============================================================================
// RobfigCronEngine Tests
// =============================================================================

func TestNewRobfigCronEngine_ShouldReturnNonNilEngine(t *testing.T) {
	engine := NewRobfigCronEngine()
	if engine == nil {
		t.Fatal("expected non-nil RobfigCronEngine")
	}
}

func TestRobfigCronEngine_ShouldImplementCronEngineInterface(t *testing.T) {
	var _ CronEngine = NewRobfigCronEngine()
}

func TestRobfigCronEngine_AddFunc_ShouldReturnEntryID(t *testing.T) {
	engine := NewRobfigCronEngine()
	defer engine.Stop()

	id, err := engine.AddFunc("@every 1h", func() {})
	if err != nil {
		t.Fatalf("AddFunc should succeed: %v", err)
	}
	if id < 0 {
		t.Errorf("expected non-negative entry ID, got %d", id)
	}
}

func TestRobfigCronEngine_AddFunc_WhenInvalidCron_ShouldReturnError(t *testing.T) {
	engine := NewRobfigCronEngine()
	defer engine.Stop()

	_, err := engine.AddFunc("not-a-cron-expression", func() {})
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

func TestRobfigCronEngine_StartAndStop_ShouldNotPanic(t *testing.T) {
	engine := NewRobfigCronEngine()
	engine.Start()
	engine.Stop()
}

func TestRobfigCronEngine_Remove_ShouldNotPanic(t *testing.T) {
	engine := NewRobfigCronEngine()
	defer engine.Stop()

	id, _ := engine.AddFunc("@every 1h", func() {})
	engine.Remove(id) // Should not panic.
}

func TestRobfigCronEngine_AddFunc_ShouldFireOnSchedule(t *testing.T) {
	engine := NewRobfigCronEngine()
	defer engine.Stop()

	var mu sync.Mutex
	fired := false

	_, err := engine.AddFunc("@every 1s", func() {
		mu.Lock()
		fired = true
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("AddFunc: %v", err)
	}

	engine.Start()

	// Wait up to 3 seconds for the job to fire.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := fired
		mu.Unlock()
		if done {
			return // success
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("expected cron job to fire within 3 seconds")
}
