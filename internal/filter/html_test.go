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

func TestMarkLinks(t *testing.T) {
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
			"no links",
			"Plain text with no links.",
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, urls := markLinks(tt.input)
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

func TestResolveLinks(t *testing.T) {
	// Simulate what Glamour produces from marked text
	input := "Visit \x1b[38;2;0;0;255m\x020;Example\x03\x1b[0m today."
	urls := []string{"https://example.com"}
	got := resolveLinks(input, urls)

	if !strings.Contains(got, "\x1b]8;;https://example.com\x1b\\") {
		t.Error("missing OSC 8 open sequence")
	}
	if !strings.Contains(got, "\x1b]8;;\x1b\\") {
		t.Error("missing OSC 8 close sequence")
	}
	if strings.Contains(got, "\x02") || strings.Contains(got, "\x03") {
		t.Error("markers should be replaced")
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
