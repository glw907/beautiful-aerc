package content

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// nbsp is the no-break space wordwrap will not split.
const nbsp = " "

// RenderBodyWithFootnotes renders blocks and harvests outbound URLs.
// The picker list spans every URL in first-seen order. The footnote
// section spans only URLs that received a [^N] marker, so short bare
// URLs appear in the picker but not the footnote list. See ADR-0086.
func RenderBodyWithFootnotes(blocks []Block, t *theme.CompiledTheme, width int) (string, []string) {
	rewritten, pickerURLs, hasMarker := harvestFootnotes(blocks)
	body := RenderBody(rewritten, t, width)
	if len(pickerURLs) == 0 {
		return body, pickerURLs
	}

	// [^N] labels index marker-bearing URLs only. The picker list spans
	// all URLs including short bare ones, so the two index spaces differ.
	var markerURLs []string
	for i, u := range pickerURLs {
		if hasMarker[i] {
			markerURLs = append(markerURLs, u)
		}
	}

	if len(markerURLs) == 0 {
		// Only short bare URLs present. No rule, no footnote section.
		return body, pickerURLs
	}

	w := width
	if w > maxBodyWidth {
		w = maxBodyWidth
	}

	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(t.HorizontalRule.Render(strings.Repeat("─", w)))
	for i, u := range markerURLs {
		b.WriteString("\n")
		// Wrap before styling: a long URL is an unbreakable token that
		// wordwrap cannot split. Hardwrap catches it so no output line
		// exceeds the width budget.
		label := fmt.Sprintf("[^%d]: %s", i+1, u)
		b.WriteString(t.Link.Render(wrap(label, w)))
	}
	return b.String(), pickerURLs
}

// harvestFootnotes returns a deep-rewritten block slice, the ordered
// picker URL list (all outbound URLs, deduped, first-seen order), and
// a parallel hasMarker slice. hasMarker[i] is true when urls[i] has a
// [^N] marker glued to it in the body. Short bare URLs are in urls but
// hasMarker[i] is false. They render inline, no marker, no footnote line.
func harvestFootnotes(blocks []Block) ([]Block, []string, []bool) {
	w := footnoteWalker{seen: make(map[string]int)}
	out := w.blocks(blocks)
	return out, w.urls, w.hasMarker
}

type footnoteWalker struct {
	seen      map[string]int
	urls      []string
	hasMarker []bool
}

// markerFor registers url in the picker list (if not already seen) and
// returns its 1-based index. The caller decides whether to flip
// hasMarker[idx-1] to true.
func (w *footnoteWalker) markerFor(url string) int {
	if n, ok := w.seen[url]; ok {
		return n
	}
	n := len(w.urls) + 1
	w.urls = append(w.urls, url)
	w.hasMarker = append(w.hasMarker, false) // default. Caller flips to true if needed.
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

// markerLabel registers url as a marker-bearing entry and returns the
// [^N] label where N is its position in the footnote-subset (not the
// picker list). Footnote-subset numbering counts only marker-bearing
// entries in picker order. [^1] is the first marker-bearing URL
// even when short bare URLs precede it in the picker list.
//
// If url was already registered (first occurrence was short bare),
// its hasMarker entry is promoted to true on the first marker use.
func (w *footnoteWalker) markerLabel(url string) string {
	n := w.markerFor(url) // registers if new. Returns 1-based picker index.
	w.hasMarker[n-1] = true
	// Count marker-bearing entries up to and including position n-1.
	m := 0
	for i := 0; i < n; i++ {
		if w.hasMarker[i] {
			m++
		}
	}
	return fmt.Sprintf("[^%d]", m)
}

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
			// Markdown link: register with marker, label by footnote-subset index.
			label := w.markerLabel(link.URL)
			out[i] = Link{Text: link.Text + nbsp + label, URL: link.URL}
		case lipgloss.Width(link.URL) > longBareURLThreshold:
			// Long bare URL: trimmed inline form + marker, labeled by footnote-subset index.
			label := w.markerLabel(link.URL)
			out[i] = Link{Text: trimURL(link.URL) + nbsp + label, URL: link.URL}
		default:
			// Short bare URL: register in picker list (hasMarker stays false).
			// Pass span through unchanged. Renders inline with no marker and no footnote line.
			w.markerFor(link.URL)
			out[i] = s
		}
	}
	return out
}
