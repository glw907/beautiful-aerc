package filter

import (
	"fmt"
	"regexp"
	"strings"
)

// linkTextMarker wraps link text so styleFootnotes can identify it precisely.
const linkTextMarker = "\x00LT\x00"

// Regexes for reference-style link processing.
var (
	// reRefDef matches pandoc reference definitions: "  [label]: url" or "  [label]:" (empty URL)
	reRefDef = regexp.MustCompile(`(?m)^ {0,3}\[([^\]]+)\]:\s*(.*)$`)
	// reRefShortcut matches shortcut reference [text] in body, optionally with [N] or []
	reRefShortcut = regexp.MustCompile(`\[([^\]]+)\](?:\[(\d*)\])?`)
	// reAutolink matches autolinks <https://...>
	reAutolink = regexp.MustCompile(`<(https?://[^>]+)>`)
	// reImageRef matches image-link references like [![alt]][ref] in reference-style output
	reImageRef = regexp.MustCompile(`!\[([^\]]*)\]`)
	// reImageLinkRef matches image wrapped in link: [![alt]][ref]
	reImageLinkRef = regexp.MustCompile(`\[!\[([^\]]*)\]\](?:\[(\d+)\])?`)
	// reEmptyTextRef matches empty-text reference links: [][ref]
	reEmptyTextRef = regexp.MustCompile(`\[\]\[(\d+)\]`)
)

// refDef holds a parsed reference definition.
type refDef struct {
	label string
	url   string
}

// convertToFootnotes transforms pandoc reference-style links into footnote
// syntax. Returns the transformed body text and a slice of "[^N]: url" strings.
// Self-referencing links (where label looks like a URL) become plain URLs.
// Image references, empty-text links, and empty-URL links are cleaned up.
func convertToFootnotes(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	var defs []refDef

	// Scan from bottom to find ref def block. Allow empty URL defs.
	i := len(lines) - 1
	for i >= 0 && strings.TrimSpace(lines[i]) == "" {
		i--
	}
	for i >= 0 {
		groups := reRefDef.FindStringSubmatch(lines[i])
		if groups == nil {
			break
		}
		defs = append(defs, refDef{label: groups[1], url: strings.TrimSpace(groups[2])})
		i--
	}
	bodyLines := lines[:i+1]

	// Reverse defs (collected bottom-up).
	for a, b := 0, len(defs)-1; a < b; a, b = a+1, b-1 {
		defs[a], defs[b] = defs[b], defs[a]
	}

	body := strings.Join(bodyLines, "\n")

	// Categorize defs: URL defs get footnote numbers; others are skipped.
	// Track which labels to strip (empty URL, image path, self-ref).
	type numberedRef struct {
		num int
		url string
	}
	labelMap := make(map[string]numberedRef)
	stripLabels := make(map[string]bool) // labels to strip brackets from in body
	var refs []string
	n := 0
	for _, d := range defs {
		if d.url == "" {
			// Empty URL def: strip brackets from body text.
			stripLabels[d.label] = true
			continue
		}
		if !isURL(d.url) {
			// Non-URL def (e.g., image path): will clean up in body.
			continue
		}
		if isSelfRef(d.label, d.url) {
			stripLabels[d.label] = true
			continue
		}
		n++
		labelMap[d.label] = numberedRef{num: n, url: d.url}
		refs = append(refs, fmt.Sprintf("[^%d]: %s", n, d.url))
	}

	// Remove image-link references [![alt]][ref] from body.
	body = reImageLinkRef.ReplaceAllString(body, "")

	// Remove standalone image references ![alt] from body.
	body = reImageRef.ReplaceAllString(body, "")

	// Remove empty-text reference links [][ref] from body.
	body = reEmptyTextRef.ReplaceAllString(body, "")

	// Replace body references with footnote markers or strip brackets.
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
				return linkTextMarker + label + linkTextMarker + fmt.Sprintf("[^%d]", ref.num)
			}
		}

		// For plain [text] form, look up by label.
		if ref, ok := labelMap[label]; ok {
			return linkTextMarker + label + linkTextMarker + fmt.Sprintf("[^%d]", ref.num)
		}

		// Strip brackets for empty-URL, self-ref, or URL-looking labels.
		if stripLabels[label] || isURL(label) {
			return label
		}

		return m
	})

	// Convert autolinks to plain URLs.
	body = reAutolink.ReplaceAllString(body, "$1")

	return body, refs
}

// footnoteColors holds ANSI parameter strings for footnote styling.
type footnoteColors struct {
	LinkText string // body link text color
	Dim      string // footnote markers and ref labels
	LinkURL  string // reference section URLs
	Reset    string
}

// reFootnoteMarker matches "[^N]" markers in body text for dimming.
var reFootnoteMarker = regexp.MustCompile(`\[\^(\d+)\]`)

// styleFootnotes applies ANSI colors to footnote-annotated text.
// Link text (wrapped in linkTextMarker) gets link color, [^N] markers get dim color.
// A separator and colored reference section are appended.
func styleFootnotes(body string, refs []string, cols int, colors *footnoteColors) string {
	if len(refs) == 0 {
		// Strip any stray markers if present.
		return strings.ReplaceAll(body, linkTextMarker, "")
	}

	lt := ""
	dim := ""
	lu := ""
	r := ""
	if colors.LinkText != "" {
		lt = "\033[" + colors.LinkText + "m"
	}
	if colors.Dim != "" {
		dim = "\033[" + colors.Dim + "m"
	}
	if colors.LinkURL != "" {
		lu = "\033[" + colors.LinkURL + "m"
	}
	if colors.Reset != "" {
		r = "\033[" + colors.Reset + "m"
	}

	// Color link text: replace marker pairs with ANSI link text color.
	body = replaceLinkTextMarkers(body, lt, r)

	// Dim footnote markers [^N].
	body = reFootnoteMarker.ReplaceAllString(body, dim+"[^${1}]"+r)

	// Build reference section.
	var sb strings.Builder
	sb.WriteString(body)
	sb.WriteString("\n" + dim + strings.Repeat("─", cols) + r + "\n")
	for _, ref := range refs {
		// Split "[^N]: url" into label and URL parts.
		colonIdx := strings.Index(ref, ": ")
		if colonIdx < 0 {
			sb.WriteString(ref + "\n")
			continue
		}
		label := ref[:colonIdx]
		url := ref[colonIdx+2:]
		sb.WriteString(dim + label + ":" + r + " " + lu + url + r + "\n")
	}
	return sb.String()
}

// replaceLinkTextMarkers converts linkTextMarker pairs to ANSI color sequences.
// First marker emits lt (link text color), second emits r (reset).
func replaceLinkTextMarkers(text, lt, r string) string {
	parts := strings.Split(text, linkTextMarker)
	var sb strings.Builder
	for i, part := range parts {
		if i == 0 {
			sb.WriteString(part)
			continue
		}
		if i%2 == 1 {
			sb.WriteString(lt)
		} else {
			sb.WriteString(r)
		}
		sb.WriteString(part)
	}
	return sb.String()
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
