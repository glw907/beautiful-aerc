// SPDX-License-Identifier: MIT

package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func spansEqual(t *testing.T, got, want []Span) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("span count: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("span[%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParseSpans(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Span
	}{
		{
			name:  "plain text",
			input: "hello world",
			want:  []Span{Text{Content: "hello world"}},
		},
		{
			name:  "bold",
			input: "hello **world**",
			want:  []Span{Text{Content: "hello "}, Bold{Content: "world"}},
		},
		{
			name:  "italic",
			input: "hello *world*",
			want:  []Span{Text{Content: "hello "}, Italic{Content: "world"}},
		},
		{
			name:  "inline code",
			input: "use `fmt.Println`",
			want:  []Span{Text{Content: "use "}, Code{Content: "fmt.Println"}},
		},
		{
			name:  "link",
			input: "visit [example](https://example.com) today",
			want: []Span{
				Text{Content: "visit "},
				Link{Text: "example", URL: "https://example.com"},
				Text{Content: " today"},
			},
		},
		{
			name:  "mixed",
			input: "**bold** and *italic* and `code`",
			want: []Span{
				Bold{Content: "bold"},
				Text{Content: " and "},
				Italic{Content: "italic"},
				Text{Content: " and "},
				Code{Content: "code"},
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSpans(tt.input)
			spansEqual(t, got, tt.want)
		})
	}
}

func TestParseBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		types []blockKind
	}{
		{
			name:  "single paragraph",
			input: "Hello world",
			types: []blockKind{kindParagraph},
		},
		{
			name:  "two paragraphs",
			input: "First paragraph.\n\nSecond paragraph.",
			types: []blockKind{kindParagraph, kindParagraph},
		},
		{
			name:  "heading",
			input: "# Title\n\nBody text.",
			types: []blockKind{kindHeading, kindParagraph},
		},
		{
			name:  "heading levels",
			input: "## Level 2\n\n### Level 3",
			types: []blockKind{kindHeading, kindHeading},
		},
		{
			name:  "blockquote",
			input: "> quoted text",
			types: []blockKind{kindBlockquote},
		},
		{
			name:  "nested blockquote",
			input: "> > deeply quoted",
			types: []blockKind{kindBlockquote},
		},
		{
			name:  "horizontal rule",
			input: "Above\n\n---\n\nBelow",
			types: []blockKind{kindParagraph, kindRule, kindParagraph},
		},
		{
			name:  "code block",
			input: "```go\nfmt.Println()\n```",
			types: []blockKind{kindCodeBlock},
		},
		{
			name:  "unordered list",
			input: "- item one\n- item two",
			types: []blockKind{kindListItem, kindListItem},
		},
		{
			name:  "ordered list",
			input: "1. first\n2. second",
			types: []blockKind{kindListItem, kindListItem},
		},
		{
			name:  "signature",
			input: "Body text.\n\n-- \nGeoff Wright\ngeoff@907.life",
			types: []blockKind{kindParagraph, kindSignature},
		},
		{
			name:  "quote attribution",
			input: "On Mon, Jan 5, Alice wrote:\n\n> quoted reply",
			types: []blockKind{kindQuoteAttribution, kindBlockquote},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseBlocks(tt.input)
			if len(blocks) != len(tt.types) {
				t.Fatalf("block count: got %d, want %d\nblocks: %v", len(blocks), len(tt.types), blocks)
			}
			for i, b := range blocks {
				if b.blockType() != tt.types[i] {
					t.Errorf("block[%d]: got kind %d, want %d", i, b.blockType(), tt.types[i])
				}
			}
		})
	}
}

func TestParseBlocksHeadingLevel(t *testing.T) {
	blocks := ParseBlocks("## Level 2")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	h, ok := blocks[0].(Heading)
	if !ok {
		t.Fatalf("expected Heading, got %T", blocks[0])
	}
	if h.Level != 2 {
		t.Errorf("level: got %d, want 2", h.Level)
	}
}

