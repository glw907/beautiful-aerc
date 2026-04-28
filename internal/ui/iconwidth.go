package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Nerd Font icons live in the Supplementary Private Use Area-A
// (U+F0000–U+FFFFD). Their rendered cell width depends on the
// terminal+font+symbol_map configuration. We set spuaCellWidth at
// startup from term.MeasureSPUACells(); see ADR-0084.
//
// In simple mode (no Nerd Font icons present in rendered strings) the
// value is 1 and displayCells degenerates to lipgloss.Width.
const (
	spuaAStart = 0xF0000
	spuaAEnd   = 0xFFFFD
)

var spuaCellWidth = 1

// SetSPUACellWidth sets the per-glyph rendered cell width for SPUA-A
// runes. Must be 1 or 2; any other value panics. Idempotent.
func SetSPUACellWidth(w int) {
	if w != 1 && w != 2 {
		panic("ui: SetSPUACellWidth requires 1 or 2")
	}
	spuaCellWidth = w
}

// displayCells returns the actual terminal display width of s, given
// the runtime-determined SPUA-A cell width.
func displayCells(s string) int {
	w := lipgloss.Width(s)
	if spuaCellWidth == 1 {
		return w
	}
	return w + (spuaCellWidth-1)*spuaCount(s)
}

// spuaCount counts SPUA-A runes in s. Fast-paths plain ASCII via a
// byte scan: SPUA-A codepoints are 4-byte UTF-8 sequences, so a string
// with no high-bit byte cannot contain one.
func spuaCount(s string) int {
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

// displayTruncate truncates the ANSI string s to at most n terminal
// display cells. ansi.Truncate uses runewidth internally and undercounts
// SPUA-A by (spuaCellWidth-1) per glyph; this wrapper decrements the
// runewidth limit until the result is within n cells. At most
// (spuaCellWidth-1)*spuaCount(s) iterations.
func displayTruncate(s string, n int) string {
	limit := n
	for {
		t := ansi.Truncate(s, limit, "")
		if displayCells(t) <= n {
			return t
		}
		limit--
		if limit < 0 {
			return ""
		}
	}
}
