// SPDX-License-Identifier: MIT

package content

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// nbsp is the no-break space wordwrap will not split.
const nbsp = " "

// RenderBodyWithFootnotes renders blocks via RenderBody and harvests
// outbound links into a numbered footnote list. Returns the rendered
// body (with `[^N]` markers glued to each link's last word) and the
// ordered URL list (indexed 1..len for caller-side dispatch).
//
// Auto-linked bare URLs (where Link.Text == Link.URL) are skipped:
// they render inline in link style with no marker, and they do not
// occupy a footnote slot.
//
// Duplicate URLs share a footnote number; only the first occurrence
// adds to the URL list.
func RenderBodyWithFootnotes(blocks []Block, t *theme.CompiledTheme, width int) (string, []string) {
	rewritten, urls := harvestFootnotes(blocks)
	body := RenderBody(rewritten, t, width)
	if len(urls) == 0 {
		return body, urls
	}

	w := width
	if w > maxBodyWidth {
		w = maxBodyWidth
	}

	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(t.HorizontalRule.Render(strings.Repeat("─", w)))
	for i, u := range urls {
		b.WriteString("\n")
		// Wrap before styling: a long URL is an unbreakable token that
		// wordwrap cannot split; hardwrap catches it so no output line
		// exceeds the width budget.
		label := fmt.Sprintf("[^%d]: %s", i+1, u)
		b.WriteString(t.Link.Render(wrap(label, w)))
	}
	return b.String(), urls
}

// harvestFootnotes returns a deep-rewritten block slice where each
// non-auto-linked Link span has ` [^N]` appended to its Text,
// and the ordered URL list (deduped, first-seen order).
func harvestFootnotes(blocks []Block) ([]Block, []string) {
	w := footnoteWalker{seen: make(map[string]int)}
	out := w.blocks(blocks)
	return out, w.urls
}

type footnoteWalker struct {
	seen map[string]int
	urls []string
}

func (w *footnoteWalker) markerFor(url string) int {
	if n, ok := w.seen[url]; ok {
		return n
	}
	n := len(w.urls) + 1
	w.urls = append(w.urls, url)
	w.seen[url] = n
	return n
}

func (w *footnoteWalker) blocks(in []Block) []Block {
	if len(in) == 0 {
		return in
	}
	out := make([]Block, len(in))
	for i, b := range in {
		out[i] = w.block(b)
	}
	return out
}

func (w *footnoteWalker) block(b Block) Block {
	switch v := b.(type) {
	case Paragraph:
		return Paragraph{Spans: w.spans(v.Spans)}
	case Heading:
		return Heading{Spans: w.spans(v.Spans), Level: v.Level}
	case Blockquote:
		return Blockquote{Blocks: w.blocks(v.Blocks), Level: v.Level}
	case QuoteAttribution:
		return QuoteAttribution{Spans: w.spans(v.Spans)}
	case Signature:
		lines := make([][]Span, len(v.Lines))
		for i, line := range v.Lines {
			lines[i] = w.spans(line)
		}
		return Signature{Lines: lines}
	case ListItem:
		return ListItem{Spans: w.spans(v.Spans), Ordered: v.Ordered, Index: v.Index}
	case Table:
		headers := make([][]Span, len(v.Headers))
		for i, h := range v.Headers {
			headers[i] = w.spans(h)
		}
		rows := make([][][]Span, len(v.Rows))
		for i, row := range v.Rows {
			rows[i] = make([][]Span, len(row))
			for j, cell := range row {
				rows[i][j] = w.spans(cell)
			}
		}
		return Table{Headers: headers, Rows: rows}
	default:
		return b
	}
}

// longBareURLThreshold is the display-cell width above which a bare URL
// gets the long-URL footnote treatment instead of inline pass-through.
const longBareURLThreshold = 30

func (w *footnoteWalker) spans(in []Span) []Span {
	if len(in) == 0 {
		return in
	}
	out := make([]Span, len(in))
	for i, s := range in {
		link, ok := s.(Link)
		if !ok || link.URL == "" {
			out[i] = s
			continue
		}
		switch {
		case link.Text != link.URL:
			n := w.markerFor(link.URL)
			out[i] = Link{Text: link.Text + nbsp + fmt.Sprintf("[^%d]", n), URL: link.URL}
		case lipgloss.Width(link.URL) > longBareURLThreshold:
			n := w.markerFor(link.URL)
			out[i] = Link{Text: trimURL(link.URL) + nbsp + fmt.Sprintf("[^%d]", n), URL: link.URL}
		default:
			out[i] = s
		}
	}
	return out
}
