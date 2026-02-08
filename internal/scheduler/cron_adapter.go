package scheduler

import (
	"github.com/robfig/cron/v3"
)

// RobfigCronEngine adapts robfig/cron/v3 to the CronEngine interface.
type RobfigCronEngine struct {
	c *cron.Cron
}

// NewRobfigCronEngine creates a new cron engine using robfig/cron/v3.
// The cron instance supports standard 5-field cron expressions by default.
func NewRobfigCronEngine() *RobfigCronEngine {
	return &RobfigCronEngine{
		c: cron.New(),
	}
}

// AddFunc adds a function to be called on the given schedule.
// Returns an entry ID that can be used with Remove.
func (r *RobfigCronEngine) AddFunc(spec string, cmd func()) (int, error) {
	id, err := r.c.AddFunc(spec, cmd)
	return int(id), err
}

// Remove removes a previously registered entry by ID.
func (r *RobfigCronEngine) Remove(id int) {
	r.c.Remove(cron.EntryID(id))
}

// Start begins the cron scheduler in its own goroutine.
func (r *RobfigCronEngine) Start() {
	r.c.Start()
}

// Stop halts the cron scheduler. It does not remove registered entries.
func (r *RobfigCronEngine) Stop() {
	r.c.Stop()
}
