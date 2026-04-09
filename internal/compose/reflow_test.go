package compose

import (
	"testing"
)

func TestQuotePrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "single level", input: "> hello", want: "> "},
		{name: "double level spaced", input: "> > hello", want: "> > "},
		{name: "double level compact", input: ">> hello", want: ">> "},
		{name: "triple level", input: "> > > hello", want: "> > > "},
		{name: "no prefix", input: "hello", want: ""},
		{name: "just arrow", input: ">text", want: ">"},
		{name: "empty line", input: "", want: ""},
		{name: "blank quoted", input: "> ", want: "> "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quotePrefix(tt.input)
			if got != tt.want {
				t.Errorf("quotePrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuoteDepth(t *testing.T) {
	tests := []struct {
		prefix string
		want   int
	}{
		{"> ", 1},
		{"> > ", 2},
		{">> ", 2},
		{"> > > ", 3},
		{">", 1},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			got := quoteDepth(tt.prefix)
			if got != tt.want {
				t.Errorf("quoteDepth(%q) = %d, want %d", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestReflowQuoted(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name: "reflow ragged quoted lines",
			input: []string{
				"> This is a long quoted line that was wrapped at an odd",
				"> point by the original sender's email client and",
				"> looks ragged.",
			},
			want: []string{
				"> This is a long quoted line that was wrapped at an odd point by the",
				"> original sender's email client and looks ragged.",
			},
		},
		{
			name: "preserve blank quoted lines as paragraph breaks",
			input: []string{
				"> First paragraph.",
				">",
				"> Second paragraph.",
			},
			want: []string{
				"> First paragraph.",
				">",
				"> Second paragraph.",
			},
		},
		{
			name: "nested quotes reflowed independently",
			input: []string{
				"> > Inner quote that is too long and should be",
				"> > reflowed by the tool.",
				"> Outer quote.",
			},
			want: []string{
				"> > Inner quote that is too long and should be reflowed by the tool.",
				"> Outer quote.",
			},
		},
		{
			name:  "unquoted body lines untouched",
			input: []string{"Hello there.", "This is my reply."},
			want:  []string{"Hello there.", "This is my reply."},
		},
		{
			name: "decorative line preserved",
			input: []string{
				"> ----------",
				"> Some text.",
			},
			want: []string{
				"> ----------",
				"> Some text.",
			},
		},
		{
			name:  "empty body",
			input: []string{},
			want:  []string{},
		},
		{
			name: "mixed quoted and unquoted",
			input: []string{
				"My reply.",
				"",
				"> Quoted text that should be",
				"> reflowed into one line.",
			},
			want: []string{
				"My reply.",
				"",
				"> Quoted text that should be reflowed into one line.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reflowQuoted(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("reflowQuoted() returned %d lines, want %d\ngot:  %q\nwant: %q", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d:\n  got:  %q\n  want: %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
