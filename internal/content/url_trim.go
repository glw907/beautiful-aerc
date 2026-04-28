// SPDX-License-Identifier: MIT

package content

import "strings"

// trimURL produces a compact inline form of a URL for the long-bare-URL
// footnote path. Strips the scheme, keeps the host (with port), and
// optionally appends "/" + the first path segment. A trailing "/" is
// preserved only when it terminates the URL. Appends "…" when anything
// was removed.
//
// The trim cuts on '/', '?', '#', '&'. Userinfo, IPv6 brackets, and
// punycode are pass-through — they do not appear in real bodies poplar
// surfaces.
func trimURL(url string) string {
	if url == "" {
		return ""
	}
	rest := stripScheme(url)
	hostEnd := strings.IndexAny(rest, "/?#&")
	if hostEnd < 0 {
		return rest
	}
	host := rest[:hostEnd]
	tail := rest[hostEnd:]
	if tail[0] != '/' {
		return host + "…"
	}
	segEnd := strings.IndexAny(tail[1:], "/?#&")
	if segEnd < 0 {
		return host + tail
	}
	segEnd++
	if tail[segEnd] == '/' && segEnd == len(tail)-1 {
		return host + tail
	}
	return host + tail[:segEnd] + "…"
}

func stripScheme(url string) string {
	colon := strings.IndexByte(url, ':')
	if colon <= 0 {
		return url
	}
	rest := url[colon+1:]
	rest = strings.TrimPrefix(rest, "//")
	return rest
}
