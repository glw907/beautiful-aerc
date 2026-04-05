package filter

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glw907/beautiful-aerc/internal/palette"
)

// markdownColors holds ANSI parameter strings for markdown syntax highlighting.
type markdownColors struct {
	Heading string // e.g. "1;38;2;163;190;140"
	Bold    string
	Italic  string
	Rule    string
	Reset   string
}

// Package-level compiled regexes.
var (
	reMozClass         = regexp.MustCompile(` class="moz-[^"]*"`)
	reMozDataAttr      = regexp.MustCompile(` data-moz-do-not-send="[^"]*"`)
	reMozAttr          = regexp.MustCompile(` moz-do-not-send="[^"]*"`)
	reTrailingBackslash = regexp.MustCompile(`(?m)\\\n`)
	reEscapedPunct      = regexp.MustCompile(`\\([^\w\s])`)
	// Stray bold markers on a line by themselves (pandoc decorative artifact).
	// The $ before \n? anchors to end-of-line so lines like **text** are preserved.
	reStrayBold = regexp.MustCompile(`(?m)^\*\*\s*$\n?`)
	reListItem  = regexp.MustCompile(`^[-*+]\s`)
	// Unicode space variants: NBSP, en/em space, thin/hair space, etc.
	reNBSP            = regexp.MustCompile(`[\x{a0}\x{2000}-\x{200a}]+`)
	// Invisible filler: zero-width, joiners, soft hyphen, word joiner, etc.
	reZeroWidth = regexp.MustCompile(`[\x{ad}\x{34f}\x{180e}\x{200b}-\x{200d}\x{2060}-\x{2064}\x{feff}]`)
	reBlankLineSpaces = regexp.MustCompile(`(?m)^ +$`)
	reExcessiveBlank  = regexp.MustCompile(`\n{3,}`)
	reLeadingBlank    = regexp.MustCompile(`\A\n+`)
	reHeading         = regexp.MustCompile(`(?m)^(#{1,6})\s+(.*)$`)
	// reBold matches **text** allowing newlines. normalizeBoldMarkers strips
	// unpaired markers before this runs, so cross-paragraph matches cannot
	// occur: any ** that would span a blank line has already been removed.
	reBold = regexp.MustCompile(`(?s)\*\*(.+?)\*\*`)
	// italic: matches *text* allowing newlines within a paragraph but not
	// across paragraph breaks (double newlines), preventing stray * from
	// pandoc \* escaping from bleeding italic across paragraphs.
	reItalic = regexp.MustCompile(`\*([^*\n]+(?:\n[^*\n]+)*)\*`)
	reRuleDashes = regexp.MustCompile(`(?m)^-{3,}$`)
	reRuleUnders = regexp.MustCompile(`(?m)^_{3,}$`)
	reListIndent     = regexp.MustCompile(`(?m)^[ ]{4,}([-*+] )`)
	reUnicodeBullet  = regexp.MustCompile(`^[ \t]*[●•◦◆▪▸‣⁃][ \t]*`)
	reANSI           = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	// reSuperscript matches pandoc's ^text^ output for HTML <sup> elements.
	// Superscripts in email are almost always footnote numbers or legal
	// markers; stripping the carets reads fine in plain terminal output.
	reSuperscript = regexp.MustCompile(`\^([^^]+)\^`)
	// reHiddenDivOpen matches the opening tag of a <div> with display:none in
	// its inline style. Used by stripHiddenElements to find the start of a
	// hidden section; nesting-aware removal handles finding the matching close.
	reHiddenDivOpen = regexp.MustCompile(`(?i)<div[^>]+style="[^"]*display:\s*none[^"]*"[^>]*>`)
	// reZeroImg matches <img> tags with zero width or height (tracking pixels).
	// Bank of America and similar senders embed these between URL parts,
	// causing pandoc to split the URL across multiple paragraphs.
	reZeroImg = regexp.MustCompile(`(?i)<img[^>]*(?:width:\s*0|height:\s*0|width="0"|height="0")[^>]*/?>`)
)

