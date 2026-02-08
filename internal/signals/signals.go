//go:build !unix

package signals

import "os"

// ShutdownSignals returns the list of signals that trigger graceful shutdown.
// On non-Unix platforms (e.g. Windows) only Interrupt is available.
func ShutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
