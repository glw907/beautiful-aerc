// SPDX-License-Identifier: MIT

package content

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
	"github.com/muesli/termenv"
)

func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

func TestRenderBodyParagraph(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{Text{Content: "Hello world"}}},
	}
	result := RenderBody(blocks, theme.Nord, 80)
	if !strings.Contains(result, "Hello world") {
		t.Errorf("expected 'Hello world' in output, got %q", result)
	}
}

func TestRenderBodyHeading(t *testing.T) {
	blocks := []Block{
		Heading{Spans: []Span{Text{Content: "Title"}}, Level: 1},
	}
	result := RenderBody(blocks, theme.Nord, 80)
	if !strings.Contains(result, "Title") {
		t.Errorf("expected 'Title' in output, got %q", result)
	}
	if result == "Title\n" {
		t.Error("heading appears unstyled")
	}
}

func TestRenderBodySignature(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{Text{Content: "Body text"}}},
		Signature{Lines: [][]Span{
			{Text{Content: "-- "}},
			{Text{Content: "Geoff"}},
		}},
	}
	result := RenderBody(blocks, theme.Nord, 80)
	if !strings.Contains(result, "Body text") {
		t.Error("missing body text")
	}
	if !strings.Contains(result, "Geoff") {
		t.Error("missing signature")
	}
}

func TestRenderBodyBoldSpan(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{
			Text{Content: "hello "},
			Bold{Content: "bold"},
			Text{Content: " world"},
		}},
	}
	result := RenderBody(blocks, theme.Nord, 80)
	if !strings.Contains(result, "bold") {
		t.Error("missing bold text")
	}
}

func TestRenderBodyLink(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{
			Link{Text: "click", URL: "https://example.com"},
		}},
	}
	result := RenderBody(blocks, theme.Nord, 80)
	visible := stripANSITest(result)
	if !strings.Contains(visible, "click") {
		t.Errorf("missing link text in visible output: %q", visible)
	}
}

func TestRenderBodyRule(t *testing.T) {
	blocks := []Block{
		Paragraph{Spans: []Span{Text{Content: "above"}}},
		Rule{},
		Paragraph{Spans: []Span{Text{Content: "below"}}},
	}
	result := RenderBody(blocks, theme.Nord, 80)
	if !strings.Contains(result, "above") || !strings.Contains(result, "below") {
		t.Error("missing content around rule")
	}
}

