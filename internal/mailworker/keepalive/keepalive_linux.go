// Forked from aerc (git.sr.ht/~rjarry/aerc) — MIT License
//go:build linux
// +build linux

package keepalive

import (
	"syscall"
)

func SetTcpKeepaliveProbes(fd, count int) error {
	return syscall.SetsockoptInt(
		fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPCNT, count)
}

func SetTcpKeepaliveInterval(fd, interval int) error {
	return syscall.SetsockoptInt(
		fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPINTVL, interval)
}
