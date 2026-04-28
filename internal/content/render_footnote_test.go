// SPDX-License-Identifier: MIT

package content

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
)

func TestFootnoteHarvestBasic(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{
			Text{Content: "see "},
			Link{Text: "the docs", URL: "https://example.com/docs"},
			Text{Content: " and "},
			Link{Text: "the spec", URL: "https://example.com/spec"},
		}},
	}
	out, urls := RenderBodyWithFootnotes(blocks, theme.Nord, 80)
	if len(urls) != 2 {
		t.Fatalf("urls: got %d, want 2", len(urls))
	}
	if urls[0] != "https://example.com/docs" || urls[1] != "https://example.com/spec" {
		t.Errorf("urls order: %v", urls)
	}
	flat := stripANSITest(out)
	for _, want := range []string{"[^1]", "[^2]", "[^1]: https://example.com/docs", "[^2]: https://example.com/spec"} {
		if !strings.Contains(flat, want) {
			t.Errorf("missing %q in output:\n%s", want, flat)
		}
	}
}

func TestFootnoteDedupe(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{
			Link{Text: "first", URL: "https://example.com"},
			Text{Content: " and "},
			Link{Text: "again", URL: "https://example.com"},
		}},
	}
	out, urls := RenderBodyWithFootnotes(blocks, theme.Nord, 80)
	if len(urls) != 1 {
		t.Fatalf("urls: got %d, want 1 (deduped)", len(urls))
	}
	flat := stripANSITest(out)
	if strings.Contains(flat, "[^2]") {
		t.Errorf("dedupe failed: [^2] appeared in output:\n%s", flat)
	}
	if strings.Count(flat, "[^1]") != 3 { // two inline + one in list
		t.Errorf("expected three [^1] markers (2 inline + 1 list), got %d in:\n%s", strings.Count(flat, "[^1]"), flat)
	}
}

func TestFootnoteSkipAutoLinked(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{
			Text{Content: "visit "},
			Link{Text: "https://bare.example.com", URL: "https://bare.example.com"},
		}},
	}
	out, urls := RenderBodyWithFootnotes(blocks, theme.Nord, 80)
	if len(urls) != 0 {
		t.Errorf("auto-linked URL must not be footnoted: got %v", urls)
	}
	flat := stripANSITest(out)
	if strings.Contains(flat, "[^") {
		t.Errorf("no marker should appear: %s", flat)
	}
	if !strings.Contains(flat, "https://bare.example.com") {
		t.Errorf("bare URL must still render inline: %s", flat)
	}
}

func TestFootnoteLastWordAtomic(t *testing.T) {
	// Wide enough that "documentation[^1]" fits but not "thorough documentation[^1]".
	blocks := []Block{
		Paragraph{Spans: []Span{
			Text{Content: strings.Repeat("a ", 30)},
			Link{Text: "thorough documentation", URL: "https://example.com"},
		}},
	}
	out, _ := RenderBodyWithFootnotes(blocks, theme.Nord, 50)
	flat := stripANSITest(out)
	// Find the line containing "[^1]" and assert "documentation[^1]" is unbroken.
	for _, line := range strings.Split(flat, "\n") {
		if strings.Contains(line, "[^1]") && !strings.Contains(line, "[^1]: ") {
			if !strings.Contains(line, "documentation"+nbsp+"[^1]") && !strings.Contains(line, "documentation [^1]") {
				// Wordwrap should not have split documentation from [^1].
				t.Errorf("link last word split from marker: %q", line)
			}
		}
	}
}

func TestFootnoteEmptyURL(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{
			Link{Text: "weird", URL: ""},
		}},
	}
	out, urls := RenderBodyWithFootnotes(blocks, theme.Nord, 80)
	if len(urls) != 0 {
		t.Errorf("empty URL must not be footnoted: %v", urls)
	}
	if strings.Contains(stripANSITest(out), "[^") {
		t.Error("empty URL must produce no marker")
	}
}

func TestFootnoteListAfterRule(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{
			Link{Text: "click", URL: "https://x.example"},
		}},
	}
	out, _ := RenderBodyWithFootnotes(blocks, theme.Nord, 40)
	flat := stripANSITest(out)
	ruleIdx := strings.Index(flat, strings.Repeat("─", 40))
	listIdx := strings.Index(flat, "[^1]: ")
	if ruleIdx < 0 || listIdx < 0 {
		t.Fatalf("rule or list missing; rule=%d list=%d output:\n%s", ruleIdx, listIdx, flat)
	}
	if listIdx < ruleIdx {
		t.Errorf("list must follow rule; rule=%d list=%d", ruleIdx, listIdx)
	}
}

func TestLongBareURLFootnoted(t *testing.T) {
	url := "https://example.com/very/long/path/that/exceeds/thirty/cells?query=1"
	blocks := []Block{Paragraph{Spans: []Span{Link{Text: url, URL: url}}}}
	rewritten, urls := harvestFootnotes(blocks)
	if len(urls) != 1 || urls[0] != url {
		t.Fatalf("expected one harvested url=%q, got %v", url, urls)
	}
	p := rewritten[0].(Paragraph)
	link := p.Spans[0].(Link)
	want := "example.com/very…" + nbsp + "[^1]"
	if link.Text != want {
		t.Fatalf("link.Text = %q, want %q", link.Text, want)
	}
	if link.URL != url {
		t.Fatalf("link.URL = %q, want %q", link.URL, url)
	}
}

func TestShortBareURLPassThrough(t *testing.T) {
	url := "https://example.com/foo"
	blocks := []Block{Paragraph{Spans: []Span{Link{Text: url, URL: url}}}}
	rewritten, urls := harvestFootnotes(blocks)
	if len(urls) != 0 {
		t.Fatalf("expected no harvested urls, got %v", urls)
	}
	p := rewritten[0].(Paragraph)
	link := p.Spans[0].(Link)
	if link.Text != url {
		t.Fatalf("link.Text = %q, want unchanged %q", link.Text, url)
	}
}

func TestLongBareURLDedupedWithTextLink(t *testing.T) {
	url := "https://example.com/very/long/path/that/exceeds/thirty/cells?q=1"
	blocks := []Block{
		Paragraph{Spans: []Span{Link{Text: url, URL: url}}},
		Paragraph{Spans: []Span{Link{Text: "click here", URL: url}}},
	}
	_, urls := harvestFootnotes(blocks)
	if len(urls) != 1 {
		t.Fatalf("expected one harvested url after dedupe, got %v", urls)
	}
}
