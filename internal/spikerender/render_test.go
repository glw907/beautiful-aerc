package spikerender

import (
	"strings"
	"testing"
)

func TestRenderPlainText(t *testing.T) {
	out := Render([]byte("Hello world"), "text/plain", MsgHeaders{})
	if out != "Hello world" {
		t.Errorf("plain text passthrough: got %q", out)
	}
}

func TestRenderHTMLReturnsString(t *testing.T) {
	html := []byte("<p>Hello <strong>world</strong></p>")
	out := Render(html, "text/html", MsgHeaders{})
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
			name:  "strip view in web browser variant",
			input: "[View in web browser](https://example.com/view)\nNewsletter content",
			want:  "Newsletter content",
		},
		{
			name:  "strip view as heading",
			input: "# [View in browser](https://example.com/view)\n\nActual content",
			want:  "Actual content",
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

// R8: readability guard for forward/reply headers.

func TestContainsForwardReplyHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "gmail forwarded message marker",
			input: `<div class="gmail_attr">---------- Forwarded message ---------<br>From: Spring Ortega</div>`,
			want:  true,
		},
		{
			name:  "outlook bold from+date",
			input: `<div><b>From: </b>Alice <a@b.com><br><b>Date: </b>Monday<br><b>To: </b>Bob<br></div>`,
			want:  true,
		},
		{
			name:  "outlook bold from+sent",
			input: `<div><b>From: </b>Alice<br><b>Sent: </b>Monday<br></div>`,
			want:  true,
		},
		{
			name:  "plain email without reply header",
			input: `<p>Hello world, here is your newsletter.</p>`,
			want:  false,
		},
		{
			name:  "email with only from but no date/sent",
			input: `<p>From: Alice</p>`,
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsForwardReplyHeader(tt.input)
			if got != tt.want {
				t.Errorf("containsForwardReplyHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatReplyForwardBlock(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:  "outlook inline header split into blockquote lines",
			input: "Hello,\n\nI agree.\n\n**From:** Alice <a@example.com> **Date:** Monday, July 14 **To:** Bob **Subject:** The meeting\n\nHi Bob,\n\nContent here.",
			wantContains: []string{
				"Hello,",
				"I agree.",
				"> **From:**",
				"> **Date:**",
				"> **To:**",
				"> **Subject:**",
			},
		},
		{
			name:  "no forward/reply header — unchanged",
			input: "Just a plain message without any reply header.",
			wantContains: []string{
				"Just a plain message without any reply header.",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatReplyForwardBlock(tt.input)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output does not contain %q\nfull output:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("output should not contain %q\nfull output:\n%s", absent, got)
				}
			}
		})
	}
}

// R9: empty-body header synthesis.

func TestSynthesizeFromHeaders(t *testing.T) {
	tests := []struct {
		name        string
		subject     string
		fromDisplay string
		wantContain string
	}{
		{
			name:        "accepted calendar event",
			subject:     "Accepted: Team Planning",
			fromDisplay: "Gary Snyder",
			wantContain: "Gary Snyder has accepted",
		},
		{
			name:        "declined calendar event",
			subject:     "Declined: Team Planning",
			fromDisplay: "Sue Jones",
			wantContain: "Sue Jones has declined",
		},
		{
			name:        "tentatively accepted",
			subject:     "Tentative: Team Planning",
			fromDisplay: "Sam Lee",
			wantContain: "Sam Lee has tentatively accepted",
		},
		{
			name:        "event name in output",
			subject:     "Accepted: East XC Coaches Dinner",
			fromDisplay: "Gary",
			wantContain: "East XC Coaches Dinner",
		},
		{
			name:        "non-calendar subject falls back to subject line",
			subject:     "Your order has shipped",
			fromDisplay: "Support",
			wantContain: "Your order has shipped",
		},
		{
			name:        "no message included marker appears",
			subject:     "Accepted: Team Planning",
			fromDisplay: "Gary",
			wantContain: "No message included",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := synthesizeFromHeaders(tt.subject, tt.fromDisplay)
			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("synthesizeFromHeaders(%q, %q) = %q; want it to contain %q",
					tt.subject, tt.fromDisplay, got, tt.wantContain)
			}
		})
	}
}

func TestRenderEmptyBodySynthesis(t *testing.T) {
	emptyHTML := []byte(`<html><head></head><body><div><br></div></body></html>`)
	hdr := MsgHeaders{Subject: "Accepted: East XC Coaches Dinner", FromDisplay: "Snyder Gary"}
	out := Render(emptyHTML, "text/html", hdr)
	if !strings.Contains(out, "Snyder Gary") {
		t.Errorf("empty body synthesis: output %q should contain sender name", out)
	}
	if !strings.Contains(out, "East XC Coaches Dinner") {
		t.Errorf("empty body synthesis: output %q should contain event name", out)
	}
}

// R10: generalized trailing-boilerplate stripper.

