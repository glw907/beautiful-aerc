// SPDX-License-Identifier: MIT

package term

import (
	"strings"
	"testing"
)

func TestParseFcList(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantIn []string // families that must appear in result
	}{
		{
			name:   "empty output",
			input:  "",
			wantIn: nil,
		},
		{
			name:   "single family no style",
			input:  "DejaVu Sans\n",
			wantIn: []string{"DejaVu Sans"},
		},
		{
			name:   "family with style suffix",
			input:  "JetBrainsMono Nerd Font:style=Regular\n",
			wantIn: []string{"JetBrainsMono Nerd Font"},
		},
		{
			name:   "multiple families on one line",
			input:  "JetBrainsMono Nerd Font,JetBrainsMono NF:style=Thin Italic\n",
			wantIn: []string{"JetBrainsMono Nerd Font", "JetBrainsMono NF"},
		},
		{
			name:   "multiple lines",
			input:  "Hack Nerd Font:style=Bold\nInter:style=Regular\n",
			wantIn: []string{"Hack Nerd Font", "Inter"},
		},
		{
			name:   "duplicate families are deduplicated",
			input:  "Hack Nerd Font:style=Regular\nHack Nerd Font:style=Bold\n",
			wantIn: []string{"Hack Nerd Font"},
		},
		{
			name:   "blank lines are skipped",
			input:  "\nHack NF:style=Regular\n\n",
			wantIn: []string{"Hack NF"},
		},
		{
			// Real-world output of `fc-list` without `-f` — leading
			// path before the first ": ", then family list, then style.
			// Locks in the parser's tolerance for the legacy shape.
			name: "leading file path stripped",
			input: "/home/u/.local/share/fonts/JetBrainsMonoNerdFont-Regular.ttf: " +
				"JetBrainsMono Nerd Font,JetBrainsMono NF:style=Regular\n",
			wantIn: []string{"JetBrainsMono Nerd Font", "JetBrainsMono NF"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFcList(tt.input)
			for _, want := range tt.wantIn {
				found := false
				for _, f := range got {
					if f == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("parseFcList(%q) = %v; missing %q", tt.input, got, want)
				}
			}
			// Check deduplication: no family appears twice.
			seen := make(map[string]int)
			for _, f := range got {
				seen[f]++
			}
			for f, n := range seen {
				if n > 1 {
					t.Errorf("parseFcList produced duplicate family %q (%d times)", f, n)
				}
			}
			// For "duplicate families" test: confirm only one entry.
			if tt.name == "duplicate families are deduplicated" {
				if len(got) != 1 {
					t.Errorf("expected 1 unique family, got %d: %v", len(got), got)
				}
			}
			_ = strings.Join(got, ",") // ensure no panics on empty
		})
	}
}

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
