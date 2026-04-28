// SPDX-License-Identifier: MIT

package tidy

import (
	"testing"
)

func TestIsQuoted(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"", false},
		{"> text", true},
		{">text", true},
		{"  > text", true},
		{"\t> text", true},
		{"> > nested", true},
		{"regular text", false},
		{"not > quoted", false},
		{">", true},
		{"  >", true},
	}
	for _, tc := range tests {
		got := isQuoted(tc.line)
		if got != tc.want {
			t.Errorf("isQuoted(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestSplitQuoted(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantAuthor  string
		wantBlocks  []QuotedBlock
	}{
		{
			name:       "no quotes",
			input:      "Hello world\nThis is text\n",
			wantAuthor: "Hello world\nThis is text\n",
			wantBlocks: nil,
		},
		{
			name:       "empty input",
			input:      "",
			wantAuthor: "",
			wantBlocks: nil,
		},
		{
			name:       "all quoted",
			input:      "> first\n> second\n",
			wantAuthor: "",
			wantBlocks: []QuotedBlock{
				{StartLine: 0, Lines: []string{"> first", "> second"}},
			},
		},
		{
			name:       "mixed author then quoted",
			input:      "My reply\n\n> Original text\n> More original\n",
			wantAuthor: "My reply\n\n\n\n",
			wantBlocks: []QuotedBlock{
				{StartLine: 2, Lines: []string{"> Original text", "> More original"}},
			},
		},
		{
			name:       "indented quotes",
			input:      "Reply\n  > indented quote\n",
			wantAuthor: "Reply\n\n",
			wantBlocks: []QuotedBlock{
				{StartLine: 1, Lines: []string{"  > indented quote"}},
			},
		},
		{
			name:       "nested quotes",
			input:      "> > deeply nested\n> one level\n",
			wantAuthor: "",
			wantBlocks: []QuotedBlock{
				{StartLine: 0, Lines: []string{"> > deeply nested", "> one level"}},
			},
		},
		{
			name:       "blank lines between quoted blocks",
			input:      "> block one\n\n> block two\n",
			wantAuthor: "",
			wantBlocks: []QuotedBlock{
				{StartLine: 0, Lines: []string{"> block one"}},
				{StartLine: 2, Lines: []string{"> block two"}},
			},
		},
		{
			name:       "quote that is blank (just >)",
			input:      "Author\n>\n> text\n",
			wantAuthor: "Author\n\n\n",
			wantBlocks: []QuotedBlock{
				{StartLine: 1, Lines: []string{">", "> text"}},
			},
		},
		{
			name:       "author between quoted blocks",
			input:      "> first block\nmy text\n> second block\n",
			wantAuthor: "\nmy text\n\n",
			wantBlocks: []QuotedBlock{
				{StartLine: 0, Lines: []string{"> first block"}},
				{StartLine: 2, Lines: []string{"> second block"}},
			},
		},
		{
			name:       "whitespace only author after stripping quotes",
			input:      "   \n> quoted\n  \n",
			wantAuthor: "",
			wantBlocks: []QuotedBlock{
				{StartLine: 1, Lines: []string{"> quoted"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotAuthor, gotBlocks := SplitQuoted(tc.input)

			if gotAuthor != tc.wantAuthor {
				t.Errorf("SplitQuoted() author = %q, want %q", gotAuthor, tc.wantAuthor)
			}

			if len(gotBlocks) != len(tc.wantBlocks) {
				t.Fatalf("SplitQuoted() blocks count = %d, want %d\ngot:  %+v\nwant: %+v",
					len(gotBlocks), len(tc.wantBlocks), gotBlocks, tc.wantBlocks)
			}

			for i, gb := range gotBlocks {
				wb := tc.wantBlocks[i]
				if gb.StartLine != wb.StartLine {
					t.Errorf("block[%d].StartLine = %d, want %d", i, gb.StartLine, wb.StartLine)
				}
				if len(gb.Lines) != len(wb.Lines) {
					t.Errorf("block[%d].Lines count = %d, want %d\ngot:  %v\nwant: %v",
						i, len(gb.Lines), len(wb.Lines), gb.Lines, wb.Lines)
					continue
				}
				for j, gl := range gb.Lines {
					if gl != wb.Lines[j] {
						t.Errorf("block[%d].Lines[%d] = %q, want %q", i, j, gl, wb.Lines[j])
					}
				}
			}
		})
	}
}

func TestReassemble(t *testing.T) {
	tests := []struct {
		name          string
		corrected     string
		originalInput string
		want          string
	}{
		{
			name:          "no quotes - all author text replaced",
			corrected:     "Hello world.\nThis is text.\n",
			originalInput: "Hello world\nThis is text\n",
			want:          "Hello world.\nThis is text.\n",
		},
		{
			name:          "all quoted - corrected is empty",
			corrected:     "",
			originalInput: "> quoted line\n> another\n",
			want:          "> quoted line\n> another\n",
		},
		{
			name:          "mixed - corrected author with preserved quotes",
			corrected:     "My reply.\n",
			originalInput: "My reply\n\n> Original text\n> More original\n",
			want:          "My reply.\n\n> Original text\n> More original\n",
		},
		{
			name:          "corrected has more lines than original author",
			corrected:     "Line one.\nLine two.\nExtra line.\n",
			originalInput: "Line one\n\n> quoted\n",
			want:          "Line one.\nLine two.\n> quoted\nExtra line.\n",
		},
		{
			name:          "blank lines preserved in position",
			corrected:     "Author text.\n",
			originalInput: "Author text\n\n> quoted line\n",
			want:          "Author text.\n\n> quoted line\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Reassemble(tc.corrected, tc.originalInput)
			if got != tc.want {
				t.Errorf("Reassemble() =\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}
