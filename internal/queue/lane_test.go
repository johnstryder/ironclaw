package queue

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// NewLaneQueue tests
// =============================================================================

func TestNewLaneQueue_ShouldReturnNonNilQueue(t *testing.T) {
	q := NewLaneQueue()
	if q == nil {
		t.Fatal("expected non-nil LaneQueue")
	}
}

func TestNewLaneQueue_ShouldStartWithZeroLanes(t *testing.T) {
	q := NewLaneQueue()
	if q.LaneCount() != 0 {
		t.Errorf("expected 0 lanes, got %d", q.LaneCount())
	}
}

// =============================================================================
// Do — basic execution tests
// =============================================================================

func TestDo_WhenWorkProvided_ShouldExecuteIt(t *testing.T) {
	q := NewLaneQueue()
	executed := false
	err := q.Do(context.Background(), "lane-1", func() error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !executed {
		t.Error("expected work function to be executed")
	}
}

func TestDo_WhenWorkReturnsError_ShouldPropagateError(t *testing.T) {
	q := NewLaneQueue()
	expected := errors.New("work failed")
	err := q.Do(context.Background(), "lane-1", func() error {
		return expected
	})
	if !errors.Is(err, expected) {
		t.Errorf("want %v, got %v", expected, err)
	}
}

func TestDo_WhenWorkReturnsNil_ShouldReturnNil(t *testing.T) {
	q := NewLaneQueue()
	err := q.Do(context.Background(), "lane-1", func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// =============================================================================
// Do — lane ID validation
// =============================================================================

func TestDo_WhenEmptyLaneID_ShouldReturnError(t *testing.T) {
	q := NewLaneQueue()
	err := q.Do(context.Background(), "", func() error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for empty lane ID")
	}
	if !errors.Is(err, ErrEmptyLaneID) {
		t.Errorf("want ErrEmptyLaneID, got %v", err)
	}
}

func TestDo_WhenEmptyLaneID_ShouldNotExecuteWork(t *testing.T) {
	q := NewLaneQueue()
	executed := false
	_ = q.Do(context.Background(), "", func() error {
		executed = true
		return nil
	})
	if executed {
		t.Error("work should not execute with empty lane ID")
	}
}

// =============================================================================
// Do — serialization within same lane
// =============================================================================

func TestDo_WhenSameLane_ShouldSerializeExecution(t *testing.T) {
	q := NewLaneQueue()

	var concurrent int64
	var maxConcurrent int64

	started := make(chan struct{})
	proceed := make(chan struct{})

	var wg sync.WaitGroup

	// First work item: blocks until we signal
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = q.Do(context.Background(), "lane-1", func() error {
			cur := atomic.AddInt64(&concurrent, 1)
			defer atomic.AddInt64(&concurrent, -1)
			for {
				old := atomic.LoadInt64(&maxConcurrent)
				if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
					break
				}
			}
			close(started)
			<-proceed
			return nil
		})
	}()

	<-started // Wait for first work to start

	// Second work item: should block until first completes
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = q.Do(context.Background(), "lane-1", func() error {
			cur := atomic.AddInt64(&concurrent, 1)
			defer atomic.AddInt64(&concurrent, -1)
			for {
				old := atomic.LoadInt64(&maxConcurrent)
				if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
					break
				}
			}
			return nil
		})
	}()

	// Give goroutine 2 time to attempt acquisition
	time.Sleep(50 * time.Millisecond)

	// Release first work
	close(proceed)
	wg.Wait()

	if atomic.LoadInt64(&maxConcurrent) > 1 {
		t.Errorf("max concurrent was %d, expected 1 (serial execution)", maxConcurrent)
	}
}

func TestDo_WhenSameLane_ShouldPreserveFIFOOrder(t *testing.T) {
	q := NewLaneQueue()
	const n = 10
	var order []int
	var mu sync.Mutex

	// Block the lane with a gate; use gateStarted to confirm the worker is running.
	gate := make(chan struct{})
	gateStarted := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = q.Do(context.Background(), "fifo", func() error {
			close(gateStarted) // signal: worker is running and lane is blocked
			<-gate
			return nil
		})
	}()

	<-gateStarted // deterministic: gate-holder is definitely running

	// Queue up n work items while lane is blocked.
	// Each send goes into the buffered channel (non-blocking). Yield and
	// sleep between launches so goroutines enqueue in creation order.
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = q.Do(context.Background(), "fifo", func() error {
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
				return nil
			})
		}()
		runtime.Gosched()             // yield so the goroutine can run
		time.Sleep(10 * time.Millisecond) // ample time for the send to reach the channel
	}

	close(gate) // Release the blocking work
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(order) != n {
		t.Fatalf("expected %d entries, got %d", n, len(order))
	}
	for i := 0; i < n; i++ {
		if order[i] != i {
			t.Errorf("position %d: expected %d, got %d (order: %v)", i, i, order[i], order)
			break
		}
	}
}

// =============================================================================
// Do — cross-lane concurrency
// =============================================================================

