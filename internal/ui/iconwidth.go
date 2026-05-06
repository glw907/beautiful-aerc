// SPDX-License-Identifier: MIT

package ui

import (
	"github.com/glw907/poplar/internal/ui/uicore"
)

// SetSPUACellWidth sets the per-glyph rendered cell width for SPUA-A
// runes. Must be 1 or 2. Any other value panics. Idempotent.
// Delegates to uicore.SetSPUACellWidth.
func SetSPUACellWidth(w int) {
	uicore.SetSPUACellWidth(w)
}

// spuaCellWidth returns the current SPUA-A cell width. Callers inside
// internal/ui that compare against this value use this function.
func spuaCellWidth() int { return uicore.SPUACellWidth() }

// displayCells returns the actual terminal display width of s.
// Delegates to uicore.DisplayCells.
func displayCells(s string) int { return uicore.DisplayCells(s) }

// spuaCount counts SPUA-A runes in s.
// Delegates to uicore.SpuaCount.
func spuaCount(s string) int { return uicore.SpuaCount(s) }

// displayTruncateEllipsis truncates s to fit n cells with '…'.
// Delegates to uicore.DisplayTruncateEllipsis.
func displayTruncateEllipsis(s string, n int) string {
	return uicore.DisplayTruncateEllipsis(s, n)
}

// displayTruncate truncates the ANSI string s to at most n terminal
// display cells. Delegates to uicore.DisplayTruncate.
func displayTruncate(s string, n int) string { return uicore.DisplayTruncate(s, n) }

// displayPadOrTruncate pads or truncates s to exactly n display cells.
// Delegates to uicore.DisplayPadOrTruncate.
func displayPadOrTruncate(s string, n int) string { return uicore.DisplayPadOrTruncate(s, n) }