// boldPlaceholder is used to hide bold markers during italic processing.
const boldPlaceholder = "\x00BOLD\x00"

// cleanMozAttributes removes Mozilla-specific HTML attributes (sed stage).
func cleanMozAttributes(html string) string {
	html = reMozClass.ReplaceAllString(html, "")
	html = reMozDataAttr.ReplaceAllString(html, "")
	html = reMozAttr.ReplaceAllString(html, "")
	return html
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

// stripZeroImages removes <img> tags with zero width or height before pandoc
// conversion. Bank of America and similar senders embed these tracking pixels
// inline between text fragments (e.g. "myhealth." + <img/> + "bankofamerica."
// + <img/> + "com"), causing pandoc to split content across paragraphs.
func stripZeroImages(body string) string {
	return reZeroImg.ReplaceAllString(body, "")
}

// cleanPandocArtifacts removes trailing backslash line-breaks,
// backslash-escaped punctuation, stray bold markers, and superscript
// caret markers (^text^) that pandoc emits for HTML <sup> elements.
func cleanPandocArtifacts(text string) string {
	text = reTrailingBackslash.ReplaceAllString(text, "\n")
	text = reEscapedPunct.ReplaceAllString(text, "$1")
	text = reStrayBold.ReplaceAllString(text, "")
	text = reSuperscript.ReplaceAllString(text, "$1")
	return text
}

// normalizeBoldMarkers ensures bold markers (**) are balanced within each
// paragraph. It handles two pandoc artifacts:
//
//  1. Cross-paragraph bold: pandoc emits ** that opens in one paragraph and
//     closes in another. Since reBold no longer uses (?s), these would render
//     as stray markers. This function strips any unpaired trailing ** per
//     paragraph, which also eliminates them.
//
//  2. Unpaired trailing **: pandoc sometimes emits a ** at the end of a phrase
//     with no matching opener. The odd-count check strips the last one.
func normalizeBoldMarkers(text string) string {
	paragraphs := strings.Split(text, "\n\n")
	for i, para := range paragraphs {
		// Count ** occurrences (not escaped, just literal occurrences).
		// strings.Count counts non-overlapping instances of "**".
		count := strings.Count(para, "**")
		if count%2 == 0 {
			// Balanced; nothing to do.
			continue
		}
		// Odd count: strip the last occurrence of ** in this paragraph.
		idx := strings.LastIndex(para, "**")
		paragraphs[i] = para[:idx] + para[idx+2:]
	}
	return strings.Join(paragraphs, "\n\n")
}

// compactLooseLists removes blank lines between consecutive list items.
// Pandoc emits "loose" lists (blank line between items) for HTML <li>
// elements, which double-spaces the rendered output unnecessarily.
func compactLooseLists(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	inList := false
	pendingBlanks := 0
	flush := func() {
		for range pendingBlanks {
			out = append(out, "")
		}
		pendingBlanks = 0
	}
	for _, line := range lines {
		isItem := reListItem.MatchString(line)
		isBlank := strings.TrimSpace(line) == ""
		isCont := inList && !isBlank && line[0] == ' '
		switch {
		case isItem:
			if !inList {
				flush()
			}
			pendingBlanks = 0
			inList = true
			out = append(out, line)
		case isCont:
			flush()
			out = append(out, line)
		case isBlank && inList:
			pendingBlanks++
		default:
			inList = false
			flush()
			out = append(out, line)
		}
	}
	if !inList {
		flush()
	}
	return strings.Join(out, "\n")
}

// normalizeUnicodeBullets converts lines starting with Unicode bullet
// characters (●, •, etc.) into markdown list items with proper
// continuation-line indentation. Marketing emails often use these
// instead of <li> elements.
func normalizeUnicodeBullets(text string) string {
	lines := strings.Split(text, "\n")
	inItem := false
	for i, line := range lines {
		if replaced := reUnicodeBullet.ReplaceAllString(line, "- "); replaced != line {
			lines[i] = replaced
			inItem = true
		} else if inItem {
			if strings.TrimSpace(line) == "" {
				inItem = false
			} else {
				lines[i] = "  " + line
			}
		}
	}
	return strings.Join(lines, "\n")
}

// normalizeListIndent strips excessive indentation from list items that
// pandoc emits when converting deeply nested HTML structures.
func normalizeListIndent(text string) string {
	return reListIndent.ReplaceAllString(text, "$1")
}

// normalizeWhitespace collapses non-breaking spaces, zero-width characters,
// blank lines with spaces, excessive blank lines, and leading blank lines.
func normalizeWhitespace(text string) string {
	text = reNBSP.ReplaceAllString(text, " ")
	text = reZeroWidth.ReplaceAllString(text, "")
	text = reBlankLineSpaces.ReplaceAllString(text, "")
	text = reExcessiveBlank.ReplaceAllString(text, "\n\n")
	text = reLeadingBlank.ReplaceAllString(text, "")
	return text
}

// highlightMarkdown applies ANSI colors to markdown syntax: headings, bold,
// italic, and horizontal rules.
func highlightMarkdown(text string, colors *markdownColors) string {
	esc := func(params string) string {
		if params == "" {
			return ""
		}
		return "\033[" + params + "m"
	}
	h := esc(colors.Heading)
	b := esc(colors.Bold)
	it := esc(colors.Italic)
	ru := esc(colors.Rule)
	r := esc(colors.Reset)

	// Headings
	text = reHeading.ReplaceAllStringFunc(text, func(m string) string {
		groups := reHeading.FindStringSubmatch(m)
		if groups == nil {
			return m
		}
		return groups[1] + " " + h + groups[2] + r
	})

	// Bold: replace with placeholder first to avoid matching italic inside bold
	text = reBold.ReplaceAllStringFunc(text, func(m string) string {
		groups := reBold.FindStringSubmatch(m)
		if groups == nil {
			return m
		}
		return boldPlaceholder + groups[1] + boldPlaceholder
	})

	// Italic: now safe to match single * because ** has been replaced
	text = reItalic.ReplaceAllStringFunc(text, func(m string) string {
		groups := reItalic.FindStringSubmatch(m)
		return wrapLines(groups[1], it, r)
	})

	// Restore bold placeholders: pairs of placeholder tokens wrap content.
	// Replace first placeholder with b, second with r, alternating.
	text = replaceMarkerPairs(text, boldPlaceholder, b, r)

	// Horizontal rules (dashes and underscores)
	text = reRuleDashes.ReplaceAllString(text, ru+"$0"+r)
	text = reRuleUnders.ReplaceAllString(text, ru+"$0"+r)

	return text
}

// wrapLines makes ANSI styling work in aerc's per-line viewer by
// re-emitting open/close codes around each line of content.
func wrapLines(content, open, close string) string {
	if !strings.Contains(content, "\n") {
		return open + content + close
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = open + line + close
	}
	return strings.Join(lines, "\n")
}

// replaceMarkerPairs splits text on sentinel and alternates open/close ANSI
// sequences at each boundary. Odd splits get open, even splits get close.
func replaceMarkerPairs(text, sentinel, open, close string) string {
	parts := strings.Split(text, sentinel)
	var sb strings.Builder
	for i, part := range parts {
		if i == 0 {
			sb.WriteString(part)
			continue
		}
		if i%2 == 1 {
			// Opening: wrap each line so aerc's per-line viewer sees codes.
			sb.WriteString(wrapLines(part, open, close))
		}
		// Even parts (after close) are plain text, written directly.
		if i%2 == 0 {
			sb.WriteString(part)
		}
	}
	return sb.String()
}

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	return reANSI.ReplaceAllString(s, "")
}

