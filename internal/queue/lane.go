package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrEmptyLaneID is returned when Do is called with an empty lane ID.
var ErrEmptyLaneID = errors.New("queue: lane ID must not be empty")

// workItem is a unit of work submitted to a lane.
type workItem struct {
	ctx  context.Context
	fn   func() error
	done chan error
}

// lane processes work items sequentially via a single goroutine.
type lane struct {
	work chan workItem
}

// run is the lane's worker loop. It processes items from the work channel in
// FIFO order. If a work function panics, the error is recovered and sent back.
func (l *lane) run() {
	for item := range l.work {
		if item.ctx.Err() != nil {
			item.done <- item.ctx.Err()
			continue
		}
		item.done <- l.safeExec(item.fn)
	}
}

// safeExec runs fn and recovers from panics, converting them to errors.
func (l *lane) safeExec(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("queue: panic: %v", r)
		}
	}()
	return fn()
}

// defaultLaneBufferSize is the capacity of each lane's work channel.
// Tests in this package may override it to exercise full-buffer paths.
var defaultLaneBufferSize = 4096

// LaneQueue serializes work per lane (channel). Different lanes execute
// concurrently, but work within the same lane is processed in FIFO order.
// Each lane has a single worker goroutine backed by a buffered channel.
type LaneQueue struct {
	mu    sync.Mutex
	lanes map[string]*lane
}

// NewLaneQueue creates a new LaneQueue ready for use.
func NewLaneQueue() *LaneQueue {
	return &LaneQueue{
		lanes: make(map[string]*lane),
	}
}

// Do executes fn serially within the given lane. It blocks until the work
// completes or the context is cancelled. Returns the error from fn, or
// ctx.Err() if the context is cancelled while waiting.
func (q *LaneQueue) Do(ctx context.Context, laneID string, fn func() error) error {
	if laneID == "" {
		return ErrEmptyLaneID
	}

	l := q.getOrCreateLane(laneID)
	item := workItem{
		ctx:  ctx,
		fn:   fn,
		done: make(chan error, 1),
	}

	// Submit to the lane's work channel.
	select {
	case l.work <- item:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Wait for the result.
	select {
	case err := <-item.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// getOrCreateLane returns the lane for laneID, creating it (with a worker
// goroutine) if it doesn't exist.
func (q *LaneQueue) getOrCreateLane(laneID string) *lane {
	q.mu.Lock()
	defer q.mu.Unlock()
	if l, ok := q.lanes[laneID]; ok {
		return l
	}
	l := &lane{
		work: make(chan workItem, defaultLaneBufferSize),
	}
	q.lanes[laneID] = l
	go l.run()
	return l
}

// LaneCount returns the number of active lanes.
func (q *LaneQueue) LaneCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.lanes)
}
