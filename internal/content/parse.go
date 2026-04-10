package content

import (
	"regexp"
	"strings"
)

var (
	reHeading     = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	reCodeFence   = regexp.MustCompile("^```(\\w*)$")
	reRule        = regexp.MustCompile(`^(-{3,}|_{3,}|\*{3,})$`)
	reUnordered   = regexp.MustCompile(`^[-*+]\s+(.+)$`)
	reOrdered     = regexp.MustCompile(`^(\d+)\.\s+(.+)$`)
	reQuotePrefix = regexp.MustCompile(`^(>+)\s?(.*)$`)
	reAttribution = regexp.MustCompile(`(?i)^on\s.+wrote:\s*$`)
	reSignature   = regexp.MustCompile(`^-- $`)
)

// ParseBlocks parses normalized markdown into email-aware block types.
func ParseBlocks(markdown string) []Block {
	blocks := parseBlocksAtLevel(markdown, 1)
	return wrapImpliedQuotes(blocks)
}

// wrapImpliedQuotes fixes HTML emails where the first reply level lacks
// a <blockquote> tag. When a QuoteAttribution at the top level is
// followed by non-Blockquote content, all subsequent blocks are wrapped
// in a Blockquote{Level: 1}. Only triggers at the top level to avoid
// compounding nesting on recursive attributions.
func wrapImpliedQuotes(blocks []Block) []Block {
	// Find the first QuoteAttribution.
	attrIdx := -1
	for i, b := range blocks {
		if _, ok := b.(QuoteAttribution); ok {
			attrIdx = i
			break
		}
	}
	if attrIdx < 0 || attrIdx+1 >= len(blocks) {
		return blocks
	}

	// If the first block after the attribution is already a Blockquote,
	// the HTML had a proper <blockquote> tag — no wrapping needed.
	if _, ok := blocks[attrIdx+1].(Blockquote); ok {
		return blocks
	}

	// Wrap all blocks after the attribution in a level-1 Blockquote.
	// Existing Blockquotes within get their levels incremented.
	inner := make([]Block, len(blocks[attrIdx+1:]))
	for i, b := range blocks[attrIdx+1:] {
		inner[i] = incrementQuoteLevels(b)
	}

	result := make([]Block, attrIdx+1, attrIdx+2)
	copy(result, blocks[:attrIdx+1])
	result = append(result, Blockquote{Level: 1, Blocks: inner})
	return result
}

func incrementQuoteLevels(b Block) Block {
	bq, ok := b.(Blockquote)
	if !ok {
		return b
	}
	inner := make([]Block, len(bq.Blocks))
	for i, child := range bq.Blocks {
		inner[i] = incrementQuoteLevels(child)
	}
	return Blockquote{Level: bq.Level + 1, Blocks: inner}
}

func parseBlocksAtLevel(markdown string, quoteLevel int) []Block {
	lines := strings.Split(markdown, "\n")
	var blocks []Block
	i := 0

	for i < len(lines) {
		line := lines[i]

		// Skip blank lines between blocks
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}

		// Signature: "-- " marker, everything after is signature
		if reSignature.MatchString(line) {
			var sigLines [][]Span
			for i++; i < len(lines); i++ {
				sigLines = append(sigLines, parseSpans(lines[i]))
			}
			if len(sigLines) > 0 {
				blocks = append(blocks, Signature{Lines: sigLines})
			}
			break
		}

		// Code fence
		if m := reCodeFence.FindStringSubmatch(line); m != nil {
			lang := m[1]
			var codeLines []string
			i++
			for i < len(lines) && !strings.HasPrefix(lines[i], "```") {
				codeLines = append(codeLines, lines[i])
				i++
			}
			if i < len(lines) {
				i++ // skip closing fence
			}
			blocks = append(blocks, CodeBlock{
				Text: strings.Join(codeLines, "\n"),
				Lang: lang,
			})
			continue
		}

		// Heading
		if m := reHeading.FindStringSubmatch(line); m != nil {
			blocks = append(blocks, Heading{
				Level: len(m[1]),
				Spans: parseSpans(m[2]),
			})
			i++
			continue
		}

		// Horizontal rule
		if reRule.MatchString(strings.TrimSpace(line)) {
			blocks = append(blocks, Rule{})
			i++
			continue
		}

		// Quote attribution (must check before blockquote)
		if reAttribution.MatchString(strings.TrimSpace(line)) {
			blocks = append(blocks, QuoteAttribution{Spans: parseSpans(strings.TrimSpace(line))})
			i++
			continue
		}

		// Blockquote
		if reQuotePrefix.MatchString(line) {
			var quoteLines []string
			for i < len(lines) && reQuotePrefix.MatchString(lines[i]) {
				quoteLines = append(quoteLines, lines[i])
				i++
			}
			blocks = append(blocks, parseBlockquote(quoteLines, quoteLevel))
			continue
		}

		// List items (unordered)
		if m := reUnordered.FindStringSubmatch(line); m != nil {
			blocks = append(blocks, ListItem{
				Spans:   parseSpans(m[1]),
				Ordered: false,
			})
			i++
			continue
		}

		// List items (ordered)
		if m := reOrdered.FindStringSubmatch(line); m != nil {
			idx := 0
			for _, c := range m[1] {
				idx = idx*10 + int(c-'0')
			}
			blocks = append(blocks, ListItem{
				Spans:   parseSpans(m[2]),
				Ordered: true,
				Index:   idx,
			})
			i++
			continue
		}

		// Paragraph: collect consecutive non-blank, non-special lines
		var paraLines []string
		for i < len(lines) {
			l := lines[i]
			if strings.TrimSpace(l) == "" {
				break
			}
			if reHeading.MatchString(l) || reCodeFence.MatchString(l) ||
				reRule.MatchString(strings.TrimSpace(l)) || reQuotePrefix.MatchString(l) ||
				reUnordered.MatchString(l) || reOrdered.MatchString(l) ||
				reSignature.MatchString(l) {
				break
			}
			paraLines = append(paraLines, l)
			i++
		}
		if len(paraLines) > 0 {
			blocks = append(blocks, Paragraph{
				Spans: parseSpans(strings.Join(paraLines, " ")),
			})
		}
	}

	return blocks
}

