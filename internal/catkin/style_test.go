package catkin

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestTokenizeBoldItalicCodeLink(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		kinds []spanKind
		texts []string
	}{
		{
			name:  "plain",
			in:    "hello world",
			kinds: []spanKind{spanText},
			texts: []string{"hello world"},
		},
		{
			name:  "bold double-star",
			in:    "a **b** c",
			kinds: []spanKind{spanText, spanBold, spanText},
			texts: []string{"a ", "**b**", " c"},
		},
		{
			name:  "bold double-underscore",
			in:    "__bold__ trailing",
			kinds: []spanKind{spanBold, spanText},
			texts: []string{"__bold__", " trailing"},
		},
		{
			name:  "italic single-star",
			in:    "an *em* word",
			kinds: []spanKind{spanText, spanItalic, spanText},
			texts: []string{"an ", "*em*", " word"},
		},
		{
			name:  "code wins over emphasis",
			in:    "use `**not-bold**` here",
			kinds: []spanKind{spanText, spanCode, spanText},
			texts: []string{"use ", "`**not-bold**`", " here"},
		},
		{
			name:  "triple-star is bold-italic",
			in:    "***both***",
			kinds: []spanKind{spanBoldItalic},
			texts: []string{"***both***"},
		},
		{
			name:  "longest wins: ** before *",
			in:    "**bold**",
			kinds: []spanKind{spanBold},
			texts: []string{"**bold**"},
		},
		{
			name:  "link",
			in:    "see [docs](https://example.com) now",
			kinds: []spanKind{spanText, spanLink, spanText},
			texts: []string{"see ", "[docs](https://example.com)", " now"},
		},
		{
			name:  "broken bold passes through",
			in:    "**unclosed bold",
			kinds: []spanKind{spanText},
			texts: []string{"**unclosed bold"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenize(tc.in)
			if len(got) != len(tc.kinds) {
				t.Fatalf("span count: got %d (%+v), want %d", len(got), got, len(tc.kinds))
			}
			var rebuilt strings.Builder
			for i, sp := range got {
				if sp.kind != tc.kinds[i] {
					t.Errorf("span[%d] kind = %d, want %d", i, sp.kind, tc.kinds[i])
				}
				if sp.text != tc.texts[i] {
					t.Errorf("span[%d] text = %q, want %q", i, sp.text, tc.texts[i])
				}
				rebuilt.WriteString(sp.text)
			}
			if rebuilt.String() != tc.in {
				t.Errorf("lossless rebuild = %q, want %q", rebuilt.String(), tc.in)
			}
		})
	}
}

func TestTokenizeLinkParts(t *testing.T) {
	got := tokenize("[t](u)")
	if len(got) != 1 || got[0].kind != spanLink {
		t.Fatalf("got %+v", got)
	}
	if got[0].linkText != "t" || got[0].linkURL != "u" {
		t.Errorf("link parts: text=%q url=%q", got[0].linkText, got[0].linkURL)
	}
}

func TestRenderSpansAppliesStyles(t *testing.T) {
	st := Styles{
		Bold:   lipgloss.NewStyle().Bold(true),
		Italic: lipgloss.NewStyle().Italic(true),
	}
	out := renderSpans(tokenize("a **b** *c*"), st)
	if !strings.Contains(out, "\x1b[1m") {
		t.Errorf("expected bold escape in %q", out)
	}
	if !strings.Contains(out, "\x1b[3m") {
		t.Errorf("expected italic escape in %q", out)
	}
}

func TestHighlightFenceFallback(t *testing.T) {
	out := highlightFence("nope-not-a-lexer", "abc\ndef", Styles{})
	if len(out) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(out))
	}
	// Zero CodeBlock style → output equals raw.
	if out[0] != "abc" || out[1] != "def" {
		t.Errorf("fallback content: %q, %q", out[0], out[1])
	}
}

func TestHighlightFenceKnownLexerEmitsTokens(t *testing.T) {
	out := highlightFence("go", "package main\n", Styles{})
	if len(out) == 0 {
		t.Fatal("expected at least one styled line")
	}
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "package") {
		t.Errorf("styled output dropped source: %q", joined)
	}
}
