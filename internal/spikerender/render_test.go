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

// R14: inline word-split repair.

func TestRepairWordSplits(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			// Both tags are removed so inlineBoundaryPad has no injection site.
			name:  "join word split at span boundary",
			input: `on S</span><span style="font-weight: bold;">unday, September 27`,
			want:  `on Sunday, September 27`,
		},
		{
			name:  "join split at strong boundary",
			input: `<strong>D</strong><strong>onations`,
			want:  `<strong>Donations`,
		},
		{
			name:  "leave space-separated inline tags unchanged",
			input: `Hello </span> <span>World`,
			want:  `Hello </span> <span>World`,
		},
		{
			name:  "leave punctuation boundary unchanged",
			input: `end.</span><span>Next`,
			want:  `end.</span><span>Next`,
		},
		{
			name:  "handle chain of splits in successive passes",
			input: `A</span><span>B</span><span>C`,
			want:  `ABC`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repairWordSplits(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// R15: image-alt residue drop.

func TestDropImageAltResidues(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "remove link whose text starts with Image of",
			input: "[Image of Ghurka Blazer](https://example.com/img)\n\nReal content",
			want:  "Real content",
		},
		{
			name:  "remove link whose text ends with Logo",
			input: "[Brooks Brothers Logo](https://example.com/logo)\n\nReal content",
			want:  "Real content",
		},
		{
			name:  "remove link whose text starts with A woman",
			input: "[A woman is packaging a parcel on a table.](https://example.com/)\n\nReal content",
			want:  "Real content",
		},
		{
			name:  "remove credit line",
			input: "Real content\n\nIllustration: Aida Amer/Axios. Stock: Getty Images",
			want:  "Real content",
		},
		{
			name:  "remove illustration caption",
			input: "Real content\n\nIllustration of the Capitol building circling in a recursive style.",
			want:  "Real content",
		},
		{
			name:  "remove AI generated disclaimer",
			input: "Real content\n\nAI-generated content may be incorrect.",
			want:  "Real content",
		},
		{
			name:  "remove standalone GitHub logo line",
			input: "GitHub\n\n## CI workflow run",
			want:  "## CI workflow run",
		},
		{
			name:  "leave descriptive link text unchanged",
			input: "[Read the full article](https://example.com/article)\n\nReal content",
			want:  "[Read the full article](https://example.com/article)\n\nReal content",
		},
		{
			name:  "leave mixed-content lines unchanged",
			input: "Call us at 555-1234 · A black and white letter heading",
			want:  "Call us at 555-1234 · A black and white letter heading",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dropImageAltResidues(tt.input)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// R16: hidden preheader/preview-text drop.

func TestStripHiddenPreheaders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "remove div with snippet-container id",
			input: `<div id="snippet-container"><p>Preheader text here</p></div><h1>Real content</h1>`,
			want:  `<h1>Real content</h1>`,
		},
		{
			name:  "remove div with preheader class",
			input: `<div class="preheader" style="display:none">Hidden text</div><p>Visible</p>`,
			want:  `<p>Visible</p>`,
		},
		{
			name:  "remove span with display:none style",
			input: `<span style="display:none">hidden</span>visible text`,
			want:  `visible text`,
		},
		{
			name:  "remove h1 with font-size 11px",
			input: `<h1 style="font-size:11px;"><a href="http://x.com">Tiny heading</a></h1><h2>Real heading</h2>`,
			want:  `<h2>Real heading</h2>`,
		},
		{
			name:  "leave normal elements unchanged",
			input: `<div><p>Normal content</p></div>`,
			want:  `<div><p>Normal content</p></div>`,
		},
		{
			name:  "leave h1 with large font-size unchanged",
			input: `<h1 style="font-size:24px;">Big heading</h1>`,
			want:  `<h1 style="font-size:24px;">Big heading</h1>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHiddenPreheaders(tt.input)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// R17: redirect-wrapper unwrap and tracking param extension.

func TestUnwrapRedirectLinks(t *testing.T) {
	// aHR0cHM6Ly93d3cuZXhhbXBsZS5jb20v decodes to https://www.example.com/
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "decode Axios-style base64url redirect",
			input: "[Click here](https://link.axios.com/click/1/aHR0cHM6Ly93d3cuZXhhbXBsZS5jb20v/abc)",
			want:  "[Click here](https://www.example.com/)",
		},
		{
			name:  "strip utm params from decoded URL",
			input: "[Click here](https://link.axios.com/click/1/aHR0cHM6Ly93d3cuZXhhbXBsZS5jb20vP3V0bV9zb3VyY2U9ZW1haWw/abc)",
			want:  "[Click here](https://www.example.com/)",
		},
		{
			name:  "leave non-redirect URLs unchanged",
			input: "[Visit](https://www.example.com/page)",
			want:  "[Visit](https://www.example.com/page)",
		},
		{
			name:  "leave short path segments unchanged",
			input: "[Visit](https://example.com/a/b/c)",
			want:  "[Visit](https://example.com/a/b/c)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unwrapRedirectLinks(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrackingParamsExtended(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip ch tracking param",
			input: `[RSVP](https://www.rsvp.ebay.com/signup?ch=osgood&id=123)`,
			want:  `[RSVP](https://www.rsvp.ebay.com/signup?id=123)`,
		},
		{
			name:  "strip c tracking param",
			input: `[See terms](https://example.com/?c=base64value&q=real)`,
			want:  `[See terms](https://example.com/?q=real)`,
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

// R18: style-driven heading promotion.

func TestPromoteStyledHeadings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "promote td with large font-size to h2",
			input: `<td style="font-size: 26px;">Order Details</td>`,
			want:  `<td><h2>Order Details</h2></td>`,
		},
		{
			name:  "promote span with large font-size to h2",
			input: `<span style="font-size: 25px; color: blue;">Photo Contest</span>`,
			want:  `<h2>Photo Contest</h2>`,
		},
		{
			name:  "skip td with small font-size",
			input: `<td style="font-size: 14px;">Small text</td>`,
			want:  `<td style="font-size: 14px;">Small text</td>`,
		},
		{
			name:  "skip td with HTML child elements",
			input: `<td style="font-size: 26px;"><strong>Bold heading</strong></td>`,
			want:  `<td style="font-size: 26px;"><strong>Bold heading</strong></td>`,
		},
		{
			name:  "skip span with long text",
			input: `<span style="font-size: 26px;">` + strings.Repeat("x", 121) + `</span>`,
			want:  `<span style="font-size: 26px;">` + strings.Repeat("x", 121) + `</span>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := promoteStyledHeadings(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// R19: serializer cleanup.

func TestFixSerializerArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "remove space before comma after backtick",
			input: "see `url.origin` , for details",
			want:  "see `url.origin`, for details",
		},
		{
			name:  "remove space before period after link close",
			input: "see the [release notes](https://example.com/releases) .",
			want:  "see the [release notes](https://example.com/releases).",
		},
		{
			name:  "remove space before period after bold close",
			input: "**close tomorrow** .",
			want:  "**close tomorrow**.",
		},
		{
			name:  "remove space before period after italic close",
			input: "*941 words, 3.5 minutes* .",
			want:  "*941 words, 3.5 minutes*.",
		},
		{
			name:  "fix unnecessary backslash before underscore",
			input: "Transaction ID: ch\\_3Tu8a8EG4pw3",
			want:  "Transaction ID: ch_3Tu8a8EG4pw3",
		},
		{
			name:  "fix unnecessary backslash before star before digit",
			input: "Payment: Visa \\*9111",
			want:  "Payment: Visa *9111",
		},
		{
			name:  "convert black circle bullet to list item",
			input: "● Baby",
			want:  "- Baby",
		},
		{
			name:  "convert middle dot bullet to list item",
			input: "·   Photos (interior and exterior)",
			want:  "- Photos (interior and exterior)",
		},
		{
			name:  "convert bullet inside blockquote",
			input: "> · Item one",
			want:  "> - Item one",
		},
		{
			name:  "leave space before period mid-sentence unchanged",
			input: "The U.S. government said so.",
			want:  "The U.S. government said so.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixSerializerArtifacts(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
