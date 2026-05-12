package content

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/theme"
)

// nbsp is the no-break space wordwrap will not split.
const nbsp = " "

// FootnoteRow locates one ribbon entry in the rendered output.
// Row is the 0-based line index of the entry's first line (the
// "[^N]: " line; wrap continuations are not click targets).
// PickerIndex indexes the URL in the returned picker slice, which
// spans every harvested URL, not just marker-bearing ones.
type FootnoteRow struct {
	Row         int
	PickerIndex int
}

// RenderBodyWithFootnotes renders blocks and harvests outbound URLs.
// The picker list spans every URL in first-seen order. The footnote
// section spans only URLs that received a [^N] marker, so short bare
// URLs appear in the picker but not in the footnote list (ADR-0086).
// The returned footnotes slice carries the click-target row for each
// ribbon entry.
func RenderBodyWithFootnotes(blocks []Block, t *theme.CompiledTheme, width int) (string, []string, []FootnoteRow) {
	rewritten, pickerURLs, hasMarker := harvestFootnotes(blocks)
	body := RenderBody(rewritten, t, width)
	if len(pickerURLs) == 0 {
		return body, pickerURLs, nil
	}

	// [^N] labels index marker-bearing URLs only. The picker list
	// spans every URL, so the two index spaces differ.
	type marker struct {
		url    string
		picker int
	}
	var markers []marker
	for i, u := range pickerURLs {
		if hasMarker[i] {
			markers = append(markers, marker{u, i})
		}
	}

	if len(markers) == 0 {
		return body, pickerURLs, nil
	}

	w := width
	if w > maxBodyWidth {
		w = maxBodyWidth
	}

	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(t.HorizontalRule.Render(strings.Repeat("─", w)))
	row := lipgloss.Height(body) + 1
	footnotes := make([]FootnoteRow, 0, len(markers))
	for i, m := range markers {
		row++
		footnotes = append(footnotes, FootnoteRow{Row: row, PickerIndex: m.picker})
		b.WriteString("\n")
		label := fmt.Sprintf("[^%d]: %s", i+1, m.url)
		wrapped := wrap(label, w)
		b.WriteString(t.Link.Render(wrapped))
		row += lipgloss.Height(wrapped) - 1
	}
	return b.String(), pickerURLs, footnotes
}

// harvestFootnotes returns a deep-rewritten block slice, the ordered
// picker URL list (deduped, first-seen), and a parallel hasMarker
// slice. hasMarker[i] is true when urls[i] has a [^N] marker glued
// to it in the body. Short bare URLs sit in urls with hasMarker
// false and render inline without a marker or footnote line.
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

// markerFor registers url in the picker list and returns its 1-based
// index. The caller decides whether to flip hasMarker[idx-1] to true.
func (w *footnoteWalker) markerFor(url string) int {
	if n, ok := w.seen[url]; ok {
		return n
	}
	n := len(w.urls) + 1
	w.urls = append(w.urls, url)
	w.hasMarker = append(w.hasMarker, false)
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

// longBareURLThreshold is the display-cell width above which a bare
// URL is footnoted instead of left inline.
const longBareURLThreshold = 30

// markerLabel registers url as a marker-bearing entry and returns its
// [^N] label, where N counts marker-bearing entries in picker order.
// [^1] is the first marker-bearing URL even when short bare URLs
// precede it in the picker list. A short-bare-first occurrence gets
// promoted to hasMarker on the first marker use.
func (w *footnoteWalker) markerLabel(url string) string {
	n := w.markerFor(url)
	w.hasMarker[n-1] = true
	m := 0
	for i := range n {
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
			label := w.markerLabel(link.URL)
			out[i] = Link{Text: link.Text + nbsp + label, URL: link.URL}
		case lipgloss.Width(link.URL) > longBareURLThreshold:
			label := w.markerLabel(link.URL)
			out[i] = Link{Text: trimURL(link.URL) + nbsp + label, URL: link.URL}
		default:
			// Short bare URLs: register for the picker, render inline.
			w.markerFor(link.URL)
			out[i] = s
		}
	}
	return out
}
