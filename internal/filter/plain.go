// SPDX-License-Identifier: MIT

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

// CleanPlain normalizes plain text email content to markdown.
// Detects HTML-in-plain-text and routes through CleanHTML if found.
func CleanPlain(text string) string {
	// Strip carriage returns from CRLF line endings (RFC 2822 7bit/8bit bodies)
	// before any other processing, including the HTML detection branch.
	text = strings.ReplaceAll(text, "\r", "")
	if detectHTML(text) {
		return CleanHTML(text)
	}
	text = html.UnescapeString(text)
	// Normalize tab-indented list items to prevent 8-space expansion.
	text = reTabListItem.ReplaceAllString(text, "$1")
	return text
}
