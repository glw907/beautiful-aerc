package spikerender

import (
	"strings"
	"testing"
)

func TestRenderPlainText(t *testing.T) {
	out := Render([]byte("Hello world"), "text/plain")
	if out != "Hello world" {
		t.Errorf("plain text passthrough: got %q", out)
	}
}

func TestRenderHTMLReturnsString(t *testing.T) {
	html := []byte("<p>Hello <strong>world</strong></p>")
	out := Render(html, "text/html")
	if out == "" {
		t.Error("Render returned empty string for non-empty HTML")
	}
	if strings.Contains(out, "<p>") || strings.Contains(out, "<strong>") {
		t.Errorf("Render left raw HTML tags in output: %q", out)
	}
}

func TestTextLen(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "plain text", input: "hello", want: 5},
		{name: "single tag", input: "<p>hi</p>", want: 2},
		{name: "nested tags", input: "<div><p>abc</p></div>", want: 3},
		{name: "empty", input: "", want: 0},
		{name: "only tags", input: "<br/>", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textLen(tt.input)
			if got != tt.want {
				t.Errorf("textLen(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripGitHubTracking(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip tracking params from workflow run link",
			input: `[View workflow run](https://github.com/owner/repo/actions/runs/123?email_source=notifications&email_token=LONG_SECRET_TOKEN)`,
			want:  `[View workflow run](https://github.com/owner/repo/actions/runs/123)`,
		},
		{
			name:  "strip tracking from settings link",
			input: `[Manage notifications](https://github.com/settings/notifications?email_source=notifications&email_token=ABCDEF)`,
			want:  `[Manage notifications](https://github.com/settings/notifications)`,
		},
		{
			name:  "leave non-github links unchanged",
			input: `[Visit](https://example.com/path?foo=bar)`,
			want:  `[Visit](https://example.com/path?foo=bar)`,
		},
		{
			name:  "leave github links without tracking unchanged",
			input: `[PR](https://github.com/owner/repo/pull/42)`,
			want:  `[PR](https://github.com/owner/repo/pull/42)`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripGitHubTracking(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripGitHubFooter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip em-dash plus subscribed notification footer",
			input: "Content here\n\n—\nYou are receiving this because you are subscribed to this thread.\n[Manage](https://github.com/settings/notifications)",
			want:  "Content here",
		},
		{
			name:  "strip admin notification footer without em-dash",
			input: "Content here\n\nYou are receiving this because you are an administrator for the glw907 account.\n\nGitHub, Inc. ・88 Colin P Kelly Jr Street ・San Francisco, CA 94107",
			want:  "Content here",
		},
		{
			name:  "strip GitHub company address line",
			input: "Some content\n\nGitHub, Inc. ・88 Colin P Kelly Jr Street ・San Francisco, CA 94107",
			want:  "Some content",
		},
		{
			name:  "strip trailing em-dash separator",
			input: "Content\n\n[link](https://github.com/runs/123)\n\n—",
			want:  "Content\n\n[link](https://github.com/runs/123)",
		},
		{
			name:  "leave unrelated content unchanged",
			input: "Plain content with no footer",
			want:  "Plain content with no footer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripGitHubFooter(tt.input)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripGCalFooter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip attendee disclaimer",
			input: "Event details here\n\nYou are receiving this email because you are an attendee on the event.\n\nForwarding this invitation could allow...",
			want:  "Event details here",
		},
		{
			name:  "strip invitation from line",
			input: "Event details here\n\nInvitation from [Google Calendar](https://calendar.google.com/calendar/)",
			want:  "Event details here",
		},
		{
			name:  "leave unrelated content unchanged",
			input: "Plain content",
			want:  "Plain content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripGCalFooter(tt.input)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripGroupsIoFooter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip groups.io separator and footer",
			input: "Event content here\n\n\\_.\\_,\\_.\\_,_\n\n* * *\n\nGroups.io Links:\n\n[Unsubscribe](https://example.groups.io/unsubscribe)",
			want:  "Event content here",
		},
		{
			name:  "strip you receive all messages line",
			input: "Event content here\n\nYou receive all messages sent to this group.\n[Reply](mailto:group@example.com)",
			want:  "Event content here",
		},
		{
			name:  "leave unrelated content unchanged",
			input: "Plain content without groups.io",
			want:  "Plain content without groups.io",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripGroupsIoFooter(tt.input)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripViewInBrowser(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip view in browser link",
			input: "[View in browser](https://link.axios.com/view)\nAxios Hill Leaders",
			want:  "Axios Hill Leaders",
		},
		{
			name:  "strip view online variant",
			input: "[View online](https://example.com/view)\nNewsletter content",
			want:  "Newsletter content",
		},
		{
			name:  "leave unrelated links unchanged",
			input: "[Read more](https://example.com/article)\nSome content",
			want:  "[Read more](https://example.com/article)\nSome content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripViewInBrowser(tt.input)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripSelfLinks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bare URL where text equals href",
			input: `[https://example.com/about](https://example.com/about)`,
			want:  `https://example.com/about`,
		},
		{
			name:  "leave descriptive links unchanged",
			input: `[About page](https://example.com/about)`,
			want:  `[About page](https://example.com/about)`,
		},
		{
			name:  "leave links where text differs from href unchanged",
			input: `[example.com](https://example.com/about)`,
			want:  `[example.com](https://example.com/about)`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripSelfLinks(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
