// SPDX-License-Identifier: MIT

package content

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/glw907/poplar/internal/theme"
)

const maxBodyWidth = 72

// wrap is the renderer's width-honoring word wrap. Wordwrap respects
// word boundaries; Hardwrap catches the residue when a single token
// (long URL, code identifier) exceeds width. Together they guarantee
// no output line is wider than width — the contract every block-
// renderer below relies on.
func wrap(text string, width int) string {
	if width < 1 {
		width = 1
	}
	return ansi.Hardwrap(ansi.Wordwrap(text, width, ""), width, false)
}

// RenderBody renders blocks into a styled string using lipgloss.
// Width is capped at maxBodyWidth for readability.
func RenderBody(blocks []Block, t *theme.CompiledTheme, width int) string {
	w := width
	if w > maxBodyWidth {
		w = maxBodyWidth
	}

	return joinRenderedBlocks(blocks, t, w)
}

// joinRenderedBlocks renders blocks and joins them with appropriate
// separators: single newline between consecutive list items, double
// newline between all other blocks.
func joinRenderedBlocks(blocks []Block, t *theme.CompiledTheme, width int) string {
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	for i, block := range blocks {
		if i > 0 {
			_, prevIsList := blocks[i-1].(ListItem)
			_, currIsList := block.(ListItem)
			if prevIsList && currIsList {
				b.WriteString("\n")
			} else {
				b.WriteString("\n\n")
			}
		}
		b.WriteString(renderBlock(block, t, width))
	}
	return b.String()
}

func renderBlock(block Block, t *theme.CompiledTheme, width int) string {
	switch b := block.(type) {
	case Paragraph:
		text := renderSpans(b.Spans, t)
		text = strings.TrimLeft(text, " \t")
		text = wrap(text, width)
		return t.Paragraph.Render(text)

	case Heading:
		text := renderSpans(b.Spans, t)
		prefix := strings.Repeat("#", b.Level) + " "
		text = wrap(prefix+text, width)
		return t.Heading.Render(text)

	case Blockquote:
		style := t.Quote
		if b.Level > 1 {
			style = t.DeepQuote
		}
		prefix := "> " // single level; structural nesting handles depth
		// Use lipgloss.Width (display cells) not len (bytes) for the
		// prefix deduction so wide-char prefixes don't undercount.
		content := joinRenderedBlocks(b.Blocks, t, width-lipgloss.Width(prefix))
		var lines []string
		for _, line := range strings.Split(content, "\n") {
			if line == "" {
				lines = append(lines, style.Render(">"))
			} else {
				lines = append(lines, style.Render(prefix)+line)
			}
		}
		return strings.Join(lines, "\n")

	case QuoteAttribution:
		text := renderSpans(b.Spans, t)
		text = wrap(text, width)
		return t.Attribution.Render(text)

	case Signature:
		var lines []string
		for _, spans := range b.Lines {
			text := renderSpans(spans, t)
			text = wrap(text, width)
			lines = append(lines, t.Signature.Render(text))
		}
		return strings.Join(lines, "\n")

	case Rule:
		line := strings.Repeat("─", width)
		return t.HorizontalRule.Render(line)

	case CodeBlock:
		return t.CodeBlock.Render(b.Text)

	case Table:
		return renderTable(b, t)

	case ListItem:
		text := renderSpans(b.Spans, t)
		prefix := "- "
		if b.Ordered {
			prefix = string(rune('0'+b.Index%10)) + ". "
		}
		// Use lipgloss.Width (display cells) not len (bytes) for the
		// prefix deduction and indent width.
		prefixW := lipgloss.Width(prefix)
		indent := strings.Repeat(" ", prefixW)
		wrapped := wrap(text, width-prefixW)
		lines := strings.Split(wrapped, "\n")
		for i, line := range lines {
			if i == 0 {
				lines[i] = prefix + line
			} else {
				lines[i] = indent + line
			}
		}
		return t.Paragraph.Render(strings.Join(lines, "\n"))

	default:
		return ""
	}
}

func renderSpans(spans []Span, t *theme.CompiledTheme) string {
	var parts []string
	for _, span := range spans {
		switch s := span.(type) {
		case Text:
			parts = append(parts, s.Content)
		case Bold:
			parts = append(parts, t.Bold.Render(s.Content))
		case Italic:
			parts = append(parts, t.Italic.Render(s.Content))
		case Code:
			parts = append(parts, t.CodeInline.Render(s.Content))
		case Link:
			parts = append(parts, t.Link.Render(s.Text))
		}
	}
	return strings.Join(parts, "")
}

