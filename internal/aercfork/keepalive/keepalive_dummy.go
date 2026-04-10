// Forked from aerc (git.sr.ht/~rjarry/aerc) — MIT License
//go:build !linux
// +build !linux

package keepalive

func SetTcpKeepaliveProbes(fd, count int) error {
	return nil
}

func SetTcpKeepaliveInterval(fd, interval int) error {
	return nil
}
