package content

import (
	"strings"
)

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
