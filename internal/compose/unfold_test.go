package compose

import (
	"testing"
)

func TestUnfoldHeaders(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "no continuation lines",
			input: []string{"From: alice@dom", "To: bob@dom", "Subject: Hello"},
			want:  []string{"From: alice@dom", "To: bob@dom", "Subject: Hello"},
		},
		{
			name:  "space continuation",
			input: []string{"To: alice@dom,", " bob@dom"},
			want:  []string{"To: alice@dom, bob@dom"},
		},
		{
			name:  "tab continuation",
			input: []string{"To: alice@dom,", "\tbob@dom"},
			want:  []string{"To: alice@dom, bob@dom"},
		},
		{
			name:  "multiple continuations",
			input: []string{"To: alice@dom,", " bob@dom,", " charlie@dom"},
			want:  []string{"To: alice@dom, bob@dom, charlie@dom"},
		},
		{
			name:  "mixed headers and continuations",
			input: []string{"From: alice@dom", "To: bob@dom,", " charlie@dom", "Subject: Hi"},
			want:  []string{"From: alice@dom", "To: bob@dom, charlie@dom", "Subject: Hi"},
		},
		{
			name:  "empty input",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "continuation with extra whitespace",
			input: []string{"To: alice@dom,", "   bob@dom"},
			want:  []string{"To: alice@dom, bob@dom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unfoldHeaders(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("unfoldHeaders() returned %d lines, want %d\ngot:  %q\nwant: %q", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
