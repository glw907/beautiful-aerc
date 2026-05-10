package content

import (
	"strings"
	"testing"
)

func TestExtractPlainText(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		contains string
		empty    bool
	}{
		{
			name:     "non-RFC822 passes through",
			input:    "just a markdown body\n\nwith paragraphs",
			contains: "just a markdown body",
		},
		{
			name: "text-only RFC822",
			input: "From: a@x\r\nTo: b@y\r\nSubject: hi\r\n" +
				"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
				"hello world body text",
			contains: "hello world body text",
		},
		{
			name: "multipart prefers text/plain",
			input: "From: a@x\r\nTo: b@y\r\nSubject: hi\r\n" +
				"Content-Type: multipart/alternative; boundary=B\r\n\r\n" +
				"--B\r\nContent-Type: text/plain\r\n\r\nplain wins\r\n" +
				"--B\r\nContent-Type: text/html\r\n\r\n<p>html loses</p>\r\n" +
				"--B--\r\n",
			contains: "plain wins",
		},
		{
			name: "html-only falls through CleanHTML",
			input: "From: a@x\r\nTo: b@y\r\nSubject: hi\r\n" +
				"Content-Type: text/html; charset=utf-8\r\n\r\n" +
				"<p>hello <b>html</b> body</p>",
			contains: "hello",
		},
		{
			name: "calendar-only returns empty",
			input: "From: a@x\r\nTo: b@y\r\nSubject: invite\r\n" +
				"Content-Type: text/calendar; method=REQUEST\r\n\r\n" +
				"BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
			empty: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractPlainText([]byte(tc.input))
			if err != nil {
				t.Fatalf("ExtractPlainText: %v", err)
			}
			if tc.empty {
				if got != "" {
					t.Errorf("expected empty, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.contains) {
				t.Errorf("got %q, want substring %q", got, tc.contains)
			}
		})
	}
}
