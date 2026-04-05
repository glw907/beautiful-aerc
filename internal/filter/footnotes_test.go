package filter

import (
	"strings"
	"testing"
)

func TestConvertToFootnotes(t *testing.T) {
	m := linkTextMarker
	tests := []struct {
		name     string
		input    string
		wantBody string
		wantRefs []footnoteRef
	}{
		{
			"single link",
			"Click [here] to continue.\n\n  [here]: https://example.com\n",
			"Click " + m + "here" + m + "[^1] to continue.",
			[]footnoteRef{{1, "https://example.com"}},
		},
		{
			"multiple links",
			"Visit [home] and [about].\n\n  [home]: https://example.com\n  [about]: https://example.com/about\n",
			"Visit " + m + "home" + m + "[^1] and " + m + "about" + m + "[^2].",
			[]footnoteRef{{1, "https://example.com"}, {2, "https://example.com/about"}},
		},
		{
			"duplicate link text with numeric fallback",
			"[Click here] and [Click here][1]\n\n  [Click here]: https://example.com/a\n  [1]: https://example.com/b\n",
			m + "Click here" + m + "[^1] and " + m + "Click here" + m + "[^2]",
			[]footnoteRef{{1, "https://example.com/a"}, {2, "https://example.com/b"}},
		},
		{
			"self-referencing link becomes plain URL",
			"Visit <https://example.com> for info.\n",
			"Visit https://example.com for info.",
			nil,
		},
		{
			"autolink with no ref defs",
			"See <https://example.com>.\n",
			"See https://example.com.",
			nil,
		},
		{
			"no links",
			"Just plain text.\n",
			"Just plain text.",
			nil,
		},
		{
			"self-ref link in ref defs skipped",
			"Visit [https://example.com] for info.\n\n  [https://example.com]: https://example.com\n",
			"Visit https://example.com for info.",
			nil,
		},
		{
			"standalone image stripped",
			"![Logo][1]\n\n  [Logo]: https://example.com/logo.png\n  [1]: https://example.com/logo.png\n",
			"",
			nil,
		},
		{
			"image without alt text stripped",
			"![][1]\n\n  [1]: https://example.com/pixel.png\n",
			"",
			nil,
		},
		{
			"image link ref becomes footnoted text",
			"[![Banner]][1]\n\n  [Banner]: https://example.com/banner.png\n  [1]: https://example.com\n",
			m + "Banner" + m + "[^1]",
			[]footnoteRef{{1, "https://example.com"}},
		},
		{
			"mailto link gets footnote",
			"[CONTACT US]\n\n  [CONTACT US]: mailto:help@example.com\n",
			m + "CONTACT US" + m + "[^1]",
			[]footnoteRef{{1, "mailto:help@example.com"}},
		},
		{
			"leading space in label matches trimmed ref def",
			"[ Reply to Amy]\n\n  [Reply to Amy]: https://example.com/reply\n",
			m + "Reply to Amy" + m + "[^1]",
			[]footnoteRef{{1, "https://example.com/reply"}},
		},
		{
			"schemeless self-ref stripped",
			"[rmd.me/abc123]\n\n  [rmd.me/abc123]: http://rmd.me/abc123\n",
			"rmd.me/abc123",
			nil,
		},
		{
			"empty-text ref stripped",
			"Hello [][1] world.\n\n  [1]: https://example.com\n",
			"Hello  world.",
			nil,
		},
		{
			"emphasis stripped from link text",
			"[*click here*]\n\n  [*click here*]: https://example.com\n",
			m + "click here" + m + "[^1]",
			[]footnoteRef{{1, "https://example.com"}},
		},
		{
			"bold emphasis stripped from link text",
			"[**click here**]\n\n  [**click here**]: https://example.com\n",
			m + "click here" + m + "[^1]",
			[]footnoteRef{{1, "https://example.com"}},
		},
		{
			"ref def with title continuation line",
			"[Click here][1]\n\n  [1]: https://example.com\n    \"Example\"\n",
			m + "Click here" + m + "[^1]",
			[]footnoteRef{{1, "https://example.com"}},
		},
		{
			"text-label shortcut ref gets footnote",
			"[REGISTER]\n\n  [REGISTER]: https://example.com/register\n",
			m + "REGISTER" + m + "[^1]",
			[]footnoteRef{{1, "https://example.com/register"}},
		},
		{
			"unresolved ref brackets stripped",
			"Hello [unknown ref] world.\n",
			"Hello unknown ref world.",
			nil,
		},
		{
			"empty label ref def does not block scanner",
			"[Click here]\n\n  []: https://example.com/empty\n  [Click here]: https://example.com\n",
			m + "Click here" + m + "[^1]",
			[]footnoteRef{{1, "https://example.com"}},
		},
		{
			// Social media icon links have space-only link text; they should be
			// silently dropped with no footnote marker or orphaned ref entry.
			"empty display text ref produces no marker or footnote",
			"Before [ ][1] after.\n\n  [1]: https://example.com/icon\n",
			"Before  after.",
			nil,
		},
		{
			// When two refs have empty display text, both are dropped and
			// neither appears in the footnote section.
			"multiple empty display text refs all dropped",
			"[ ][1] [ ][2]\n\n  [1]: https://fb.com/\n  [2]: https://twitter.com/\n",
			"",
			nil,
		},
		{
			// Pandoc duplicate-anchor pattern: [Text][ ][Text] after space ref
			// is stripped produces two adjacent identical footnoted links.
			"adjacent duplicate footnoted links collapsed",
			"read [Privacy Policy][ ][Privacy Policy] now.\n\n  [Privacy Policy]: https://example.com/privacy\n",
			"read " + m + "Privacy Policy" + m + "[^1] now.",
			[]footnoteRef{{1, "https://example.com/privacy"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, refs := convertToFootnotes(tt.input)
			body = strings.TrimSpace(body)
			if body != tt.wantBody {
				t.Errorf("body:\ngot:  %q\nwant: %q", body, tt.wantBody)
			}
			if len(refs) != len(tt.wantRefs) {
				t.Errorf("refs count: got %d, want %d\nrefs: %v", len(refs), len(tt.wantRefs), refs)
				return
			}
			for i, want := range tt.wantRefs {
				if refs[i] != want {
					t.Errorf("refs[%d]:\ngot:  %v\nwant: %v", i, refs[i], want)
				}
			}
		})
	}
}

func TestStyleFootnotes(t *testing.T) {
	colors := &footnoteColors{
		LinkText: "38;2;136;192;208",
		Dim:      "38;2;97;110;136",
		LinkURL:  "38;2;97;110;136",
		Reset:    "0",
	}
	m := linkTextMarker

	t.Run("body link text colored", func(t *testing.T) {
		body := m + "click here" + m + "[^1] to go"
		refs := []footnoteRef{{1, "https://example.com"}}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, "\033[38;2;136;192;208mclick here\033[0m") {
			t.Errorf("link text not colored: %q", got)
		}
	})

	t.Run("footnote marker dimmed", func(t *testing.T) {
		body := m + "click here" + m + "[^1] to go"
		refs := []footnoteRef{{1, "https://example.com"}}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, "\033[38;2;97;110;136m[^1]\033[0m") {
			t.Errorf("marker not dimmed: %q", got)
		}
	})

	t.Run("separator line present", func(t *testing.T) {
		body := m + "text" + m + "[^1]"
		refs := []footnoteRef{{1, "https://example.com"}}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, strings.Repeat("─", 40)) {
			t.Errorf("separator missing: %q", got)
		}
	})

	t.Run("reference URL colored with OSC 8 hyperlink", func(t *testing.T) {
		body := m + "text" + m + "[^1]"
		refs := []footnoteRef{{1, "https://example.com"}}
		got := styleFootnotes(body, refs, 40, colors)
		// URL should be wrapped in OSC 8 hyperlink escape sequences.
		want := "\033[38;2;97;110;136m\033]8;;https://example.com\033\\https://example.com\033]8;;\033\\\033[0m"
		if !strings.Contains(got, want) {
			t.Errorf("URL not colored with OSC 8 hyperlink:\ngot:  %q\nwant substring: %q", got, want)
		}
	})

	t.Run("long URL truncated visually", func(t *testing.T) {
		body := m + "text" + m + "[^1]"
		longURL := "https://example.com/very/long/path/that/exceeds/the/column/width"
		refs := []footnoteRef{{1, longURL}}
		cols := 30
		got := styleFootnotes(body, refs, cols, colors)
		// Full URL should be in OSC 8 escape (clickable).
		if !strings.Contains(got, "\033]8;;"+longURL+"\033\\") {
			t.Errorf("full URL not in OSC 8 hyperlink: %q", got)
		}
		// Display text should be truncated: "[^1]: " is 6 chars, leaving 24 for URL.
		maxDisplay := cols - len("[^1]: ")
		truncated := longURL[:maxDisplay-1] + "…"
		if !strings.Contains(got, truncated) {
			t.Errorf("URL not visually truncated:\ngot:  %q\nwant substring: %q", got, truncated)
		}
	})

	t.Run("no refs no separator", func(t *testing.T) {
		body := "just text"
		got := styleFootnotes(body, nil, 40, colors)
		if strings.Contains(got, "─") {
			t.Errorf("separator should not appear with no refs: %q", got)
		}
		if got != "just text" {
			t.Errorf("body changed: %q", got)
		}
	})

	t.Run("only link text is colored not surrounding text", func(t *testing.T) {
		body := "see " + m + "here" + m + "[^1] for details"
		refs := []footnoteRef{{1, "https://example.com"}}
		got := styleFootnotes(body, refs, 40, colors)
		// "see " should NOT be colored
		if strings.Contains(got, "\033[38;2;136;192;208msee") {
			t.Errorf("surrounding text should not be colored: %q", got)
		}
		// "here" SHOULD be colored
		if !strings.Contains(got, "\033[38;2;136;192;208mhere\033[0m") {
			t.Errorf("link text not colored: %q", got)
		}
	})
}

