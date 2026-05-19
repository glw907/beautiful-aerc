package catkin

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Render produces Catkin's view: styled text plus a cursor block,
// soft-wrapped at width and clipped to height. Styling is a pure
// render-time overlay; the zero Styles value yields plain output.
func Render(src string, width, height, top, cursor int, styles Styles, mode DisplayMode) string {
	return RenderAnnotated(src, width, height, top, cursor, styles, mode, nil)
}

// RenderAnnotated overlays ann's decorations before the cursor block.
// ann may be nil to match Render.
func RenderAnnotated(src string, width, height, top, cursor int, styles Styles, mode DisplayMode, ann *AnnotationSet) string {
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	cursorRow, cursorCol := offsetToRowCol(src, cursor)
	fenceLines := renderFences(lines, ctxs, styles, top, top+height)
	focusFirst, focusLast := -1, -1
	if mode.focus() {
		focusFirst, focusLast = activeParagraphRange(ctxs, cursorRow)
	}

	var rowOffsets []int
	if ann != nil {
		rowOffsets = ann.rowStarts
	}

	var visual []string
	for i := top; i < len(lines) && len(visual) < height; i++ {
		raw := lines[i]
		var matchCol = -1
		var matchCh rune
		if i == cursorRow {
			if mc, ok := bracketMatchAt(raw, cursorCol); ok && mc != cursorCol {
				matchRunes := []rune(raw)
				if mc < len(matchRunes) {
					matchCh = matchRunes[mc]
					matchCol = lipgloss.Width(string(matchRunes[:mc]))
				}
			}
			raw = insertCursorBlock(raw, cursorCol)
		}
		styled := styleLine(raw, ctxs[i], styles, fenceLines, i, i == cursorRow)
		if ann != nil {
			cursorByteInLine := -1
			if i == cursorRow {
				cursorByteInLine = runeToByteOffset(lines[i], cursorCol)
			}
			styled = applyAnnotationsToLine(styled, lines[i], rowOffsets[i], cursorByteInLine, ann.rangesOnRow(src, i))
		}
		if matchCol >= 0 {
			styled = overlayMatch(styled, matchCol, matchCh, styles.MatchHighlight)
		}
		if mode.focus() && (i < focusFirst || i > focusLast) {
			styled = styles.Dim.Render(ansi.Strip(styled))
		}
		for _, w := range visualWrap(styled, width, ctxs[i], styles) {
			if len(visual) >= height {
				break
			}
			visual = append(visual, w)
		}
	}
	for len(visual) < height {
		visual = append(visual, "")
	}
	return strings.Join(visual, "\n")
}

// applyAnnotationsToLine splices annotation styles into the styled line
// at column offsets derived from plain. cursorByteCol is the byte offset
// of the cursor on its row, or -1. Annotation ranges enclosing the
// cursor split around it so the cursor cell survives the splice.
func applyAnnotationsToLine(styled, plain string, lineOffset, cursorByteCol int, anns []Annotation) string {
	if len(anns) == 0 {
		return styled
	}
	out := styled
	for _, a := range anns {
		startInLine := a.Range.Start - lineOffset
		endInLine := a.Range.End - lineOffset
		if startInLine < 0 {
			startInLine = 0
		}
		if endInLine > len(plain) {
			endInLine = len(plain)
		}
		if startInLine >= endInLine {
			continue
		}
		if cursorByteCol >= 0 && startInLine <= cursorByteCol && cursorByteCol < endInLine {
			_, runeBytes := utf8.DecodeRuneInString(plain[cursorByteCol:])
			out = spliceAnnotationRange(out, plain, startInLine, cursorByteCol, a.Style)
			out = spliceAnnotationRange(out, plain, cursorByteCol+runeBytes, endInLine, a.Style)
			continue
		}
		out = spliceAnnotationRange(out, plain, startInLine, endInLine, a.Style)
	}
	return out
}

func spliceAnnotationRange(out, plain string, startByte, endByte int, style lipgloss.Style) string {
	if startByte >= endByte {
		return out
	}
	preCol := lipgloss.Width(plain[:startByte])
	target := plain[startByte:endByte]
	return ansiSpliceAtCol(out, preCol, lipgloss.Width(target), style.Render(target))
}

func ansiSpliceAtCol(styled string, col, width int, replacement string) string {
	left := ansi.Truncate(styled, col, "")
	rest := ansi.TruncateLeft(styled, col, "")
	right := ansi.TruncateLeft(rest, width, "")
	return left + replacement + right
}

