package catkin

import (
	"strings"
	"unicode/utf8"
)

// Reflow rewraps paragraphs and quoted blocks to width while preserving
// headings, code fences, indented code, tables, and blank lines. Long
// single tokens (URLs longer than width) are emitted on their own line.
//
// The returned cursor offset points at the same logical character as
// oldCursor did in src. If oldCursor is past the end of src, the new
// cursor snaps to the new end.
func Reflow(src string, width int, oldCursor int) (string, int) {
	if width <= 0 {
		return src, oldCursor
	}
	lines := strings.Split(src, "\n")
	ctx := Classify(lines)

	var out []string
	srcOffset := 0
	newOffset := 0
	newCursor := -1

	i := 0
	for i < len(lines) {
		end := groupEnd(lines, ctx, i)
		group := lines[i:end]
		groupCtx := ctx[i:end]

		groupStart := srcOffset
		groupChars := 0
		for j, l := range group {
			groupChars += utf8.RuneCountInString(l)
			if j < len(group)-1 || end < len(lines) {
				groupChars++ // newline
			}
		}

		var emitted []string
		if isPreservedBlock(groupCtx[0]) {
			emitted = group
		} else {
			emitted = reflowGroup(group, groupCtx, width)
		}

		if newCursor < 0 && oldCursor >= groupStart && oldCursor <= groupStart+groupChars {
			rel := oldCursor - groupStart
			rel = remapCursor(group, emitted, rel)
			newCursor = newOffset + rel
		}

		for _, l := range emitted {
			out = append(out, l)
			newOffset += utf8.RuneCountInString(l) + 1
		}
		if end >= len(lines) {
			newOffset--
		}

		srcOffset = groupStart + groupChars
		i = end
	}

	if newCursor < 0 {
		newCursor = newOffset
	}
	return strings.Join(out, "\n"), newCursor
}

// groupEnd returns the exclusive end index of the group starting at i.
// Lines belong to the same group when they share Kind, QuoteDepth, and
// ListMarker. A blank line is always its own group.
func groupEnd(lines []string, ctx []LineContext, i int) int {
	first := ctx[i]
	if first.Kind == BlockBlank {
		return i + 1
	}
	for j := i + 1; j < len(lines); j++ {
		if ctx[j].Kind == BlockBlank {
			return j
		}
		if ctx[j].Kind != first.Kind ||
			ctx[j].QuoteDepth != first.QuoteDepth ||
			ctx[j].ListMarker != first.ListMarker {
			return j
		}
	}
	return len(lines)
}

func isPreservedBlock(c LineContext) bool {
	switch c.Kind {
	case BlockHeading, BlockTable, BlockCodeFence, BlockCodeIndent, BlockBlank:
		return true
	}
	return false
}

func reflowGroup(lines []string, ctx []LineContext, width int) []string {
	prefix := buildPrefix(ctx[0])
	budget := width - utf8.RuneCountInString(prefix)
	if budget < 1 {
		budget = 1
	}

	var words []string
	for _, c := range ctx {
		words = append(words, strings.Fields(c.PostPrefix)...)
	}

	if len(words) == 0 {
		return []string{prefix}
	}

	var out []string
	current := prefix
	currentLen := utf8.RuneCountInString(prefix)
	prefixLen := currentLen
	for _, w := range words {
		wLen := utf8.RuneCountInString(w)
		if currentLen == prefixLen {
			current += w
			currentLen += wLen
			continue
		}
		if currentLen+1+wLen <= prefixLen+budget {
			current += " " + w
			currentLen += 1 + wLen
		} else {
			out = append(out, current)
			current = prefix + w
			currentLen = prefixLen + wLen
		}
	}
	out = append(out, current)
	return out
}

func buildPrefix(c LineContext) string {
	var sb strings.Builder
	for d := 0; d < c.QuoteDepth; d++ {
		sb.WriteString("> ")
	}
	if c.ListMarker != "" {
		sb.WriteString(c.ListMarker)
		sb.WriteString(" ")
	}
	return sb.String()
}

// remapCursor approximates the post-reflow cursor by aligning on
// non-whitespace rune count; whitespace boundaries shift but content
// chars do not.
func remapCursor(orig, emitted []string, rel int) int {
	if len(orig) == 0 || len(emitted) == 0 {
		return rel
	}
	origJoined := strings.Join(orig, "\n")
	emitJoined := strings.Join(emitted, "\n")

	nonWS := 0
	runeIdx := 0
	for _, r := range origJoined {
		if runeIdx >= rel {
			break
		}
		if !isReflowWS(r) {
			nonWS++
		}
		runeIdx++
	}

	count := 0
	runeIdx = 0
	for _, r := range emitJoined {
		if count >= nonWS && !isReflowWS(r) {
			return runeIdx
		}
		if !isReflowWS(r) {
			count++
		}
		runeIdx++
	}
	return utf8.RuneCountInString(emitJoined)
}

func isReflowWS(r rune) bool { return r == ' ' || r == '\n' || r == '\t' }
