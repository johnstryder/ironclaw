package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// Job represents a scheduled task that injects a prompt into the brain.
type Job struct {
	ID       string // Unique identifier for the job
	Name     string // Human-readable name (optional)
	CronExpr string // Cron expression (e.g. "*/5 * * * *")
	Prompt   string // Prompt to inject as a system event
}

// EventHandler is called when a scheduled job fires. The handler receives
// the context and the job definition, and should inject the prompt into
// the brain as a system event.
type EventHandler func(ctx context.Context, job Job) error

// CronEngine abstracts the cron scheduler for testability.
// The real implementation wraps robfig/cron/v3.
type CronEngine interface {
	AddFunc(spec string, cmd func()) (int, error)
	Remove(id int)
	Start()
	Stop()
}

// Option is a functional option for configuring a Scheduler.
type Option func(*Scheduler)

// WithLogger sets a structured logger for the Scheduler. If l is nil it is
// ignored and the default slog logger is used.
func WithLogger(l *slog.Logger) Option {
	return func(s *Scheduler) {
		if l != nil {
			s.logger = l
		}
	}
}

// Sentinel errors for validation.
var (
	ErrEmptyJobID   = errors.New("scheduler: job ID must not be empty")
	ErrEmptyCron    = errors.New("scheduler: cron expression must not be empty")
	ErrEmptyPrompt  = errors.New("scheduler: prompt must not be empty")
	ErrDuplicateJob = errors.New("scheduler: job with this ID already exists")
)

// jobEntry tracks a registered job and its cron entry ID.
type jobEntry struct {
	job     Job
	entryID int
}

// Scheduler manages cron-based scheduled jobs. When a job fires, it calls
// the EventHandler with the job's prompt, allowing the agent to act on it.
type Scheduler struct {
	engine  CronEngine
	handler EventHandler
	logger  *slog.Logger
	mu      sync.RWMutex
	jobs    map[string]jobEntry
}

// NewScheduler creates a new Scheduler. Both engine and handler must not be nil.
func NewScheduler(engine CronEngine, handler EventHandler, opts ...Option) *Scheduler {
	if engine == nil {
		panic("scheduler: engine must not be nil")
	}
	if handler == nil {
		panic("scheduler: handler must not be nil")
	}
	s := &Scheduler{
		engine:  engine,
		handler: handler,
		jobs:    make(map[string]jobEntry),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// log returns the Scheduler's logger, falling back to the default slog logger.
func (s *Scheduler) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// AddJob registers a new scheduled job. Returns an error if the job fails
// validation or if a job with the same ID already exists.
func (s *Scheduler) AddJob(job Job) error {
	if job.ID == "" {
		return ErrEmptyJobID
	}
	if job.CronExpr == "" {
		return ErrEmptyCron
	}
	if job.Prompt == "" {
		return ErrEmptyPrompt
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateJob, job.ID)
	}

	// Capture job for the closure.
	capturedJob := job
	entryID, err := s.engine.AddFunc(job.CronExpr, func() {
		s.log().Info("job fired",
			"job_id", capturedJob.ID,
			"job_name", capturedJob.Name,
			"cron_expr", capturedJob.CronExpr,
		)
		if handlerErr := s.handler(context.Background(), capturedJob); handlerErr != nil {
			s.log().Warn("job handler failed",
				"job_id", capturedJob.ID,
				"error", handlerErr,
			)
		}
	})
	if err != nil {
		return fmt.Errorf("scheduler: failed to register cron job %q: %w", job.ID, err)
	}

	s.jobs[job.ID] = jobEntry{job: job, entryID: entryID}
	s.log().Info("job registered",
		"job_id", job.ID,
		"job_name", job.Name,
		"cron_expr", job.CronExpr,
	)
	return nil
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.engine.Start()
}

// Stop halts the cron scheduler.
func (s *Scheduler) Stop() {
	s.engine.Stop()
}

// RemoveJob unregisters a scheduled job by ID. Returns an error if the
// job ID is empty or the job does not exist.
func (s *Scheduler) RemoveJob(id string) error {
	if id == "" {
		return ErrEmptyJobID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("scheduler: job %q not found", id)
	}

	s.engine.Remove(entry.entryID)
	delete(s.jobs, id)
	s.log().Info("job removed", "job_id", id)
	return nil
}

// ListJobs returns a copy of all registered jobs. The returned slice is
// never nil (empty slice when no jobs are registered).
func (s *Scheduler) ListJobs() []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]Job, 0, len(s.jobs))
	for _, entry := range s.jobs {
		jobs = append(jobs, entry.job)
	}
	return jobs
}

// GetJob returns the job with the given ID, or false if not found.
func (s *Scheduler) GetJob(id string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.jobs[id]
	if !ok {
		return Job{}, false
	}
	return entry.job, true
}
