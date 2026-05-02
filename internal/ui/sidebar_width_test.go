// SPDX-License-Identifier: MIT

package ui

import "testing"

func TestSidebarWidthFor(t *testing.T) {
	cases := []struct {
		termWidth int
		want      int
	}{
		{60, 24},
		{79, 24},
		{80, 24},
		{81, 25},
		{82, 26},
		{83, 27},
		{84, 28},
		{85, 29},
		{86, 30},
		{120, 30},
		{200, 30},
		{0, 24},
	}
	for _, tc := range cases {
		got := sidebarWidthFor(tc.termWidth)
		if got != tc.want {
			t.Errorf("sidebarWidthFor(%d) = %d, want %d",
				tc.termWidth, got, tc.want)
		}
	}
}
