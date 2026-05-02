// SPDX-License-Identifier: MIT

package mailimap

import (
	"net"
	"strings"
)

// looksSelfHosted reports whether host appears to be a self-hosted
// or homelab address (RFC 1918 IPv4, IPv6 ULA fc00::/7, .local
// mDNS, or 127.x). Used to gate the "set insecure-tls = true if
// self-signed" hint on TLS errors.
func looksSelfHosted(host string) bool {
	if strings.HasSuffix(host, ".local") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() {
		return true
	}
	return false
}
