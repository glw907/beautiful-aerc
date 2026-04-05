package filter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// linkTextMarker wraps link text so styleFootnotes can identify it precisely.
const linkTextMarker = "\x00LT\x00"

// Regexes for reference-style link processing.
var (
	// reRefDef matches pandoc reference definitions: "  [label]: url", "  [label]:" (empty URL),
	// or "  []: url" (empty label, e.g. after zero-width character stripping).
	reRefDef = regexp.MustCompile(`(?m)^ {0,3}\[([^\]]*)\]:\s*(.*)$`)
	// reRefTitle matches pandoc ref def title continuation: '    "title"'
	reRefTitle = regexp.MustCompile(`^\s+"[^"]*"\s*$`)
	// reRefShortcut matches shortcut reference [text] in body, optionally with [N] or []
	reRefShortcut = regexp.MustCompile(`\[([^\]]+)\](?:\[(\d*)\])?`)
	// reAutolink matches autolinks <https://...>
	reAutolink = regexp.MustCompile(`<(https?://[^>]+)>`)
	// reImageRef matches standalone image refs: ![alt], ![alt][N], ![alt][]
	reImageRef = regexp.MustCompile(`!\[([^\]]*)\](?:\[(\d*)\])?`)
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

// footnoteRef holds a numbered footnote reference for the reference section.
type footnoteRef struct {
	num int
	url string
}

// convertToFootnotes transforms pandoc reference-style links into footnote
// syntax. Returns the transformed body text and a slice of footnote references.
// Self-referencing links (where label looks like a URL) become plain URLs.
// Image references, empty-text links, and empty-URL links are cleaned up.
func convertToFootnotes(text string) (string, []footnoteRef) {
	lines := strings.Split(text, "\n")
	var defs []refDef

	// Scan from bottom to find ref def block. Allow empty URL defs.
	i := len(lines) - 1
	for i >= 0 && strings.TrimSpace(lines[i]) == "" {
		i--
	}
	for i >= 0 {
		// Skip title continuation lines (e.g. '    "Facebook"')
		if reRefTitle.MatchString(lines[i]) {
			i--
			continue
		}
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

	// Convert image-link refs [![alt]][N] to regular text refs [alt][N]
	// so the alt text becomes footnoted link text.
	body = reImageLinkRef.ReplaceAllStringFunc(body, func(m string) string {
		groups := reImageLinkRef.FindStringSubmatch(m)
		alt := strings.TrimSpace(groups[1])
		if alt == "" {
			return ""
		}
		if groups[2] != "" {
			return "[" + alt + "][" + groups[2] + "]"
		}
		return ""
	})
	// Strip standalone image refs ![alt] and ![alt][N] (no wrapping link).
	body = reImageRef.ReplaceAllString(body, "")
	body = reEmptyTextRef.ReplaceAllString(body, "")

	// Build a set of labels still referenced in the body after image stripping.
	// Include trimmed versions to handle pandoc's occasional leading-space labels.
	bodyRefs := make(map[string]bool)
	for _, m := range reRefShortcut.FindAllStringSubmatch(body, -1) {
		if m[2] != "" {
			// [text][N] form: only the numeric label is a ref lookup key.
			bodyRefs[m[2]] = true
		} else {
			// [text] shortcut form: the text is both display and lookup key.
			bodyRefs[m[1]] = true
			if trimmed := strings.TrimSpace(m[1]); trimmed != m[1] {
				bodyRefs[trimmed] = true
			}
		}
	}

	// Categorize defs: only URL defs with surviving body references get footnotes.
	type numberedRef struct {
		num int
		url string
	}
	labelMap := make(map[string]numberedRef)
	stripLabels := make(map[string]bool)
	var refs []footnoteRef
	n := 0
	for _, d := range defs {
		if d.url == "" {
			stripLabels[d.label] = true
			continue
		}
		if !isURL(d.url) {
			continue
		}
		if !bodyRefs[d.label] {
			continue
		}
		if isSelfRef(d.label, d.url) {
			stripLabels[d.label] = true
			continue
		}
		n++
		labelMap[d.label] = numberedRef{num: n, url: d.url}
		refs = append(refs, footnoteRef{num: n, url: d.url})
	}

	// usedRefs tracks which footnote numbers actually appear in the body.
	usedRefs := make(map[int]bool)

	// Replace body references with footnote markers or strip brackets.
	body = reRefShortcut.ReplaceAllStringFunc(body, func(m string) string {
		groups := reRefShortcut.FindStringSubmatch(m)
		label := groups[1]
		numericLabel := groups[2]
		display := strings.TrimSpace(stripEmphasis(label))

		if numericLabel != "" {
			if ref, ok := labelMap[numericLabel]; ok {
				if display == "" {
					return ""
				}
				usedRefs[ref.num] = true
				return linkTextMarker + display + linkTextMarker + fmt.Sprintf("[^%d]", ref.num)
			}
		}

		if ref, ok := labelMap[label]; ok {
			if display == "" {
				return ""
			}
			usedRefs[ref.num] = true
			return linkTextMarker + display + linkTextMarker + fmt.Sprintf("[^%d]", ref.num)
		}
		// Try trimmed label for pandoc's leading-space cases.
		if trimmed := strings.TrimSpace(label); trimmed != label {
			if ref, ok := labelMap[trimmed]; ok {
				if display == "" {
					return ""
				}
				usedRefs[ref.num] = true
				return linkTextMarker + display + linkTextMarker + fmt.Sprintf("[^%d]", ref.num)
			}
		}

		return display
	})

	// Remove consecutive adjacent duplicate footnoted links.
	// e.g. {LT}text{LT}[^1]{LT}text{LT}[^1] → {LT}text{LT}[^1]
	body = deduplicateAdjacentFootnotes(body)

	// Filter refs to only those actually referenced in the body.
	var usedRefList []footnoteRef
	for _, r := range refs {
		if usedRefs[r.num] {
			usedRefList = append(usedRefList, r)
		}
	}
	refs = usedRefList

	// Convert autolinks to plain URLs.
	body = reAutolink.ReplaceAllString(body, "$1")

	// Collapse blank lines left behind by image stripping.
	body = reExcessiveBlank.ReplaceAllString(body, "\n\n")
	body = reLeadingBlank.ReplaceAllString(body, "")

	return body, refs
}

// FootnoteLink holds a labeled URL extracted from footnote conversion output.
type FootnoteLink struct {
	Label string
	URL   string
}

// reLabeledRef matches linkTextMarker-wrapped label followed by [^N].
var reLabeledRef = regexp.MustCompile(`\x00LT\x00([^\x00]+)\x00LT\x00\[\^(\d+)\]`)

// ExtractFootnoteLinks parses the pre-styled body (containing linkTextMarker
// pairs) and footnote refs to produce labeled URL pairs.
func ExtractFootnoteLinks(body string, refs []footnoteRef) []FootnoteLink {
	refMap := make(map[int]string, len(refs))
	for _, r := range refs {
		refMap[r.num] = r.url
	}

	seen := make(map[string]bool)
	var links []FootnoteLink
	for _, m := range reLabeledRef.FindAllStringSubmatch(body, -1) {
		label := strings.TrimSpace(m[1])
		num, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		url, ok := refMap[num]
		if !ok || label == "" {
			continue
		}
		if seen[url] {
			continue
		}
		seen[url] = true
		links = append(links, FootnoteLink{Label: label, URL: url})
	}
	return links
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

// reDupFootnoteUnit matches a single footnoted link: linkTextMarker+text+linkTextMarker+[^N].
var reDupFootnoteUnit = regexp.MustCompile(`\x00LT\x00[^\x00]+\x00LT\x00\[\^\d+\]`)

// styleFootnotes applies ANSI colors to footnote-annotated text.
// Link text (wrapped in linkTextMarker) gets link color, [^N] markers get dim color.
// A separator and colored reference section are appended.
func styleFootnotes(body string, refs []footnoteRef, cols int, colors *footnoteColors) string {
	if len(refs) == 0 {
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

	body = replaceMarkerPairs(body, linkTextMarker, lt, r)
	body = reFootnoteMarker.ReplaceAllString(body, dim+"[^${1}]"+r)

	var sb strings.Builder
	sb.WriteString(body)
	sb.WriteString("\n" + dim + strings.Repeat("─", cols) + r + "\n")
	for _, ref := range refs {
		// Prefix "[^N]: " width — must match the format string below.
		prefixLen := len(fmt.Sprintf("[^%d]: ", ref.num))
		maxURL := cols - prefixLen
		display := ref.url
		if maxURL > 0 && len(display) > maxURL {
			if maxURL > 1 {
				display = display[:maxURL-1] + "…"
			} else {
				display = "…"
			}
		}
		// OSC 8 wraps full URL for click support; display may be truncated.
		sb.WriteString(fmt.Sprintf("%s[^%d]:%s %s\033]8;;%s\033\\%s\033]8;;\033\\%s\n",
			dim, ref.num, r, lu, ref.url, display, r))
	}
	return sb.String()
}

// deduplicateAdjacentFootnotes removes consecutive identical footnoted link
// sequences. Pandoc sometimes emits duplicate anchors for the same link (e.g.
// tracking wrappers), resulting in "Text[^1]Text[^1]" after conversion.
// Only exact back-to-back duplicates are collapsed; non-adjacent occurrences
// are left untouched.
func deduplicateAdjacentFootnotes(body string) string {
	units := reDupFootnoteUnit.FindAllStringIndex(body, -1)
	if len(units) < 2 {
		return body
	}
	var sb strings.Builder
	pos := 0 // current write position in body
	prev := ""
	prevEnd := 0
	for i, loc := range units {
		start, end := loc[0], loc[1]
		unit := body[start:end]
		if i > 0 && start == prevEnd && unit == prev {
			// Adjacent duplicate: advance pos past it without writing.
			pos = end
			prevEnd = end
			continue
		}
		// Write any text between pos and this unit's start.
		sb.WriteString(body[pos:start])
		sb.WriteString(unit)
		pos = end
		prev = unit
		prevEnd = end
	}
	// Write any trailing text after the last unit.
	sb.WriteString(body[pos:])
	return sb.String()
}

// isSelfRef returns true when a reference label is effectively its own URL.
// Handles scheme-less labels like "rmd.me/path" matching "http://rmd.me/path".
func isSelfRef(label, url string) bool {
	urlStripped := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	labelStripped := strings.TrimPrefix(strings.TrimPrefix(label, "https://"), "http://")
	return labelStripped == urlStripped
}

// isURL returns true if s looks like a URL or mailto link.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "mailto:")
}

// stripEmphasis removes markdown emphasis markers from link display text.
// Pandoc wraps linked text in *...* or **...** when the HTML had <em>/<strong>.
// Link color already provides visual distinction, so emphasis is redundant.
func stripEmphasis(s string) string {
	s = strings.TrimPrefix(s, "**")
	s = strings.TrimSuffix(s, "**")
	s = strings.TrimPrefix(s, "*")
	s = strings.TrimSuffix(s, "*")
	return s
}
