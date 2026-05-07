package content

import "strings"

// trimURL produces a compact inline form of a URL for the long-bare-
// URL footnote path. The scheme is dropped, the host (with port) is
// kept, and the first path segment is appended when one exists. A
// trailing "/" survives only at end-of-URL. "…" marks anything cut.
// Cuts fall on '/', '?', '#', '&'. A single oversized opaque segment
// (Google/Facebook tracking URLs often look like this) is byte-capped
// at maxPathSegmentLen. Userinfo, IPv6 brackets, and punycode pass
// through, since they don't appear in real poplar surfaces.
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

// maxPathSegmentLen caps the inlined first path segment, leading "/"
// excluded. Trailing bytes get elided with "…".
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
