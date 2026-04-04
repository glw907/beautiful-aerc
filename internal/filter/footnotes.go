package filter

import (
	"fmt"
	"regexp"
	"strings"
)

// Regexes for reference-style link processing.
var (
	// reRefDef matches pandoc reference definitions: "  [label]: url"
	reRefDef = regexp.MustCompile(`(?m)^ {0,3}\[([^\]]+)\]:\s+(.+)$`)
	// reRefShortcut matches shortcut reference [text] in body (not followed by : which is a def)
	reRefShortcut = regexp.MustCompile(`\[([^\]]+)\](?:\[(\d+)\])?`)
	// reAutolink matches autolinks <https://...>
	reAutolink = regexp.MustCompile(`<(https?://[^>]+)>`)
)

// refDef holds a parsed reference definition.
type refDef struct {
	label string
	url   string
}

// convertToFootnotes transforms pandoc reference-style links into footnote
// syntax. Returns the transformed body text and a slice of "[^N]: url" strings.
// Self-referencing links (where label looks like a URL) become plain URLs.
func convertToFootnotes(text string) (string, []string) {
	// Split into body and reference definitions.
	// Ref defs are indented lines at the end matching [label]: url.
	lines := strings.Split(text, "\n")
	var defs []refDef

	// Scan from bottom to find ref def block.
	i := len(lines) - 1
	for i >= 0 && strings.TrimSpace(lines[i]) == "" {
		i--
	}
	for i >= 0 {
		groups := reRefDef.FindStringSubmatch(lines[i])
		if groups == nil {
			break
		}
		defs = append(defs, refDef{label: groups[1], url: groups[2]})
		i--
	}
	bodyLines := lines[:i+1]

	// Reverse defs (collected bottom-up).
	for a, b := 0, len(defs)-1; a < b; a, b = a+1, b-1 {
		defs[a], defs[b] = defs[b], defs[a]
	}

	body := strings.Join(bodyLines, "\n")

	// Build label-to-def mapping and assign footnote numbers.
	// Self-referencing links (label is a URL) are excluded.
	type numberedRef struct {
		num int
		url string
	}
	labelMap := make(map[string]numberedRef)
	var refs []string
	n := 0
	for _, d := range defs {
		if isSelfRef(d.label, d.url) {
			continue
		}
		n++
		labelMap[d.label] = numberedRef{num: n, url: d.url}
		refs = append(refs, fmt.Sprintf("[^%d]: %s", n, d.url))
	}

	// Replace body references with footnote markers.
	body = reRefShortcut.ReplaceAllStringFunc(body, func(m string) string {
		groups := reRefShortcut.FindStringSubmatch(m)
		if groups == nil {
			return m
		}
		label := groups[1]
		numericLabel := groups[2] // non-empty for [text][1] form

		// For [text][N] form, the numeric label is the explicit reference; prefer it.
		if numericLabel != "" {
			if ref, ok := labelMap[numericLabel]; ok {
				return label + fmt.Sprintf("[^%d]", ref.num)
			}
		}

		// For plain [text] form, look up by label.
		if ref, ok := labelMap[label]; ok {
			return label + fmt.Sprintf("[^%d]", ref.num)
		}

		// Self-referencing: strip brackets.
		if isURL(label) {
			return label
		}

		return m
	})

	// Convert autolinks to plain URLs.
	body = reAutolink.ReplaceAllString(body, "$1")

	return body, refs
}

// isSelfRef returns true when a reference label is effectively its own URL.
func isSelfRef(label, url string) bool {
	return strings.TrimPrefix(strings.TrimPrefix(label, "https://"), "http://") ==
		strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
}

// isURL returns true if s looks like a URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
