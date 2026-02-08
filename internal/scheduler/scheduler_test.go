package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Mock CronEngine for testing (avoids real cron dependency)
// =============================================================================

type mockCronEngine struct {
	mu       sync.Mutex
	funcs    map[int]func()
	nextID   int
	started  bool
	stopped  bool
	addErr   error // when non-nil, AddFunc returns this error
	removed  []int // track removed entry IDs
}

func newMockCronEngine() *mockCronEngine {
	return &mockCronEngine{
		funcs:  make(map[int]func()),
		nextID: 1,
	}
}

func (m *mockCronEngine) AddFunc(spec string, cmd func()) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.addErr != nil {
		return 0, m.addErr
	}
	id := m.nextID
	m.nextID++
	m.funcs[id] = cmd
	return id, nil
}

func (m *mockCronEngine) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, id)
	delete(m.funcs, id)
}

func (m *mockCronEngine) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
}

func (m *mockCronEngine) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
}

// fire simulates a cron trigger for the given entry ID.
func (m *mockCronEngine) fire(id int) {
	m.mu.Lock()
	fn, ok := m.funcs[id]
	m.mu.Unlock()
	if ok {
		fn()
	}
}

// fireAll simulates all registered cron jobs firing.
func (m *mockCronEngine) fireAll() {
	m.mu.Lock()
	fns := make([]func(), 0, len(m.funcs))
	for _, fn := range m.funcs {
		fns = append(fns, fn)
	}
	m.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// =============================================================================
// NewScheduler Tests
// =============================================================================

func TestNewScheduler_ShouldReturnNonNilScheduler(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }

	s := NewScheduler(engine, handler)

	if s == nil {
		t.Fatal("expected non-nil Scheduler")
	}
}

func TestNewScheduler_WhenNilHandler_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewScheduler(engine, nil) should panic")
		}
	}()
	engine := newMockCronEngine()
	NewScheduler(engine, nil)
}

func TestNewScheduler_WhenNilEngine_ShouldPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewScheduler(nil, handler) should panic")
		}
	}()
	handler := func(ctx context.Context, job Job) error { return nil }
	NewScheduler(nil, handler)
}

// =============================================================================
// AddJob Tests
// =============================================================================

func TestScheduler_AddJob_ShouldReturnNoError(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	job := Job{
		ID:       "job-1",
		Name:     "Test Job",
		CronExpr: "*/5 * * * *",
		Prompt:   "Hello, world!",
	}

	err := s.AddJob(job)
	if err != nil {
		t.Fatalf("AddJob should succeed, got error: %v", err)
	}
}

func TestScheduler_AddJob_WhenEmptyID_ShouldReturnError(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	job := Job{
		ID:       "",
		CronExpr: "*/5 * * * *",
		Prompt:   "test",
	}

	err := s.AddJob(job)
	if err == nil {
		t.Fatal("expected error for empty job ID")
	}
}

func TestScheduler_AddJob_WhenEmptyCronExpr_ShouldReturnError(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	job := Job{
		ID:       "job-1",
		CronExpr: "",
		Prompt:   "test",
	}

	err := s.AddJob(job)
	if err == nil {
		t.Fatal("expected error for empty cron expression")
	}
}

func TestScheduler_AddJob_WhenEmptyPrompt_ShouldReturnError(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	job := Job{
		ID:       "job-1",
		CronExpr: "*/5 * * * *",
		Prompt:   "",
	}

	err := s.AddJob(job)
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestScheduler_AddJob_WhenDuplicateID_ShouldReturnError(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	job := Job{ID: "job-1", CronExpr: "*/5 * * * *", Prompt: "test"}

	if err := s.AddJob(job); err != nil {
		t.Fatalf("first AddJob should succeed: %v", err)
	}

	err := s.AddJob(job)
	if err == nil {
		t.Fatal("expected error for duplicate job ID")
	}
}

func TestScheduler_AddJob_WhenCronEngineReturnsError_ShouldReturnError(t *testing.T) {
	engine := newMockCronEngine()
	engine.addErr = errors.New("invalid cron expression")
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	job := Job{ID: "job-1", CronExpr: "bad-cron", Prompt: "test"}

	err := s.AddJob(job)
	if err == nil {
		t.Fatal("expected error when cron engine fails")
	}
}

// =============================================================================
// Start / Stop Tests
// =============================================================================

func TestScheduler_Start_ShouldStartCronEngine(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	s.Start()

	if !engine.started {
		t.Error("expected cron engine to be started")
	}
}

func TestScheduler_Stop_ShouldStopCronEngine(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	s.Start()
	s.Stop()

	if !engine.stopped {
		t.Error("expected cron engine to be stopped")
	}
}

// =============================================================================
// RemoveJob Tests
// =============================================================================

func TestScheduler_RemoveJob_ShouldRemoveExistingJob(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	job := Job{ID: "job-1", CronExpr: "*/5 * * * *", Prompt: "test"}
	_ = s.AddJob(job)

	err := s.RemoveJob("job-1")
	if err != nil {
		t.Fatalf("RemoveJob should succeed, got error: %v", err)
	}

	// Verify the engine was told to remove the entry.
	if len(engine.removed) == 0 {
		t.Error("expected cron engine Remove to be called")
	}
}

func TestScheduler_RemoveJob_WhenJobDoesNotExist_ShouldReturnError(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	err := s.RemoveJob("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job ID")
	}
}

