// Package filter implements aerc email content filters.
package filter

import (
	"bytes"
	"fmt"
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

// linkColors holds ANSI parameter strings for link styling.
type linkColors struct {
	Text  string
	URL   string
	Reset string
}

// Package-level compiled regexes.
var (
	reMozClass         = regexp.MustCompile(` class="moz-[^"]*"`)
	reMozDataAttr      = regexp.MustCompile(` data-moz-do-not-send="[^"]*"`)
	reMozAttr          = regexp.MustCompile(` moz-do-not-send="[^"]*"`)
	reTrailingBackslash = regexp.MustCompile(`(?m)\\\n`)
	reEscapedPunct      = regexp.MustCompile(`\\([^\w\s])`)
	// image-link: [![alt](img)](url) optionally with whitespace/newlines inside outer brackets
	reImageLink       = regexp.MustCompile(`\[(\s*!\[([^\]]*)\]\([^)]*\)\s*\n?\s*([^\]]*?)\s*)\]\(([^)]+)\)`)
	reStandaloneImage = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)\n?`)
	reEmptyTextLink   = regexp.MustCompile(`\[\s*\]\([^)]+\)\n?`)
	reEmptyURLLink    = regexp.MustCompile(`\[([^\]]+)\]\(\)`)
	reMultilineLink   = regexp.MustCompile(`\[([^\]]+?)\]\(([^)]+)\)`)
	reNBSP            = regexp.MustCompile(`[\x{a0}]+`)
	reZeroWidth       = regexp.MustCompile(`[\x{200c}\x{200b}\x{feff}]`)
	reBlankLineSpaces = regexp.MustCompile(`(?m)^ +$`)
	reExcessiveBlank  = regexp.MustCompile(`\n{3,}`)
	reLeadingBlank    = regexp.MustCompile(`\A\n+`)
	reHeading         = regexp.MustCompile(`(?m)^(#{1,6})\s+(.*)$`)
	reBold            = regexp.MustCompile(`\*\*(.+?)\*\*`)
	// italic: matches *text* where * is not adjacent to another *
	// handled via bold-placeholder approach; this regex matches single *
	reItalic     = regexp.MustCompile(`\*([^*\n]+?)\*`)
	reRuleDashes = regexp.MustCompile(`(?m)^-{3,}$`)
	reRuleUnders = regexp.MustCompile(`(?m)^_{3,}$`)
	reLink       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
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

// cleanImages removes image constructs: image-links, standalone images,
// empty-text links, and empty-URL links.
func cleanImages(text string) string {
	// Image-link [![alt](img)](url) -> [alt](url) (or empty if no text)
	text = reImageLink.ReplaceAllStringFunc(text, func(m string) string {
		groups := reImageLink.FindStringSubmatch(m)
		if groups == nil {
			return m
		}
		altText := strings.TrimSpace(groups[2])
		extraText := strings.TrimSpace(groups[3])
		url := groups[4]
		combined := strings.TrimSpace(altText + " " + extraText)
		if combined == "" {
			return ""
		}
		return "[" + combined + "](" + url + ")"
	})
	// Standalone images
	text = reStandaloneImage.ReplaceAllString(text, "")
	// Empty-text links
	text = reEmptyTextLink.ReplaceAllString(text, "")
	// Empty-URL links: [text]() -> text
	text = reEmptyURLLink.ReplaceAllString(text, "$1")
	return text
}

// joinMultilineLinks collapses newlines inside link text to a single space.
func joinMultilineLinks(text string) string {
	return reMultilineLink.ReplaceAllStringFunc(text, func(m string) string {
		groups := reMultilineLink.FindStringSubmatch(m)
		if groups == nil {
			return m
		}
		linkText := groups[1]
		url := groups[2]
		linkText = regexp.MustCompile(`\s*\n\s*`).ReplaceAllString(linkText, " ")
		return "[" + linkText + "](" + url + ")"
	})
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
	text = replaceBoldPlaceholders(text, b, r)

	// Horizontal rules (dashes and underscores)
	text = reRuleDashes.ReplaceAllString(text, ru+"$0"+r)
	text = reRuleUnders.ReplaceAllString(text, ru+"$0"+r)

	return text
}

// replaceBoldPlaceholders converts boldPlaceholder tokens to ANSI sequences.
// Tokens appear in pairs: first marks start (emit b), second marks end (emit r).
func replaceBoldPlaceholders(text, b, r string) string {
	parts := strings.Split(text, boldPlaceholder)
	var sb strings.Builder
	for i, part := range parts {
		if i == 0 {
			sb.WriteString(part)
			continue
		}
		// Odd index: opening marker (emit b before content).
		// Even index: closing marker (emit r before content).
		if i%2 == 1 {
			sb.WriteString(b)
		} else {
			sb.WriteString(r)
		}
		sb.WriteString(part)
	}
	return sb.String()
}

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	return reANSI.ReplaceAllString(s, "")
}

// styleLinks applies ANSI coloring to markdown-format links [text](url).
// When clean is true, links are replaced with plain text only (no brackets, no URL).
func styleLinks(text string, colors *linkColors, clean bool) string {
	lt := ""
	lu := ""
	r := ""
	if colors.Text != "" {
		lt = "\033[" + colors.Text + "m"
	}
	if colors.URL != "" {
		lu = "\033[" + colors.URL + "m"
	}
	if colors.Reset != "" {
		r = "\033[" + colors.Reset + "m"
	}

	return reLink.ReplaceAllStringFunc(text, func(m string) string {
		groups := reLink.FindStringSubmatch(m)
		if groups == nil {
			return m
		}
		linkText := strings.TrimSpace(groups[1])
		url := stripANSI(groups[2])

		if clean {
			return linkText
		}
		return lt + "[" + linkText + "]" + r + lu + "(" + url + ")" + r
	})
}

// runPandoc pipes input through pandoc for HTML-to-markdown conversion.
func runPandoc(input io.Reader, luaFilter string, cols int) (string, error) {
	args := []string{
		"-f", "html",
		"-t", "markdown-raw_html-native_divs-native_spans-header_attributes-bracketed_spans-fenced_divs-inline_code_attributes-link_attributes",
		"-L", luaFilter,
		"--wrap=auto",
		fmt.Sprintf("--columns=%d", cols),
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
// It runs the pandoc pipeline, cleans up artifacts, highlights markdown
// syntax, and styles links using palette p. cols sets pandoc's column width.
// When cleanLinks is true, links are rendered as plain text without URLs.
func HTML(r io.Reader, w io.Writer, p *palette.Palette, cols int, cleanLinks bool) error {
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

	// First perl stage: cleanup
	md = cleanPandocArtifacts(md)
	md = cleanImages(md)
	md = joinMultilineLinks(md)
	md = normalizeWhitespace(md)

	// Markdown syntax highlighting
	mc := &markdownColors{
		Heading: p.Get("C_HEADING"),
		Bold:    p.Get("C_BOLD"),
		Italic:  p.Get("C_ITALIC"),
		Rule:    p.Get("C_RULE"),
		Reset:   "0",
	}
	md = highlightMarkdown(md, mc)

	// Colorize (aerc built-in URL/email highlighter)
	md, err = runColorize(md)
	if err != nil {
		return fmt.Errorf("colorize: %w", err)
	}

	// Second perl stage: link styling
	lc := &linkColors{
		Text:  p.Get("C_LINK_TEXT"),
		URL:   p.Get("C_LINK_URL"),
		Reset: "0",
	}
	md = styleLinks(md, lc, cleanLinks)

	// Write leading newline + result
	if _, err := fmt.Fprint(w, "\n"+md); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}
