// SPDX-License-Identifier: MIT

package filter

import (
	gohtml "html"
	"regexp"
	"strings"
)

// Package-level compiled regexes.
var (
	reMozClass      = regexp.MustCompile(` class="moz-[^"]*"`)
	reMozDataAttr   = regexp.MustCompile(` data-moz-do-not-send="[^"]*"`)
	reMozAttr       = regexp.MustCompile(` moz-do-not-send="[^"]*"`)
	reHiddenDivOpen = regexp.MustCompile(`(?i)<div[^>]+style="[^"]*display:\s*none[^"]*"[^>]*>`)
	reZeroImg       = regexp.MustCompile(`(?i)<img[^>]*(?:width:\s*0|height:\s*0|width="0"|height="0")[^>]*/?>`)
	// Post-conversion whitespace normalization: strip invisible filler
	// characters that email senders embed for preheader text, collapse
	// excessive blank lines, and strip leading blanks.
	reNBSP          = regexp.MustCompile(`[\x{a0}\x{2000}-\x{200a}]+`)
	reZeroWidth     = regexp.MustCompile(`[\x{ad}\x{34f}\x{180e}\x{200b}-\x{200d}\x{2060}-\x{2064}\x{feff}]`)
	reBlankSpaces   = regexp.MustCompile(`(?m)^ +$`)
	reExcessiveBlanks = regexp.MustCompile(`\n{3,}`)
	reLeadingBlanks = regexp.MustCompile(`\A\n+`)

	// Markdown link patterns.
	reMdLink      = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reEmptyMdLink = regexp.MustCompile(`\[\]\([^)]+\)`)

	// Ordered list item: "1.", "2)", etc.
	reOrderedList = regexp.MustCompile(`^\d+[.)]`)

	// Paren-style list items: "1)" or "1\)" → "1."
	// Handles both top-level and blockquote-prefixed lines.
	reParenList = regexp.MustCompile(`(?m)^((?:>\s?)*)(\d+)\\?\)\s`)
)

// prepareHTML cleans the raw HTML before conversion: strips Mozilla-specific
// attributes, hidden elements (display:none divs), and zero-size tracking images.
func prepareHTML(body string) string {
	body = reMozClass.ReplaceAllString(body, "")
	body = reMozDataAttr.ReplaceAllString(body, "")
	body = reMozAttr.ReplaceAllString(body, "")
	body = stripHiddenElements(body)
	body = reZeroImg.ReplaceAllString(body, "")
	return body
}

// stripHiddenElements removes <div> elements whose inline style contains
// display:none. Responsive HTML emails (Apple receipts, etc.) embed a hidden
// duplicate of the body in such a div, often containing many nested <div>s.
// A simple non-greedy regex closes at the first inner </div>, so we use a
// nesting-aware approach: find each hidden-div open tag, then walk forward
// counting <div> opens and </div> closes until depth reaches zero.
func stripHiddenElements(body string) string {
	for {
		loc := reHiddenDivOpen.FindStringIndex(body)
		if loc == nil {
			break
		}
		start := loc[0]
		// Walk from end of opening tag, tracking nesting depth.
		// Depth starts at 1 (we have already seen the opening <div>).
		rest := body[loc[1]:]
		depth := 1
		pos := 0
		for depth > 0 && pos < len(rest) {
			nextOpen := strings.Index(rest[pos:], "<div")
			nextClose := strings.Index(rest[pos:], "</div>")
			if nextClose < 0 {
				// No closing tag found; remove to end of string.
				pos = len(rest)
				break
			}
			if nextOpen >= 0 && nextOpen < nextClose {
				depth++
				pos += nextOpen + len("<div")
			} else {
				depth--
				pos += nextClose + len("</div>")
			}
		}
		end := loc[1] + pos
		if end > len(body) {
			end = len(body)
		}
		body = body[:start] + body[end:]
	}
	return body
}