func renderTable(table Table, t *theme.CompiledTheme) string {
	var rows []string

	if len(table.Headers) > 0 {
		var cells []string
		for _, hdr := range table.Headers {
			cells = append(cells, t.Bold.Render(renderSpans(hdr, t)))
		}
		rows = append(rows, strings.Join(cells, " | "))
		var seps []string
		for range table.Headers {
			seps = append(seps, "---")
		}
		rows = append(rows, strings.Join(seps, " | "))
	}

	for _, row := range table.Rows {
		var cells []string
		for _, cell := range row {
			cells = append(cells, renderSpans(cell, t))
		}
		rows = append(rows, strings.Join(cells, " | "))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// RenderHeaders renders parsed headers into a styled string. The
// Subject is hoisted as a title above the From/To/Cc/Bcc/Date block;
// a blank line separates the title from the metadata.
//
// Every styled segment carries t.BgElevated so the panel surface has
// no terminal-default-bg gaps between Render() calls. Without this,
// lipgloss issue #209 leaks through: an outer Background wrapper
// can't refill bg across a child component's embedded \x1b[0m.
func RenderHeaders(h ParsedHeaders, t *theme.CompiledTheme, width int) string {
	bg := lipgloss.NewStyle().Background(t.BgElevated)

	var lines []string
	if h.Subject != "" {
		lines = append(lines,
			t.SubjectTitle.Background(t.BgElevated).Render(wrap(h.Subject, width)),
			"",
		)
	}
	if len(h.From) > 0 {
		lines = append(lines, renderHeaderAddresses("From", h.From, t, width, bg)...)
	}
	if len(h.To) > 0 {
		lines = append(lines, renderHeaderAddresses("To", h.To, t, width, bg)...)
	}
	if len(h.Cc) > 0 {
		lines = append(lines, renderHeaderAddresses("Cc", h.Cc, t, width, bg)...)
	}
	if len(h.Bcc) > 0 {
		lines = append(lines, renderHeaderAddresses("Bcc", h.Bcc, t, width, bg)...)
	}
	if h.Date != "" {
		lines = append(lines, renderHeaderScalar("Date", h.Date, t, bg))
	}
	return strings.Join(lines, "\n")
}

// headerKeyColWidth is the cell width of the label column. The
// longest visible label after the Subject hoist is "date" (4 cells);
// the column stays at 8 cells for alignment headroom and to keep
// values landing where the prior layout placed them.
const headerKeyColWidth = 8

// metadataIndent is the leading whitespace prefix on every metadata
// row (From/To/Cc/Bcc/Date). The two-cell inset reads as a margin
// annotation, distinct from the subject and body which sit flush
// at the pane's existing 1-cell padding.
const metadataIndent = "  "

// metadataPrefixWidth is the display-cell width of the row prefix —
// metadataIndent + headerKeyColWidth + the trailing space — used by
// the wrap accumulator. Computed against raw strings so it stays
// correct after surface-baking wraps the prefix in ANSI.
const metadataPrefixWidth = len(metadataIndent) + headerKeyColWidth + 1

func renderHeaderKey(key string, t *theme.CompiledTheme, bg lipgloss.Style) string {
	label := strings.ToUpper(key)
	pad := headerKeyColWidth - len(label)
	if pad < 0 {
		pad = 0
	}
	return t.HeaderDim.Background(t.BgElevated).Render(label) + bg.Render(strings.Repeat(" ", pad))
}

func renderHeaderScalar(key, value string, t *theme.CompiledTheme, bg lipgloss.Style) string {
	return bg.Render(metadataIndent) +
		renderHeaderKey(key, t, bg) +
		bg.Render(" ") +
		t.HeaderValue.Background(t.BgElevated).Render(value)
}

// visibleAddrWidth returns the printed width of an Address as
// rendered by renderHeaderAddresses. Used by the wrap accumulator to
// decide where to break.
func visibleAddrWidth(a Address) int {
	switch {
	case a.Name != "" && a.Email != "":
		return len(a.Name) + len(a.Email) + 3 // " <" + ">"
	case a.Name != "":
		return len(a.Name)
	default:
		return len(a.Email)
	}
}

func renderHeaderAddresses(key string, addrs []Address, t *theme.CompiledTheme, width int, bg lipgloss.Style) []string {
	headerValue := t.HeaderValue.Background(t.BgElevated)
	headerDim := t.HeaderDim.Background(t.BgElevated)

	keyStr := bg.Render(metadataIndent) + renderHeaderKey(key, t, bg)
	indent := bg.Render(metadataIndent + strings.Repeat(" ", headerKeyColWidth+1))

	formatted := make([]string, len(addrs))
	for i, a := range addrs {
		switch {
		case a.Name != "" && a.Email != "":
			formatted[i] = headerValue.Render(a.Name) + bg.Render(" ") + headerDim.Render("<"+a.Email+">")
		case a.Name != "":
			formatted[i] = headerValue.Render(a.Name)
		default:
			formatted[i] = headerValue.Render(a.Email)
		}
	}

	var lines []string
	current := keyStr + bg.Render(" ")
	currentVisible := metadataPrefixWidth

	for i, addr := range formatted {
		addrVisible := visibleAddrWidth(addrs[i])

		sep := ""
		sepLen := 0
		if i > 0 {
			sep = ", "
			sepLen = 2
		}

		if currentVisible+sepLen+addrVisible > width && i > 0 {
			lines = append(lines, current)
			current = indent + addr
			currentVisible = metadataPrefixWidth + addrVisible
		} else {
			current += bg.Render(sep) + addr
			currentVisible += sepLen + addrVisible
		}
	}
	lines = append(lines, current)

	return lines
}
