// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"
)

func TestPlaceOverlay(t *testing.T) {
	t.Run("plain text overlay over plain text background", func(t *testing.T) {
		bg := "AAAAAAAAAA\nBBBBBBBBBB\nCCCCCCCCCC"
		fg := "XY\nXY"
		got := PlaceOverlay(2, 1, fg, bg)
		lines := strings.Split(got, "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
		}
		// Row 0 unchanged.
		if lines[0] != "AAAAAAAAAA" {
			t.Errorf("row 0 = %q, want %q", lines[0], "AAAAAAAAAA")
		}
		// Row 1: BB then XY then BBBBBB.
		if !strings.HasPrefix(lines[1], "BB") {
			t.Errorf("row 1 should start with BB: %q", lines[1])
		}
		if !strings.Contains(lines[1], "XY") {
			t.Errorf("row 1 should contain XY: %q", lines[1])
		}
		// Row 2 also gets the fg overlay (fg spans rows 1 and 2).
		if !strings.Contains(lines[2], "XY") {
			t.Errorf("row 2 should contain XY (second fg row): %q", lines[2])
		}
		// Outside-overlay bg content should still be present (CC prefix).
		if !strings.HasPrefix(lines[2], "CC") {
			t.Errorf("row 2 should still start with CC: %q", lines[2])
		}
	})

	t.Run("ANSI-styled overlay over ANSI-styled background — bg ANSI preserved outside overlay", func(t *testing.T) {
		// Blue background rows, red overlay.
		blue := "\x1b[34m"
		red := "\x1b[31m"
		reset := "\x1b[0m"
		bg := blue + "AAAAAAAAAA" + reset + "\n" + blue + "BBBBBBBBBB" + reset
		fg := red + "XY" + reset

		got := PlaceOverlay(0, 0, fg, bg)
		lines := strings.Split(got, "\n")

		// Row 0 should contain the red overlay bytes.
		if !strings.Contains(lines[0], red) {
			t.Errorf("row 0 missing red ANSI: %q", lines[0])
		}
		// Row 1 (outside overlay) should preserve the blue ANSI.
		if !strings.Contains(lines[1], blue) {
			t.Errorf("row 1 (outside overlay) missing blue ANSI: %q", lines[1])
		}
	})

	t.Run("off-edge overlay clips without panic", func(t *testing.T) {
		bg := "AAAA\nBBBB"
		// Overlay wider than background — PlaceOverlay should clamp and not panic.
		fg := "XXXXXXXXXXXX" // 12 cells wide, bg is 4
		got := PlaceOverlay(0, 0, fg, bg)
		// Since fg is wider than bg in one dimension, fg is returned directly.
		if got != fg && !strings.Contains(got, "XXXX") {
			t.Errorf("off-edge overlay returned unexpected result: %q", got)
		}
	})

	t.Run("overlay at x=0 y=0 replaces top-left", func(t *testing.T) {
		bg := "0123456789\n0123456789"
		fg := "AB\nCD"
		got := PlaceOverlay(0, 0, fg, bg)
		lines := strings.Split(got, "\n")
		if !strings.HasPrefix(lines[0], "AB") {
			t.Errorf("row 0 should start with AB: %q", lines[0])
		}
		if !strings.HasPrefix(lines[1], "CD") {
			t.Errorf("row 1 should start with CD: %q", lines[1])
		}
		// Right side of bg preserved.
		if !strings.Contains(lines[0], "23456789") {
			t.Errorf("row 0 bg right side missing: %q", lines[0])
		}
	})
}
