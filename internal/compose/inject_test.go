package compose

import (
	"testing"
)

func TestInjectCcBcc(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "both missing inserted after To",
			input: []string{"From: alice@dom", "To: bob@dom", "Subject: Hi"},
			want:  []string{"From: alice@dom", "To: bob@dom", "Cc:", "Bcc:", "Subject: Hi"},
		},
		{
			name:  "Cc present Bcc missing",
			input: []string{"From: alice@dom", "To: bob@dom", "Cc: charlie@dom", "Subject: Hi"},
			want:  []string{"From: alice@dom", "To: bob@dom", "Cc: charlie@dom", "Bcc:", "Subject: Hi"},
		},
		{
			name:  "both present no change",
			input: []string{"From: alice@dom", "To: bob@dom", "Cc:", "Bcc:", "Subject: Hi"},
			want:  []string{"From: alice@dom", "To: bob@dom", "Cc:", "Bcc:", "Subject: Hi"},
		},
		{
			name: "To with continuation lines",
			input: []string{
				"From: alice@dom",
				"To: bob@dom,",
				"    charlie@dom",
				"Subject: Hi",
			},
			want: []string{
				"From: alice@dom",
				"To: bob@dom,",
				"    charlie@dom",
				"Cc:",
				"Bcc:",
				"Subject: Hi",
			},
		},
		{
			name:  "no To header",
			input: []string{"From: alice@dom", "Subject: Hi"},
			want:  []string{"From: alice@dom", "Subject: Hi"},
		},
		{
			name:  "empty input",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "empty To",
			input: []string{"From: alice@dom", "To:", "Subject: Hi"},
			want:  []string{"From: alice@dom", "To:", "Cc:", "Bcc:", "Subject: Hi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := injectCcBcc(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("injectCcBcc() returned %d lines, want %d\ngot:  %q\nwant: %q", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