func TestStripTrailingBoilerplate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip unsubscribe link at end",
			input: "Real content here.\n\n[Unsubscribe](https://example.com/unsub)",
			want:  "Real content here.",
		},
		{
			name:  "strip mailing list boilerplate",
			input: "Real content.\n\nYou have received this message from the mailing list of Example Org. If you would prefer not to receive these emails, go to the opt-out page.",
			want:  "Real content.",
		},
		{
			name:  "strip postal address line at end",
			input: "Real content.\n\nExample Corp, 123 Main St., Anytown, CA 94107, USA",
			want:  "Real content.",
		},
		{
			name:  "strip github thread footer variant",
			input: "PR comment text.\n\n— Reply to this email directly, [view it on GitHub](https://github.com/org/repo/issues/1#comment), or [unsubscribe](https://github.com/notifications/unsubscribe). You are receiving this because you authored the thread. Message ID: <org/repo/issues/1/comment@github.com>",
			want:  "PR comment text.",
		},
		{
			name:  "strip trailing separator after boilerplate removal",
			input: "Real content.\n\n* * *\n\n[Unsubscribe](https://example.com/unsub)",
			want:  "Real content.",
		},
		{
			name:  "leave content unchanged when no boilerplate",
			input: "Just a plain message with no footer.",
			want:  "Just a plain message with no footer.",
		},
		{
			name:  "strip social follow links run",
			input: "Newsletter body.\n\n[Follow on Twitter](https://twitter.com/brand)\n[Follow on Facebook](https://facebook.com/brand)\n[Follow on Instagram](https://instagram.com/brand)",
			want:  "Newsletter body.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripTrailingBoilerplate(tt.input)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// R11: generic tracking-query-param strip.

func TestStripTrackingParams(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip utm params from link",
			input: `[Read more](https://blog.example.com/post/?utm_source=email&utm_medium=newsletter&utm_campaign=July2026)`,
			want:  `[Read more](https://blog.example.com/post/)`,
		},
		{
			name:  "strip ebay tracking params",
			input: `[RSVP](https://www.rsvp.ebay.com/signup?mkevt=1&mkpid=2&mkcid=8&trkId=abc)`,
			want:  `[RSVP](https://www.rsvp.ebay.com/signup)`,
		},
		{
			name:  "preserve non-tracking params",
			input: `[Visit](https://example.com/search?q=golang&page=2)`,
			want:  `[Visit](https://example.com/search?q=golang&page=2)`,
		},
		{
			name:  "mixed tracking and real params",
			input: `[Visit](https://example.com/help?id=4822&utm_source=email)`,
			want:  `[Visit](https://example.com/help?id=4822)`,
		},
		{
			name:  "leave bare URLs unchanged",
			input: `Plain text https://example.com/post`,
			want:  `Plain text https://example.com/post`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripTrackingParams(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// R12: key-value paragraph merge.

func TestMergeKeyValueParagraphs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "merge single label-value pair",
			input: "**Date:**\n\n2026-07-17",
			want:  "**Date:** 2026-07-17",
		},
		{
			name:  "merge multiple label-value pairs",
			input: "**Order ID:**\n\nR18339298\n\n**Payment Method:**\n\nVisa *9111",
			want:  "**Order ID:** R18339298\n\n**Payment Method:** Visa *9111",
		},
		{
			name:  "merge address family into single line",
			input: "**Address:**\n\n4070 Warwick Place\n\n**City:**\n\nAnchorage\n\n**State/Province:**\n\nAK\n\n**ZIP Code:**\n\n99508\n\n**Country:**\n\nUnited States",
			want:  "**Address:** 4070 Warwick Place, Anchorage, AK 99508, United States",
		},
		{
			name:  "leave normal paragraph unchanged",
			input: "This is a normal sentence.\n\nAnd another one here.",
			want:  "This is a normal sentence.\n\nAnd another one here.",
		},
		{
			name:  "label with trailing space still merges",
			input: "**Date:** \n\n2026-07-17",
			want:  "**Date:** 2026-07-17",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeKeyValueParagraphs(tt.input)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// R13: nav link wall strip.

func TestStripNavLinkWall(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip 4+ short links in one block",
			input: "# Brand\n\n[MEN](https://x.com/men) [SHIRTS](https://x.com/shirts) [TAILORED](https://x.com/t) [WOMEN](https://x.com/w) [SALE](https://x.com/s)\n\n# Real Content",
			want:  "# Brand\n\n# Real Content",
		},
		{
			name:  "leave 3 or fewer links unchanged",
			input: "# Brand\n\n[Home](https://x.com) [About](https://x.com/about) [Contact](https://x.com/contact)\n\n# Real Content",
			want:  "# Brand\n\n[Home](https://x.com) [About](https://x.com/about) [Contact](https://x.com/contact)\n\n# Real Content",
		},
		{
			name:  "leave link block with long text unchanged",
			input: "# Brand\n\n[Browse our full collection](https://x.com) [Shop now for deals](https://y.com) [Find your local store](https://z.com) [View all categories](https://w.com)\n\n# Content",
			want:  "# Brand\n\n[Browse our full collection](https://x.com) [Shop now for deals](https://y.com) [Find your local store](https://z.com) [View all categories](https://w.com)\n\n# Content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripNavLinkWall(tt.input)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
