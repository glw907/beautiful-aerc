package filter

import (
	gohtml "html"
	"regexp"
	"strings"
)

// inlineBoundaryTags is the closed set of inline HTML elements whose
// boundaries must carry an explicit space when they abut text. Block
// tags (p, div, h1-h6, li, …) are excluded. The html→markdown
// converter separates those with blank lines.
var inlineBoundaryTags = []string{
	"a", "b", "code", "em", "i", "span", "strong", "u",
}

var reInlineBoundary = regexp.MustCompile(
	`(?i)<(?:/)?(?:` + strings.Join(inlineBoundaryTags, "|") + `)(?:\s[^>]*)?\s*/?>`,
)

var reBR = regexp.MustCompile(`(?i)<br\s*/?>`)

// inlineBoundaryPad spaces around inline-element boundaries so
// adjacent text runs survive tag stripping. <br> becomes a plain
// space, since it marks a soft line break, not a hard markdown break.
func inlineBoundaryPad(body string) string {
	body = reBR.ReplaceAllString(body, " ")
	return reInlineBoundary.ReplaceAllStringFunc(body, func(m string) string {
		return " " + m + " "
	})
}

var (
	reMozClass        = regexp.MustCompile(` class="moz-[^"]*"`)
	reMozDataAttr     = regexp.MustCompile(` data-moz-do-not-send="[^"]*"`)
	reMozAttr         = regexp.MustCompile(` moz-do-not-send="[^"]*"`)
	reHiddenDivOpen   = regexp.MustCompile(`(?i)<div[^>]+style="[^"]*display:\s*none[^"]*"[^>]*>`)
	reZeroImg         = regexp.MustCompile(`(?i)<img[^>]*(?:width:\s*0|height:\s*0|width="0"|height="0")[^>]*/?>`)
	reNBSP            = regexp.MustCompile(`[\x{a0}\x{2000}-\x{200a}]+`)
	reZeroWidth       = regexp.MustCompile(`[\x{ad}\x{34f}\x{180e}\x{200b}-\x{200d}\x{2060}-\x{2064}\x{feff}]`)
	reBlankSpaces     = regexp.MustCompile(`(?m)^ +$`)
	reExcessiveBlanks = regexp.MustCompile(`\n{3,}`)
	reLeadingBlanks   = regexp.MustCompile(`\A\n+`)

	reMdLink      = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reEmptyMdLink = regexp.MustCompile(`\[\]\([^)]+\)`)

	reOrderedList = regexp.MustCompile(`^\d+[.)]`)

	// reParenList matches "1)" or "1\)" list markers (the latter shows
	// up after html→markdown escapes the close paren) at top level or
	// inside a blockquote prefix, rewriting them to "1.".
	reParenList = regexp.MustCompile(`(?m)^((?:>\s?)*)(\d+)\\?\)\s`)
)

// prepareHTML cleans raw HTML before conversion. It strips Mozilla-
// specific attributes, display:none divs, zero-size tracking images,
// and pads inline-element boundaries against word fusion.
func prepareHTML(body string) string {
	body = reMozClass.ReplaceAllString(body, "")
	body = reMozDataAttr.ReplaceAllString(body, "")
	body = reMozAttr.ReplaceAllString(body, "")
	body = stripHiddenElements(body)
	body = reZeroImg.ReplaceAllString(body, "")
	body = inlineBoundaryPad(body)
	return body
}

