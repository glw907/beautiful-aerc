package content

import (
	"net/textproto"
	"strings"
)

// Unsubscribe carries the parsed result of RFC 2369 List-Unsubscribe
// and RFC 8058 List-Unsubscribe-Post headers. Available reports
// whether at least one actionable form is present.
type Unsubscribe struct {
	// OneClick is the https URL eligible for an RFC 8058 one-click
	// POST. Set only when List-Unsubscribe-Post advertises
	// List-Unsubscribe=One-Click and at least one https URL is
	// present in List-Unsubscribe. Empty otherwise.
	OneClick string

	// Mailto is the first mailto: URL from List-Unsubscribe, "" when
	// none is present.
	Mailto string

	// HTTP is the first http(s) URL from List-Unsubscribe when not
	// promoted to OneClick. Used as the URLOpener fallback.
	HTTP string
}

// Available reports whether any actionable form is set.
func (u Unsubscribe) Available() bool {
	return u.OneClick != "" || u.Mailto != "" || u.HTTP != ""
}

// ParseListUnsubscribe parses RFC 2369 List-Unsubscribe and RFC 8058
// List-Unsubscribe-Post headers and returns the resolved value.
func ParseListUnsubscribe(h textproto.MIMEHeader) Unsubscribe {
	raw := h.Get("List-Unsubscribe")
	if raw == "" {
		return Unsubscribe{}
	}

	var mailto, httpURL string
	httpsURLs := make([]string, 0, 2)
	for _, entry := range splitListUnsubscribe(raw) {
		switch {
		case strings.HasPrefix(entry, "mailto:"):
			if mailto == "" {
				mailto = entry
			}
		case strings.HasPrefix(entry, "https://"):
			httpsURLs = append(httpsURLs, entry)
			if httpURL == "" {
				httpURL = entry
			}
		case strings.HasPrefix(entry, "http://"):
			if httpURL == "" {
				httpURL = entry
			}
		}
	}

	u := Unsubscribe{Mailto: mailto, HTTP: httpURL}

	if len(httpsURLs) > 0 && hasOneClickPost(h.Get("List-Unsubscribe-Post")) {
		u.OneClick = httpsURLs[0]
		// Promotion consumes the URL from HTTP so callers don't double-route.
		if u.HTTP == u.OneClick {
			u.HTTP = ""
		}
	}

	return u
}

// splitListUnsubscribe tokenizes a comma-separated List-Unsubscribe
// value into individual URIs, stripping enclosing angle brackets and
// surrounding whitespace. Tolerates missing brackets.
func splitListUnsubscribe(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "<")
		p = strings.TrimSuffix(p, ">")
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// hasOneClickPost reports whether the List-Unsubscribe-Post header
// value advertises RFC 8058 one-click. The key is matched
// case-insensitively per RFC 8058 §3; the value comparison is
// likewise case-insensitive.
func hasOneClickPost(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	eq := strings.IndexByte(v, '=')
	if eq < 0 {
		return false
	}
	key := strings.TrimSpace(v[:eq])
	val := strings.TrimSpace(v[eq+1:])
	return strings.EqualFold(key, "List-Unsubscribe") &&
		strings.EqualFold(val, "One-Click")
}
