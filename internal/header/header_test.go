package header

import (
	"strings"
	"testing"
)

func TestExtractFrom(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "name and angle brackets",
			input: "From: Sean Walsh <seanwalsh144@gmail.com>\r\nTo: info@example.com\r\nSubject: Hello\r\n\r\nBody text",
			want:  "seanwalsh144@gmail.com",
		},
		{
			name:  "bare email",
			input: "From: seanwalsh144@gmail.com\r\nSubject: Hello\r\n\r\n",
			want:  "seanwalsh144@gmail.com",
		},
		{
			name:  "quoted name",
			input: "From: \"Walsh, Sean\" <seanwalsh144@gmail.com>\r\nSubject: Hi\r\n\r\n",
			want:  "seanwalsh144@gmail.com",
		},
		{
			name:  "missing from",
			input: "To: info@example.com\r\nSubject: Hello\r\n\r\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFrom(strings.NewReader(tt.input))
			if got != tt.want {
				t.Errorf("ExtractFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractSubject(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple subject",
			input: "From: a@b.com\r\nSubject: Hello world\r\n\r\n",
			want:  "Hello world",
		},
		{
			name:  "strips Re prefix",
			input: "From: a@b.com\r\nSubject: Re: Hello world\r\n\r\n",
			want:  "Hello world",
		},
		{
			name:  "strips Fwd prefix",
			input: "From: a@b.com\r\nSubject: Fwd: Hello world\r\n\r\n",
			want:  "Hello world",
		},
		{
			name:  "strips nested Re/Fwd",
			input: "From: a@b.com\r\nSubject: Re: Fwd: Re: Hello world\r\n\r\n",
			want:  "Hello world",
		},
		{
			name:  "folded header",
			input: "From: a@b.com\r\nSubject: This is a very long\r\n subject line\r\n\r\n",
			want:  "This is a very long subject line",
		},
		{
			name:  "missing subject",
			input: "From: a@b.com\r\n\r\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSubject(strings.NewReader(tt.input))
			if got != tt.want {
				t.Errorf("ExtractSubject() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractTo(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single To address",
			input: "From: a@b.com\r\nTo: recipient@example.com\r\nSubject: Hi\r\n\r\n",
			want:  []string{"recipient@example.com"},
		},
		{
			name:  "multiple To addresses",
			input: "From: a@b.com\r\nTo: one@example.com, two@example.com\r\nSubject: Hi\r\n\r\n",
			want:  []string{"one@example.com", "two@example.com"},
		},
		{
			name:  "To and Cc combined",
			input: "From: a@b.com\r\nTo: to@example.com\r\nCc: cc@example.com\r\nSubject: Hi\r\n\r\n",
			want:  []string{"to@example.com", "cc@example.com"},
		},
		{
			name:  "only Cc no To",
			input: "From: a@b.com\r\nCc: cc-only@example.com\r\nSubject: Hi\r\n\r\n",
			want:  []string{"cc-only@example.com"},
		},
		{
			name:  "name and angle brackets",
			input: "From: a@b.com\r\nTo: Matt Flickinger <matt@example.com>\r\nCc: \"Committee\" <committee@example.com>\r\nSubject: Hi\r\n\r\n",
			want:  []string{"matt@example.com", "committee@example.com"},
		},
		{
			name:  "missing both headers",
			input: "From: a@b.com\r\nSubject: Hello\r\n\r\n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTo(strings.NewReader(tt.input))
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractTo() returned %d addresses, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractTo()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
