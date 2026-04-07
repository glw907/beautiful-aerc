package tidy

import (
	"strings"
)

// QuotedBlock represents a contiguous run of quoted lines from the input.
type QuotedBlock struct {
	StartLine int      // 0-based index of the first line in the original input
	Lines     []string // the quoted lines, without trailing newline
}

// isQuoted reports whether line starts with optional whitespace then '>'.
func isQuoted(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, ">")
}

// SplitQuoted separates input into author text and quoted blocks.
//
// A line is quoted if it starts with optional whitespace then '>'.
// Consecutive quoted lines form a block. In the returned author string,
// quoted lines are replaced with blank lines to preserve paragraph
// structure. If all non-whitespace lines are quoted, author is "".
func SplitQuoted(input string) (author string, quoted []QuotedBlock) {
	if input == "" {
		return "", nil
	}

	// Split preserving trailing newline awareness: remove a trailing newline
	// before splitting so we don't get a phantom empty final element, then
	// track whether the input ended with a newline.
	trailingNewline := strings.HasSuffix(input, "\n")
	body := input
	if trailingNewline {
		body = input[:len(input)-1]
	}

	lines := strings.Split(body, "\n")

	// Build the author lines (quoted lines become blank) and collect quoted blocks.
	authorLines := make([]string, len(lines))
	copy(authorLines, lines)

	var blocks []QuotedBlock
	i := 0
	for i < len(lines) {
		if !isQuoted(lines[i]) {
			i++
			continue
		}
		// Start of a quoted block.
		start := i
		var blockLines []string
		for i < len(lines) && isQuoted(lines[i]) {
			blockLines = append(blockLines, lines[i])
			authorLines[i] = "" // replace with blank in author view
			i++
		}
		blocks = append(blocks, QuotedBlock{StartLine: start, Lines: blockLines})
	}

	// Determine if author is effectively empty (only whitespace lines remain).
	allBlank := true
	for _, l := range authorLines {
		if strings.TrimSpace(l) != "" {
			allBlank = false
			break
		}
	}
	// If any quoted blocks exist and all non-quoted content is blank, return
	// empty author. But we still return the blank-line structure when there is
	// actual whitespace-only non-quoted content mixed with quotes.
	// The spec says: "If all lines are quoted (or only whitespace remains),
	// author is empty string." — only emit empty when we have quoted blocks
	// and nothing non-whitespace remains in author.
	if len(blocks) > 0 && allBlank {
		// Check that every original line was either quoted or whitespace-only.
		onlyQuotedOrBlank := true
		for _, l := range lines {
			if !isQuoted(l) && strings.TrimSpace(l) != "" {
				onlyQuotedOrBlank = false
				break
			}
		}
		if onlyQuotedOrBlank {
			return "", blocks
		}
	}

	// Reconstruct author string.
	authorStr := strings.Join(authorLines, "\n")
	if trailingNewline {
		authorStr += "\n"
	}

	return authorStr, blocks
}

// Reassemble combines corrected author text with the original quoted blocks.
//
// It walks the original input line by line: quoted lines are preserved
// verbatim, non-quoted lines are replaced with lines from corrected in order.
// Any extra corrected lines (if the model split a line) are appended at the end.
func Reassemble(corrected, _, originalInput string) string {
	if originalInput == "" {
		return corrected
	}

	trailingNewline := strings.HasSuffix(originalInput, "\n")
	body := originalInput
	if trailingNewline {
		body = originalInput[:len(originalInput)-1]
	}

	origLines := strings.Split(body, "\n")

	// Split corrected into lines for sequential consumption.
	var corrLines []string
	if corrected != "" {
		corrBody := corrected
		corrTrailing := strings.HasSuffix(corrected, "\n")
		if corrTrailing {
			corrBody = corrected[:len(corrected)-1]
		}
		corrLines = strings.Split(corrBody, "\n")
	}

	corrIdx := 0
	var out []string

	for _, ol := range origLines {
		if isQuoted(ol) {
			out = append(out, ol)
		} else {
			if corrIdx < len(corrLines) {
				out = append(out, corrLines[corrIdx])
				corrIdx++
			} else {
				// Original had more non-quoted lines than corrected supplied;
				// preserve original line to avoid data loss.
				out = append(out, ol)
			}
		}
	}

	// Append any extra corrected lines the model produced.
	for corrIdx < len(corrLines) {
		out = append(out, corrLines[corrIdx])
		corrIdx++
	}

	result := strings.Join(out, "\n")
	if trailingNewline || len(out) > 0 {
		result += "\n"
	}
	return result
}
