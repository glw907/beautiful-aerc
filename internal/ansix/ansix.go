// Package ansix is poplar's SPUA-aware width-math layer over
// charmbracelet/x/ansi. Nerd Font icons live in Supplementary Private Use
// Area-A; their rendered cell width is terminal+font+symbol_map dependent
// and lipgloss/displaywidth measure them as 1 cell. cmd/poplar resolves
// the cell width at startup via term.MeasureSPUACells and constructs a
// Measurer once; subpackage models hold their own copy and use the
// methods here in place of the lipgloss equivalents whenever a string
// may contain icon glyphs. ADR-0084, ADR-0181, ADR-0209.
package ansix

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	spuaAStart = 0xF0000
	spuaAEnd   = 0xFFFFD
)

// Measurer carries the resolved per-glyph cell width for SPUA-A
// runes. The zero value behaves as cellWidth=1 (simple mode), so
// tests that don't care about icon widths can construct one with
// var m Measurer.
type Measurer struct {
	cellWidth int
}

// NewMeasurer returns a Measurer with the given cell width.
// w must be 1 or 2.
func NewMeasurer(w int) Measurer {
	if w != 1 && w != 2 {
		panic("ansix: NewMeasurer requires 1 or 2")
	}
	return Measurer{cellWidth: w}
}

// CellWidth returns the configured cell width (1 or 2).
func (m Measurer) CellWidth() int {
	if m.cellWidth == 0 {
		return 1
	}
	return m.cellWidth
}

// Width returns the terminal display width of s in cells, with SPUA-A
// glyphs counted at the runtime-resolved width.
func (m Measurer) Width(s string) int {
	w := lipgloss.Width(s)
	cw := m.CellWidth()
	if cw == 1 {
		return w
	}
	return w + (cw-1)*SpuaCount(s)
}

// SpuaCount reports the number of SPUA-A runes in s. The count is
// independent of cell width, so it lives as a package function.
func SpuaCount(s string) int {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return spuaCountSlow(s)
		}
	}
	return 0
}

func spuaCountSlow(s string) int {
	n := 0
	for _, r := range s {
		if r >= spuaAStart && r <= spuaAEnd {
			n++
		}
	}
	return n
}

// TruncateEllipsis truncates s to fit n cells with '…' as the final cell
// when truncation occurred. n==1 returns the bare '…' so callers get a
// single-cell sentinel even when the budget is too tight for any payload.
func (m Measurer) TruncateEllipsis(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if m.Width(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return m.Truncate(s, n-1) + "…"
}

// Truncate is the SPUA-aware analogue of ansi.Truncate: at most n display
// cells, no ellipsis appended.
func (m Measurer) Truncate(s string, n int) string {
	// ansi.Truncate measures with runewidth, so SPUA-A glyphs come back
	// undercounted by (cellWidth-1) cells each. Step the runewidth
	// budget down until the result fits.
	limit := n
	for {
		t := ansi.Truncate(s, limit, "")
		if m.Width(t) <= n {
			return t
		}
		limit--
		if limit < 0 {
			return ""
		}
	}
}

// PadOrTruncate fits s to exactly n cells, right-padding with spaces or
// truncating without an ellipsis.
func (m Measurer) PadOrTruncate(s string, n int) string {
	w := m.Width(s)
	if w == n {
		return s
	}
	if w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return m.Truncate(s, n)
}
