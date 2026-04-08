package compose

import (
	"testing"
)

func TestStripBrackets(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "bare address after colon",
			input: []string{"From: <alice@dom>"},
			want:  []string{"From: alice@dom"},
		},
		{
			name:  "bare address after comma",
			// net/mail quotes single-word display names on round-trip
			input: []string{"To: Bob <bob@dom>, <charlie@dom>"},
			want:  []string{`To: "Bob" <bob@dom>, charlie@dom`},
		},
		{
			name:  "named address preserved",
			// net/mail quotes single-word display names on round-trip
			input: []string{"To: Alice <alice@dom>"},
			want:  []string{`To: "Alice" <alice@dom>`},
		},
		{
			name:  "non-address header untouched",
			input: []string{"Subject: <important>"},
			want:  []string{"Subject: <important>"},
		},
		{
			name:  "date header untouched",
			input: []string{"Date: Mon, 1 Jan 2026 12:00:00 +0000"},
			want:  []string{"Date: Mon, 1 Jan 2026 12:00:00 +0000"},
		},
		{
			name:  "quoted display name with comma",
			input: []string{`To: "Smith, John" <john@dom>`},
			want:  []string{`To: "Smith, John" <john@dom>`},
		},
		{
			name:  "multiple bare addresses",
			input: []string{"To: <alice@dom>, <bob@dom>"},
			want:  []string{"To: alice@dom, bob@dom"},
		},
		{
			name:  "mixed bare and named",
			// net/mail quotes single-word display names on round-trip
			input: []string{"To: Alice <alice@dom>, <bob@dom>, Charlie <charlie@dom>"},
			want:  []string{`To: "Alice" <alice@dom>, bob@dom, "Charlie" <charlie@dom>`},
		},
		{
			name:  "empty value passes through",
			input: []string{"To:"},
			want:  []string{"To:"},
		},
		{
			name:  "empty value with space passes through",
			input: []string{"To: "},
			want:  []string{"To: "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripBrackets(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("stripBrackets() returned %d lines, want %d\ngot:  %q\nwant: %q", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
