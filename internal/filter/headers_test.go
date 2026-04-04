package filter

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "simple headers",
			input: "From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>\r\nSubject: Hello\r\n\r\n",
			want: map[string]string{
				"from":    " Alice <alice@example.com>",
				"to":      " Bob <bob@example.com>",
				"subject": " Hello",
			},
		},
		{
			name:  "folded header",
			input: "To: Alice <alice@example.com>,\r\n Bob <bob@example.com>\r\nSubject: Test\r\n\r\n",
			want: map[string]string{
				"to":      " Alice <alice@example.com>,\n Bob <bob@example.com>",
				"subject": " Test",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHeaders(strings.NewReader(tt.input))
			for k, want := range tt.want {
				if got.values[k] != want {
					t.Errorf("header[%q] = %q, want %q", k, got.values[k], want)
				}
			}
		})
	}
}

func TestStripBareAngles(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bare at start", "<alice@example.com>", "alice@example.com"},
		{"bare after comma", "Bob, <alice@example.com>", "Bob, alice@example.com"},
		{"with name", "Alice <alice@example.com>", "Alice <alice@example.com>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripBareAngles(tt.input)
			if got != tt.want {
				t.Errorf("stripBareAngles(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWrapAddresses(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		addrs string
		cols  int
		want  int // expected number of output lines
	}{
		{
			name:  "short fits one line",
			key:   "To:",
			addrs: "alice@example.com",
			cols:  80,
			want:  1,
		},
		{
			name:  "long wraps",
			key:   "To:",
			addrs: "alice@example.com, bob@example.com, charlie@example.com, dave@example.com",
			cols:  40,
			want:  2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := wrapAddresses(tt.key, tt.addrs, tt.cols)
			if len(lines) < tt.want {
				t.Errorf("wrapAddresses produced %d lines, want at least %d", len(lines), tt.want)
			}
		})
	}
}

func TestHeadersFilter(t *testing.T) {
	input := "From: Alice <alice@example.com>\r\nSubject: Hello World\r\nDate: Mon, 01 Jan 2026 00:00:00 +0000\r\nTo: Bob <bob@example.com>\r\nX-Mailer: test\r\n\r\n"

	var buf bytes.Buffer
	err := Headers(strings.NewReader(input), &buf, noColors(), 80)
	if err != nil {
		t.Fatalf("Headers: %v", err)
	}
	out := buf.String()

	// Verify header order: From before To before Date before Subject
	fromIdx := strings.Index(out, "From:")
	toIdx := strings.Index(out, "To:")
	dateIdx := strings.Index(out, "Date:")
	subjectIdx := strings.Index(out, "Subject:")

	if fromIdx < 0 || toIdx < 0 || dateIdx < 0 || subjectIdx < 0 {
		t.Fatalf("missing headers in output: %q", out)
	}
	if fromIdx > toIdx || toIdx > dateIdx || dateIdx > subjectIdx {
		t.Errorf("headers not in expected order (From, To, Date, Subject)")
	}

	// X-Mailer should be dropped
	if strings.Contains(out, "X-Mailer") {
		t.Error("X-Mailer should be dropped")
	}

	// Separator should be present
	if !strings.Contains(out, "─") {
		t.Error("separator line not found")
	}
}

// noColors returns a ColorSet with no ANSI codes for testing.
func noColors() *ColorSet {
	return &ColorSet{}
}
