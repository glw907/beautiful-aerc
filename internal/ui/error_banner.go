package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// renderErrorBanner formats an ErrorMsg for the single banner row
// above the status bar. Returns "" when msg.Err is nil. Output is
// at most width display cells wide; longer text is truncated with
// "…". When msg.Op is empty, the format is "⚠ <err>"; otherwise
// "⚠ <op>: <err>".
func renderErrorBanner(msg ErrorMsg, width int, styles Styles) string {
	if msg.Err == nil {
		return ""
	}
	text := "⚠ "
	if msg.Op != "" {
		text += msg.Op + ": "
	}
	text += msg.Err.Error()
	text = truncateToWidth(text, width)
	return styles.ErrorBanner.Render(text)
}

// truncateToWidth shortens s to at most width display cells, adding
// "…" when truncation occurs. Splits on rune boundaries — never
// inside a multi-byte glyph. Counts cells via lipgloss.Width so
// double-width CJK characters are accounted for correctly.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	const ellipsis = "…"
	if width == 1 {
		return ellipsis
	}
	limit := width - lipgloss.Width(ellipsis)
	out := make([]rune, 0, len(s))
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > limit {
			break
		}
		out = append(out, r)
		w += rw
	}
	return string(out) + ellipsis
}
