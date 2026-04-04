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
		wantRefs []string
	}{
		{
			"single link",
			"Click [here] to continue.\n\n  [here]: https://example.com\n",
			"Click " + m + "here" + m + "[^1] to continue.",
			[]string{"[^1]: https://example.com"},
		},
		{
			"multiple links",
			"Visit [home] and [about].\n\n  [home]: https://example.com\n  [about]: https://example.com/about\n",
			"Visit " + m + "home" + m + "[^1] and " + m + "about" + m + "[^2].",
			[]string{"[^1]: https://example.com", "[^2]: https://example.com/about"},
		},
		{
			"duplicate link text with numeric fallback",
			"[Click here] and [Click here][1]\n\n  [Click here]: https://example.com/a\n  [1]: https://example.com/b\n",
			m + "Click here" + m + "[^1] and " + m + "Click here" + m + "[^2]",
			[]string{"[^1]: https://example.com/a", "[^2]: https://example.com/b"},
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
					t.Errorf("refs[%d]:\ngot:  %q\nwant: %q", i, refs[i], want)
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
		refs := []string{"[^1]: https://example.com"}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, "\033[38;2;136;192;208mclick here\033[0m") {
			t.Errorf("link text not colored: %q", got)
		}
	})

	t.Run("footnote marker dimmed", func(t *testing.T) {
		body := m + "click here" + m + "[^1] to go"
		refs := []string{"[^1]: https://example.com"}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, "\033[38;2;97;110;136m[^1]\033[0m") {
			t.Errorf("marker not dimmed: %q", got)
		}
	})

	t.Run("separator line present", func(t *testing.T) {
		body := m + "text" + m + "[^1]"
		refs := []string{"[^1]: https://example.com"}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, strings.Repeat("─", 40)) {
			t.Errorf("separator missing: %q", got)
		}
	})

	t.Run("reference URL colored", func(t *testing.T) {
		body := m + "text" + m + "[^1]"
		refs := []string{"[^1]: https://example.com"}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, "\033[38;2;97;110;136mhttps://example.com\033[0m") {
			t.Errorf("URL not colored: %q", got)
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
		refs := []string{"[^1]: https://example.com"}
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
