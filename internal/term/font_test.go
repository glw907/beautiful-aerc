// SPDX-License-Identifier: MIT

package term

import "testing"

func TestHasNerdFontFromList(t *testing.T) {
	tests := []struct {
		name     string
		families []string
		want     bool
	}{
		{"empty list", nil, false},
		{"none match", []string{"DejaVu Sans Mono", "Inter", "Hack"}, false},
		{"Nerd Font suffix", []string{"DejaVu Sans Mono", "JetBrainsMonoNL Nerd Font"}, true},
		{"NF abbreviation", []string{"Hack NF"}, true},
		{"case-insensitive", []string{"hack nerd font"}, true},
		{"trailing whitespace tolerated", []string{"  Hack Nerd Font  "}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasNerdFontIn(tt.families)
			if got != tt.want {
				t.Errorf("hasNerdFontIn(%v) = %v, want %v", tt.families, got, tt.want)
			}
		})
	}
}
