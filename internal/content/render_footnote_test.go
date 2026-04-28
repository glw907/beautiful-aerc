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
	// Short bare URL (≤30 cells): appears in picker list but not in the
	// footnote section. No [^N] marker in body, no horizontal rule, no
	// footnote line — just inline rendering.
	blocks := []Block{
		Paragraph{Spans: []Span{
			Text{Content: "visit "},
			Link{Text: "https://bare.example.com", URL: "https://bare.example.com"},
		}},
	}
	out, urls := RenderBodyWithFootnotes(blocks, theme.Nord, 80)
	if len(urls) != 1 {
		t.Errorf("short bare URL must appear in picker list: got %v", urls)
	}
	flat := stripANSITest(out)
	if strings.Contains(flat, "[^") {
		t.Errorf("no marker should appear in body: %s", flat)
	}
	if !strings.Contains(flat, "https://bare.example.com") {
		t.Errorf("bare URL must still render inline: %s", flat)
	}
	// No footnote section — no horizontal rule for short-bare-only bodies.
	if strings.Contains(flat, "─────") {
		t.Errorf("no footnote rule should appear for short bare URLs: %s", flat)
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
	rewritten, urls, hasMarker := harvestFootnotes(blocks)
	if len(urls) != 1 || urls[0] != url {
		t.Fatalf("expected one harvested url=%q, got %v", url, urls)
	}
	if !hasMarker[0] {
		t.Fatalf("expected hasMarker[0]=true for long bare URL")
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
	// Short bare URL appears in the picker list (hasMarker=false) but
	// the span is passed through unchanged — no [^N] marker glued to it.
	url := "https://example.com/foo"
	blocks := []Block{Paragraph{Spans: []Span{Link{Text: url, URL: url}}}}
	rewritten, urls, hasMarker := harvestFootnotes(blocks)
	if len(urls) != 1 || urls[0] != url {
		t.Fatalf("expected one picker url=%q, got %v", url, urls)
	}
	if hasMarker[0] {
		t.Fatalf("expected hasMarker[0]=false for short bare URL")
	}
	p := rewritten[0].(Paragraph)
	link := p.Spans[0].(Link)
	if link.Text != url {
		t.Fatalf("link.Text = %q, want unchanged %q", link.Text, url)
	}
}

// TestHarvestBareURLLong verifies that a bare URL longer than 30 cells
// emitted by parseSpans flows through to a footnote entry.
func TestHarvestBareURLLong(t *testing.T) {
	url := "https://example.com/very/long/path/that/exceeds/thirty/cells?q=1"
	// ParseBlocks runs parseSpans which now auto-links bare URLs.
	blocks := ParseBlocks("See " + url + " for details.")
	rewritten, urls, _ := harvestFootnotes(blocks)
	if len(urls) != 1 {
		t.Fatalf("expected 1 harvested url, got %v", urls)
	}
	if urls[0] != url {
		t.Errorf("url = %q, want %q", urls[0], url)
	}
	// The rewritten link text must contain a [^1] marker.
	p, ok := rewritten[0].(Paragraph)
	if !ok {
		t.Fatalf("expected Paragraph, got %T", rewritten[0])
	}
	var found bool
	for _, s := range p.Spans {
		if l, ok := s.(Link); ok && l.URL == url {
			found = true
			if !strings.Contains(l.Text, "[^1]") {
				t.Errorf("link text %q missing [^1] marker", l.Text)
			}
		}
	}
	if !found {
		t.Errorf("no Link span with url=%q found in rewritten block", url)
	}
}

// TestHarvestBareURLShort verifies that a short bare URL (≤30 cells)
// emitted by parseSpans is registered in the picker list (hasMarker=false)
// and renders inline without a [^N] marker.
func TestHarvestBareURLShort(t *testing.T) {
	url := "https://example.com/foo"
	blocks := ParseBlocks("See " + url + " now.")
	_, urls, hasMarker := harvestFootnotes(blocks)
	if len(urls) != 1 || urls[0] != url {
		t.Errorf("short bare URL must appear in picker list; got %v", urls)
	}
	if hasMarker[0] {
		t.Errorf("short bare URL must have hasMarker=false; got true")
	}
}

func TestLongBareURLDedupedWithTextLink(t *testing.T) {
	url := "https://example.com/very/long/path/that/exceeds/thirty/cells?q=1"
	blocks := []Block{
		Paragraph{Spans: []Span{Link{Text: url, URL: url}}},
		Paragraph{Spans: []Span{Link{Text: "click here", URL: url}}},
	}
	_, urls, _ := harvestFootnotes(blocks)
	if len(urls) != 1 {
		t.Fatalf("expected one harvested url after dedupe, got %v", urls)
	}
}

// TestHarvestShortBareURLAppearsInPickerNotFootnoteSection verifies the core
// fix: a short bare URL (≤30 cells) is registered in the picker list so Tab
// and 1-9 dispatch work, but the rendered body has no horizontal rule, no
// [^1]: footnote line, and no [^1] marker glued to the URL.
//
// Numbering design: picker index is position in urls (1-based); footnote
// marker [^N] is position in the marker-bearing subset only. When only short
// bare URLs are present the marker-bearing subset is empty, so no markers exist.
func TestHarvestShortBareURLAppearsInPickerNotFootnoteSection(t *testing.T) {
	url := "https://1password.com" // 21 cells — well under the 30-cell threshold
	blocks := []Block{Paragraph{Spans: []Span{Link{Text: url, URL: url}}}}

	_, urls, hasMarker := harvestFootnotes(blocks)
	if len(urls) != 1 {
		t.Fatalf("picker list: got %d urls, want 1", len(urls))
	}
	if urls[0] != url {
		t.Errorf("picker urls[0] = %q, want %q", urls[0], url)
	}
	if hasMarker[0] {
		t.Errorf("hasMarker[0] must be false for short bare URL")
	}

	out, pickerURLs := RenderBodyWithFootnotes(blocks, theme.Nord, 80)
	if len(pickerURLs) != 1 {
		t.Fatalf("RenderBodyWithFootnotes picker: got %d urls, want 1", len(pickerURLs))
	}
	flat := stripANSITest(out)
	if strings.Contains(flat, "[^") {
		t.Errorf("no [^N] marker should appear in body: %s", flat)
	}
	if strings.Contains(flat, "─────") {
		t.Errorf("no footnote rule should appear for short-bare-only body: %s", flat)
	}
	if !strings.Contains(flat, url) {
		t.Errorf("URL must still render inline: %s", flat)
	}
}

// TestHarvestLongBareURLAppearsInBoth verifies that a long bare URL (>30 cells)
// is both in the picker list and has a [^1] footnote marker + footnote line.
func TestHarvestLongBareURLAppearsInBoth(t *testing.T) {
	url := "https://example.com/very/long/path/that/exceeds/thirty/cells?q=1"
	blocks := []Block{Paragraph{Spans: []Span{Link{Text: url, URL: url}}}}

	_, urls, hasMarker := harvestFootnotes(blocks)
	if len(urls) != 1 {
		t.Fatalf("picker list: got %d urls, want 1", len(urls))
	}
	if !hasMarker[0] {
		t.Errorf("hasMarker[0] must be true for long bare URL")
	}

	out, pickerURLs := RenderBodyWithFootnotes(blocks, theme.Nord, 80)
	if len(pickerURLs) != 1 {
		t.Fatalf("RenderBodyWithFootnotes picker: got %d urls, want 1", len(pickerURLs))
	}
	flat := stripANSITest(out)
	if !strings.Contains(flat, "[^1]") {
		t.Errorf("body must contain [^1] marker: %s", flat)
	}
	if !strings.Contains(flat, "[^1]: "+url) {
		t.Errorf("footnote section must contain [^1]: <url>: %s", flat)
	}
	if !strings.Contains(flat, "─────") {
		t.Errorf("footnote rule must appear: %s", flat)
	}
}

// TestHarvestMixed verifies behavior when a short bare URL precedes a long
// bare URL. Both appear in the picker list (len==2). Only the long URL has a
// marker + footnote line. The footnote section labels the long URL [^1] (it is
// the first and only marker-bearing entry), even though it is picker index 2.
//
// Design note: [^N] in body refers to the Nth marker-bearing URL (footnote-
// subset numbering); picker index 1..N spans all URLs including short bare
// ones. These two numbering schemes deliberately diverge when short bare URLs
// are present. The picker is the canonical URL launcher; markers are a reading
// aid for marker-bearing links only.
func TestHarvestMixed(t *testing.T) {
	shortURL := "https://1password.com"
	longURL := "https://example.com/very/long/path/that/exceeds/thirty/cells?q=1"
	blocks := []Block{
		Paragraph{Spans: []Span{Link{Text: shortURL, URL: shortURL}}},
		Paragraph{Spans: []Span{Link{Text: longURL, URL: longURL}}},
	}

	_, urls, hasMarker := harvestFootnotes(blocks)
	// Picker list must contain both, short first.
	if len(urls) != 2 {
		t.Fatalf("picker list: got %d urls, want 2; urls=%v", len(urls), urls)
	}
	if urls[0] != shortURL {
		t.Errorf("picker urls[0] = %q, want %q", urls[0], shortURL)
	}
	if urls[1] != longURL {
		t.Errorf("picker urls[1] = %q, want %q", urls[1], longURL)
	}
	// Short URL has no marker; long URL has marker.
	if hasMarker[0] {
		t.Errorf("hasMarker[0] must be false for short bare URL")
	}
	if !hasMarker[1] {
		t.Errorf("hasMarker[1] must be true for long bare URL")
	}

	out, pickerURLs := RenderBodyWithFootnotes(blocks, theme.Nord, 80)
	if len(pickerURLs) != 2 {
		t.Fatalf("RenderBodyWithFootnotes picker: got %d urls, want 2", len(pickerURLs))
	}
	flat := stripANSITest(out)
	// Short URL: no marker in body.
	if strings.Contains(flat, shortURL+"[^") {
		t.Errorf("short URL must not have a glued marker: %s", flat)
	}
	// Long URL: [^1] marker (first in the marker-bearing subset, not picker index 2).
	if !strings.Contains(flat, "[^1]") {
		t.Errorf("long URL must have [^1] marker (footnote-subset index): %s", flat)
	}
	if !strings.Contains(flat, "[^1]: "+longURL) {
		t.Errorf("footnote section must list long URL as [^1]: %s", flat)
	}
	// No [^2] should exist — the short URL does not appear in the footnote section.
	if strings.Contains(flat, "[^2]") {
		t.Errorf("no [^2] should appear — short URL is not in footnote section: %s", flat)
	}
}