func TestExtractFootnoteLinks(t *testing.T) {
	m := linkTextMarker
	tests := []struct {
		name     string
		body     string
		refs     []footnoteRef
		wantLinks []FootnoteLink
	}{
		{
			"single link",
			"Click " + m + "here" + m + "[^1] to go.",
			[]footnoteRef{{1, "https://example.com"}},
			[]FootnoteLink{{Label: "here", URL: "https://example.com"}},
		},
		{
			"multiple links",
			"Visit " + m + "home" + m + "[^1] and " + m + "about" + m + "[^2].",
			[]footnoteRef{{1, "https://example.com"}, {2, "https://example.com/about"}},
			[]FootnoteLink{
				{Label: "home", URL: "https://example.com"},
				{Label: "about", URL: "https://example.com/about"},
			},
		},
		{
			"duplicate URLs deduped",
			m + "link A" + m + "[^1] and " + m + "link B" + m + "[^2]",
			[]footnoteRef{{1, "https://example.com"}, {2, "https://example.com"}},
			[]FootnoteLink{{Label: "link A", URL: "https://example.com"}},
		},
		{
			"no markers returns nil",
			"just plain text",
			[]footnoteRef{{1, "https://example.com"}},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFootnoteLinks(tt.body, tt.refs)
			if len(got) != len(tt.wantLinks) {
				t.Errorf("count: got %d, want %d\ngot: %v", len(got), len(tt.wantLinks), got)
				return
			}
			for i, want := range tt.wantLinks {
				if got[i] != want {
					t.Errorf("[%d]: got %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

func TestStripEmphasis(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello", "hello"},
		{"single italic", "*hello*", "hello"},
		{"double bold", "**hello**", "hello"},
		{"no markers", "hello world", "hello world"},
		{"only opening", "*hello", "hello"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripEmphasis(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
