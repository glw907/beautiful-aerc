package catkin

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Render produces Catkin's view content: styled text plus a
// cursor block, soft-wrapped at width and clipped to height.
// Styling is a pure render-time overlay. The raw source is not
// touched. The zero Styles value yields plain output identical
// to Pass 9.
func Render(src string, width, height, top, cursor int, styles Styles, mode DisplayMode) string {
	return RenderAnnotated(src, width, height, top, cursor, styles, mode, nil)
}

// RenderAnnotated produces Catkin's view content with optional
// annotations overlaid before the cursor block. ann may be nil.
func RenderAnnotated(src string, width, height, top, cursor int, styles Styles, mode DisplayMode, ann *AnnotationSet) string {
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	cursorRow, cursorCol := offsetToRowCol(src, cursor)
	fenceLines := renderFences(lines, ctxs, styles, top, top+height)
	focusFirst, focusLast := -1, -1
	if mode.focus() {
		focusFirst, focusLast = activeParagraphRange(ctxs, cursorRow)
	}

	rowOffsets := computeRowOffsets(src)

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
			styled = applyAnnotationsToLine(styled, lines[i], rowOffsets[i], ann.rangesOnRow(src, i))
		}
		if matchCol >= 0 {
			styled = overlayMatch(styled, matchCol, matchCh, styles.MatchHighlight)
		}
		if mode.focus() && (i < focusFirst || i > focusLast) {
			styled = styles.Dim.Render(ansi.Strip(styled))
		}
		for _, w := range softWrap(styled, width) {
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

// computeRowOffsets returns the byte offset of the start of each
// line in src. The slice has one entry per line (including an empty
// final line if src ends with '\n').
func computeRowOffsets(src string) []int {
	out := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			out = append(out, i+1)
		}
	}
	return out
}

// applyAnnotationsToLine overlays annotation styles onto the
// already-styled line. Each annotation's Style.Render is applied
// to the plain-text byte range, then spliced back into the styled
// string by display-cell position. Multiple annotations on one line
// are applied left-to-right. Later splices do not disturb earlier
// ones because ansiSpliceAtCol works on column offsets from the
// original plain string.
func applyAnnotationsToLine(styled, plain string, lineOffset int, anns []Annotation) string {
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
		preCol := lipgloss.Width(plain[:startInLine])
		targetPlain := plain[startInLine:endInLine]
		styledTarget := a.Style.Render(targetPlain)
		out = ansiSpliceAtCol(out, preCol, lipgloss.Width(targetPlain), styledTarget)
	}
	return out
}

// ansiSpliceAtCol replaces a fixed-width column range of an
// ANSI-styled string with replacement. Uses ansi.Truncate and
// ansi.TruncateLeft to slice around the target region.
func ansiSpliceAtCol(styled string, col, width int, replacement string) string {
	left := ansi.Truncate(styled, col, "")
	rest := ansi.TruncateLeft(styled, col, "")
	right := ansi.TruncateLeft(rest, width, "")
	return left + replacement + right
}

// renderFences runs chroma over each fenced block whose
// interior intersects [top, bottom) and maps the styled output
// back to the source-line indexes inside the fence (marker
// lines excluded). Blocks fully outside the viewport are
// skipped, keeping per-render cost bounded by visible fences.
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

// styleLine applies block-aware styling to one source line.
// On the cursor row the chroma-pre-rendered fence output is
// skipped: the rune-replaced source no longer matches the
// pre-tokenised version.
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
			// Cursor is on one of the # runes. Render whole line as
			// the heading face and skip span tokenizing.
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

// styleListLine splits prefix (marker, optional task box,
// trailing space) from body and styles each part. When the
// cursor has mutated the prefix into a non-matching shape it
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

// softWrap splits a (possibly styled) line into width-bounded
// chunks. ANSI escape sequences pass through ansi.Hardwrap
// without being broken.
func softWrap(line string, width int) []string {
	if width <= 0 || lipgloss.Width(line) <= width {
		return []string{line}
	}
	return strings.Split(ansi.Hardwrap(line, width, true), "\n")
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

func insertCursorBlock(line string, col int) string {
	runes := []rune(line)
	if col >= len(runes) {
		return line + "█"
	}
	return string(runes[:col]) + "█" + string(runes[col+1:])
}