func TestParseBlocksCodeBlockLang(t *testing.T) {
	blocks := ParseBlocks("```python\nprint('hi')\n```")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	cb, ok := blocks[0].(CodeBlock)
	if !ok {
		t.Fatalf("expected CodeBlock, got %T", blocks[0])
	}
	if cb.Lang != "python" {
		t.Errorf("lang: got %q, want %q", cb.Lang, "python")
	}
	if cb.Text != "print('hi')" {
		t.Errorf("text: got %q, want %q", cb.Text, "print('hi')")
	}
}

func TestParseBlocksSignatureContent(t *testing.T) {
	blocks := ParseBlocks("Body.\n\n-- \nLine 1\nLine 2")
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	sig, ok := blocks[1].(Signature)
	if !ok {
		t.Fatalf("expected Signature, got %T", blocks[1])
	}
	if len(sig.Lines) != 2 {
		t.Errorf("signature lines: got %d, want 2", len(sig.Lines))
	}
}

func TestParseBlocksBlockquoteLevel(t *testing.T) {
	blocks := ParseBlocks("> > nested quote")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	bq, ok := blocks[0].(Blockquote)
	if !ok {
		t.Fatalf("expected Blockquote, got %T", blocks[0])
	}
	if bq.Level != 1 {
		t.Errorf("outer level: got %d, want 1", bq.Level)
	}
	if len(bq.Blocks) != 1 {
		t.Fatalf("inner blocks: got %d, want 1", len(bq.Blocks))
	}
	inner, ok := bq.Blocks[0].(Blockquote)
	if !ok {
		t.Fatalf("expected inner Blockquote, got %T", bq.Blocks[0])
	}
	if inner.Level != 2 {
		t.Errorf("inner level: got %d, want 2", inner.Level)
	}
}

func TestWrapImpliedQuotes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		types []blockKind
	}{
		{
			name:  "attribution followed by paragraph wraps",
			input: "Reply text\n\nOn Mon, Jan 5, Alice wrote:\nUnquoted content",
			types: []blockKind{kindParagraph, kindQuoteAttribution, kindBlockquote},
		},
		{
			name:  "attribution followed by blockquote unchanged",
			input: "Reply text\n\nOn Mon, Jan 5, Alice wrote:\n> Quoted content",
			types: []blockKind{kindParagraph, kindQuoteAttribution, kindBlockquote},
		},
		{
			name:  "no attribution unchanged",
			input: "Just a paragraph",
			types: []blockKind{kindParagraph},
		},
		{
			name:  "attribution at end unchanged",
			input: "Text\n\nOn Mon, Jan 5, Alice wrote:",
			types: []blockKind{kindParagraph, kindQuoteAttribution},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseBlocks(tt.input)
			if len(blocks) != len(tt.types) {
				t.Fatalf("block count: got %d, want %d\nblocks: %v", len(blocks), len(tt.types), blocks)
			}
			for i, b := range blocks {
				if b.blockType() != tt.types[i] {
					t.Errorf("block[%d]: got kind %d, want %d", i, b.blockType(), tt.types[i])
				}
			}
		})
	}
}

func TestWrapImpliedQuotesIncrementsLevels(t *testing.T) {
	// Simulate Yahoo-style HTML: attribution + unquoted text + already-quoted inner thread
	input := "Reply\n\nOn Mon, Alice wrote:\nFirst level text\n\n> On Sun, Bob wrote:\n> Inner text"
	blocks := ParseBlocks(input)
	if len(blocks) != 3 {
		t.Fatalf("block count: got %d, want 3", len(blocks))
	}

	// Block 2 should be a Blockquote wrapping everything after the attribution
	bq, ok := blocks[2].(Blockquote)
	if !ok {
		t.Fatalf("block[2]: got %T, want Blockquote", blocks[2])
	}
	if bq.Level != 1 {
		t.Errorf("wrapper level: got %d, want 1", bq.Level)
	}

	// Inside the wrapper: Paragraph + Blockquote{Level: 2}
	if len(bq.Blocks) < 2 {
		t.Fatalf("inner block count: got %d, want >=2", len(bq.Blocks))
	}
	if _, ok := bq.Blocks[0].(Paragraph); !ok {
		t.Errorf("inner[0]: got %T, want Paragraph", bq.Blocks[0])
	}

	innerBQ, ok := bq.Blocks[1].(Blockquote)
	if !ok {
		t.Fatalf("inner[1]: got %T, want Blockquote", bq.Blocks[1])
	}
	if innerBQ.Level != 2 {
		t.Errorf("inner blockquote level: got %d, want 2 (incremented from 1)", innerBQ.Level)
	}
}

