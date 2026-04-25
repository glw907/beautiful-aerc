package content

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		from    []Address
		to      []Address
		subject string
	}{
		{
			name:    "simple",
			input:   "From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>\r\nDate: Mon, 5 Jan 2026\r\nSubject: Hello\r\n\r\n",
			from:    []Address{{Name: "Alice", Email: "alice@example.com"}},
			to:      []Address{{Name: "Bob", Email: "bob@example.com"}},
			subject: "Hello",
		},
		{
			name:    "bare email",
			input:   "From: alice@example.com\r\nSubject: Test\r\n\r\n",
			from:    []Address{{Email: "alice@example.com"}},
			subject: "Test",
		},
		{
			name:  "multiple recipients",
			input: "From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>, Carol <carol@example.com>\r\nSubject: Group\r\n\r\n",
			from:  []Address{{Name: "Alice", Email: "alice@example.com"}},
			to: []Address{
				{Name: "Bob", Email: "bob@example.com"},
				{Name: "Carol", Email: "carol@example.com"},
			},
			subject: "Group",
		},
		{
			name:    "folded header",
			input:   "From: Alice <alice@example.com>\r\nSubject: This is a very\r\n long subject line\r\n\r\n",
			from:    []Address{{Name: "Alice", Email: "alice@example.com"}},
			subject: "This is a very long subject line",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := ParseHeaders(tt.input)
			if len(h.From) != len(tt.from) {
				t.Fatalf("From count: got %d, want %d", len(h.From), len(tt.from))
			}
			for i, a := range h.From {
				if a != tt.from[i] {
					t.Errorf("From[%d]: got %v, want %v", i, a, tt.from[i])
				}
			}
			if len(h.To) != len(tt.to) {
				t.Fatalf("To count: got %d, want %d", len(h.To), len(tt.to))
			}
			for i, a := range h.To {
				if a != tt.to[i] {
					t.Errorf("To[%d]: got %v, want %v", i, a, tt.to[i])
				}
			}
			if h.Subject != tt.subject {
				t.Errorf("Subject: got %q, want %q", h.Subject, tt.subject)
			}
		})
	}
}

// TestRenderHeadersAddressUnitAtomic locks the wrap rule: when a To/Cc
// list cannot fit on one line, the break happens between addresses,
// never inside a `Name <email>` unit. Regression target for the
// viewer prototype, which renders headers at the panel content width.
func TestRenderHeadersAddressUnitAtomic(t *testing.T) {
	h := ParsedHeaders{
		To: []Address{
			{Name: "Alice Anderson", Email: "alice.anderson@longdomain.example.com"},
			{Name: "Bob Bjornson", Email: "bob.bjornson@longdomain.example.com"},
		},
	}
	out := RenderHeaders(h, theme.Nord, 60)
	for _, line := range strings.Split(out, "\n") {
		stripped := stripANSITest(line)
		opens := strings.Count(stripped, "<")
		closes := strings.Count(stripped, ">")
		if opens != closes {
			t.Errorf("address unit split across lines: %q (opens=%d, closes=%d)", stripped, opens, closes)
		}
		if w := lipgloss.Width(line); w > 60 && !strings.Contains(stripped, "<") {
			// The wrap algorithm allows a single oversized address unit
			// past the limit (better than splitting the unit) but should
			// not exceed when an alternative break exists.
			t.Errorf("line exceeds width 60 without containing an address unit: %q (w=%d)", stripped, w)
		}
	}
	flat := stripANSITest(out)
	for _, want := range []string{"alice.anderson@longdomain.example.com", "bob.bjornson@longdomain.example.com"} {
		if !strings.Contains(flat, want) {
			t.Errorf("missing address in output: %s", want)
		}
	}
}