// normalizeWhitespace collapses non-breaking spaces, zero-width filler
// characters (preheader padding), blank lines with only spaces, excessive
// blank lines, and leading blank lines.
func normalizeWhitespace(text string) string {
	text = reNBSP.ReplaceAllString(text, " ")
	text = reZeroWidth.ReplaceAllString(text, "")
	text = reBlankSpaces.ReplaceAllString(text, "")
	text = reExcessiveBlanks.ReplaceAllString(text, "\n\n")
	text = reLeadingBlanks.ReplaceAllString(text, "")
	return text
}

// normalizeListMarkers converts paren-style list items (1), 2), 1\))
// to standard markdown (1., 2.) so the block parser recognizes them.
func normalizeListMarkers(text string) string {
	return reParenList.ReplaceAllString(text, "${1}${2}. ")
}

// unflattenQuotes detects email attribution lines followed by inline >
// markers and reconstructs them as proper markdown blockquotes. Outlook
// mobile (and some other clients) flatten quoted reply text into a single
// <p> with literal &gt; characters where line breaks originally were.
// The html-to-markdown library preserves these as "&gt;" entities.
//
// Input:  "Person wrote: &gt; line1 &gt; &gt; line2 &gt; line3"
// Output: "Person wrote:\n\n> line1\n>\n> line2 line3"
func unflattenQuotes(text string) string {
	blocks := strings.Split(text, "\n\n")
	for i, block := range blocks {
		// Match "wrote:" followed by either "&gt;" or ">" quote markers.
		wroteIdx, sep := findQuoteStart(block)
		if wroteIdx < 0 {
			continue
		}

		attribution := block[:wroteIdx+len("wrote:")]
		rest := strings.TrimSpace(block[wroteIdx+len("wrote:"):])
		rest = strings.TrimPrefix(rest, sep+" ")

		// Split on " <sep> " to find original line boundaries. Parts
		// starting with "<sep> " indicate a paragraph break (from the
		// original "> \n> " that became " > > " when flattened).
		splitOn := " " + sep + " "
		parts := strings.Split(rest, splitOn)

		var paragraphs [][]string
		current := []string{parts[0]}

		for j := 1; j < len(parts); j++ {
			part := parts[j]
			if strings.HasPrefix(part, sep+" ") {
				if len(current) > 0 {
					paragraphs = append(paragraphs, current)
				}
				current = []string{strings.TrimPrefix(part, sep+" ")}
			} else {
				current = append(current, part)
			}
		}
		if len(current) > 0 {
			paragraphs = append(paragraphs, current)
		}

		var quoteLines []string
		for j, para := range paragraphs {
			quoteLines = append(quoteLines, "> "+strings.Join(para, " "))
			if j < len(paragraphs)-1 {
				quoteLines = append(quoteLines, ">")
			}
		}

		blocks[i] = attribution + "\n\n" + strings.Join(quoteLines, "\n")
	}
	return strings.Join(blocks, "\n\n")
}

// findQuoteStart locates "wrote:" followed by inline quote markers in a
// block. Returns the index of "wrote:" and the separator string ("&gt;"
// or ">"), or -1 if not found.
func findQuoteStart(block string) (int, string) {
	if idx := strings.Index(block, "wrote: &gt; "); idx >= 0 {
		return idx, "&gt;"
	}
	if idx := strings.Index(block, "wrote: > "); idx >= 0 {
		return idx, ">"
	}
	return -1, ""
}

// blockKey returns a normalized form of a block for deduplication.
// Markdown link URLs are stripped so blocks differing only in URL
// (e.g. tracking variants of the same image-link) compare equal.
func blockKey(block string) string {
	return reMdLink.ReplaceAllString(block, "[$1](#)")
}

// deduplicateBlocks collapses consecutive blocks with the same visible
// text into a single occurrence. Comparison ignores markdown link URLs
// so image-links with different tracking URLs but the same alt text are
// treated as duplicates.
func deduplicateBlocks(text string) string {
	blocks := strings.Split(text, "\n\n")
	var out []string
	prevKey := ""
	for _, block := range blocks {
		if block == "" {
			continue
		}
		key := blockKey(block)
		if key == prevKey {
			continue
		}
		prevKey = key
		out = append(out, block)
	}
	return strings.Join(out, "\n\n")
}

