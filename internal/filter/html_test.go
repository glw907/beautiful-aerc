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

func TestExtractLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantURLs []string
	}{
		{
			"single link",
			"Visit [Example](https://example.com) today.",
			[]string{"https://example.com"},
		},
		{
			"multiple links",
			"See [A](https://a.com) and [B](https://b.com).",
			[]string{"https://a.com", "https://b.com"},
		},
		{
			"empty text link stripped",
			"Title\n\n[](https://tracking.example.com/click?id=abc)\n\nBody",
			nil,
		},
		{
			"no links",
			"Plain text with no links.",
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, urls := extractLinks(tt.input)
			if len(urls) != len(tt.wantURLs) {
				t.Fatalf("got %d URLs, want %d", len(urls), len(tt.wantURLs))
			}
			for i, u := range urls {
				if u != tt.wantURLs[i] {
					t.Errorf("URL[%d] = %q, want %q", i, u, tt.wantURLs[i])
				}
			}
		})
	}
}

func TestExtractLinksCleanMarkdown(t *testing.T) {
	input := "See [A](https://a.com) and [B](https://b.com)."
	cleaned, _ := extractLinks(input)
	if cleaned != "See [A](#) and [B](#)." {
		t.Errorf("got %q, want URLs replaced with #", cleaned)
	}
}

func TestInjectOSC8(t *testing.T) {
	linkStyle := "\x1b[38;2;0;0;255m"
	// Simulate Glamour output: styled link text between linkStyle and reset
	input := "Visit " + linkStyle + "Example\x1b[0m today."
	urls := []string{"https://example.com"}
	got := injectOSC8(input, urls, linkStyle)

	if !strings.Contains(got, "\x1b]8;;https://example.com\x1b\\") {
		t.Error("missing OSC 8 open sequence")
	}
	if !strings.Contains(got, "\x1b]8;;\x1b\\") {
		t.Error("missing OSC 8 close sequence")
	}
}

func TestHTMLLinksClickable(t *testing.T) {
	th := testTheme(t)
	input := `<p>Check <a href="https://tracking.example.com/click?id=abc123">this product</a> out.</p>`
	var buf bytes.Buffer
	if err := HTML(strings.NewReader(input), &buf, th, 80); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	plain := stripANSI(out)
	if !strings.Contains(plain, "this product") {
		t.Error("link text should be preserved")
	}
	if strings.Contains(plain, "tracking.example.com") {
		t.Error("tracking URL should not appear as visible text")
	}
	if !strings.Contains(out, "\x1b]8;;https://tracking.example.com/click?id=abc123\x1b\\") {
		t.Error("OSC 8 hyperlink should be present")
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
	plain := stripANSI(buf.String())
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
