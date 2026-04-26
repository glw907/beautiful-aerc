//go:build linux

// Vendored from git.sr.ht/~rjarry/aerc — MIT License.
// Modifications: package path moved to internal/mailauth/keepalive;
// package name and all code are otherwise byte-identical to the source.
package keepalive

import (
	"syscall"
)

// SetTcpKeepaliveProbes sets the number of keepalive probes on the given fd.
func SetTcpKeepaliveProbes(fd, count int) error {
	return syscall.SetsockoptInt(
		fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPCNT, count)
}

// SetTcpKeepaliveInterval sets the keepalive interval on the given fd.
func SetTcpKeepaliveInterval(fd, interval int) error {
	return syscall.SetsockoptInt(
		fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPINTVL, interval)
}