// renderFences runs chroma over fenced blocks whose interior intersects
// [top, bottom) and maps styled output back to source-line indexes
// (marker lines excluded). Blocks outside the viewport are skipped.
func renderFences(lines []string, ctxs []LineContext, st Styles, top, bottom int) map[int]string {
	out := map[int]string{}
	i := 0
	for i < len(lines) {
		if ctxs[i].Kind != BlockCodeFence || ctxs[i].InsideFence {
			i++
			continue
		}
		start := i + 1
		j := start
		for j < len(lines) && ctxs[j].InsideFence {
			j++
		}
		if start < j && start < bottom && j > top {
			info := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(lines[i]), "`~"))
			body := strings.Join(lines[start:j], "\n")
			styled := highlightFence(info, body, st)
			for k, sline := range styled {
				if start+k < j {
					out[start+k] = sline
				}
			}
		}
		if j < len(lines) && ctxs[j].Kind == BlockCodeFence {
			i = j + 1
		} else {
			i = j
		}
	}
	return out
}

// styleLine applies block-aware styling to one source line. The cursor
// row skips the pre-rendered chroma output because the rune-replaced
// source no longer matches the pre-tokenised version.
func styleLine(line string, ctx LineContext, st Styles, fenceLines map[int]string, idx int, isCursorRow bool) string {
	switch ctx.Kind {
	case BlockCodeFence:
		if ctx.InsideFence && !isCursorRow {
			if styled, ok := fenceLines[idx]; ok {
				return styled
			}
		}
		return st.CodeBlock.Render(line)

	case BlockHeading:
		level := ctx.HeadingLevel
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		pfx := strings.Repeat("#", level) + " "
		if !strings.HasPrefix(line, pfx) {
			// Cursor lives on one of the # runes; skip span tokenizing.
			return st.Heading[level-1].Render(line)
		}
		body := line[len(pfx):]
		return st.ListMarker.Render(pfx) + st.Heading[level-1].Render(renderSpans(tokenize(body), st))

	case BlockQuote:
		face := st.Quote
		if ctx.QuoteDepth >= 2 {
			face = st.DeepQuote
		}
		return face.Render(line)

	case BlockListItem, BlockTaskItem:
		return styleListLine(line, ctx, st)

	case BlockCodeIndent:
		return st.CodeBlock.Render(line)

	case BlockTable, BlockBlank, BlockParagraph:
		fallthrough
	default:
		return renderSpans(tokenize(line), st)
	}
}

// styleListLine splits prefix (marker, optional task box, trailing
// space) from body and styles each part. A cursor-mutated prefix
// falls back to whole-line tokenization.
func styleListLine(line string, ctx LineContext, st Styles) string {
	at := strings.Index(line, ctx.PostPrefix)
	if at <= 0 {
		return renderSpans(tokenize(line), st)
	}
	prefix := line[:at]
	body := line[at:]

	if ctx.Kind == BlockTaskItem {
		lb := strings.Index(prefix, "[")
		rb := strings.Index(prefix, "]")
		if lb >= 0 && rb > lb {
			marker := prefix[:lb]
			box := prefix[lb : rb+1]
			trailing := prefix[rb+1:]
			return st.ListMarker.Render(marker) +
				st.TaskBox.Render(box) +
				st.ListMarker.Render(trailing) +
				renderSpans(tokenize(body), st)
		}
	}
	return st.ListMarker.Render(prefix) + renderSpans(tokenize(body), st)
}

// visualWrap splits a possibly-styled source line into width-bounded
// visual rows. For quoted lines (ctx.QuoteDepth > 0) the prefix is
// rendered separately and prepended to every emitted row; the body is
// wrapped against the reduced budget. Non-quoted lines wrap directly.
// Uses ansi.Wrap, which is word-aware and hardwraps overlong tokens
// while preserving ANSI escape codes across breaks.
func visualWrap(line string, width int, ctx LineContext, styles Styles) []string {
	if width <= 0 {
		return []string{line}
	}
	if ctx.QuoteDepth == 0 || ctx.InsideFence {
		if lipgloss.Width(line) <= width {
			return []string{line}
		}
		return strings.Split(ansi.Wrap(line, width, ""), "\n")
	}
	// Quote-aware path lands in Task 3.
	if lipgloss.Width(line) <= width {
		return []string{line}
	}
	return strings.Split(ansi.Wrap(line, width, ""), "\n")
}

func offsetToRowCol(src string, off int) (row, col int) {
	pos := 0
	for _, r := range src {
		if pos >= off {
			return row, col
		}
		if r == '\n' {
			row++
			col = 0
		} else {
			col++
		}
		pos++
	}
	return row, col
}

// runeToByteOffset returns the byte offset of the col-th rune (or
// len(line) when col runs past the end).
func runeToByteOffset(line string, col int) int {
	n := 0
	for i := range line {
		if n == col {
			return i
		}
		n++
	}
	return len(line)
}

func insertCursorBlock(line string, col int) string {
	runes := []rune(line)
	if col >= len(runes) {
		return line + "█"
	}
	return string(runes[:col]) + "█" + string(runes[col+1:])
}
