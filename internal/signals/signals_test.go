package signals

import (
	"os"
	"testing"
)

func TestShutdownSignals_ShouldReturnNonEmptySlice(t *testing.T) {
	sigs := ShutdownSignals()
	if len(sigs) == 0 {
		t.Error("ShutdownSignals() should return at least one signal")
	}
}

func TestShutdownSignals_ShouldIncludeInterrupt(t *testing.T) {
	sigs := ShutdownSignals()
	var found bool
	for _, s := range sigs {
		if s == os.Interrupt {
			found = true
			break
		}
	}
	if !found {
		t.Error("ShutdownSignals() should include os.Interrupt for cross-platform graceful shutdown")
	}
}
