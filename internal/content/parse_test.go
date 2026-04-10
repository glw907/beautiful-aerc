package content

import (
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
