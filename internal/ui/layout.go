// SPDX-License-Identifier: MIT

package ui

import "math"

// LayoutMode is the resolved set of layout decisions for a given
// terminal width. Computed once per WindowSizeMsg by ComputeLayout
// and threaded into Sidebar and MessageList. Pure data, no I/O,
// no side effects, no methods that change observable behavior
// based on flags outside the struct.
type LayoutMode struct {
	// Sidebar is the sidebar width in cells (folder list). Range
	// [14, 30]. Floor 14 fits "Archive" with icons off. Ceiling
	// 30 gives breathing room for nested custom folder names.
	Sidebar int

	// Sender is the message-list sender column width in cells.
	// Range [22, 32]. Floor 22 covers ~86% of real sender names
	// untruncated. Ceiling 32 covers ~95%.
	Sender int

	// Date is the message-list date column width in cells.
	//   0: column hidden
	//   3: compact relative format ("now", "5m", "Apr", "'24")
	//   5: short absolute format ("04-30" / "3:04p")
	Date int

	// FlagColumn is true when the message-list flag/status icon
	// column is rendered. When false, the column is omitted
	// entirely (saves 4 cells: glyph + inter-column gap).
	FlagColumn bool

	// Icons is true when the sidebar renders folder icons. When
	// false, the icon block is omitted from each sidebar row,
	// shrinking the lead from 8 (fancy) to 2 cells.
	Icons bool
}

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
