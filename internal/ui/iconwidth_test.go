// SPDX-License-Identifier: MIT

package ui

import "testing"

func TestDisplayCells(t *testing.T) {
	// SPUA-A test glyph: U+F01EE.
	const glyph = "\U000F01EE"

	tests := []struct {
		name      string
		cellWidth int
		in        string
		want      int
	}{
		{"ascii w=1", 1, "abc", 3},
		{"ascii w=2", 2, "abc", 3},
		{"empty w=1", 1, "", 0},
		{"empty w=2", 2, "", 0},
		{"glyph alone w=1", 1, glyph, 1},
		{"glyph alone w=2", 2, glyph, 2},
		{"glyph + ascii w=1", 1, "x" + glyph + "y", 3},
		{"glyph + ascii w=2", 2, "x" + glyph + "y", 4},
		{"two glyphs w=2", 2, glyph + glyph, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetSPUACellWidth(tt.cellWidth)
			defer SetSPUACellWidth(1) // restore default for other tests
			got := displayCells(tt.in)
			if got != tt.want {
				t.Errorf("displayCells(%q) @ w=%d = %d, want %d", tt.in, tt.cellWidth, got, tt.want)
			}
		})
	}
}

func TestSetSPUACellWidthRejectsBadValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("SetSPUACellWidth(3) should panic")
		}
		SetSPUACellWidth(1) // restore
	}()
	SetSPUACellWidth(3)
}
