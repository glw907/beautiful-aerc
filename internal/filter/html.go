package filter

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
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

	// Marker replacement after Glamour rendering.
	reLinkMarkerOpen = regexp.MustCompile("\x02(\\d+);")
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

// markLinks replaces markdown [text](url) with [STX+idx+text+ETX](#) and
// returns the modified text plus the extracted URLs. Glamour suppresses
// display of fragment-only URLs (#) while still styling the link text.
// The STX/ETX markers survive Glamour rendering and are replaced with
// OSC 8 hyperlink sequences by resolveLinks. Empty-text links [](url)
// are removed entirely — they have no useful display content.
func markLinks(text string) (string, []string) {
	text = reEmptyMdLink.ReplaceAllString(text, "")
	var urls []string
	idx := 0
	marked := reMdLink.ReplaceAllStringFunc(text, func(match string) string {
		sub := reMdLink.FindStringSubmatch(match)
		urls = append(urls, sub[2])
		result := fmt.Sprintf("[\x02%d;%s\x03](#)", idx, sub[1])
		idx++
		return result
	})
	return marked, urls
}

// resolveLinks replaces STX/ETX markers in Glamour's ANSI output with
// OSC 8 hyperlink open/close sequences, making styled link text clickable
// in terminals that support OSC 8 (kitty, WezTerm, foot, etc.).
func resolveLinks(text string, urls []string) string {
	text = reLinkMarkerOpen.ReplaceAllStringFunc(text, func(match string) string {
		sub := reLinkMarkerOpen.FindStringSubmatch(match)
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx >= len(urls) {
			return ""
		}
		return fmt.Sprintf("\x1b]8;;%s\x1b\\", urls[idx])
	})
	text = strings.ReplaceAll(text, "\x03", "\x1b]8;;\x1b\\")
	return text
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
	md, urls := markLinks(md)

	style := t.GlamourStyle()
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(wrapWidth),
	)
	if err != nil {
		return fmt.Errorf("creating renderer: %w", err)
	}

	styled, err := renderer.Render(md)
	if err != nil {
		return fmt.Errorf("rendering markdown: %w", err)
	}
	styled = resolveLinks(styled, urls)

	if _, err := fmt.Fprint(w, styled); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}
