//go:build unix

package signals

import (
	"os"
	"syscall"
)

// ShutdownSignals returns the list of signals that trigger graceful shutdown.
// On Unix this includes SIGTERM (e.g. from Docker or process managers).
func ShutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
