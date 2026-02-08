package security

import "errors"

// ErrRunningAsRoot is returned when the process effective user ID is 0 (root).
var ErrRunningAsRoot = errors.New("refusing to run as root: run as a non-root user for security")

// effectiveUIDGetter is set by init in root_unix.go on Unix; otherwise defaultEUID (returns -1).
var effectiveUIDGetter func() int = defaultEUID

// defaultEUID returns -1 (not root); used when not on Unix so the default getter is testable.
func defaultEUID() int { return -1 }

// EffectiveUIDGetter returns the platform effective-UID getter for use with RequireNonRoot.
func EffectiveUIDGetter() func() int {
	return effectiveUIDGetter
}

// RequireNonRoot returns an error if the effective user ID from the given getter is 0 (root).
// Callers pass a getter (e.g. EffectiveUIDGetter() or a test double) so the check is testable.
func RequireNonRoot(euidGetter func() int) error {
	if euidGetter == nil {
		return nil
	}
	if euidGetter() == 0 {
		return ErrRunningAsRoot
	}
	return nil
}
