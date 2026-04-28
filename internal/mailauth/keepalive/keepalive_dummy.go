//go:build !linux

// SPDX-License-Identifier: MIT
// Vendored from git.sr.ht/~rjarry/aerc — MIT License.
// Modifications: package path moved to internal/mailauth/keepalive;
// package name and all code are otherwise byte-identical to the source.
package keepalive

// SetTcpKeepaliveProbes is a no-op on non-Linux platforms.
func SetTcpKeepaliveProbes(fd, count int) error {
	return nil
}

// SetTcpKeepaliveInterval is a no-op on non-Linux platforms.
func SetTcpKeepaliveInterval(fd, interval int) error {
	return nil
}
