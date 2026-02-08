//go:build excludemain

package main

// waitForShutdownSignalStub is true when building with -tags=excludemain (coverage build).
var waitForShutdownSignalStub = true

func init() {
	daemonWaitForShutdown = waitForShutdownSignal
}

// waitForShutdownSignal is a no-op when building with -tags=excludemain for coverage.
func waitForShutdownSignal() {
	_ = 0
}