func TestScheduler_RemoveJob_ShouldAllowReAddingJobAfterRemoval(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	job := Job{ID: "job-1", CronExpr: "*/5 * * * *", Prompt: "test"}
	_ = s.AddJob(job)
	_ = s.RemoveJob("job-1")

	// Should be able to re-add after removal.
	err := s.AddJob(job)
	if err != nil {
		t.Fatalf("should be able to re-add after removal, got: %v", err)
	}
}

// =============================================================================
// ListJobs Tests
// =============================================================================

func TestScheduler_ListJobs_ShouldReturnAllRegisteredJobs(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	_ = s.AddJob(Job{ID: "a", CronExpr: "*/1 * * * *", Prompt: "p1"})
	_ = s.AddJob(Job{ID: "b", CronExpr: "*/2 * * * *", Prompt: "p2"})

	jobs := s.ListJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Should contain both jobs.
	found := map[string]bool{}
	for _, j := range jobs {
		found[j.ID] = true
	}
	if !found["a"] || !found["b"] {
		t.Errorf("expected jobs 'a' and 'b', got %v", jobs)
	}
}

func TestScheduler_ListJobs_WhenNoJobs_ShouldReturnEmptySlice(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	jobs := s.ListJobs()
	if jobs == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestScheduler_ListJobs_ShouldNotIncludeRemovedJobs(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	_ = s.AddJob(Job{ID: "a", CronExpr: "*/1 * * * *", Prompt: "p1"})
	_ = s.AddJob(Job{ID: "b", CronExpr: "*/2 * * * *", Prompt: "p2"})
	_ = s.RemoveJob("a")

	jobs := s.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job after removal, got %d", len(jobs))
	}
	if jobs[0].ID != "b" {
		t.Errorf("expected remaining job 'b', got %q", jobs[0].ID)
	}
}

// =============================================================================
// Event Handler Invocation Tests
// =============================================================================

func TestScheduler_WhenCronFires_ShouldCallHandlerWithCorrectJob(t *testing.T) {
	engine := newMockCronEngine()
	var receivedJob Job
	var handlerCalled bool
	handler := func(ctx context.Context, job Job) error {
		handlerCalled = true
		receivedJob = job
		return nil
	}
	s := NewScheduler(engine, handler)

	job := Job{ID: "job-1", Name: "Test", CronExpr: "*/5 * * * *", Prompt: "Hello agent!"}
	_ = s.AddJob(job)

	// Simulate cron trigger (entry ID 1 is the first registered job).
	engine.fire(1)

	if !handlerCalled {
		t.Fatal("expected handler to be called when cron fires")
	}
	if receivedJob.ID != "job-1" {
		t.Errorf("expected job ID 'job-1', got %q", receivedJob.ID)
	}
	if receivedJob.Prompt != "Hello agent!" {
		t.Errorf("expected prompt 'Hello agent!', got %q", receivedJob.Prompt)
	}
	if receivedJob.Name != "Test" {
		t.Errorf("expected name 'Test', got %q", receivedJob.Name)
	}
}

func TestScheduler_WhenCronFires_ShouldProvideNonNilContext(t *testing.T) {
	engine := newMockCronEngine()
	var receivedCtx context.Context
	handler := func(ctx context.Context, job Job) error {
		receivedCtx = ctx
		return nil
	}
	s := NewScheduler(engine, handler)

	_ = s.AddJob(Job{ID: "job-1", CronExpr: "*/5 * * * *", Prompt: "test"})
	engine.fire(1)

	if receivedCtx == nil {
		t.Fatal("expected non-nil context in handler")
	}
}

func TestScheduler_WhenMultipleJobsFire_ShouldCallHandlerForEach(t *testing.T) {
	engine := newMockCronEngine()
	var mu sync.Mutex
	receivedIDs := []string{}
	handler := func(ctx context.Context, job Job) error {
		mu.Lock()
		receivedIDs = append(receivedIDs, job.ID)
		mu.Unlock()
		return nil
	}
	s := NewScheduler(engine, handler)

	_ = s.AddJob(Job{ID: "a", CronExpr: "*/1 * * * *", Prompt: "p1"})
	_ = s.AddJob(Job{ID: "b", CronExpr: "*/2 * * * *", Prompt: "p2"})

	engine.fireAll()

	mu.Lock()
	defer mu.Unlock()
	if len(receivedIDs) != 2 {
		t.Fatalf("expected 2 handler calls, got %d", len(receivedIDs))
	}
}

func TestScheduler_WhenHandlerReturnsError_ShouldNotPanic(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error {
		return errors.New("handler failed")
	}
	s := NewScheduler(engine, handler)

	_ = s.AddJob(Job{ID: "job-1", CronExpr: "*/5 * * * *", Prompt: "test"})

	// Should not panic even if handler returns error.
	engine.fire(1)
}

