//go:build !excludemain

package main

import (
	"os"
	"os/signal"

	"ironclaw/internal/signals"
)

var waitForShutdownSignalStub = false

func init() {
	daemonWaitForShutdown = waitForShutdownSignal
}

func waitForShutdownSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals.ShutdownSignals()...)
	<-ch
}
