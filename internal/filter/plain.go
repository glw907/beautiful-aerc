package filter

import (
	"html"
	"regexp"
	"strings"
)

var (
	htmlTagRe     = regexp.MustCompile(`(?i)<(div|html|body|table|span|br|p[ />])`)
	reTabListItem = regexp.MustCompile(`(?m)^\t+([-*+] )`)
)

func detectHTML(text string) bool {
	lines := strings.SplitN(text, "\n", 51)
	if len(lines) > 50 {
		lines = lines[:50]
	}
	sample := strings.Join(lines, "\n")
	return htmlTagRe.MatchString(sample)
}

// CleanPlain normalizes plain-text email content to markdown,
// rerouting through CleanHTML when HTML-in-plain-text is detected.
func CleanPlain(text string) string {
	// Strip CRs before HTML detection so the sniff sees a clean buffer.
	text = strings.ReplaceAll(text, "\r", "")
	if detectHTML(text) {
		return CleanHTML(text)
	}
	text = html.UnescapeString(text)
	// Drop tab indentation on list items. Otherwise the renderer
	// treats it as 8-space indent and the list visually disintegrates.
	text = reTabListItem.ReplaceAllString(text, "$1")
	return text
}
