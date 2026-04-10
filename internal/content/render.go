package content

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

const maxBodyWidth = 78

// RenderBody renders blocks into a styled string using lipgloss.
// Width is capped at maxBodyWidth for readability.
func RenderBody(blocks []Block, t *theme.CompiledTheme, width int) string {
	w := width
	if w > maxBodyWidth {
		w = maxBodyWidth
	}

	var sections []string
	for _, block := range blocks {
		sections = append(sections, renderBlock(block, t, w))
	}
	return strings.Join(sections, "\n\n")
}

func renderBlock(block Block, t *theme.CompiledTheme, width int) string {
	switch b := block.(type) {
	case Paragraph:
		text := renderSpans(b.Spans, t)
		text = ansi.Wordwrap(text, width, "")
		return t.Paragraph.Render(text)

	case Heading:
		text := renderSpans(b.Spans, t)
		prefix := strings.Repeat("#", b.Level) + " "
		text = ansi.Wordwrap(prefix+text, width, "")
		return t.Heading.Render(text)

	case Blockquote:
		style := t.Quote
		if b.Level > 1 {
			style = t.DeepQuote
		}
		prefix := "> " // single level; structural nesting handles depth
		var inner []string
		for _, child := range b.Blocks {
			inner = append(inner, renderBlock(child, t, width-len(prefix)))
		}
		content := strings.Join(inner, "\n\n")
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
		text = ansi.Wordwrap(text, width, "")
		return t.Attribution.Render(text)

	case Signature:
		var lines []string
		for _, spans := range b.Lines {
			text := renderSpans(spans, t)
			text = ansi.Wordwrap(text, width, "")
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
		text = ansi.Wordwrap(prefix+text, width, "")
		return t.Paragraph.Render(text)

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

// RenderHeaders renders parsed headers into a styled string.
// Headers use the full terminal width for address wrapping.
func RenderHeaders(h ParsedHeaders, t *theme.CompiledTheme, width int) string {
	var lines []string

	if len(h.From) > 0 {
		lines = append(lines, renderHeaderAddresses("From", h.From, t, width)...)
	}
	if len(h.To) > 0 {
		lines = append(lines, renderHeaderAddresses("To", h.To, t, width)...)
	}
	if len(h.Cc) > 0 {
		lines = append(lines, renderHeaderAddresses("Cc", h.Cc, t, width)...)
	}
	if len(h.Bcc) > 0 {
		lines = append(lines, renderHeaderAddresses("Bcc", h.Bcc, t, width)...)
	}
	if h.Date != "" {
		lines = append(lines, renderHeaderScalar("Date", h.Date, t))
	}
	if h.Subject != "" {
		lines = append(lines, renderHeaderScalar("Subject", h.Subject, t))
	}

	sep := t.HeaderDim.Render(strings.Repeat("─", width))
	lines = append(lines, sep)

	return strings.Join(lines, "\n")
}

func renderHeaderScalar(key, value string, t *theme.CompiledTheme) string {
	return t.HeaderKey.Render(key+":") + " " + t.HeaderValue.Render(value)
}

func renderHeaderAddresses(key string, addrs []Address, t *theme.CompiledTheme, width int) []string {
	keyStr := t.HeaderKey.Render(key + ":")
	indent := strings.Repeat(" ", len(key)+2)

	var formatted []string
	for _, a := range addrs {
		if a.Name != "" {
			formatted = append(formatted, fmt.Sprintf("%s %s",
				t.HeaderValue.Render(a.Name),
				t.HeaderDim.Render("<"+a.Email+">")))
		} else {
			formatted = append(formatted, t.HeaderValue.Render(a.Email))
		}
	}

	var lines []string
	current := keyStr + " "
	currentVisible := len(key) + 2

	for i, addr := range formatted {
		addrVisible := len(addrs[i].Name) + len(addrs[i].Email) + 3
		if addrs[i].Name == "" {
			addrVisible = len(addrs[i].Email)
		}

		sep := ""
		sepLen := 0
		if i > 0 {
			sep = ", "
			sepLen = 2
		}

		if currentVisible+sepLen+addrVisible > width && i > 0 {
			lines = append(lines, current)
			current = indent + addr
			currentVisible = len(indent) + addrVisible
		} else {
			current += sep + addr
			currentVisible += sepLen + addrVisible
		}
	}
	lines = append(lines, current)

	return lines
}