func TestDo_WhenDifferentLanes_ShouldAllowConcurrentExecution(t *testing.T) {
	q := NewLaneQueue()

	var concurrent int64
	var maxConcurrent int64
	var wg sync.WaitGroup

	barrier := make(chan struct{})

	for i := 0; i < 5; i++ {
		wg.Add(1)
		laneID := string(rune('A' + i))
		go func() {
			defer wg.Done()
			_ = q.Do(context.Background(), laneID, func() error {
				cur := atomic.AddInt64(&concurrent, 1)
				defer atomic.AddInt64(&concurrent, -1)
				// Update max
				for {
					old := atomic.LoadInt64(&maxConcurrent)
					if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
						break
					}
				}
				<-barrier // All goroutines wait here
				return nil
			})
		}()
	}

	// Give goroutines time to reach the barrier
	time.Sleep(100 * time.Millisecond)
	close(barrier)
	wg.Wait()

	if atomic.LoadInt64(&maxConcurrent) < 2 {
		t.Errorf("max concurrent was %d, expected at least 2 (cross-lane parallelism)", maxConcurrent)
	}
}

// =============================================================================
// Do — context cancellation
// =============================================================================

func TestDo_WhenContextCancelledWhileWaiting_ShouldReturnContextError(t *testing.T) {
	q := NewLaneQueue()

	// Block the lane
	gate := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = q.Do(context.Background(), "lane-1", func() error {
			<-gate
			return nil
		})
	}()

	// Ensure blocker is running
	time.Sleep(30 * time.Millisecond)

	// Submit with a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- q.Do(ctx, "lane-1", func() error {
			return nil
		})
	}()

	// Give time for the second Do to start waiting, then cancel
	time.Sleep(30 * time.Millisecond)
	cancel()

	err := <-errCh
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}

	close(gate)
	wg.Wait()
}

func TestDo_WhenContextAlreadyCancelled_ShouldReturnContextError(t *testing.T) {
	q := NewLaneQueue()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := q.Do(ctx, "lane-1", func() error {
		t.Error("work should not execute with cancelled context")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
}

// =============================================================================
// LaneCount tests
// =============================================================================

func TestLaneCount_WhenNoWork_ShouldReturnZero(t *testing.T) {
	q := NewLaneQueue()
	if q.LaneCount() != 0 {
		t.Errorf("expected 0 lanes, got %d", q.LaneCount())
	}
}

func TestLaneCount_WhenWorkSubmitted_ShouldTrackLanes(t *testing.T) {
	q := NewLaneQueue()
	_ = q.Do(context.Background(), "lane-A", func() error { return nil })
	_ = q.Do(context.Background(), "lane-B", func() error { return nil })

	if q.LaneCount() != 2 {
		t.Errorf("expected 2 lanes, got %d", q.LaneCount())
	}
}

func TestLaneCount_WhenSameLane_ShouldNotDuplicate(t *testing.T) {
	q := NewLaneQueue()
	_ = q.Do(context.Background(), "lane-A", func() error { return nil })
	_ = q.Do(context.Background(), "lane-A", func() error { return nil })

	if q.LaneCount() != 1 {
		t.Errorf("expected 1 lane, got %d", q.LaneCount())
	}
}

// =============================================================================
// Stress tests
// =============================================================================

func TestDo_WhenConcurrentSameLane_ShouldBeSafe(t *testing.T) {
	q := NewLaneQueue()

	const goroutines = 100
	var total int64
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = q.Do(context.Background(), "shared", func() error {
				atomic.AddInt64(&total, 1)
				return nil
			})
		}()
	}
	wg.Wait()

	if atomic.LoadInt64(&total) != goroutines {
		t.Errorf("expected %d executions, got %d", goroutines, total)
	}
}

func TestDo_WhenConcurrentDifferentLanes_ShouldBeSafe(t *testing.T) {
	q := NewLaneQueue()

	const goroutines = 100
	var total int64
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		laneID := string(rune('A' + i%26))
		go func() {
			defer wg.Done()
			_ = q.Do(context.Background(), laneID, func() error {
				atomic.AddInt64(&total, 1)
				return nil
			})
		}()
	}
	wg.Wait()

	if atomic.LoadInt64(&total) != goroutines {
		t.Errorf("expected %d executions, got %d", goroutines, total)
	}
}

func TestDo_WhenChannelFullAndContextCancelled_ShouldReturnContextError(t *testing.T) {
	// Use a tiny buffer so we can fill it and force the submit-phase select
	// to hit the ctx.Done() branch.
	old := defaultLaneBufferSize
	defaultLaneBufferSize = 1
	defer func() { defaultLaneBufferSize = old }()

	q := NewLaneQueue()

	// Block the lane worker so it doesn't drain the buffer.
	gate := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = q.Do(context.Background(), "lane-1", func() error {
			<-gate
			return nil
		})
	}()

	time.Sleep(20 * time.Millisecond) // ensure blocker is running in the worker

	// Fill the buffer (capacity 1). This goroutine's item sits in the channel buffer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = q.Do(context.Background(), "lane-1", func() error {
			return nil
		})
	}()

	time.Sleep(20 * time.Millisecond) // ensure buffer is now full

	// Now try to submit with a cancelled context. The channel is full, so the
	// send blocks and the select must fall through to ctx.Done().
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := q.Do(ctx, "lane-1", func() error {
		t.Error("work should not execute when context is cancelled and channel is full")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}

	close(gate)
	wg.Wait()
}

func TestDo_WhenWorkPanics_ShouldRecoverAndReturnError(t *testing.T) {
	q := NewLaneQueue()

	err := q.Do(context.Background(), "lane-1", func() error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected error when work panics")
	}

	// Lane should still be usable after panic
	err = q.Do(context.Background(), "lane-1", func() error {
		return nil
	})
	if err != nil {
		t.Errorf("lane should be usable after panic, got: %v", err)
	}
}