func TestParseSpansBareHTTPSURL(t *testing.T) {
	got := parseSpans("Visit https://example.com today")
	want := []Span{
		Text{Content: "Visit "},
		Link{Text: "https://example.com", URL: "https://example.com"},
		Text{Content: " today"},
	}
	spansEqual(t, got, want)
}

func TestParseSpansBareHTTPURL(t *testing.T) {
	got := parseSpans("Visit http://example.com today")
	want := []Span{
		Text{Content: "Visit "},
		Link{Text: "http://example.com", URL: "http://example.com"},
		Text{Content: " today"},
	}
	spansEqual(t, got, want)
}

func TestParseSpansBareMailtoURL(t *testing.T) {
	got := parseSpans("Email mailto:foo@example.com now")
	want := []Span{
		Text{Content: "Email "},
		Link{Text: "mailto:foo@example.com", URL: "mailto:foo@example.com"},
		Text{Content: " now"},
	}
	spansEqual(t, got, want)
}

func TestParseSpansBareURLAtEndWithPunctuation(t *testing.T) {
	got := parseSpans("see https://example.com.")
	if len(got) < 2 {
		t.Fatalf("expected at least 2 spans, got %d: %v", len(got), got)
	}
	link, ok := got[0].(Link)
	if !ok {
		// Allow "see " prefix to be span[0]; link may be span[1].
		if len(got) < 2 {
			t.Fatalf("no Link span found: %v", got)
		}
		link, ok = got[1].(Link)
		if !ok {
			t.Fatalf("span[1] not a Link: %v", got[1])
		}
	}
	if link.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q (trailing . must be trimmed)", link.URL, "https://example.com")
	}
}

func TestParseSpansBareURLInsideParens(t *testing.T) {
	got := parseSpans("(see https://example.com)")
	var found bool
	for _, s := range got {
		if link, ok := s.(Link); ok {
			found = true
			if link.URL != "https://example.com" {
				t.Errorf("URL = %q, want %q (trailing ) must be trimmed)", link.URL, "https://example.com")
			}
		}
	}
	if !found {
		t.Fatalf("no Link span found in %v", got)
	}
}

func TestParseSpansMarkdownLinkUntouched(t *testing.T) {
	got := parseSpans("[text](https://example.com)")
	var links int
	for _, s := range got {
		if _, ok := s.(Link); ok {
			links++
		}
	}
	if links != 1 {
		t.Errorf("expected exactly 1 Link span (no double-processing), got %d in %v", links, got)
	}
}

func TestParseSpansMultipleBareURLs(t *testing.T) {
	got := parseSpans("a https://one.com b https://two.com c")
	var links []Link
	for _, s := range got {
		if l, ok := s.(Link); ok {
			links = append(links, l)
		}
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 Link spans, got %d in %v", len(links), got)
	}
	if links[0].URL != "https://one.com" {
		t.Errorf("link[0].URL = %q, want https://one.com", links[0].URL)
	}
	if links[1].URL != "https://two.com" {
		t.Errorf("link[1].URL = %q, want https://two.com", links[1].URL)
	}
}

func TestParseSpansNoURL(t *testing.T) {
	input := "plain text with no URLs here"
	got := parseSpans(input)
	want := []Span{Text{Content: input}}
	spansEqual(t, got, want)
}

func TestParseBlocksCRLFParagraph(t *testing.T) {
	// Paranoia/lock-in: even if CR slips past the filter layer, no \r should
	// survive in rendered span text.
	input := "Para line 1\r\nPara line 2\r\n"
	blocks := ParseBlocks(input)
	for _, b := range blocks {
		p, ok := b.(Paragraph)
		if !ok {
			continue
		}
		for _, s := range p.Spans {
			text, ok := s.(Text)
			if !ok {
				continue
			}
			if strings.Contains(text.Content, "\r") {
				t.Errorf("span text contains \\r: %q", text.Content)
			}
		}
	}
}