func TestRenderHeaders(t *testing.T) {
	h := ParsedHeaders{
		From:    []Address{{Name: "Alice", Email: "alice@example.com"}},
		To:      []Address{{Name: "Bob", Email: "bob@example.com"}},
		Date:    "Mon, 5 Jan 2026",
		Subject: "Hello World",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	visible := stripANSITest(result)
	if !strings.Contains(visible, "from ") {
		t.Error("missing From header")
	}
	if !strings.Contains(visible, "Alice") {
		t.Error("missing From name")
	}
	if !strings.Contains(visible, "Hello World") {
		t.Error("missing Subject title text")
	}
	if !strings.Contains(visible, "Hello World") {
		t.Error("missing Subject value")
	}
}

func TestRenderHeadersOrder(t *testing.T) {
	h := ParsedHeaders{
		From:    []Address{{Name: "Alice", Email: "alice@example.com"}},
		To:      []Address{{Name: "Bob", Email: "bob@example.com"}},
		Date:    "Mon, 5 Jan 2026",
		Subject: "TheSubjectTitle",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	visible := stripANSITest(result)
	subjectIdx := strings.Index(visible, "TheSubjectTitle")
	fromIdx := strings.Index(visible, "from ")
	toIdx := strings.Index(visible, "to ")
	dateIdx := strings.Index(visible, "date ")

	if subjectIdx > fromIdx {
		t.Error("Subject title should appear before From")
	}
	if fromIdx > toIdx {
		t.Error("From should appear before To")
	}
	if toIdx > dateIdx {
		t.Error("To should appear before Date")
	}
}

func TestRenderHeadersSkipsEmpty(t *testing.T) {
	h := ParsedHeaders{
		From:    []Address{{Name: "Alice", Email: "alice@example.com"}},
		Subject: "Test",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	visible := stripANSITest(result)
	if strings.Contains(visible, "to ") {
		t.Error("should not render empty To header")
	}
	if strings.Contains(visible, "cc ") {
		t.Error("should not render empty Cc header")
	}
}

func TestRenderHeadersSeparator(t *testing.T) {
	h := ParsedHeaders{
		From:    []Address{{Email: "alice@example.com"}},
		Subject: "Test",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	if !strings.Contains(result, "─") {
		t.Error("missing separator line")
	}
}

func TestRenderHeadersAddressWrap(t *testing.T) {
	var addrs []Address
	for i := 0; i < 10; i++ {
		addrs = append(addrs, Address{
			Name:  "Recipient Name",
			Email: "recipient@example.com",
		})
	}
	h := ParsedHeaders{
		From:    []Address{{Email: "sender@example.com"}},
		To:      addrs,
		Subject: "Test",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	visible := stripANSITest(result)
	lines := strings.Split(visible, "\n")
	toLines := 0
	for _, line := range lines {
		if strings.Contains(line, "Recipient") {
			toLines++
		}
	}
	if toLines < 2 {
		t.Errorf("expected To: to wrap across multiple lines, got %d", toLines)
	}
}

func TestRenderHeadersMetadataIndented(t *testing.T) {
	h := ParsedHeaders{
		From:    []Address{{Name: "Alice", Email: "alice@example.com"}},
		To:      []Address{{Email: "bob@example.com"}},
		Date:    "Mon, 5 Jan 2026",
		Subject: "Hello",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	visible := stripANSITest(result)
	for _, label := range []string{"from ", "to ", "date "} {
		idx := strings.Index(visible, label)
		if idx < 0 {
			t.Fatalf("missing label %q in render", label)
		}
		// The label must be preceded by a newline (or start-of-string)
		// followed by exactly the metadataIndent (2 spaces).
		var prefixStart int
		if nl := strings.LastIndex(visible[:idx], "\n"); nl >= 0 {
			prefixStart = nl + 1
		}
		prefix := visible[prefixStart:idx]
		if prefix != "  " {
			t.Errorf("label %q prefix = %q, want two spaces", label, prefix)
		}
	}
}

func TestRenderBodyNestedBlockquotePrefix(t *testing.T) {
	// Nested blockquote should render with correct prefix depth,
	// not double-count from both Level field and structural nesting.
	blocks := []Block{
		Blockquote{Level: 1, Blocks: []Block{
			Paragraph{Spans: []Span{Text{Content: "level one"}}},
			Blockquote{Level: 2, Blocks: []Block{
				Paragraph{Spans: []Span{Text{Content: "level two"}}},
			}},
		}},
	}
	result := RenderBody(blocks, theme.Nord, 80)
	visible := stripANSITest(result)

	// Level 1 content should have "> " prefix (one level)
	if !strings.Contains(visible, "> level one") {
		t.Errorf("expected '> level one', got:\n%s", visible)
	}
	// Level 2 content should have "> > " prefix (two levels), not "> > > "
	if !strings.Contains(visible, "> > level two") {
		t.Errorf("expected '> > level two', got:\n%s", visible)
	}
	if strings.Contains(visible, "> > > level two") {
		t.Error("triple-nested prefix found — Level double-counting bug")
	}
}

func TestRenderBodyImpliedQuoteWrapping(t *testing.T) {
	// End-to-end: parse markdown with missing first-level blockquote,
	// then render and verify the output has correct quoting.
	input := "Reply text\n\nOn Mon, Alice wrote:\nFirst level content\n\n> Inner quoted"
	blocks := ParseBlocks(input)
	result := RenderBody(blocks, theme.Nord, 80)
	visible := stripANSITest(result)

	if !strings.Contains(visible, "Reply text") {
		t.Error("missing reply text")
	}
	if !strings.Contains(visible, "> First level content") {
		t.Errorf("first-level content should be quoted:\n%s", visible)
	}
	if !strings.Contains(visible, "> > Inner quoted") {
		t.Errorf("inner content should be double-quoted:\n%s", visible)
	}
}

func TestRenderBodyWidthCap(t *testing.T) {
	// Caller passes a generous width; renderer caps at maxBodyWidth.
	long := strings.Repeat("alpha bravo charlie ", 20)
	blocks := []Block{Paragraph{Spans: []Span{Text{Content: long}}}}
	result := RenderBody(blocks, theme.Nord, 200)
	for _, line := range strings.Split(stripANSITest(result), "\n") {
		if w := lipgloss.Width(line); w > maxBodyWidth {
			t.Errorf("line exceeds cap: %q (w=%d, cap=%d)", line, w, maxBodyWidth)
		}
	}
}

func TestRenderBodyWrapStressParagraph(t *testing.T) {
	// Paragraph mixing styled spans straddling the wrap column.
	blocks := []Block{Paragraph{Spans: []Span{
		Text{Content: "This is a "},
		Bold{Content: "very important"},
		Text{Content: " message about the "},
		Italic{Content: "quarterly earnings"},
		Text{Content: " review and the upcoming "},
		Code{Content: "config.yaml"},
		Text{Content: " deployment changes that need attention from everyone on the team before the deadline next week."},
	}}}
	result := RenderBody(blocks, theme.Nord, 100)
	for _, line := range strings.Split(stripANSITest(result), "\n") {
		if w := lipgloss.Width(line); w > maxBodyWidth {
			t.Errorf("styled paragraph overflow: %q (w=%d)", line, w)
		}
	}
}

func TestRenderBodyLongURL(t *testing.T) {
	// A URL longer than the cap is hard-wrapped so no line exceeds
	// the width contract. The renderer's job is to honor its width
	// argument; an unbroken URL would overflow into adjacent panes
	// in the bubbletea layout, so hardwrap wins over readability.
	url := "https://news.example.com/items?category=engineering&id=" + strings.Repeat("0123456789", 6)
	blocks := []Block{Paragraph{Spans: []Span{
		Text{Content: "see "},
		Link{Text: url, URL: url},
	}}}
	result := RenderBody(blocks, theme.Nord, maxBodyWidth)
	for _, line := range strings.Split(stripANSITest(result), "\n") {
		if w := lipgloss.Width(line); w > maxBodyWidth {
			t.Errorf("long-URL line exceeds cap: %q (w=%d, cap=%d)", line, w, maxBodyWidth)
		}
	}
}

func TestRenderBodyNestedQuoteWrap(t *testing.T) {
	// Two-level quote with long inner content. Outer prefix "> " (2),
	// inner prefix another "> " (2), so inner content wraps at 68.
	long := strings.Repeat("alpha bravo charlie delta echo foxtrot ", 5)
	blocks := []Block{Blockquote{Level: 1, Blocks: []Block{
		Blockquote{Level: 2, Blocks: []Block{
			Paragraph{Spans: []Span{Text{Content: long}}},
		}},
	}}}
	result := RenderBody(blocks, theme.Nord, maxBodyWidth)
	for _, line := range strings.Split(stripANSITest(result), "\n") {
		if w := lipgloss.Width(line); w > maxBodyWidth {
			t.Errorf("nested-quote overflow: %q (w=%d)", line, w)
		}
	}
}

func TestRenderBodyListHangingIndent(t *testing.T) {
	long := strings.Repeat("alpha bravo charlie delta ", 6)
	blocks := []Block{ListItem{Spans: []Span{Text{Content: long}}}}
	result := RenderBody(blocks, theme.Nord, maxBodyWidth)
	visible := stripANSITest(result)
	lines := strings.Split(strings.TrimRight(visible, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrap to multiple lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "- ") {
		t.Errorf("first line missing list prefix: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  ") || strings.HasPrefix(lines[1], "- ") {
		t.Errorf("continuation line missing hanging indent: %q", lines[1])
	}
}

func TestRenderBodyFootnoteEdge(t *testing.T) {
	// Construct text where the link's last word would land near the cap.
	// The nbsp glue should keep "word[^1]" on one line.
	pad := strings.Repeat("a ", 25) // 50 chars of padding
	blocks := []Block{Paragraph{Spans: []Span{
		Text{Content: pad},
		Link{Text: "documentation", URL: "https://example.com/x"},
	}}}
	out, _ := RenderBodyWithFootnotes(blocks, theme.Nord, maxBodyWidth)
	visible := stripANSITest(out)
	for _, line := range strings.Split(visible, "\n") {
		if strings.Contains(line, "[^1]") && !strings.Contains(line, "[^1]: ") {
			if !strings.Contains(line, "documentation"+nbsp+"[^1]") &&
				!strings.Contains(line, "documentation [^1]") {
				t.Errorf("footnote marker orphaned from last word: %q", line)
			}
		}
	}
}

// stripANSITest removes ANSI escape sequences for visible length checks.
func stripANSITest(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' {
			i++
			if i < len(s) && s[i] == '[' {
				i++
				for i < len(s) && s[i] < 0x40 {
					i++
				}
				if i < len(s) {
					i++
				}
			}
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
