package filter

import (
	"strings"
	"testing"
)

func TestCleanPandocArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trailing backslash", "hello\\\n", "hello\n"},
		{"escaped punctuation", "hello\\!", "hello!"},
		{"escaped period", "end\\.", "end."},
		{"stray bold stripped", "text\n**\n**\nnext", "text\nnext"},
		{"stray bold with backslash", "text\n**\\\n**\nnext", "text\nnext"},
		{"stray bold at end no newline", "text\n**", "text\n"},
		{"real bold preserved", "this is **bold** text", "this is **bold** text"},
		{"consecutive bold collapsed", "**Contact us**\n****Please do not reply**", "**Contact us**\n**Please do not reply**"},
		{"six stars collapsed", "text******more", "text**more"},
		{"nested heading collapsed", "### ### Quilted art piece", "### Quilted art piece"},
		{"nested heading h2", "## ## ## Section", "## Section"},
		{"single heading preserved", "### Normal heading", "### Normal heading"},
		{"empty heading stripped", "text\n###  \nnext", "text\nnext"},
		{"empty heading at end", "text\n### ", "text\n"},
		{"no change", "normal text", "normal text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanPandocArtifacts(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeBoldMarkers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"balanced bold unchanged",
			"this is **bold** text",
			"this is **bold** text",
		},
		{
			"unpaired trailing marker stripped",
			"Additional resources**",
			"Additional resources",
		},
		{
			"unpaired leading marker stripped",
			"ClouDNS** received notification",
			"ClouDNS received notification",
		},
		{
			"cross-paragraph bold split",
			"Build faster**\n\n**Explore Workers",
			"Build faster\n\nExplore Workers",
		},
		{
			"balanced across same paragraph unchanged",
			"See **bold text** here",
			"See **bold text** here",
		},
		{
			"multiple balanced in one para unchanged",
			"**a** and **b**",
			"**a** and **b**",
		},
		{
			"three markers: last stripped",
			"**bold** and stray**",
			"**bold** and stray",
		},
		{
			"no markers unchanged",
			"plain text\n\nanother paragraph",
			"plain text\n\nanother paragraph",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeBoldMarkers(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeUnicodeBullets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"filled circle", "● Item one", "- Item one"},
		{"bullet", "• Item two", "- Item two"},
		{"leading whitespace", "  ● Item", "- Item"},
		{"multiple bullets", "● First\n● Second", "- First\n- Second"},
		{"triangle bullet", "▸ Item", "- Item"},
		{"no bullet", "Normal text", "Normal text"},
		{"mid-line bullet unchanged", "Click ● here", "Click ● here"},
		{"continuation indented", "● First line\nsecond line", "- First line\n  second line"},
		{"continuation stops at blank", "● Item\ncont\n\nNext para", "- Item\n  cont\n\nNext para"},
		{"multi-item continuation", "● A\nwrap\n● B\nwrap", "- A\n  wrap\n- B\n  wrap"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeUnicodeBullets(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompactLooseLists(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single item unchanged", "- item one", "- item one"},
		{"tight list unchanged", "- one\n- two\n- three", "- one\n- two\n- three"},
		{"loose list compacted", "- one\n\n- two\n\n- three", "- one\n- two\n- three"},
		{"pandoc wide bullets", "-   one\n\n-   two", "-   one\n-   two"},
		{"continuation preserved", "-   item long\n    wrap\n\n-   next", "-   item long\n    wrap\n-   next"},
		{"paragraph after list kept", "- one\n\n- two\n\nnext para", "- one\n- two\n\nnext para"},
		{"paragraph before list kept", "text\n\n- one\n\n- two", "text\n\n- one\n- two"},
		{"multi-blank between items", "- one\n\n\n- two", "- one\n- two"},
		{"trailing blanks in list dropped", "- one\n\n- two\n\n", "- one\n- two"},
		{"deep continuation", "-   item\n      deep indent\n\n-   next", "-   item\n      deep indent\n-   next"},
		{"no list unchanged", "hello\n\nworld", "hello\n\nworld"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compactLooseLists(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeListIndent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"8 space indent", "        - item one", "- item one"},
		{"4 space indent", "    - item two", "- item two"},
		{"normal list", "- item three", "- item three"},
		{"asterisk list", "        * item", "* item"},
		{"plus list", "        + item", "+ item"},
		{"3 space no change", "   - not enough", "   - not enough"},
		{"preserves content after", "        - item\n  continuation", "- item\n  continuation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeListIndent(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"nbsp", "hello\u00a0world", "hello world"},
		{"zero-width chars", "he\u200cllo\u200bwor\uFEFFld", "helloworld"},
		{"combining grapheme joiner", "hello\u034fworld", "helloworld"},
		{"soft hyphen", "hello\u00adworld", "helloworld"},
		{"word joiner", "hello\u2060world", "helloworld"},
		{"preheader filler", "text \u034f \u034f \u034f", "text   "},
		{"trailing spaces on blank line", "hello\n   \nworld", "hello\n\nworld"},
		{"filler lines collapse", "hello\n \u034f \u034f\n \u034f\nworld", "hello\n\nworld"},
		{"excessive blank lines", "hello\n\n\n\nworld", "hello\n\nworld"},
		{"leading blank lines", "\n\n\nhello", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHighlightMarkdown(t *testing.T) {
	colors := &markdownColors{
		Heading: "1;32",
		Bold:    "1",
		Italic:  "3",
		Rule:    "2",
		Reset:   "0",
	}
	tests := []struct {
		name  string
		input string
		check func(string) bool
		desc  string
	}{
		{
			"heading",
			"## Hello World",
			func(s string) bool { return strings.Contains(s, "\033[1;32m") && strings.Contains(s, "Hello World") },
			"should contain heading color + text",
		},
		{
			"bold",
			"this is **bold** text",
			func(s string) bool { return strings.Contains(s, "\033[1m") && strings.Contains(s, "bold") },
			"should contain bold ANSI + text",
		},
		{
			"italic",
			"this is *italic* text",
			func(s string) bool { return strings.Contains(s, "\033[3m") && strings.Contains(s, "italic") },
			"should contain italic ANSI + text",
		},
		{
			"multiline bold per-line codes",
			"**line one\nline two**",
			func(s string) bool {
				return strings.Contains(s, "\033[1mline one\033[0m") &&
					strings.Contains(s, "\033[1mline two\033[0m")
			},
			"should bold each line independently",
		},
		{
			"multiline bold italic per-line codes",
			"***line one\nline two***",
			func(s string) bool {
				return strings.Contains(s, "\033[1m\033[3mline one\033[0m\033[0m") &&
					strings.Contains(s, "\033[1m\033[3mline two\033[0m\033[0m")
			},
			"should apply both bold and italic to each line",
		},
		{
			"stray asterisk no cross-paragraph italic",
			"products.* Get started\n\nparagraph\n\n*Visit site",
			func(s string) bool {
				// Neither stray * should trigger italic across paragraphs
				return !strings.Contains(s, "\033[3m")
			},
			"stray * must not bleed italic across paragraphs",
		},
		{
			"horizontal rule dashes",
			"---",
			func(s string) bool { return strings.Contains(s, "\033[2m") },
			"should contain rule color",
		},
		{
			"horizontal rule underscores",
			"___",
			func(s string) bool { return strings.Contains(s, "\033[2m") },
			"should contain rule color",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := highlightMarkdown(tt.input, colors)
			if !tt.check(got) {
				t.Errorf("%s: got %q", tt.desc, got)
			}
		})
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
		{
			"no style unchanged",
			`<div>content</div>`,
			`<div>content</div>`,
		},
		{
			"mso-hide display:none removed",
			`<div style="display:none;mso-hide:all">hidden content here</div>rest`,
			`rest`,
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

func TestCleanPandocArtifactsSuperscript(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single superscript", "text^1^", "text1"},
		{"multiple superscripts", "Save 3%^1^ Apply now^2^", "Save 3%1 Apply now2"},
		{"superscript with letters", "Company^TM^", "CompanyTM"},
		{"no superscript", "plain text", "plain text"},
		{"empty carets not matched", "price^^ sale", "price^^ sale"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanPandocArtifacts(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no escapes", "plain text", "plain text"},
		{"color reset", "\033[0mtext", "text"},
		{"bold color", "\033[1;32mgreen\033[0m", "green"},
		{"multiple sequences", "\033[38;2;136;192;208m[Click here]\033[0m", "[Click here]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
