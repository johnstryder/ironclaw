//go:build unix

package security

import "syscall"

func init() {
	effectiveUIDGetter = syscall.Geteuid
}
