package compose

import (
	"testing"
)

func TestFoldAddresses(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "short list stays on one line",
			input: []string{"To: alice@dom, bob@dom"},
			want:  []string{"To: alice@dom, bob@dom"},
		},
		{
			name: "long list wraps at 72 columns",
			input: []string{
				"To: Alice Example <alice@example.com>, Bob Example <bob@example.com>, Charlie Example <charlie@example.com>",
			},
			want: []string{
				`To: "Alice Example" <alice@example.com>, "Bob Example" <bob@example.com>,`,
				`    "Charlie Example" <charlie@example.com>`,
			},
		},
		{
			name:  "single recipient unchanged",
			input: []string{"To: alice@example.com"},
			want:  []string{"To: alice@example.com"},
		},
		{
			name:  "non-address header untouched",
			input: []string{"Subject: This is a very long subject line that exceeds seventy-two characters easily"},
			want:  []string{"Subject: This is a very long subject line that exceeds seventy-two characters easily"},
		},
		{
			name: "Cc indent matches key length",
			input: []string{
				"Cc: Alice Example <alice@example.com>, Bob Example <bob@example.com>, Charlie Example <charlie@example.com>",
			},
			want: []string{
				`Cc: "Alice Example" <alice@example.com>, "Bob Example" <bob@example.com>,`,
				`    "Charlie Example" <charlie@example.com>`,
			},
		},
		{
			name: "Bcc indent matches key length",
			input: []string{
				"Bcc: Alice Example <alice@example.com>, Bob Example <bob@example.com>, Charlie Example <charlie@example.com>",
			},
			want: []string{
				`Bcc: "Alice Example" <alice@example.com>,`,
				`     "Bob Example" <bob@example.com>,`,
				`     "Charlie Example" <charlie@example.com>`,
			},
		},
		{
			name:  "empty To passes through",
			input: []string{"To:"},
			want:  []string{"To:"},
		},
		{
			name:  "From header not folded",
			input: []string{"From: alice@example.com"},
			want:  []string{"From: alice@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := foldAddresses(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("foldAddresses() returned %d lines, want %d\ngot:  %q\nwant: %q", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d:\n  got:  %q\n  want: %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