// runPandoc pipes input through pandoc for HTML-to-markdown conversion.
func runPandoc(input io.Reader, luaFilter string, cols int) (string, error) {
	args := []string{
		"-f", "html",
		"-t", "markdown-raw_html-native_divs-native_spans-header_attributes-bracketed_spans-fenced_divs-inline_code_attributes-link_attributes",
		"-L", luaFilter,
		"--wrap=auto",
		fmt.Sprintf("--columns=%d", cols),
		"--reference-links",
	}
	cmd := exec.Command("pandoc", args...)
	cmd.Stdin = input
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running pandoc: %w", err)
	}
	return out.String(), nil
}

// runColorize pipes text through aerc's colorize filter.
func runColorize(input string) (string, error) {
	path, err := findColorize()
	if err != nil {
		return input, nil // colorize is optional; pass through if missing
	}
	cmd := exec.Command(path)
	cmd.Stdin = strings.NewReader(input)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return input, nil // pass through on failure
	}
	return out.String(), nil
}

// findColorize locates the aerc colorize binary.
func findColorize() (string, error) {
	const fixed = "/usr/local/libexec/aerc/filters/colorize"
	if _, err := os.Stat(fixed); err == nil {
		return fixed, nil
	}
	path, err := exec.LookPath("colorize")
	if err == nil {
		return path, nil
	}
	return "", fmt.Errorf("colorize not found")
}

