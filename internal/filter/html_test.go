// SPDX-License-Identifier: MIT

package filter

import (
	"strings"
	"testing"
)

func TestCleanHTML(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "simple paragraph",
			html: "<p>Hello world</p>",
			want: "Hello world",
		},
		{
			name: "heading rendered",
			html: "<h2>Title</h2><p>Body</p>",
			want: "Title",
		},
		{
			name: "link text preserved",
			html: `<p><a href="https://example.com">Click</a></p>`,
			want: "Click",
		},
		{
			name: "tracking image stripped",
			html: `<p>Text</p><img width="0" height="0" src="https://track.example.com/pixel.gif">`,
			want: "Text",
		},
		{
			name: "display none stripped",
			html: `<p>Visible</p><div style="display:none">Hidden</div>`,
			want: "Visible",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanHTML(tt.html)
			if !strings.Contains(got, tt.want) {
				t.Errorf("output missing %q\ngot: %s", tt.want, got)
			}
		})
	}
}

func TestCleanHTMLDisplayNoneNotInOutput(t *testing.T) {
	input := `<p>Show</p><div style="display:none"><p>Secret</p></div>`
	got := CleanHTML(input)
	if strings.Contains(got, "Secret") {
		t.Error("display:none content should be stripped")
	}
}

func TestStripHiddenElements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"display:none div removed",
			`<body><div style="display:none;max-height:0">hidden</div><p>visible</p></body>`,
			`<body><p>visible</p></body>`,
		},
		{
			"display: none with space removed",
			`<div style="display: none">hidden</div><p>ok</p>`,
			`<p>ok</p>`,
		},
		{
			"visible div preserved",
			`<div style="color:red">visible</div>`,
			`<div style="color:red">visible</div>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHiddenElements(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripEmptyLinks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"empty text link stripped",
			"Title\n\n[](https://tracking.example.com/click?id=abc)\n\nBody",
			"Title\n\n\n\nBody",
		},
		{
			"normal link preserved",
			"Visit [Example](https://example.com) today.",
			"Visit [Example](https://example.com) today.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripEmptyLinks(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeduplicateBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"consecutive identical blocks collapsed",
			"[Product](https://example.com)\n\n[Product](https://example.com)\n\n[Product](https://example.com)",
			"[Product](https://example.com)",
		},
		{
			"same text different URLs collapsed",
			"[Product](https://track.com/1)\n\n[Product](https://track.com/2)\n\n[Product](https://track.com/3)",
			"[Product](https://track.com/1)",
		},
		{
			"different blocks preserved",
			"[A](https://a.com)\n\n[B](https://b.com)\n\n[C](https://c.com)",
			"[A](https://a.com)\n\n[B](https://b.com)\n\n[C](https://c.com)",
		},
		{
			"non-adjacent duplicates preserved",
			"Alpha\n\nBeta\n\nAlpha",
			"Alpha\n\nBeta\n\nAlpha",
		},
		{
			"mixed repeated and unique",
			"Header\n\n[Img](url1)\n\n[Img](url2)\n\n[Img](url3)\n\nBody text",
			"Header\n\n[Img](url1)\n\nBody text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateBlocks(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollapseShortBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"step tracker collapsed",
			"Ordered\n\nShipped\n\nOut for delivery\n\nDelivered",
			"Ordered · Shipped · Out for delivery · Delivered",
		},
		{
			"two short blocks not collapsed",
			"Hello\n\nWorld",
			"Hello\n\nWorld",
		},
		{
			"mixed short and long preserved",
			"Ordered\n\nShipped\n\nDelivered\n\nThis is a real paragraph with actual content.",
			"Ordered · Shipped · Delivered\n\nThis is a real paragraph with actual content.",
		},
		{
			"links not collapsed",
			"[A](https://a.com)\n\n[B](https://b.com)\n\n[C](https://c.com)",
			"[A](https://a.com)\n\n[B](https://b.com)\n\n[C](https://c.com)",
		},
		{
			"headings not collapsed",
			"# One\n\n# Two\n\n# Three",
			"# One\n\n# Two\n\n# Three",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collapseShortBlocks(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompactLineRuns(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "signature block compacted",
			input: "Sincerely,\n\nJane Doe\n\nDirector of Sales\n\nAcme Corp",
			want:  "Sincerely,\n\nJane Doe  \nDirector of Sales  \nAcme Corp",
		},
		{
			name:  "sentences stay separate",
			input: "First point.\n\nSecond point.\n\nThird point.",
			want:  "First point.\n\nSecond point.\n\nThird point.",
		},
		{
			name:  "short run under threshold",
			input: "Line A\n\nLine B",
			want:  "Line A\n\nLine B",
		},
		{
			name:  "contact block with links",
			input: "p: 555-1234\n\ne: [a@b.com](mailto:a@b.com)\n\nw: [site.com](http://site.com)",
			want:  "p: 555-1234  \ne: [a@b.com](mailto:a@b.com)  \nw: [site.com](http://site.com)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compactLineRuns(tt.input)
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestUnflattenQuotes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "html entity markers",
			input: "On Mon, Person wrote: &gt; Hello &gt; &gt; How are you &gt; doing today",
			want:  "On Mon, Person wrote:\n\n> Hello\n>\n> How are you doing today",
		},
		{
			name:  "literal gt markers",
			input: "Person wrote: > Hello > > Goodbye",
			want:  "Person wrote:\n\n> Hello\n>\n> Goodbye",
		},
		{
			name:  "no quotes unchanged",
			input: "Just a regular paragraph with no quotes.",
			want:  "Just a regular paragraph with no quotes.",
		},
		{
			name:  "preserves surrounding blocks",
			input: "First paragraph.\n\nPerson wrote: &gt; Quoted text\n\nLast paragraph.",
			want:  "First paragraph.\n\nPerson wrote:\n\n> Quoted text\n\nLast paragraph.",
		},
		{
			name:  "multiple continuation lines",
			input: "Person wrote: &gt; Line one &gt; line two &gt; line three",
			want:  "Person wrote:\n\n> Line one line two line three",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unflattenQuotes(tt.input)
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

