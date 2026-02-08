package security

import (
	"errors"
	"testing"
)

func TestRequireNonRoot_WhenEffectiveUIDIsZero_ShouldReturnErrRunningAsRoot(t *testing.T) {
	err := RequireNonRoot(func() int { return 0 })
	if err == nil {
		t.Fatal("expected error when running as root (euid 0)")
	}
	if !errors.Is(err, ErrRunningAsRoot) {
		t.Errorf("expected ErrRunningAsRoot, got %v", err)
	}
}

func TestRequireNonRoot_WhenEffectiveUIDIsNonZero_ShouldReturnNil(t *testing.T) {
	err := RequireNonRoot(func() int { return 1000 })
	if err != nil {
		t.Errorf("expected nil when not root, got %v", err)
	}
}

func TestRequireNonRoot_WhenEffectiveUIDIsOne_ShouldReturnNil(t *testing.T) {
	err := RequireNonRoot(func() int { return 1 })
	if err != nil {
		t.Errorf("expected nil when euid 1 (e.g. daemon), got %v", err)
	}
}

func TestRequireNonRoot_WhenGetterIsNil_ShouldReturnNil(t *testing.T) {
	err := RequireNonRoot(nil)
	if err != nil {
		t.Errorf("expected nil when getter is nil, got %v", err)
	}
}

func TestDefaultEUID_ReturnsMinusOne(t *testing.T) {
	if got := defaultEUID(); got != -1 {
		t.Errorf("defaultEUID() = %d, want -1", got)
	}
}

func TestEffectiveUIDGetter_ShouldReturnNonNilFunction(t *testing.T) {
	fn := EffectiveUIDGetter()
	if fn == nil {
		t.Fatal("EffectiveUIDGetter should return non-nil function")
	}
	// Call it; on non-Unix it returns -1, on Unix it returns actual euid
	euid := fn()
	_ = euid
	// Use with RequireNonRoot to cover the return path
	if err := RequireNonRoot(fn); err != nil && euid == 0 {
		if !errors.Is(err, ErrRunningAsRoot) {
			t.Errorf("expected ErrRunningAsRoot when euid 0, got %v", err)
		}
	}
}
