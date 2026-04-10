package filter

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/theme"
)

func testTheme(t *testing.T) *theme.Theme {
	t.Helper()
	content := `name = "test"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"

[tokens]
heading = { color = "color_success", bold = true }
bold = { bold = true }
italic = { italic = true }
link_text = { color = "accent_secondary" }
rule = { color = "fg_dim" }
hdr_key = { color = "accent_primary", bold = true }
hdr_value = { color = "fg_base" }
hdr_dim = { color = "fg_dim" }
`
	dir := t.TempDir()
	path := dir + "/test.toml"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(path)
	if err != nil {
		t.Fatalf("loading theme: %v", err)
	}
	return th
}

func TestHTML(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string // substring in ANSI-stripped output
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
			name: "data table rendered",
			html: `<table><thead><tr><th>A</th><th>B</th></tr></thead>
                    <tbody><tr><td>1</td><td>2</td></tr></tbody></table>`,
			want: "A",
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
			th := testTheme(t)
			var buf bytes.Buffer
			err := HTML(strings.NewReader(tt.html), &buf, th, 80)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			plain := stripANSI(buf.String())
			if !strings.Contains(plain, tt.want) {
				t.Errorf("output missing %q\ngot: %s", tt.want, plain)
			}
		})
	}
}

func TestHTMLDisplayNoneNotInOutput(t *testing.T) {
	th := testTheme(t)
	input := `<p>Show</p><div style="display:none"><p>Secret</p></div>`
	var buf bytes.Buffer
	if err := HTML(strings.NewReader(input), &buf, th, 80); err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(buf.String())
	if strings.Contains(plain, "Secret") {
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

func TestHTMLLinksVisible(t *testing.T) {
	th := testTheme(t)
	input := `<p>Check <a href="https://example.com/product">this product</a> out.</p>`
	var buf bytes.Buffer
	if err := HTML(strings.NewReader(input), &buf, th, 80); err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(buf.String())
	if !strings.Contains(plain, "this product") {
		t.Error("link text should be preserved")
	}
	if !strings.Contains(plain, "example.com") {
		t.Error("URL should be visible for terminal click detection")
	}
}

func TestHTMLWrapWidth(t *testing.T) {
	th := testTheme(t)
	// Long paragraph that will wrap — no line should exceed wrapWidth
	// visible characters, and no word should be orphaned (single short
	// word alone on a non-final line).
	input := `<p>The Stock Investing Account is a limited-discretion investment product offered by Wealthfront Advisers LLC, an SEC-registered investment advisor. Brokerage products and services are provided by Wealthfront Brokerage LLC, Member FINRA/SIPC.</p>`
	var buf bytes.Buffer
	if err := HTML(strings.NewReader(input), &buf, th, 80); err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(buf.String())
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	for i, line := range lines {
		visible := strings.TrimRight(line, " ")
		if len(visible) > wrapWidth {
			t.Errorf("line %d exceeds %d chars: %q (%d)", i+1, wrapWidth, visible, len(visible))
		}
		// Orphan check: a non-final, non-blank line with <= 3 visible
		// chars followed by a non-blank line indicates a bad wrap.
		if i < len(lines)-1 && len(visible) > 0 && len(visible) <= 3 {
			next := strings.TrimRight(lines[i+1], " ")
			if len(next) > 0 {
				t.Errorf("orphaned word %q on line %d", visible, i+1)
			}
		}
	}
}

func TestReflowParagraph(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		check func(t *testing.T, got string)
	}{
		{
			name:  "even distribution avoids orphans",
			input: "This is a test of the minimum raggedness algorithm that should distribute words evenly across lines rather than greedily filling each line and leaving a short runt at the end.",
			width: 60,
			check: func(t *testing.T, got string) {
				t.Helper()
				lines := strings.Split(got, "\n")
				for i, line := range lines {
					if len(line) > 60 {
						t.Errorf("line %d exceeds 60 chars: %q (%d)", i+1, line, len(line))
					}
					// No orphaned short word on a non-final line.
					if i < len(lines)-1 && len(strings.TrimSpace(line)) > 0 && len(strings.TrimSpace(line)) <= 5 {
						t.Errorf("orphaned short fragment %q on line %d", strings.TrimSpace(line), i+1)
					}
				}
			},
		},
		{
			name:  "respects width limit",
			input: "The Stock Investing Account is a limited-discretion investment product offered by Wealthfront Advisers LLC, an SEC-registered investment advisor.",
			width: 78,
			check: func(t *testing.T, got string) {
				t.Helper()
				for i, line := range strings.Split(got, "\n") {
					if len(line) > 78 {
						t.Errorf("line %d exceeds 78 chars: %q (%d)", i+1, line, len(line))
					}
				}
			},
		},
		{
			name:  "short text unchanged",
			input: "Hello world",
			width: 78,
			check: func(t *testing.T, got string) {
				t.Helper()
				if got != "Hello world" {
					t.Errorf("got %q, want %q", got, "Hello world")
				}
			},
		},
		{
			name:  "empty input",
			input: "",
			width: 78,
			check: func(t *testing.T, got string) {
				t.Helper()
				if got != "" {
					t.Errorf("got %q, want empty", got)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reflowParagraph(tt.input, tt.width)
			tt.check(t, got)
		})
	}
}

func TestMarkdownTokensKeepsLinksAtomic(t *testing.T) {
	input := "Visit our [Help Center](#) or reply."
	tokens := markdownTokens(input)
	for _, tok := range tokens {
		if tok == "[Help" || tok == "Center](#)" {
			t.Errorf("link text split into separate tokens: %q", tokens)
			break
		}
	}
	found := false
	for _, tok := range tokens {
		if tok == "[Help Center](#)" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected atomic [Help Center](#) token, got: %q", tokens)
	}
}

func TestReflowKeepsLinkTextTogether(t *testing.T) {
	input := "Questions? Visit our [Help Center](#) or reply to this email for help."
	got := reflowParagraph(input, 78)
	if strings.Contains(got, "[Help\n") || strings.Contains(got, "Help\nCenter") {
		t.Errorf("link text split across lines:\n%s", got)
	}
}

func TestHTMLParagraphSpacing(t *testing.T) {
	th := testTheme(t)
	input := `<p>First paragraph.</p><p>Second paragraph.</p><p>Third paragraph.</p>`
	var buf bytes.Buffer
	if err := HTML(strings.NewReader(input), &buf, th, 80); err != nil {
		t.Fatal(err)
	}
	plain := stripTrailingSpaces(stripANSI(buf.String()))
	// Paragraphs should be separated by blank lines.
	if !strings.Contains(plain, "First paragraph.\n\n") {
		t.Errorf("missing blank line between first and second paragraphs:\n%q", plain)
	}
	if !strings.Contains(plain, "Second paragraph.\n\n") {
		t.Errorf("missing blank line between second and third paragraphs:\n%q", plain)
	}
}

func TestReflowMarkdownPreservesNonParagraphs(t *testing.T) {
	input := "# Heading\n\nParagraph text that is long enough to need wrapping at seventy-eight columns for proper display.\n\n- list item\n\n> blockquote"
	got := reflowMarkdown(input, 78)
	if !strings.HasPrefix(got, "# Heading") {
		t.Error("heading should be preserved")
	}
	if !strings.Contains(got, "- list item") {
		t.Error("list should be preserved")
	}
	if !strings.Contains(got, "> blockquote") {
		t.Error("blockquote should be preserved")
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

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no escapes", "plain text", "plain text"},
		{"color reset", "\033[0mtext", "text"},
		{"bold color", "\033[1;32mgreen\033[0m", "green"},
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

func TestIsParagraphSkipsHardBreaks(t *testing.T) {
	block := "Line one  \nLine two  \nLine three"
	if isParagraph(block) {
		t.Error("block with hard breaks should not be a paragraph")
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

func TestIsBlockquote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"single quote line", "> Hello", true},
		{"multi-line with separator", "> Hello\n>\n> World", true},
		{"plain text", "Hello world", false},
		{"mixed", "> Quoted\nNot quoted", false},
		{"bare separator only", ">", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBlockquote(tt.input)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReflowBlockquote(t *testing.T) {
	input := "> Short line.\n>\n> This is a longer paragraph that should be reflowed to fit within the width minus the blockquote prefix."
	got := reflowBlockquote(input, 40)

	// Every non-empty line must start with "> " or be bare ">"
	for _, line := range strings.Split(got, "\n") {
		if line == "" {
			continue
		}
		if line != ">" && !strings.HasPrefix(line, "> ") {
			t.Errorf("line missing blockquote prefix: %q", line)
		}
	}

	// No content line should exceed 40 chars
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 40 {
			t.Errorf("line exceeds width: %q (%d chars)", line, len(line))
		}
	}

	// Paragraph separator must be preserved
	if !strings.Contains(got, "\n>\n") {
		t.Error("paragraph separator lost")
	}
}

// stripTrailingSpaces removes trailing whitespace from each line.
// Glamour pads lines to the word wrap width with spaces.
func stripTrailingSpaces(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}
