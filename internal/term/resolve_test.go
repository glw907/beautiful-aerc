// SPDX-License-Identifier: MIT

package term

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name        string
		cfg         string
		hasNerdFont bool
		probe       int // 0 = failed
		wantMode    IconMode
		wantWidth   int
	}{
		{"auto + NF + probe=1", "auto", true, 1, IconModeFancy, 1},
		{"auto + NF + probe=2", "auto", true, 2, IconModeFancy, 2},
		{"auto + NF + probe=0", "auto", true, 0, IconModeFancy, 2},    // assume Mono on probe failure
		{"auto + no NF", "auto", false, 0, IconModeSimple, 1},
		{"auto + no NF + probe=1 ignored", "auto", false, 1, IconModeSimple, 1},
		{"simple forced + NF", "simple", true, 2, IconModeSimple, 1},
		{"simple forced + no NF", "simple", false, 0, IconModeSimple, 1},
		{"fancy forced + probe=1", "fancy", false, 1, IconModeFancy, 1},
		{"fancy forced + probe=2", "fancy", false, 2, IconModeFancy, 2},
		{"fancy forced + probe=0", "fancy", false, 0, IconModeFancy, 2},
		{"fancy forced + NF + probe=1", "fancy", true, 1, IconModeFancy, 1},
		{"unknown defaults to auto+no-NF", "garbage", false, 0, IconModeSimple, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, width := Resolve(tt.cfg, tt.hasNerdFont, tt.probe)
			if mode != tt.wantMode || width != tt.wantWidth {
				t.Errorf("Resolve(%q,%v,%d)=(%v,%d), want (%v,%d)",
					tt.cfg, tt.hasNerdFont, tt.probe, mode, width, tt.wantMode, tt.wantWidth)
			}
		})
	}
}
