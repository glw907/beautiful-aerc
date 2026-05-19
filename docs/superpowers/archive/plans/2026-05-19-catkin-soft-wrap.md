# Catkin Render-Time Soft Wrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move catkin's soft wrap to render time (Typora / iA Writer model). Source stays pristine. Each paint word-wraps via `ansi.Wrap`; quoted-line continuations carry `> ` × `QuoteDepth` on every visual row. Scroll-off becomes visual-row aware.

**Architecture:** Replace `softWrap` in `internal/catkin/render.go` with a context-aware `visualWrap` that splits prefix from body for quoted lines, wraps the body against the budget `width - prefixCells`, then re-prefixes every emitted row. Drop the auto-`Reflow` call from `Model.WithWidth`. Convert `applyScrollOff` math to visual-row space via a new `cursorVisualRow` helper.

**Tech Stack:** Go 1.26, `github.com/charmbracelet/x/ansi@v0.11.7` (`ansi.Wrap`), `charm.land/lipgloss/v2`. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-05-19-catkin-soft-wrap-design.md`

---

## File Structure

- Modify: `internal/catkin/render.go` — `softWrap` → `visualWrap(line, width, ctx, styles)`, new `styledQuotePrefix(ctx, styles)`, threaded `ctxs[i]` through `Render`/`RenderAnnotated`.
- Modify: `internal/catkin/catkin.go` — `WithWidth` drops the `Reflow` call.
- Modify: `internal/catkin/scrolloff.go` — new `cursorVisualRow(lines, ctxs, width, cursorRow, cursorCol)` helper; `applyScrollOff` converts to visual rows.
- Add tests in existing files: `render_test.go`, `scrolloff_test.go`, `wrap_test.go`.

Each task below is TDD: failing test → minimal impl → green → commit.

---

### Task 1: Drop auto-Reflow from `WithWidth`

**Files:**
- Modify: `internal/catkin/catkin.go:262-273`
- Test: `internal/catkin/catkin_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/catkin/catkin_test.go`:

```go
func TestWithWidthDoesNotReflow(t *testing.T) {
	src := "> this is a moderately long quoted paragraph that exceeds twenty cells"
	m := New().WithValue(src).WithSize(80, 10)
	m = m.WithWidth(20)
	if got := m.Value(); got != src {
		t.Errorf("WithWidth must not mutate source:\n got %q\nwant %q", got, src)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/catkin -run TestWithWidthDoesNotReflow -v`
Expected: FAIL — current `WithWidth` calls `Reflow`, mutating the buffer.

- [ ] **Step 3: Edit `WithWidth`**

Replace `internal/catkin/catkin.go:262-273` with:

```go
// WithWidth sets the body wrap width. Source is not mutated; wrap is
// resolved at render time.
func (m Model) WithWidth(w int) Model {
	if w == m.width {
		return m
	}
	m.width = w
	m.buf = m.buf.WithWidth(w)
	return m
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags=dev ./internal/catkin -run TestWithWidthDoesNotReflow -v`
Expected: PASS.

Also run the full catkin suite to confirm no other test was relying on auto-reflow:

Run: `go test -tags=dev ./internal/catkin -v`
Expected: all PASS. (If a test fails because it asserted reflow side-effects on `WithWidth`, update that test to call `Reflowed()` explicitly — the manual surface — and note the change in the commit message.)

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/catkin.go internal/catkin/catkin_test.go
git commit -m "catkin: drop auto-Reflow from WithWidth

Source stays pristine; wrap moves to render time in subsequent
tasks. Reflowed() remains for callers that want the legacy
wrap-into-source behaviour.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `visualWrap` for non-quoted lines (passthrough via `ansi.Wrap`)

**Files:**
- Modify: `internal/catkin/render.go` (`softWrap` → `visualWrap`)
- Test: `internal/catkin/wrap_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/catkin/wrap_test.go`:

```go
func TestVisualWrapPlainWordBoundary(t *testing.T) {
	got := visualWrap("alpha beta gamma delta", 10, LineContext{Kind: BlockParagraph}, Styles{})
	want := []string{"alpha beta", "gamma", "delta"}
	if !slicesEqual(got, want) {
		t.Errorf("visualWrap plain word-boundary:\n got %q\nwant %q", got, want)
	}
}

func TestVisualWrapHardwrapsLongToken(t *testing.T) {
	url := "https://example.com/very/long/path/that/exceeds/the/budget"
	got := visualWrap(url, 15, LineContext{Kind: BlockParagraph}, Styles{})
	if len(got) < 2 {
		t.Fatalf("expected hardwrap into multiple rows, got %d: %q", len(got), got)
	}
	if joined := strings.Join(got, ""); joined != url {
		t.Errorf("hardwrap lost content:\n got %q\nwant %q", joined, url)
	}
}
```

If `slicesEqual` is not already defined in this package, add it once at the top of `wrap_test.go`:

```go
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

(Check first; the file already imports `strings` for existing tests.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/catkin -run TestVisualWrap -v`
Expected: FAIL — `visualWrap` does not exist.

- [ ] **Step 3: Replace `softWrap` with `visualWrap`**

In `internal/catkin/render.go`, replace the `softWrap` function (currently lines 237-243) with:

```go
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
```

Update the lone call site in `RenderAnnotated` (currently `render.go:64`) — defer the `ctx` threading to Task 5; for now keep `softWrap`'s call signature working by adding a temporary shim:

In `RenderAnnotated`, replace the existing `for _, w := range softWrap(styled, width) {` block with:

```go
for _, w := range visualWrap(styled, width, ctxs[i], styles) {
    if len(visual) >= height {
        break
    }
    visual = append(visual, w)
}
```

- [ ] **Step 4: Run tests**

Run: `go test -tags=dev ./internal/catkin -run TestVisualWrap -v`
Expected: PASS.

Run: `go test -tags=dev ./internal/catkin -v`
Expected: all PASS (existing render tests still rely on the same wrap behaviour for non-quoted lines).

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/render.go internal/catkin/wrap_test.go
git commit -m "catkin: visualWrap stub, word-aware via ansi.Wrap

Replaces ansi.Hardwrap with ansi.Wrap (word-aware, overlong
tokens hardwrap). Quote-aware continuation prefix lands next.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Quoted-line continuation prefix

**Files:**
- Modify: `internal/catkin/render.go`
- Test: `internal/catkin/wrap_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/catkin/wrap_test.go`:

```go
func TestVisualWrapQuotedSingleDepth(t *testing.T) {
	// Source line includes the "> " prefix; styleLine has already
	// rendered the whole line through the Quote face. For test
	// hygiene we feed a plain string and zero Styles; the only
	// invariant under test is "every visual row begins with > ".
	line := "> alpha beta gamma delta epsilon"
	ctx := LineContext{Kind: BlockQuote, QuoteDepth: 1, PrefixWidth: 2}
	got := visualWrap(line, 14, ctx, Styles{})
	if len(got) < 2 {
		t.Fatalf("expected ≥2 visual rows, got %d: %q", len(got), got)
	}
	for i, row := range got {
		if !strings.HasPrefix(row, "> ") {
			t.Errorf("row %d missing quote prefix: %q", i, row)
		}
		if w := lipgloss.Width(row); w > 14 {
			t.Errorf("row %d exceeds width 14: %d cells, %q", i, w, row)
		}
	}
}

func TestVisualWrapQuotedDepthTwo(t *testing.T) {
	line := "> > alpha beta gamma delta epsilon zeta"
	ctx := LineContext{Kind: BlockQuote, QuoteDepth: 2, PrefixWidth: 4}
	got := visualWrap(line, 18, ctx, Styles{})
	if len(got) < 2 {
		t.Fatalf("expected ≥2 rows, got %d: %q", len(got), got)
	}
	for i, row := range got {
		if !strings.HasPrefix(row, "> > ") {
			t.Errorf("row %d missing depth-2 prefix: %q", i, row)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/catkin -run TestVisualWrapQuoted -v`
Expected: FAIL — continuation rows have no prefix.

- [ ] **Step 3: Implement the quote-aware path**

In `internal/catkin/render.go`, add the helper above `visualWrap`:

```go
// styledQuotePrefix returns the quote-prefix string ("> " × depth)
// rendered through the face styleLine would choose at this depth.
// A zero Styles value yields the plain prefix.
func styledQuotePrefix(ctx LineContext, st Styles) string {
	if ctx.QuoteDepth == 0 {
		return ""
	}
	plain := strings.Repeat("> ", ctx.QuoteDepth)
	face := st.Quote
	if ctx.QuoteDepth >= 2 {
		face = st.DeepQuote
	}
	return face.Render(plain)
}
```

Replace the `visualWrap` quote-path stub with the real implementation:

```go
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

	prefix := styledQuotePrefix(ctx, styles)
	prefixCells := lipgloss.Width(prefix)
	budget := width - prefixCells
	if budget < 1 {
		budget = 1
	}

	// Strip the plain prefix from the styled line. styleLine renders
	// the whole quoted line through one face, so the styled body sits
	// after exactly ctx.PrefixWidth visible cells.
	body := ansi.TruncateLeft(line, prefixCells, "")

	if lipgloss.Width(body) <= budget {
		return []string{prefix + body}
	}
	wrapped := strings.Split(ansi.Wrap(body, budget, ""), "\n")
	out := make([]string, len(wrapped))
	for i, row := range wrapped {
		out[i] = prefix + row
	}
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test -tags=dev ./internal/catkin -run TestVisualWrap -v`
Expected: PASS.

Run: `go test -tags=dev ./internal/catkin -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/render.go internal/catkin/wrap_test.go
git commit -m "catkin: re-prefix wrapped quoted-line continuations

Continuation visual rows of a quoted source line carry > × depth
on every paint. Source is unchanged; the renderer strips, wraps,
and re-prefixes.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Render-level integration test (styled quote continuation)

**Files:**
- Test: `internal/catkin/render_test.go` (append)

This verifies the full pipeline — `Render` → `styleLine` (Quote face) → `visualWrap` → re-prefix — produces continuation rows whose prefix is styled, not plain.

- [ ] **Step 1: Write the failing test**

(It will pass if Task 3 is correct; but writing it now locks in the contract from the renderer's surface. If it fails, it indicates a regression in how `styleLine`'s render interacts with `ansi.TruncateLeft`.)

Append to `internal/catkin/render_test.go`:

```go
func TestRenderQuoteContinuationCarriesStyledPrefix(t *testing.T) {
	st := Styles{Quote: lipgloss.NewStyle().Italic(true)}
	src := "> alpha beta gamma delta epsilon zeta eta theta"
	got := Render(src, 16, 6, 0, 0, st, ModeNormal)
	rows := strings.Split(got, "\n")
	nonBlank := 0
	for _, r := range rows {
		if strings.TrimSpace(ansiStrip(r)) == "" {
			continue
		}
		nonBlank++
		if !strings.Contains(r, "\x1b[3m") {
			t.Errorf("continuation row missing italic SGR: %q", r)
		}
		plain := ansiStrip(r)
		if !strings.HasPrefix(plain, "> ") {
			t.Errorf("continuation row missing > prefix: %q", plain)
		}
	}
	if nonBlank < 2 {
		t.Fatalf("expected ≥2 continuation rows, got %d", nonBlank)
	}
}
```

If `ansiStrip` is not already defined in `render_test.go`, add it near the imports:

```go
func ansiStrip(s string) string { return ansi.Strip(s) }
```

and import `"github.com/charmbracelet/x/ansi"` in the test file if not already.

- [ ] **Step 2: Run test**

Run: `go test -tags=dev ./internal/catkin -run TestRenderQuoteContinuationCarriesStyledPrefix -v`
Expected: PASS (Task 3's implementation already satisfies this).

If it fails, debug Task 3's `ansi.TruncateLeft` interaction with the styled line — a likely cause is the styled body returned by `styleLine` putting the SGR open before the prefix; in that case, strip the prefix BEFORE styling. Update `styleLine`'s `BlockQuote` arm to render only the post-prefix body and leave prefix to `visualWrap`:

```go
case BlockQuote:
    face := st.Quote
    if ctx.QuoteDepth >= 2 {
        face = st.DeepQuote
    }
    prefix := strings.Repeat("> ", ctx.QuoteDepth)
    body := strings.TrimPrefix(line, prefix)
    return face.Render(prefix) + face.Render(body)
```

This rewrite still produces a fully-styled visible string equivalent for first-row rendering, but with a clean SGR boundary at the prefix/body seam so `ansi.TruncateLeft(line, prefixCells, "")` slices cleanly.

- [ ] **Step 3: Commit (only if the styleLine adjustment was needed)**

```bash
git add internal/catkin/render.go internal/catkin/render_test.go
git commit -m "catkin: render-level test for styled quote continuation

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

Otherwise commit just the test:

```bash
git add internal/catkin/render_test.go
git commit -m "catkin: render-level test for styled quote continuation

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Cursor on a continuation row

**Files:**
- Test: `internal/catkin/render_test.go` (append)

Pre-wrap `insertCursorBlock` places `█` at the cursor's source column. After `ansi.Wrap`, the block should land on the visual row containing that column. This task locks the contract; the existing implementation should already satisfy it.

- [ ] **Step 1: Write the failing test**

Append to `internal/catkin/render_test.go`:

```go
func TestRenderQuoteCursorLandsOnContinuationRow(t *testing.T) {
	src := "> alpha beta gamma delta epsilon"
	// Place cursor inside "epsilon" (offset 28 in the source).
	got := Render(src, 14, 6, 0, 28, Styles{}, ModeNormal)
	rows := strings.Split(got, "\n")
	cursorRowIdx := -1
	for i, r := range rows {
		if strings.Contains(ansiStrip(r), "█") {
			cursorRowIdx = i
			break
		}
	}
	if cursorRowIdx < 1 {
		t.Fatalf("cursor must land on a continuation row (>=1); got idx=%d rows=%q", cursorRowIdx, rows)
	}
	if !strings.HasPrefix(ansiStrip(rows[cursorRowIdx]), "> ") {
		t.Errorf("cursor's continuation row missing > prefix: %q", rows[cursorRowIdx])
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test -tags=dev ./internal/catkin -run TestRenderQuoteCursorLandsOnContinuationRow -v`
Expected: PASS (cursor splice runs pre-wrap; ansi.Wrap routes the `█` rune through the wrap state machine like any non-space grapheme).

If it fails, the likely cause is that the wrap budget didn't account for the cursor's extra rune width — `█` is 1 cell wide so this should be a non-issue; investigate by printing `rows`.

- [ ] **Step 3: Commit**

```bash
git add internal/catkin/render_test.go
git commit -m "catkin: lock cursor-on-continuation-row contract

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: `cursorVisualRow` helper

**Files:**
- Modify: `internal/catkin/scrolloff.go`
- Test: `internal/catkin/scrolloff_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/catkin/scrolloff_test.go`:

```go
func TestCursorVisualRowQuotedParagraph(t *testing.T) {
	// 3 source rows: blank, long quoted line that wraps to 3 visual
	// rows at width 14, blank. Cursor at end of the quoted line.
	src := "\n> alpha beta gamma delta epsilon\n"
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	// Cursor on row 1 (the quoted line), column = rune count of that line.
	cursorRow := 1
	cursorCol := len([]rune(lines[cursorRow]))
	vr, total := cursorVisualRow(lines, ctxs, 14, cursorRow, cursorCol)
	if total < 4 { // blank + 3 wrap rows + trailing blank ≥ 4
		t.Errorf("expected total visual rows ≥4, got %d", total)
	}
	if vr < 2 {
		t.Errorf("cursor at end of wrapped quoted line should sit on the last visual row of the wrap (≥2); got %d", vr)
	}
}

func TestCursorVisualRowPlainTotalMatchesSourceLineCount(t *testing.T) {
	src := "one\ntwo\nthree"
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	_, total := cursorVisualRow(lines, ctxs, 80, 0, 0)
	if total != 3 {
		t.Errorf("plain short lines: expected total=3, got %d", total)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=dev ./internal/catkin -run TestCursorVisualRow -v`
Expected: FAIL — function does not exist.

- [ ] **Step 3: Implement the helper**

In `internal/catkin/scrolloff.go`, replace the file with:

```go
package catkin

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const scrollOff = 3

// ClampViewport returns a viewport top (in source-line units) that
// keeps cursorVisual within the scroll-off band of a height-tall
// viewport when the document totals totalVisual visual rows. Returns
// 0 when total fits in height.
//
// top is interpreted in visual-row space for the band check; callers
// supply the source-row top and the band check still works because
// visual rows for the lines above cursorRow are summed into
// cursorVisual.
func ClampViewport(top, height, cursorVisual, totalVisual int) int {
	if totalVisual <= height {
		return 0
	}
	rel := cursorVisual - top
	switch {
	case rel < scrollOff:
		top = cursorVisual - scrollOff
	case rel >= height-scrollOff:
		top = cursorVisual - (height - scrollOff - 1)
	}
	if top < 0 {
		top = 0
	}
	if top > totalVisual-height {
		top = totalVisual - height
	}
	return top
}

// cursorVisualRow returns (cursor visual-row index, total visual rows)
// for the document at the configured width. Width is the full viewport
// width; quoted lines budget themselves the same way visualWrap does.
func cursorVisualRow(lines []string, ctxs []LineContext, width, cursorRow, cursorCol int) (int, int) {
	if width <= 0 {
		return cursorRow, len(lines)
	}
	total := 0
	cursorVR := 0
	for i, line := range lines {
		ctx := LineContext{}
		if i < len(ctxs) {
			ctx = ctxs[i]
		}
		h := visualHeight(line, width, ctx)
		if i == cursorRow {
			cursorVR = total + visualOffsetInLine(line, width, ctx, cursorCol)
		}
		total += h
	}
	return cursorVR, total
}

// visualHeight returns the number of visual rows a source line occupies
// at width. Mirrors visualWrap's budget arithmetic.
func visualHeight(line string, width int, ctx LineContext) int {
	if width <= 0 {
		return 1
	}
	budget := width
	if ctx.QuoteDepth > 0 && !ctx.InsideFence {
		budget = width - 2*ctx.QuoteDepth
		if budget < 1 {
			budget = 1
		}
		// Strip the plain prefix; the body wraps against budget.
		prefix := strings.Repeat("> ", ctx.QuoteDepth)
		body := strings.TrimPrefix(line, prefix)
		if lipgloss.Width(body) <= budget {
			return 1
		}
		return strings.Count(ansi.Wrap(body, budget, ""), "\n") + 1
	}
	if lipgloss.Width(line) <= width {
		return 1
	}
	return strings.Count(ansi.Wrap(line, width, ""), "\n") + 1
}

// visualOffsetInLine returns the cursor's visual-row offset within its
// source line: 0 for "still on the first visual row of this line",
// 1 for the second, and so on.
func visualOffsetInLine(line string, width int, ctx LineContext, col int) int {
	if width <= 0 || col <= 0 {
		return 0
	}
	// Walk the source line one rune at a time, simulating the wrap
	// budget. This is O(col) per cursor-row call, which is fine for
	// compose-sized buffers.
	budget := width
	skip := 0
	if ctx.QuoteDepth > 0 && !ctx.InsideFence {
		budget = width - 2*ctx.QuoteDepth
		if budget < 1 {
			budget = 1
		}
		skip = 2 * ctx.QuoteDepth
	}
	if col <= skip {
		return 0
	}
	body := []rune(strings.TrimPrefix(line, strings.Repeat("> ", ctx.QuoteDepth)))
	consumed := col - skip
	if consumed > len(body) {
		consumed = len(body)
	}
	if consumed <= 0 {
		return 0
	}
	prefix := string(body[:consumed])
	if lipgloss.Width(prefix) <= budget {
		return 0
	}
	return strings.Count(ansi.Wrap(prefix, budget, ""), "\n")
}

func applyScrollOff(m Model) Model {
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	cursorRow, cursorCol := offsetToRowCol(src, cur)
	cursorVR, totalVR := cursorVisualRow(lines, ctxs, m.width, cursorRow, cursorCol)
	if m.mode.typewriter() && totalVR > m.height && m.height > 0 {
		m.viewportTop = clampViewportTypewriter(m.height, cursorVR, totalVR)
		return m
	}
	m.viewportTop = ClampViewport(m.viewportTop, m.height, cursorVR, totalVR)
	return m
}

// clampViewportTypewriter holds the cursor at the vertical midpoint
// of height, clamped to the document range.
func clampViewportTypewriter(height, cursorVR, totalVR int) int {
	top := cursorVR - height/2
	if top < 0 {
		top = 0
	}
	if top > totalVR-height {
		top = totalVR - height
	}
	return top
}

func lineCount(s string) int { return strings.Count(s, "\n") + 1 }
```

(Note: `lineCount` is preserved for any external caller; if grep shows it unused after this change, drop it in the same commit.)

**Important compatibility note:** `viewportTop` now indexes into visual-row space, not source-row space. The existing `Render` slice `for i := top; i < len(lines)` will be wrong. Fix `Render` in the same commit:

In `internal/catkin/render.go`, change the main loop to walk source lines while accounting for `top` in visual-row units:

```go
var visual []string
skipRows := top
for i := 0; i < len(lines) && len(visual) < height; i++ {
    raw := lines[i]
    // ... existing cursor-row / styling block ...
    rows := visualWrap(styled, width, ctxs[i], styles)
    for _, w := range rows {
        if skipRows > 0 {
            skipRows--
            continue
        }
        if len(visual) >= height {
            break
        }
        visual = append(visual, w)
    }
}
```

- [ ] **Step 4: Run tests**

Run: `go test -tags=dev ./internal/catkin -run TestCursorVisualRow -v`
Expected: PASS.

Run: `go test -tags=dev ./internal/catkin -v`
Expected: all PASS. Existing scrolloff tests may have been written against source-row semantics; if any fail, update them to the visual-row contract (their assertions should reflect "cursor visual row stays in band," not "cursor source row stays in band").

Run: `go test -tags=dev ./internal/catkin -run TestRender -v`
Expected: existing render tests still PASS — they use small inputs where source rows == visual rows.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/scrolloff.go internal/catkin/scrolloff_test.go internal/catkin/render.go
git commit -m "catkin: scroll-off in visual-row space

cursorVisualRow sums per-line visual heights against the wrap
budget. viewportTop is now a visual-row index; Render skips the
first top visual rows when slicing into the viewport.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Long-quoted-paragraph scroll-off integration test

**Files:**
- Test: `internal/catkin/scrolloff_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/catkin/scrolloff_test.go`:

```go
func TestApplyScrollOffKeepsCursorVisibleInLongQuote(t *testing.T) {
	// A quoted paragraph that wraps to ~5 visual rows at width 16.
	// Viewport height 4 means the cursor at end of line must push
	// viewportTop forward so the cursor row stays visible.
	src := "padding\n> alpha beta gamma delta epsilon zeta eta theta iota\npadding"
	m := New().WithValue(src).WithSize(16, 4)
	// Cursor at end of the quoted line.
	target := len("padding\n") + len("> alpha beta gamma delta epsilon zeta eta theta iota")
	m.buf = m.buf.WithRuneOffset(target)
	m = applyScrollOff(m)
	// Compute cursor visual row; assert it sits inside [top, top+height).
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	cursorRow, cursorCol := offsetToRowCol(src, target)
	cvr, _ := cursorVisualRow(lines, ctxs, 16, cursorRow, cursorCol)
	if cvr < m.viewportTop || cvr >= m.viewportTop+m.height {
		t.Errorf("cursor visual row %d outside viewport [%d, %d)", cvr, m.viewportTop, m.viewportTop+m.height)
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test -tags=dev ./internal/catkin -run TestApplyScrollOffKeepsCursorVisibleInLongQuote -v`
Expected: PASS (Task 6's implementation already handles this).

If it fails, the most likely cause is that `m.buf.WithRuneOffset(target)` doesn't return through the right Buffer setter — check `internal/catkin/buffer.go` for the API and adjust the test setup accordingly.

- [ ] **Step 3: Commit**

```bash
git add internal/catkin/scrolloff_test.go
git commit -m "catkin: integration test — long quoted paragraph stays in band

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Live tmux capture verification

**Files:** none modified; produces a capture for the ADR.

- [ ] **Step 1: Install the local build**

Run: `make install`
Expected: builds clean, installs to `~/.local/bin/poplar`.

- [ ] **Step 2: Live render check at 120×40**

Open the Gold Nugget Mailchimp reply (the stress case named in the starter prompt) via tmux per `.claude/docs/tmux-testing.md`. Move the cursor into the middle of a deeply-quoted paragraph and type a few characters. Visually confirm:

- Every wrapped continuation row of a quoted line begins with `> ` × depth.
- The cursor never leaves the visible pane; the scroll-off band holds.
- No source mutation: pressing `Esc` and re-opening the same draft shows the same byte layout.

- [ ] **Step 3: Minimum-viable-width spot check at 60×24**

Resize tmux to 60×24. Confirm the continuation prefix budget still leaves a usable body width (≥ 1 cell).

- [ ] **Step 4: Capture for the ADR**

Run: `tmux capture-pane -p -t poplar > /tmp/catkin-soft-wrap-capture.txt`

Move the capture into `docs/poplar/captures/2026-05-19-catkin-soft-wrap.txt` and reference it from the ADR.

- [ ] **Step 5: Commit the capture**

```bash
git add docs/poplar/captures/2026-05-19-catkin-soft-wrap.txt
git commit -m "catkin: capture render-time soft wrap at 120x40

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Pass-end ritual

This task IS the pass-end consolidation; do not skip it.

- [ ] **Step 1: Run /simplify**

Invoke the `simplify` skill against the pass's diff (`git diff master~9`). Apply genuine wins inline.

- [ ] **Step 2: Idiomatic-bubbletea checklist (UI changes)**

Walk `docs/poplar/bubbletea-conventions.md` §10 against the diff. No new component was introduced; the key items to verify are the width-math discipline (`lipgloss.Width` for the prefix, `ansi.Wrap` width-aware) and the size-contract (no new `MaxWidth` clipping; the child still honours its width). Note any deviation in the ADR with rationale.

- [ ] **Step 3: Write the ADR**

Create `docs/poplar/decisions/0243-catkin-render-time-soft-wrap.md` (or the next free number if 0243 is taken — check `ls docs/poplar/decisions/ | tail`):

```markdown
---
title: Catkin render-time soft wrap (Typora / iA Writer model)
status: accepted
date: 2026-05-19
---

## Context

Pre-pass, catkin wrapped at `WithWidth` by mutating the source via
`Reflow`. Long quoted paragraphs grew past the pane and required a
resize to re-fit; the source was no longer pristine.

## Decision

Soft wrap is a render-time operation. `Render` word-wraps each
source line via `ansi.Wrap` (overlong tokens hardwrap, ANSI
preserved across breaks). Quoted lines strip the prefix, wrap the
body against `width - 2*QuoteDepth`, and re-prepend the styled
prefix to every emitted visual row. Scroll-off operates in
visual-row space via `cursorVisualRow`. `Model.WithWidth` no
longer calls `Reflow`; `Reflowed()` remains for manual rewrap.

## Consequences

- Source is pristine; typing in a deep quote no longer overflows
  the pane mid-edit.
- Resize redraws into the new width instantly; no buffer mutation.
- `viewportTop` now indexes visual rows. External callers (none
  today) would need to convert.
- List-item continuation styling, code-fence wrap behaviour, and
  binding `Reflowed()` to a manual command remain future work.
- Supersedes the `softWrap → ansi.Hardwrap` clause in
  `.claude/rules/catkin-invariants.md`.
```

- [ ] **Step 4: Update invariants**

Edit `.claude/rules/catkin-invariants.md` — replace `soft-wrap via ansi.Hardwrap` with:

> soft-wrap via render-time `ansi.Wrap` with quoted-line
> continuation prefix (`> ` × `QuoteDepth`); `Reflow` is the
> manual-rewrap operation, not invoked from `WithWidth`.

Check `docs/poplar/invariants.md` (the catkin pointer line) — no edit needed unless the new ADR number affects the cross-reference.

- [ ] **Step 5: Update STATUS.md**

In `docs/poplar/STATUS.md`:

- Mark Pass 45 done in the pass table.
- Replace the "Next starter prompt" block with the next pass's prompt (whatever Geoff queues next; if unknown, set it to a generic "Pass 46+ dogfood-driven fixes" prompt and let the next session populate it).
- Confirm the file stays ≤60 lines.

- [ ] **Step 6: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-19-catkin-soft-wrap.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-19-catkin-soft-wrap-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 7: make check**

Run: `make check`
Expected: green.

- [ ] **Step 8: Commit + push + install**

```bash
git add -A
git commit -m "Pass 45: catkin render-time soft wrap

ADR-0243. Source is pristine; renderer word-wraps each paint
via ansi.Wrap and re-prefixes quoted continuations with > ×
depth. Scroll-off moves to visual-row space. WithWidth no
longer calls Reflow.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
git push
make install
```

---

## Self-review notes

- Spec coverage: every settled point in the design doc maps to a task (Reflow-drop → T1; `ansi.Wrap` → T2; quote continuation → T3-T4; cursor on continuation → T5; scroll-off visual math → T6-T7; tmux verification → T8; pass-end → T9). Non-goals (list-item continuation, fence wrap, manual reflow keybinding) are explicitly deferred and not in the task list.
- Type/name consistency: `visualWrap`, `styledQuotePrefix`, `cursorVisualRow`, `visualHeight`, `visualOffsetInLine`, `ClampViewport` (signature unchanged but semantics moved to visual-row space). `LineContext` fields used: `QuoteDepth`, `InsideFence`, `PrefixWidth`, `Kind`. All match `internal/catkin/blocks.go`.
- One real risk: `applyScrollOff` semantics shift means existing `scrolloff_test.go` cases may need updating. Task 6 step 4 explicitly handles this.
