package filter

import (
	"strings"
	"testing"
)

func TestConvertToFootnotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBody string
		wantRefs []string
	}{
		{
			"single link",
			"Click [here] to continue.\n\n  [here]: https://example.com\n",
			"Click here[^1] to continue.",
			[]string{"[^1]: https://example.com"},
		},
		{
			"multiple links",
			"Visit [home] and [about].\n\n  [home]: https://example.com\n  [about]: https://example.com/about\n",
			"Visit home[^1] and about[^2].",
			[]string{"[^1]: https://example.com", "[^2]: https://example.com/about"},
		},
		{
			"duplicate link text with numeric fallback",
			"[Click here] and [Click here][1]\n\n  [Click here]: https://example.com/a\n  [1]: https://example.com/b\n",
			"Click here[^1] and Click here[^2]",
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