// mergeBlockRuns groups consecutive blocks matching pred into runs and
// joins runs of minRun or more using sep. Shorter runs pass through.
func mergeBlockRuns(text string, pred func(string) bool, minRun int, sep string) string {
	blocks := strings.Split(text, "\n\n")
	var out []string
	var run []string

	flush := func() {
		if len(run) >= minRun {
			out = append(out, strings.Join(run, sep))
		} else {
			out = append(out, run...)
		}
		run = nil
	}

	for _, block := range blocks {
		if pred(block) {
			run = append(run, block)
		} else {
			if len(run) > 0 {
				flush()
			}
			out = append(out, block)
		}
	}
	if len(run) > 0 {
		flush()
	}
	return strings.Join(out, "\n\n")
}

// collapseShortBlocks joins runs of 3+ consecutive short, plain-text
// blocks into a single line separated by " · ". This handles content
// from flattened table cells that was never meant to be read vertically
// — navigation bars, step trackers, tag lists, etc.
func collapseShortBlocks(text string) string {
	return mergeBlockRuns(text, isShortPlain, 3, " · ")
}

// isShortPlain returns true if the block is a single line of plain text
// under 25 characters with no markdown syntax. Blocks that look like
// sentences (ending with punctuation) are excluded — those are real
// content, not table cell fragments.
func isShortPlain(block string) bool {
	if strings.Contains(block, "\n") || len(block) > 25 || len(block) == 0 {
		return false
	}
	// Reject blocks with markdown syntax: links, headings, bold,
	// italic, list markers, blockquotes.
	if block[0] == '#' || block[0] == '>' || block[0] == '-' ||
		block[0] == '*' || block[0] == '+' {
		return false
	}
	if strings.Contains(block, "](") || strings.Contains(block, "**") {
		return false
	}
	if reOrderedList.MatchString(block) {
		return false
	}
	// Sentences end with punctuation — real content, not cell fragments.
	last := block[len(block)-1]
	if last == '.' || last == '!' || last == '?' || last == ':' || last == ';' {
		return false
	}
	return true
}

// compactLineRuns joins runs of 3+ consecutive single-line short blocks
// into a single block using markdown hard breaks (two trailing spaces).
// This handles email signatures and contact blocks where each line is a
// separate <p> in the HTML source but should render as tight lines.
func compactLineRuns(text string) string {
	return mergeBlockRuns(text, isCompactLine, 3, "  \n")
}

// isCompactLine returns true for a single-line block whose visible text
// is under 80 characters and that is not a markdown block element or a
// sentence ending with punctuation. Visible length strips markdown link
// syntax since [text](url) renders as just "text".
func isCompactLine(block string) bool {
	if strings.Contains(block, "\n") || len(block) == 0 {
		return false
	}
	visible := reMdLink.ReplaceAllString(block, "$1")
	if len(visible) > 80 {
		return false
	}
	if block[0] == '#' || block[0] == '>' || block[0] == '|' ||
		block[0] == '-' || block[0] == '*' || block[0] == '+' ||
		strings.HasPrefix(block, "```") ||
		strings.HasPrefix(block, "---") ||
		strings.HasPrefix(block, "===") ||
		reOrderedList.MatchString(block) {
		return false
	}
	last := block[len(block)-1]
	if last == '.' || last == '!' || last == '?' || last == ':' || last == ';' || last == ',' {
		return false
	}
	return true
}

// stripEmptyLinks removes markdown links with empty text like [](url).
func stripEmptyLinks(text string) string {
	return reEmptyMdLink.ReplaceAllString(text, "")
}

// CleanHTML converts raw HTML email content to normalized markdown.
// This is the content pipeline only — no rendering or styling.
func CleanHTML(html string) string {
	html = prepareHTML(html)
	md, err := convertHTML(html)
	if err != nil {
		return html
	}
	md = normalizeWhitespace(md)
	md = normalizeListMarkers(md)
	md = deduplicateBlocks(md)
	md = stripEmptyLinks(md)
	md = collapseShortBlocks(md)
	md = unflattenQuotes(md)
	md = compactLineRuns(md)
	md = gohtml.UnescapeString(md)
	return md
}
