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
	reNBSP            = regexp.MustCompile(`[\x{a0}]+`)
	reZeroWidth       = regexp.MustCompile(`[\x{200c}\x{200b}\x{feff}]`)
	reBlankLineSpaces = regexp.MustCompile(`(?m)^ +$`)
	reExcessiveBlank  = regexp.MustCompile(`\n{3,}`)
	reLeadingBlank    = regexp.MustCompile(`\A\n+`)
	reHeading         = regexp.MustCompile(`(?m)^(#{1,6})\s+(.*)$`)
	reBold            = regexp.MustCompile(`(?s)\*\*(.+?)\*\*`)
	// italic: matches *text* where * is not adjacent to another *
	// handled via bold-placeholder approach; this regex matches single *
	// (?s) allows matching across newlines for multiline emphasis
	reItalic     = regexp.MustCompile(`(?s)\*([^*]+?)\*`)
	reRuleDashes = regexp.MustCompile(`(?m)^-{3,}$`)
	reRuleUnders = regexp.MustCompile(`(?m)^_{3,}$`)
	reListIndent = regexp.MustCompile(`(?m)^[ ]{4,}([-*+] )`)
	reANSI       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
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

// cleanPandocArtifacts removes trailing backslash line-breaks and
// backslash-escaped punctuation that pandoc emits.
func cleanPandocArtifacts(text string) string {
	text = reTrailingBackslash.ReplaceAllString(text, "\n")
	text = reEscapedPunct.ReplaceAllString(text, "$1")
	return text
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
		if groups == nil {
			return m
		}
		return it + groups[1] + r
	})

	// Restore bold placeholders: pairs of placeholder tokens wrap content.
	// Replace first placeholder with b, second with r, alternating.
	text = replaceMarkerPairs(text, boldPlaceholder, b, r)

	// Horizontal rules (dashes and underscores)
	text = reRuleDashes.ReplaceAllString(text, ru+"$0"+r)
	text = reRuleUnders.ReplaceAllString(text, ru+"$0"+r)

	return text
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
			sb.WriteString(open)
		} else {
			sb.WriteString(close)
		}
		sb.WriteString(part)
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

	// sed stage: strip Mozilla-specific HTML attributes
	cleaned := cleanMozAttributes(string(raw))

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
	md = normalizeListIndent(md)
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