func TestParseSpansBareURLInsideBrackets(t *testing.T) {
	got := parseSpans("see [https://example.com] for more")
	// Expect: Text "see [" + Link{URL: "https://example.com"} + Text "] for more"
	// The "[" before the URL is not a markdown link (no "]("), so it passes through
	// as a Text span prefix. The "]" after the URL is trimmed as trailing punct.
	var link Link
	var foundLink bool
	for _, s := range got {
		if l, ok := s.(Link); ok {
			if foundLink {
				t.Fatal("more than one Link span found")
			}
			link = l
			foundLink = true
		}
	}
	if !foundLink {
		t.Fatalf("no Link span found in %v", got)
	}
	if link.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q (trailing ] must be trimmed)", link.URL, "https://example.com")
	}
	// The "] for more" suffix must appear as a Text span after the link.
	foundSuffix := false
	for _, s := range got {
		if t2, ok := s.(Text); ok && strings.Contains(t2.Content, "] for more") {
			foundSuffix = true
		}
	}
	if !foundSuffix {
		t.Errorf("suffix '] for more' not found in spans: %v", got)
	}
}

func TestParseSpansBareURLBacktickTrim(t *testing.T) {
	input := "see `https://example.com` here"
	got := parseSpans(input)
	// The backtick before the URL triggers a `code` span parse — the URL lands
	// inside a Code span, not a Link. Verify the URL is not mangled by checking
	// the Code span content directly (the bare-URL path is not reached here).
	// What we care about: no Link span whose URL has a trailing backtick.
	for _, s := range got {
		if l, ok := s.(Link); ok {
			if strings.HasSuffix(l.URL, "`") {
				t.Errorf("Link URL has trailing backtick: %q", l.URL)
			}
		}
	}
}

func TestParseSpansBareSchemeOnlyRejected(t *testing.T) {
	got := parseSpans("see https://... for details")
	for _, s := range got {
		if l, ok := s.(Link); ok {
			t.Errorf("unexpected Link span for scheme-only token: %v", l)
		}
	}
}

func TestParseSpansBareURLAtStart(t *testing.T) {
	got := parseSpans("https://example.com is the link")
	if len(got) == 0 {
		t.Fatal("expected spans, got none")
	}
	if _, ok := got[0].(Link); !ok {
		t.Errorf("span[0] = %T, want Link", got[0])
	}
	// No leading empty Text span.
	if t2, ok := got[0].(Text); ok && t2.Content == "" {
		t.Errorf("leading empty Text span found")
	}
}

func TestParseSpansBareURLOnlyToken(t *testing.T) {
	got := parseSpans("https://example.com")
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 span, got %d: %v", len(got), got)
	}
	if _, ok := got[0].(Link); !ok {
		t.Errorf("span[0] = %T, want Link", got[0])
	}
}

func TestParseSpansBareURLAtEnd(t *testing.T) {
	got := parseSpans("check https://example.com")
	if len(got) == 0 {
		t.Fatal("expected spans, got none")
	}
	last := got[len(got)-1]
	if _, ok := last.(Link); !ok {
		t.Errorf("last span = %T, want Link", last)
	}
	// No trailing empty Text span.
	if t2, ok := last.(Text); ok && t2.Content == "" {
		t.Errorf("trailing empty Text span found")
	}
}

func TestParseBlocksCorpus(t *testing.T) {
	fixtures, err := filepath.Glob("../../e2e/testdata/*.html")
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Skip("no e2e fixtures found")
	}
	for _, fix := range fixtures {
		t.Run(filepath.Base(fix), func(t *testing.T) {
			raw, err := os.ReadFile(fix)
			if err != nil {
				t.Fatal(err)
			}
			// Verify ParseBlocks doesn't panic on HTML input.
			// Real integration tested in e2e after CleanHTML is wired up.
			blocks := ParseBlocks(string(raw))
			if len(blocks) == 0 {
				t.Error("expected at least one block from HTML input")
			}
		})
	}
}
