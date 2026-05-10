// Package ansix is poplar's SPUA-aware width-math layer over
// charmbracelet/x/ansi. Nerd Font icons live in Supplementary Private Use
// Area-A; their rendered cell width is terminal+font+symbol_map dependent
// and lipgloss/displaywidth measure them as 1 cell. Callers set the
// resolved cell width once at startup via SetSPUACellWidth (see
// term.MeasureSPUACells) and use the helpers here in place of the lipgloss
// equivalents whenever a string may contain icon glyphs. ADR-0084,
// ADR-0181.
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

var spuaCellWidth = 1

// SetSPUACellWidth records the per-glyph cell width measured at startup.
// Must be 1 or 2.
func SetSPUACellWidth(w int) {
	if w != 1 && w != 2 {
		panic("ansix: SetSPUACellWidth requires 1 or 2")
	}
	spuaCellWidth = w
}

func SPUACellWidth() int { return spuaCellWidth }

// Width returns the terminal display width of s in cells, with SPUA-A
// glyphs counted at the runtime-resolved width.
func Width(s string) int {
	w := lipgloss.Width(s)
	if spuaCellWidth == 1 {
		return w
	}
	return w + (spuaCellWidth-1)*SpuaCount(s)
}

// SpuaCount reports the number of SPUA-A runes in s.
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
		if r >= spuaAStart && r <= spuaAEnd {
			n++
		}
	}
	return n
}

// TruncateEllipsis truncates s to fit n cells with '…' as the final cell
// when truncation occurred. n==1 returns the bare '…' so callers get a
// single-cell sentinel even when the budget is too tight for any payload.
func TruncateEllipsis(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if Width(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return Truncate(s, n-1) + "…"
}

// Truncate is the SPUA-aware analogue of ansi.Truncate: at most n display
// cells, no ellipsis appended.
func Truncate(s string, n int) string {
	// ansi.Truncate measures with runewidth, so SPUA-A glyphs come back
	// undercounted by (spuaCellWidth-1) cells each. Step the runewidth
	// budget down until the result fits.
	limit := n
	for {
		t := ansi.Truncate(s, limit, "")
		if Width(t) <= n {
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
func PadOrTruncate(s string, n int) string {
	w := Width(s)
	if w == n {
		return s
	}
	if w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return Truncate(s, n)
}
