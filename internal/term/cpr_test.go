// SPDX-License-Identifier: MIT

package term

import (
	"bytes"
	"testing"
)

func TestParseCPR(t *testing.T) {
	tests := []struct {
		name    string
		in      []byte
		wantRow int
		wantCol int
		wantErr bool
	}{
		{"basic", []byte("\x1b[12;34R"), 12, 34, false},
		{"col 1", []byte("\x1b[1;1R"), 1, 1, false},
		{"three digit col", []byte("\x1b[1;120R"), 1, 120, false},
		{"missing R", []byte("\x1b[12;34"), 0, 0, true},
		{"missing semicolon", []byte("\x1b[1234R"), 0, 0, true},
		{"missing CSI", []byte("12;34R"), 0, 0, true},
		{"empty", nil, 0, 0, true},
		{"only CSI", []byte("\x1b["), 0, 0, true},
		{"trailing junk after R is ignored", []byte("\x1b[1;2Rxyz"), 1, 2, false},
		{"leading junk is skipped", []byte("garbage\x1b[7;8R"), 7, 8, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col, err := parseCPR(bytes.NewReader(tt.in))
			gotErr := err != nil
			if gotErr != tt.wantErr {
				t.Fatalf("parseCPR err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (row != tt.wantRow || col != tt.wantCol) {
				t.Errorf("parseCPR = (%d,%d), want (%d,%d)", row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}
