package uicore

import (
	"strings"
	"testing"
)

func TestPadOrTruncate(t *testing.T) {
	cases := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"exact fit", "hello", 5, "hello"},
		{"pad short", "hi", 5, "hi   "},
		{"truncate long", "hello world", 8, "hello w…"},
		{"empty to width", "", 4, "    "},
		{"zero width", "hello", 0, ""},
		{"single cell truncate", "hello", 1, "…"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := PadOrTruncate(tc.input, tc.width)
			if got != tc.want {
				t.Errorf("PadOrTruncate(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
		})
	}
}

func TestTruncateToWidth(t *testing.T) {
	cases := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"fits", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncate", "hello world", 8, "hello w…"},
		{"zero width", "hello", 0, ""},
		{"one cell", "hello", 1, "…"},
		{"two cells", "hello", 2, "h…"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateToWidth(tc.input, tc.width)
			if got != tc.want {
				t.Errorf("TruncateToWidth(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
		})
	}
}

func TestClampScrollOffset(t *testing.T) {
	cases := []struct {
		name    string
		cursor  int
		visible int
		offset  int
		want    int
	}{
		{"cursor in window", 3, 5, 2, 2},
		{"cursor before window", 1, 5, 3, 1},
		{"cursor after window", 8, 5, 2, 4},
		{"cursor at bottom edge", 6, 5, 2, 2},
		{"cursor at top edge", 2, 5, 2, 2},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ClampScrollOffset(tc.cursor, tc.visible, tc.offset)
			if got != tc.want {
				t.Errorf("ClampScrollOffset(%d, %d, %d) = %d, want %d",
					tc.cursor, tc.visible, tc.offset, got, tc.want)
			}
		})
	}
}

func TestCenterOverlay(t *testing.T) {
	cases := []struct {
		name   string
		box    string
		totalW int
		totalH int
		wantX  int
		wantY  int
	}{
		{
			name:   "centered 10x3 in 80x24",
			box:    strings.Repeat("─", 10) + "\n" + strings.Repeat("─", 10) + "\n" + strings.Repeat("─", 10),
			totalW: 80,
			totalH: 24,
			wantX:  35,
			wantY:  10,
		},
		{
			// x clamps to 0 when the overlay is wider than the terminal. y still centers.
			name:   "overlay wider than terminal clamps x to zero",
			box:    strings.Repeat("─", 100),
			totalW: 80,
			totalH: 24,
			wantX:  0,
			wantY:  11,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			x, y := CenterOverlay(tc.box, tc.totalW, tc.totalH)
			if x != tc.wantX || y != tc.wantY {
				t.Errorf("CenterOverlay(%dx%d) = (%d,%d), want (%d,%d)",
					tc.totalW, tc.totalH, x, y, tc.wantX, tc.wantY)
			}
		})
	}
}