// findLuaFilter locates unwrap-tables.lua by checking standard locations.
func findLuaFilter() (string, error) {
	var candidates []string

	if aercConfig := os.Getenv("AERC_CONFIG"); aercConfig != "" {
		candidates = append(candidates, filepath.Join(aercConfig, "filters", "unwrap-tables.lua"))
	}

	// Relative to binary: ../../.config/aerc/filters/unwrap-tables.lua
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "..", ".config", "aerc", "filters", "unwrap-tables.lua"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "aerc", "filters", "unwrap-tables.lua"))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("unwrap-tables.lua not found (checked: %s)", strings.Join(candidates, ", "))
}

// HTML converts an HTML email body to styled text, writing to w.
// It runs the pandoc pipeline, cleans up artifacts, converts links to
// footnotes, and highlights markdown syntax using palette p.
// cols sets pandoc's column width.
func HTML(r io.Reader, w io.Writer, p *palette.Palette, cols int) error {
	if cols < 1 {
		cols = 72
	}

	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	// sed stage: strip Mozilla-specific HTML attributes, hidden elements,
	// and zero-size tracking images before handing off to pandoc.
	cleaned := cleanMozAttributes(string(raw))
	cleaned = stripHiddenElements(cleaned)
	cleaned = stripZeroImages(cleaned)

	// Find lua filter
	luaFilter, err := findLuaFilter()
	if err != nil {
		return fmt.Errorf("finding lua filter: %w", err)
	}

	// Run pandoc
	md, err := runPandoc(strings.NewReader(cleaned), luaFilter, cols)
	if err != nil {
		return fmt.Errorf("pandoc conversion: %w", err)
	}

	// Post-pandoc cleanup
	md = html.UnescapeString(md)
	md = cleanPandocArtifacts(md)
	md = normalizeBoldMarkers(md)
	md = normalizeUnicodeBullets(md)
	md = normalizeListIndent(md)
	md = compactLooseLists(md)
	md = normalizeWhitespace(md)

	// Footnote conversion and styling
	body, refs := convertToFootnotes(md)
	dimColor, _ := palette.HexToANSI(p.Get("FG_DIM"))
	fc := &footnoteColors{
		LinkText: p.Get("C_LINK_TEXT"),
		Dim:      dimColor,
		LinkURL:  p.Get("C_LINK_URL"),
		Reset:    "0",
	}
	md = styleFootnotes(body, refs, cols, fc)

	// Markdown syntax highlighting
	mc := &markdownColors{
		Heading: p.Get("C_HEADING"),
		Bold:    p.Get("C_BOLD"),
		Italic:  p.Get("C_ITALIC"),
		Rule:    p.Get("C_RULE"),
		Reset:   "0",
	}
	md = highlightMarkdown(md, mc)

	// Write leading newline + result
	if _, err := fmt.Fprint(w, "\n"+md); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}
