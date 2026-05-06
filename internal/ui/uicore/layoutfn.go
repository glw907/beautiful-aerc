// SPDX-License-Identifier: MIT

package uicore

import "math"

// ComputeLayout returns the layout decisions for a given terminal
// width. See ADR-0109 for the formula derivation. Sender slope is
// 0.125 (matched to sender-name coverage cliffs at 22/28/32 cells)
// and sidebar slope is 0.2 (covers 14→30 over W=80→160). Discrete
// thresholds gate the flag column (W>=90), date column (3 cells at
// W>=90, 5 cells at W>=100), and sidebar icons (sidebar>=20, i.e.
// W>=108 with the 0.2 slope and round-half-away-from-zero).
func ComputeLayout(termWidth int) LayoutMode {
	sidebar := clampInt(int(math.Round(14.0+float64(termWidth-80)*0.2)), 14, 30)
	sender := clampInt(int(math.Round(22.0+float64(termWidth-80)*0.125)), 22, 32)
	var date int
	switch {
	case termWidth < 90:
		date = 0
	case termWidth < 100:
		date = 3
	default:
		date = 5
	}
	return LayoutMode{
		Sidebar:    sidebar,
		Sender:     sender,
		Date:       date,
		FlagColumn: termWidth >= 90,
		Icons:      sidebar >= 20,
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