// stripHiddenElements removes nested display:none <div> trees.
// Responsive HTML emails (Apple receipts, etc.) hide a duplicate of
// the body inside such a div with many nested children, so a non-
// greedy regex would close on the wrong </div>. The walker counts
// <div> opens against </div> closes until depth returns to zero.
func stripHiddenElements(body string) string {
	for {
		loc := reHiddenDivOpen.FindStringIndex(body)
		if loc == nil {
			break
		}
		start := loc[0]
		rest := body[loc[1]:]
		depth := 1
		pos := 0
		for depth > 0 && pos < len(rest) {
			nextOpen := strings.Index(rest[pos:], "<div")
			nextClose := strings.Index(rest[pos:], "</div>")
			if nextClose < 0 {
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

// normalizeWhitespace collapses NBSP, zero-width preheader padding,
// space-only lines, runs of 3+ blank lines, leading blanks, and the
// stray CRs that survive HTML-to-markdown conversion.
func normalizeWhitespace(text string) string {
	text = strings.ReplaceAll(text, "\r", "")
	text = reNBSP.ReplaceAllString(text, " ")
	text = reZeroWidth.ReplaceAllString(text, "")
	text = reBlankSpaces.ReplaceAllString(text, "")
	text = reExcessiveBlanks.ReplaceAllString(text, "\n\n")
	text = reLeadingBlanks.ReplaceAllString(text, "")
	return text
}

// normalizeListMarkers rewrites "1)" / "1\)" markers as "1." so the
// block parser recognizes them.
func normalizeListMarkers(text string) string {
	return reParenList.ReplaceAllString(text, "${1}${2}. ")
}

// unflattenQuotes rebuilds blockquotes from attribution lines that
// Outlook mobile (and similar) collapse into a single <p> with
// entity-encoded '>' markers in place of line breaks.
//
//	Person wrote:&gt;line1&gt;&gt;line2&gt;line3
//
// becomes
//
//	Person wrote:
//
//	> line1
//	>
//	> line2 line3
func unflattenQuotes(text string) string {
	blocks := strings.Split(text, "\n\n")
	for i, block := range blocks {
		wroteIdx, sep := findQuoteStart(block)
		if wroteIdx < 0 {
			continue
		}

		attribution := block[:wroteIdx+len("wrote:")]
		rest := strings.TrimSpace(block[wroteIdx+len("wrote:"):])
		rest = strings.TrimPrefix(rest, sep+" ")

		// A "<sep> " prefix on a part marks a paragraph break: the
		// original "> \n> " that flattened into " > > ".
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

// findQuoteStart locates "wrote:" followed by inline quote markers
// and returns its index plus the separator ("&gt;" or ">"). The
// index is -1 when no match is found.
func findQuoteStart(block string) (int, string) {
	if idx := strings.Index(block, "wrote: &gt; "); idx >= 0 {
		return idx, "&gt;"
	}
	if idx := strings.Index(block, "wrote: > "); idx >= 0 {
		return idx, ">"
	}
	return -1, ""
}

// blockKey normalizes a block for dedup by neutralizing markdown link
// URLs so tracking variants of the same image-link compare equal.
func blockKey(block string) string {
	return reMdLink.ReplaceAllString(block, "[$1](#)")
}

// deduplicateBlocks collapses consecutive blocks with the same
// blockKey into a single occurrence.
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

// mergeBlockRuns joins runs of minRun or more consecutive pred-
// matching blocks with sep. Shorter runs pass through.
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

// collapseShortBlocks joins runs of 3+ short plain-text blocks with
// " · ". Flattened table cells (navigation bars, step trackers, tag
// lists) were never meant to read vertically.
func collapseShortBlocks(text string) string {
	return mergeBlockRuns(text, isShortPlain, 3, " · ")
}

// isShortPlain accepts single-line plain text under 25 characters
// with no markdown syntax and no terminal sentence punctuation. The
// punctuation rule keeps real sentences out of the merge.
func isShortPlain(block string) bool {
	if strings.Contains(block, "\n") || len(block) > 25 || len(block) == 0 {
		return false
	}
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
	last := block[len(block)-1]
	if last == '.' || last == '!' || last == '?' || last == ':' || last == ';' {
		return false
	}
	return true
}

// compactLineRuns joins runs of 3+ short single-line blocks with
// markdown hard breaks (two trailing spaces). Signature blocks land
// in the HTML source as separate <p>s but should render tight.
func compactLineRuns(text string) string {
	return mergeBlockRuns(text, isCompactLine, 3, "  \n")
}

// isCompactLine accepts single-line blocks whose visible text is
// under 80 characters and that aren't a markdown block element or a
// sentence ending with punctuation. Visible length strips link
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

func stripEmptyLinks(text string) string {
	return reEmptyMdLink.ReplaceAllString(text, "")
}

// CleanHTML converts raw HTML email content to normalized markdown.
// Content pipeline only. No rendering or styling.
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
