// SPDX-License-Identifier: MIT

package content

import "strings"

// trimURL produces a compact inline form of a URL for the long-bare-URL
// footnote path. Strips the scheme, keeps the host (with port), and
// optionally appends "/" + the first path segment. A trailing "/" is
// preserved only when it terminates the URL. Appends "…" when anything
// was removed.
//
// The trim cuts on '/', '?', '#', '&'. A single oversized opaque path
// segment with no further separators (Google/Facebook tracking URLs
// often look like this) is byte-capped at maxPathSegmentLen so the
// trimmed form stays compact.
//
// Userinfo, IPv6 brackets, and punycode are pass-through — they do
// not appear in real bodies poplar surfaces.
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
		if len(tail) > maxPathSegmentLen+1 {
			return host + tail[:maxPathSegmentLen+1] + "…"
		}
		return host + tail
	}
	segEnd++
	if tail[segEnd] == '/' && segEnd == len(tail)-1 {
		return host + tail
	}
	if segEnd > maxPathSegmentLen+1 {
		return host + tail[:maxPathSegmentLen+1] + "…"
	}
	return host + tail[:segEnd] + "…"
}

// maxPathSegmentLen caps the inlined first path segment (not counting
// the leading "/"). Above this, trimURL elides the rest of the
// segment with "…".
const maxPathSegmentLen = 16

func stripScheme(url string) string {
	colon := strings.IndexByte(url, ':')
	if colon <= 0 {
		return url
	}
	rest := url[colon+1:]
	rest = strings.TrimPrefix(rest, "//")
	return rest
}
