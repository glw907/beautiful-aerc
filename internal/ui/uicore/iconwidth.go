package uicore

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Nerd Font icons live in the Supplementary Private Use Area-A
// (U+F0000–U+FFFFD). Their rendered cell width depends on the
// terminal+font+symbol_map configuration. We set spuaCellWidth at
// startup from term.MeasureSPUACells(). See ADR-0084.
//
// In simple mode (no Nerd Font icons present in rendered strings) the
// value is 1 and DisplayCells degenerates to lipgloss.Width.
const (
	SpuaAStart = 0xF0000
	SpuaAEnd   = 0xFFFFD
)

var spuaCellWidth = 1

// SetSPUACellWidth sets the per-glyph rendered cell width for SPUA-A
// runes. Must be 1 or 2. Any other value panics. Idempotent.
func SetSPUACellWidth(w int) {
	if w != 1 && w != 2 {
		panic("uicore: SetSPUACellWidth requires 1 or 2")
	}
	spuaCellWidth = w
}

func SPUACellWidth() int { return spuaCellWidth }

// DisplayCells returns the actual terminal display width of s, given
// the runtime-determined SPUA-A cell width.
func DisplayCells(s string) int {
	w := lipgloss.Width(s)
	if spuaCellWidth == 1 {
		return w
	}
	return w + (spuaCellWidth-1)*SpuaCount(s)
}

// SpuaCount counts SPUA-A runes in s. Fast-paths plain ASCII via a
// byte scan: SPUA-A codepoints are 4-byte UTF-8 sequences, so a string
// with no high-bit byte cannot contain one.
func SpuaCount(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return spuaCountSlow(s)
		}
	}
	return 0
}

func spuaCountSlow(s string) int {
	n := 0
	for _, r := range s {
		if r >= SpuaAStart && r <= SpuaAEnd {
			n++
		}
	}
	return n
}

// DisplayTruncateEllipsis truncates s to fit n cells, with '…' as
// the final cell when truncation occurred. The n==1 branch returns
// the bare '…' so callers always get a single-cell sentinel rather
// than an empty string when budget is too tight for any payload.
func DisplayTruncateEllipsis(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if DisplayCells(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return DisplayTruncate(s, n-1) + "…"
}

// DisplayTruncate truncates the ANSI string s to at most n terminal
// display cells. ansi.Truncate uses runewidth internally and undercounts
// SPUA-A by (spuaCellWidth-1) per glyph. This wrapper decrements the
// runewidth limit until the result is within n cells. At most
// (spuaCellWidth-1)*SpuaCount(s) iterations.
func DisplayTruncate(s string, n int) string {
	limit := n
	for {
		t := ansi.Truncate(s, limit, "")
		if DisplayCells(t) <= n {
			return t
		}
		limit--
		if limit < 0 {
			return ""
		}
	}
}

// DisplayPadOrTruncate pads or truncates s to exactly n display cells.
// Use for icon-bearing strings. PadOrTruncate's lipgloss.Width call
// undercounts SPUA-A glyphs.
func DisplayPadOrTruncate(s string, n int) string {
	w := DisplayCells(s)
	if w == n {
		return s
	}
	if w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return DisplayTruncate(s, n)
}

// ApplyBg layers the background of bgStyle onto base. Used by row
// renderers (sidebar, message list) to compose a foreground style
// with the row's background color without clobbering already-rendered
// ANSI segments.
func ApplyBg(base, bgStyle lipgloss.Style) lipgloss.Style {
	if bg, ok := bgStyle.GetBackground().(lipgloss.Color); ok {
		return base.Background(bg)
	}
	return base
}

// FillRowToWidth fits a fully-rendered row of ANSI segments to
// exactly width display cells. Short rows are right-padded with
// bgStyle so the row's background extends to the panel edge, over-
// wide rows are truncated to width. Shared by sidebar and message
// list row renderers.
//
// Width is measured with DisplayCells so Nerd Font SPUA-A icons are
// counted at their true 2-cell width.
func FillRowToWidth(row string, width int, bgStyle lipgloss.Style) string {
	rw := DisplayCells(row)
	if rw < width {
		return row + bgStyle.Render(strings.Repeat(" ", width-rw))
	}
	if rw > width {
		return DisplayTruncate(row, width)
	}
	return row
}