// =============================================================================
// RemoveJob Edge Cases
// =============================================================================

func TestScheduler_RemoveJob_WhenEmptyID_ShouldReturnError(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	err := s.RemoveJob("")
	if err == nil {
		t.Fatal("expected error for empty job ID")
	}
}

// =============================================================================
// Logging Tests
// =============================================================================

func TestScheduler_AddJob_ShouldLogJobRegistration(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler, WithLogger(logger))

	_ = s.AddJob(Job{ID: "job-1", Name: "Nightly", CronExpr: "0 0 * * *", Prompt: "run nightly"})

	logOutput := buf.String()
	if !strings.Contains(logOutput, "job-1") {
		t.Errorf("expected log to contain job ID, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "job registered") {
		t.Errorf("expected log to contain 'job registered', got %q", logOutput)
	}
}

func TestScheduler_RemoveJob_ShouldLogJobRemoval(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler, WithLogger(logger))

	_ = s.AddJob(Job{ID: "job-1", CronExpr: "0 0 * * *", Prompt: "test"})
	buf.Reset() // clear add log
	_ = s.RemoveJob("job-1")

	logOutput := buf.String()
	if !strings.Contains(logOutput, "job-1") {
		t.Errorf("expected log to contain job ID, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "job removed") {
		t.Errorf("expected log to contain 'job removed', got %q", logOutput)
	}
}

func TestScheduler_WhenCronFires_ShouldLogExecution(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler, WithLogger(logger))

	_ = s.AddJob(Job{ID: "job-1", CronExpr: "*/5 * * * *", Prompt: "test"})
	buf.Reset() // clear add log
	engine.fire(1)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "job-1") {
		t.Errorf("expected log to contain job ID, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "job fired") {
		t.Errorf("expected log to contain 'job fired', got %q", logOutput)
	}
}

func TestScheduler_WhenHandlerFails_ShouldLogError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error {
		return errors.New("handler exploded")
	}
	s := NewScheduler(engine, handler, WithLogger(logger))

	_ = s.AddJob(Job{ID: "job-1", CronExpr: "*/5 * * * *", Prompt: "test"})
	engine.fire(1)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "handler exploded") {
		t.Errorf("expected log to contain handler error, got %q", logOutput)
	}
}

func TestWithLogger_WhenNil_ShouldUseDefaultLogger(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	// Should not panic with nil logger option.
	s := NewScheduler(engine, handler, WithLogger(nil))
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
}

// =============================================================================
// GetJob Tests
// =============================================================================

func TestScheduler_GetJob_ShouldReturnExistingJob(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	original := Job{ID: "job-1", Name: "Test", CronExpr: "*/5 * * * *", Prompt: "hello"}
	_ = s.AddJob(original)

	got, ok := s.GetJob("job-1")
	if !ok {
		t.Fatal("expected to find job")
	}
	if got.ID != "job-1" || got.Prompt != "hello" {
		t.Errorf("expected job-1 with prompt 'hello', got %+v", got)
	}
}

func TestScheduler_GetJob_WhenNotFound_ShouldReturnFalse(t *testing.T) {
	engine := newMockCronEngine()
	handler := func(ctx context.Context, job Job) error { return nil }
	s := NewScheduler(engine, handler)

	_, ok := s.GetJob("nonexistent")
	if ok {
		t.Fatal("expected not to find job")
	}
}

// =============================================================================
// Integration-style Test: Full lifecycle
// =============================================================================

func TestScheduler_FullLifecycle_AddStartFireStopRemove(t *testing.T) {
	engine := newMockCronEngine()
	var mu sync.Mutex
	events := []string{}
	handler := func(ctx context.Context, job Job) error {
		mu.Lock()
		events = append(events, "fired:"+job.ID)
		mu.Unlock()
		return nil
	}
	s := NewScheduler(engine, handler)

	// Add two jobs.
	if err := s.AddJob(Job{ID: "a", CronExpr: "*/1 * * * *", Prompt: "p1"}); err != nil {
		t.Fatalf("AddJob a: %v", err)
	}
	if err := s.AddJob(Job{ID: "b", CronExpr: "*/2 * * * *", Prompt: "p2"}); err != nil {
		t.Fatalf("AddJob b: %v", err)
	}

	// Start scheduler.
	s.Start()

	// Verify jobs are listed.
	if len(s.ListJobs()) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(s.ListJobs()))
	}

	// Fire both jobs.
	engine.fireAll()

	mu.Lock()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	mu.Unlock()

	// Remove one job.
	if err := s.RemoveJob("a"); err != nil {
		t.Fatalf("RemoveJob a: %v", err)
	}
	if len(s.ListJobs()) != 1 {
		t.Fatalf("expected 1 job after removal, got %d", len(s.ListJobs()))
	}

	// Stop scheduler.
	s.Stop()

	if !engine.started {
		t.Error("expected engine started")
	}
	if !engine.stopped {
		t.Error("expected engine stopped")
	}
}
