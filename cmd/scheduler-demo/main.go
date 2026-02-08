// Command scheduler-demo is a standalone manual test for the Scheduler.
// It registers a cron job that fires every minute and prints the system event.
//
// Usage:
//
//	go run ./cmd/scheduler-demo
//
// The program will:
//  1. Register a job that fires every 1 minute
//  2. Register a job that fires every 30 seconds (for faster testing)
//  3. Print a log line each time a job fires
//  4. Run until you press Ctrl+C
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ironclaw/internal/scheduler"
)

// exitFunc is the function used by main to exit; tests can replace it to cover main().
var exitFunc = os.Exit

func main() {
	exitFunc(run(os.Stdout, nil))
}

// run contains the demo logic. If shutdownCh is non-nil, it returns when
// shutdownCh is closed (for tests). Otherwise it blocks on OS signals.
// Returns exit code 0 on success, 1 on error.
func run(w io.Writer, shutdownCh <-chan struct{}) int {
	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))

	engine := newEngine()
	handler := func(ctx context.Context, job scheduler.Job) error {
		logger.Info("SYSTEM EVENT received",
			"job_id", job.ID,
			"job_name", job.Name,
			"prompt", job.Prompt,
			"time", time.Now().Format(time.RFC3339),
		)
		fmt.Fprintf(w, "\n  >>> [System Event: %s] %s\n\n", job.Name, job.Prompt)
		return nil
	}

	sched := scheduler.NewScheduler(engine, handler, scheduler.WithLogger(logger))

	if err := sched.AddJob(scheduler.Job{
		ID:       "every-30s",
		Name:     "Quick Test",
		CronExpr: "@every 30s",
		Prompt:   "This is a 30-second test event. The scheduler is working!",
	}); err != nil {
		logger.Error("failed to add 30s job", "error", err)
		return 1
	}

	if err := sched.AddJob(scheduler.Job{
		ID:       "every-1m",
		Name:     "Minute Check",
		CronExpr: "@every 1m",
		Prompt:   "One minute has passed. Please check system health.",
	}); err != nil {
		logger.Error("failed to add 1m job", "error", err)
		return 1
	}

	sched.Start()
	fmt.Fprintln(w, "Scheduler demo started. Registered jobs:")
	for _, j := range sched.ListJobs() {
		fmt.Fprintf(w, "  - %s (%s): %s [cron: %s]\n", j.ID, j.Name, j.Prompt, j.CronExpr)
	}
	fmt.Fprintln(w, "\nWaiting for cron triggers... (press Ctrl+C to stop)")

	if shutdownCh != nil {
		<-shutdownCh
	} else {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
	}

	fmt.Fprintln(w, "\nShutting down scheduler...")
	sched.Stop()
	fmt.Fprintln(w, "Done.")
	return 0
}

// newEngine is a package-level var so tests can inject a mock.
var newEngine = func() scheduler.CronEngine {
	return scheduler.NewRobfigCronEngine()
}
