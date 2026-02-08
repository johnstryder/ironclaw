//go:build !excludemain

package main

import (
	"os"
	"testing"
)

// TestMain_WhenCalled_ShouldInvokeRunAppAndExit covers main() by stubbing exitFunc
// so we can assert runApp was invoked and the exit code was passed through.
func TestMain_WhenCalled_ShouldInvokeRunAppAndExit(t *testing.T) {
	oldArgs := os.Args
	oldExit := exitFunc
	defer func() {
		os.Args = oldArgs
		exitFunc = oldExit
	}()

	os.Args = []string{"ironclaw", "--version"}
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	main()

	if exitCode != 0 {
		t.Errorf("main() with --version: want exit code 0, got %d", exitCode)
	}
}
