package main

import (
	"bytes"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"ironclaw/internal/scheduler"
)

// mockCronEngine for testing without real timers.
type mockCronEngine struct {
	mu      sync.Mutex
	funcs   map[int]func()
	nextID  int
	started bool
	stopped bool
	addErr  error
}

func newMockCronEngine() *mockCronEngine {
	return &mockCronEngine{funcs: make(map[int]func()), nextID: 1}
}

func (m *mockCronEngine) AddFunc(_ string, cmd func()) (int, error) {
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

func TestRun_WhenShutdownImmediately_ShouldReturnZero(t *testing.T) {
	mock := newMockCronEngine()
	origEngine := newEngine
	newEngine = func() scheduler.CronEngine { return mock }
	defer func() { newEngine = origEngine }()

	ch := make(chan struct{})
	close(ch)

	var buf bytes.Buffer
	code := run(&buf, ch)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "Scheduler demo started") {
		t.Errorf("expected startup message, got %q", out)
	}
	if !strings.Contains(out, "Done.") {
		t.Errorf("expected shutdown message, got %q", out)
	}
	if !mock.started {
		t.Error("expected scheduler to be started")
	}
	if !mock.stopped {
		t.Error("expected scheduler to be stopped")
	}
}

func TestRun_ShouldRegisterTwoJobs(t *testing.T) {
	mock := newMockCronEngine()
	origEngine := newEngine
	newEngine = func() scheduler.CronEngine { return mock }
	defer func() { newEngine = origEngine }()

	ch := make(chan struct{})
	close(ch)

	var buf bytes.Buffer
	code := run(&buf, ch)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "every-30s") {
		t.Errorf("expected every-30s job listed, got %q", out)
	}
	if !strings.Contains(out, "every-1m") {
		t.Errorf("expected every-1m job listed, got %q", out)
	}
}

func TestRun_WhenJobFires_ShouldPrintSystemEvent(t *testing.T) {
	mock := newMockCronEngine()
	origEngine := newEngine
	newEngine = func() scheduler.CronEngine { return mock }
	defer func() { newEngine = origEngine }()

	ch := make(chan struct{})

	var buf bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(&buf, ch)
	}()

	// Wait for the goroutine to register jobs and reach the blocking wait.
	deadline := time.After(2 * time.Second)
	for {
		mock.mu.Lock()
		n := len(mock.funcs)
		mock.mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for jobs to be registered")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Fire all registered jobs.
	mock.fireAll()

	// Shutdown.
	close(ch)
	code := <-done

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "SYSTEM EVENT received") {
		t.Errorf("expected system event log, got %q", out)
	}
	if !strings.Contains(out, ">>> [System Event:") {
		t.Errorf("expected system event print, got %q", out)
	}
}

func TestRun_WhenFirstAddJobFails_ShouldReturnOne(t *testing.T) {
	mock := newMockCronEngine()
	mock.addErr = &failOnceError{msg: "bad cron"}
	origEngine := newEngine
	newEngine = func() scheduler.CronEngine { return mock }
	defer func() { newEngine = origEngine }()

	ch := make(chan struct{})
	close(ch)

	var buf bytes.Buffer
	code := run(&buf, ch)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestRun_WhenSecondAddJobFails_ShouldReturnOne(t *testing.T) {
	calls := 0
	mock := &countingMockEngine{
		mockCronEngine: newMockCronEngine(),
		failOnCall:     2,
	}
	origEngine := newEngine
	newEngine = func() scheduler.CronEngine { return mock }
	defer func() { newEngine = origEngine }()
	_ = calls

	ch := make(chan struct{})
	close(ch)

	var buf bytes.Buffer
	code := run(&buf, ch)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

// failOnceError is a simple error type.
type failOnceError struct{ msg string }

func (e *failOnceError) Error() string { return e.msg }

// countingMockEngine fails AddFunc on a specific call number.
type countingMockEngine struct {
	*mockCronEngine
	failOnCall int
	callCount  int
}

func (c *countingMockEngine) AddFunc(spec string, cmd func()) (int, error) {
	c.callCount++
	if c.callCount == c.failOnCall {
		return 0, &failOnceError{msg: "fail on call " + string(rune('0'+c.failOnCall))}
	}
	return c.mockCronEngine.AddFunc(spec, cmd)
}

func TestRun_WhenNilShutdownCh_ShouldBlockOnSignalAndExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal test only on Unix")
	}

	mock := newMockCronEngine()
	origEngine := newEngine
	newEngine = func() scheduler.CronEngine { return mock }
	defer func() { newEngine = origEngine }()

	var buf bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(&buf, nil) // nil shutdownCh -> blocks on signal
	}()

	// Wait for the scheduler to be started.
	deadline := time.After(2 * time.Second)
	for {
		mock.mu.Lock()
		started := mock.started
		mock.mu.Unlock()
		if started {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for scheduler to start")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Send SIGINT to ourselves to unblock the signal wait.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
		t.Skipf("cannot send SIGINT: %v", err)
	}

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not return after SIGINT")
	}

	out := buf.String()
	if !strings.Contains(out, "Done.") {
		t.Errorf("expected shutdown message, got %q", out)
	}
}

func TestNewEngine_Default_ShouldReturnRobfigEngine(t *testing.T) {
	engine := newEngine()
	if engine == nil {
		t.Fatal("expected non-nil engine from default newEngine")
	}
	// Verify it implements the interface.
	var _ scheduler.CronEngine = engine
}

func TestMain_ShouldCallRunAndExit(t *testing.T) {
	mock := newMockCronEngine()
	origEngine := newEngine
	newEngine = func() scheduler.CronEngine { return mock }
	defer func() { newEngine = origEngine }()

	// Stub exitFunc so main() doesn't actually exit the test process.
	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Send SIGINT to unblock the nil-shutdownCh path in run().
	go func() {
		// Wait for scheduler to start, then signal.
		deadline := time.After(2 * time.Second)
		for {
			mock.mu.Lock()
			started := mock.started
			mock.mu.Unlock()
			if started {
				break
			}
			select {
			case <-deadline:
				return
			default:
				time.Sleep(5 * time.Millisecond)
			}
		}
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	main()

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}
