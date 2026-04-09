package filter

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

// Package-level compiled regexes.
var (
	reMozClass      = regexp.MustCompile(` class="moz-[^"]*"`)
	reMozDataAttr   = regexp.MustCompile(` data-moz-do-not-send="[^"]*"`)
	reMozAttr       = regexp.MustCompile(` moz-do-not-send="[^"]*"`)
	reHiddenDivOpen = regexp.MustCompile(`(?i)<div[^>]+style="[^"]*display:\s*none[^"]*"[^>]*>`)
	reZeroImg       = regexp.MustCompile(`(?i)<img[^>]*(?:width:\s*0|height:\s*0|width="0"|height="0")[^>]*/?>`)
	reANSI          = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	reOSC8          = regexp.MustCompile(`\x1b\]8;[^\x1b]*\x1b\\`)

	// Post-conversion whitespace normalization: strip invisible filler
	// characters that email senders embed for preheader text, collapse
	// excessive blank lines, and strip leading blanks.
	reNBSP          = regexp.MustCompile(`[\x{a0}\x{2000}-\x{200a}]+`)
	reZeroWidth     = regexp.MustCompile(`[\x{ad}\x{34f}\x{180e}\x{200b}-\x{200d}\x{2060}-\x{2064}\x{feff}]`)
	reBlankSpaces   = regexp.MustCompile(`(?m)^ +$`)
	reExcessiveBlanks = regexp.MustCompile(`\n{3,}`)
	reLeadingBlanks = regexp.MustCompile(`\A\n+`)

	// Markdown link patterns for URL extraction.
	reMdLink      = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reEmptyMdLink = regexp.MustCompile(`\[\]\([^)]+\)`)

	// Ordered list item: "1.", "2)", etc.
	reOrderedList = regexp.MustCompile(`^\d+[.)]`)
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

// reflowMarkdown reflows plain paragraphs in markdown text to the given width
// using minimum-raggedness line breaking. Headings, lists, blockquotes, table
// rows, and code blocks are left untouched.
func reflowMarkdown(text string, width int) string {
	blocks := strings.Split(text, "\n\n")
	for i, block := range blocks {
		if isParagraph(block) {
			blocks[i] = reflowParagraph(block, width)
		}
	}
	return strings.Join(blocks, "\n\n")
}

// isParagraph returns true if the block is a plain text paragraph (not a
// heading, list, blockquote, table, or code fence).
func isParagraph(block string) bool {
	lines := strings.Split(block, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed[0] == '#' || trimmed[0] == '>' || trimmed[0] == '|' ||
			trimmed[0] == '*' || trimmed[0] == '-' || trimmed[0] == '+' ||
			strings.HasPrefix(trimmed, "```") ||
			strings.HasPrefix(trimmed, "---") ||
			strings.HasPrefix(trimmed, "===") ||
			reOrderedList.MatchString(trimmed) {
			return false
		}
	}
	return true
}

// markdownTokens splits text into words, keeping markdown link syntax
// [text](#) as atomic units so link text is never split across lines.
func markdownTokens(text string) []string {
	raw := strings.Fields(text)
	var tokens []string
	for i := 0; i < len(raw); i++ {
		w := raw[i]
		// Start of a markdown link: [word or [word word...](#)
		if strings.HasPrefix(w, "[") && !strings.Contains(w, "](") {
			// Accumulate words until we find one ending with ](#) or similar
			link := w
			for i+1 < len(raw) {
				next := raw[i+1]
				link += " " + next
				i++
				if strings.Contains(next, "](") {
					break
				}
			}
			tokens = append(tokens, link)
			continue
		}
		tokens = append(tokens, w)
	}
	return tokens
}

// reflowParagraph joins all lines into one and re-wraps using minimum-
// raggedness dynamic programming. This avoids the orphaned short words
// that greedy wrapping produces (e.g., "offered\nby").
func reflowParagraph(text string, width int) string {
	words := markdownTokens(text)
	if len(words) == 0 {
		return ""
	}
	n := len(words)

	wordLen := make([]int, n)
	for i, w := range words {
		wordLen[i] = len(w)
	}

	// Minimum-raggedness DP: cost[i] = min cost for words[i:],
	// breaks[i] = first word on the next line after the line starting at i.
	const inf = 1 << 62
	cost := make([]int, n+1)
	breaks := make([]int, n)
	cost[n] = 0

	for i := n - 1; i >= 0; i-- {
		lineLen := -1
		best := inf
		bestBreak := n
		for j := i; j < n; j++ {
			lineLen += 1 + wordLen[j]
			if lineLen > width && j > i {
				break
			}
			var c int
			if j == n-1 {
				c = cost[j+1]
			} else {
				slack := width - lineLen
				c = slack*slack + cost[j+1]
			}
			if c < best {
				best = c
				bestBreak = j + 1
			}
		}
		cost[i] = best
		breaks[i] = bestBreak
	}

	var lines []string
	for i := 0; i < n; {
		j := breaks[i]
		lines = append(lines, strings.Join(words[i:j], " "))
		i = j
	}
	return strings.Join(lines, "\n")
}

// extractLinks extracts URLs from markdown links in order, strips empty-text
// links, and replaces all link URLs with # so Glamour styles the text without
// displaying URLs. Returns the cleaned text and the ordered URL list.
func extractLinks(text string) (string, []string) {
	text = reEmptyMdLink.ReplaceAllString(text, "")
	var urls []string
	cleaned := reMdLink.ReplaceAllStringFunc(text, func(match string) string {
		sub := reMdLink.FindStringSubmatch(match)
		urls = append(urls, sub[2])
		return "[" + sub[1] + "](#)"
	})
	return cleaned, urls
}

// injectOSC8 wraps Glamour's styled link text spans with OSC 8 hyperlink
// sequences. It finds link-styled ANSI spans (identified by the linkStyle
// prefix) and wraps consecutive runs with OSC 8 open/close, consuming URLs
// in order from the extracted list.
func injectOSC8(text string, urls []string, linkStyle string) string {
	if len(urls) == 0 || linkStyle == "" {
		return text
	}

	var b strings.Builder
	b.Grow(len(text) + len(urls)*64)

	urlIdx := 0
	inLink := false
	i := 0
	for i < len(text) {
		// Check for link style opening sequence.
		if urlIdx < len(urls) && strings.HasPrefix(text[i:], linkStyle) {
			if !inLink {
				// Start a new OSC 8 region.
				inLink = true
				fmt.Fprintf(&b, "\x1b]8;;%s\x1b\\", urls[urlIdx])
			}
			b.WriteString(linkStyle)
			i += len(linkStyle)
			continue
		}

		// Check for ANSI reset — may end a link span.
		if inLink && strings.HasPrefix(text[i:], "\x1b[0m") {
			b.WriteString("\x1b[0m")
			i += 4
			// Look ahead: if the next non-reset content re-opens with
			// linkStyle, this is a word-wrapped continuation of the same
			// link. Otherwise the link has ended.
			rest := text[i:]
			// Skip any resets or whitespace-only content before next style.
			if !strings.HasPrefix(rest, linkStyle) &&
				!strings.HasPrefix(rest, "\x1b[0m") {
				// Link ended — close OSC 8.
				b.WriteString("\x1b]8;;\x1b\\")
				inLink = false
				urlIdx++
			}
			continue
		}

		b.WriteByte(text[i])
		i++
	}

	// Close any trailing open link.
	if inLink {
		b.WriteString("\x1b]8;;\x1b\\")
	}
	return b.String()
}

// stripANSI removes ANSI escape sequences (CSI and OSC 8) from s.
func stripANSI(s string) string {
	s = reOSC8.ReplaceAllString(s, "")
	return reANSI.ReplaceAllString(s, "")
}

// Markdown converts HTML email to clean markdown without ANSI styling.
// Used by the markdown subcommand for reply templates.
func Markdown(r io.Reader, w io.Writer, cols int) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	cleaned := prepareHTML(string(raw))
	md, err := convertHTML(cleaned)
	if err != nil {
		return fmt.Errorf("converting html: %w", err)
	}
	md = normalizeWhitespace(md)
	md = reflowMarkdown(md, cols)

	if _, err := fmt.Fprint(w, md+"\n"); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// wrapWidth is the fixed line width for rendered email, matching the
// standard email prose width. Using a fixed width rather than the
// terminal width avoids awkward reflows when the pane is very wide
// or very narrow.
const wrapWidth = 78

// HTML reads raw HTML email from r, converts it to markdown, and renders
// it to styled ANSI output via Glamour using theme t.
func HTML(r io.Reader, w io.Writer, t *theme.Theme, _ int) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	cleaned := prepareHTML(string(raw))
	md, err := convertHTML(cleaned)
	if err != nil {
		return fmt.Errorf("converting html: %w", err)
	}
	md = normalizeWhitespace(md)
	md, urls := extractLinks(md)
	md = reflowMarkdown(md, wrapWidth)

	style := t.GlamourStyle()
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return fmt.Errorf("creating renderer: %w", err)
	}

	styled, err := renderer.Render(md)
	if err != nil {
		return fmt.Errorf("rendering markdown: %w", err)
	}
	styled = injectOSC8(styled, urls, t.GlamourLinkStyle())

	if _, err := fmt.Fprint(w, styled); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}