// parseBlockquote parses collected quote-prefixed lines into a
// Blockquote with recursive nesting.
func parseBlockquote(lines []string, level int) Blockquote {
	// Strip one level of "> " prefix
	var stripped []string
	for _, line := range lines {
		m := reQuotePrefix.FindStringSubmatch(line)
		if m != nil {
			prefixLen := len(m[1])
			if prefixLen > 1 {
				stripped = append(stripped, strings.Repeat(">", prefixLen-1)+" "+m[2])
			} else {
				stripped = append(stripped, m[2])
			}
		} else {
			stripped = append(stripped, line)
		}
	}

	inner := strings.Join(stripped, "\n")
	return Blockquote{
		Blocks: parseBlocksAtLevel(inner, level+1),
		Level:  level,
	}
}

// parseSpans parses inline markdown formatting into a slice of Span values.
// Handles: **bold**, *italic*, `code`, [text](url).
func parseSpans(input string) []Span {
	if input == "" {
		return nil
	}
	var spans []Span
	remaining := input

	for len(remaining) > 0 {
		// Find the earliest inline marker
		boldIdx := strings.Index(remaining, "**")
		italicIdx := -1
		// Only match single * that isn't part of **
		for i := 0; i < len(remaining); i++ {
			if remaining[i] == '*' {
				if i+1 < len(remaining) && remaining[i+1] == '*' {
					i++ // skip **
					continue
				}
				italicIdx = i
				break
			}
		}
		codeIdx := strings.Index(remaining, "`")
		linkIdx := strings.Index(remaining, "[")

		// Find the earliest marker
		best := len(remaining)
		bestKind := -1
		for _, candidate := range []struct {
			idx  int
			kind int
		}{
			{boldIdx, 0},
			{italicIdx, 1},
			{codeIdx, 2},
			{linkIdx, 3},
		} {
			if candidate.idx >= 0 && candidate.idx < best {
				best = candidate.idx
				bestKind = candidate.kind
			}
		}

		if bestKind == -1 {
			spans = append(spans, Text{Content: remaining})
			break
		}

		// Add any text before the marker
		if best > 0 {
			spans = append(spans, Text{Content: remaining[:best]})
		}

		switch bestKind {
		case 0: // **bold**
			end := strings.Index(remaining[best+2:], "**")
			if end < 0 {
				spans = append(spans, Text{Content: remaining[best:]})
				remaining = ""
				continue
			}
			spans = append(spans, Bold{Content: remaining[best+2 : best+2+end]})
			remaining = remaining[best+2+end+2:]

		case 1: // *italic*
			end := strings.Index(remaining[best+1:], "*")
			if end < 0 {
				spans = append(spans, Text{Content: remaining[best:]})
				remaining = ""
				continue
			}
			spans = append(spans, Italic{Content: remaining[best+1 : best+1+end]})
			remaining = remaining[best+1+end+1:]

		case 2: // `code`
			end := strings.Index(remaining[best+1:], "`")
			if end < 0 {
				spans = append(spans, Text{Content: remaining[best:]})
				remaining = ""
				continue
			}
			spans = append(spans, Code{Content: remaining[best+1 : best+1+end]})
			remaining = remaining[best+1+end+1:]

		case 3: // [text](url)
			closeBracket := strings.Index(remaining[best:], "](")
			if closeBracket < 0 {
				spans = append(spans, Text{Content: remaining[best:]})
				remaining = ""
				continue
			}
			closeParen := strings.Index(remaining[best+closeBracket+2:], ")")
			if closeParen < 0 {
				spans = append(spans, Text{Content: remaining[best:]})
				remaining = ""
				continue
			}
			linkText := remaining[best+1 : best+closeBracket]
			linkURL := remaining[best+closeBracket+2 : best+closeBracket+2+closeParen]
			spans = append(spans, Link{Text: linkText, URL: linkURL})
			remaining = remaining[best+closeBracket+2+closeParen+1:]
		}
	}

	return spans
}
