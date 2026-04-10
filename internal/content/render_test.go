package content

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/theme"
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
	if !strings.Contains(visible, "From:") {
		t.Error("missing From header")
	}
	if !strings.Contains(visible, "Alice") {
		t.Error("missing From name")
	}
	if !strings.Contains(visible, "Subject:") {
		t.Error("missing Subject header")
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
		Subject: "Test",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	visible := stripANSITest(result)
	fromIdx := strings.Index(visible, "From:")
	toIdx := strings.Index(visible, "To:")
	dateIdx := strings.Index(visible, "Date:")
	subjectIdx := strings.Index(visible, "Subject:")

	if fromIdx > toIdx {
		t.Error("From should appear before To")
	}
	if toIdx > dateIdx {
		t.Error("To should appear before Date")
	}
	if dateIdx > subjectIdx {
		t.Error("Date should appear before Subject")
	}
}

func TestRenderHeadersSkipsEmpty(t *testing.T) {
	h := ParsedHeaders{
		From:    []Address{{Name: "Alice", Email: "alice@example.com"}},
		Subject: "Test",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	visible := stripANSITest(result)
	if strings.Contains(visible, "To:") {
		t.Error("should not render empty To header")
	}
	if strings.Contains(visible, "Cc:") {
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
