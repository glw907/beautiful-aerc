# Catkin render-time soft wrap

**Pass 45 design.** Move catkin from reflow-at-width to the Typora /
iA Writer model: the source stays pristine, the renderer word-wraps
visually each paint, and a wrapped continuation of a quoted line
carries `> ` (× `QuoteDepth`) on every visual row.

## Problem

Today `catkin.Model.WithWidth` calls `Reflow`, which mutates the
buffer to fit the new width. Two consequences:

- Typing inside a long quoted paragraph keeps growing the source
  line until the next manual resize; the live view overflows the
  pane width.
- Resize is required to re-fit; mid-edit lines silently exceed
  width.
- `Reflow` permanently destroys the source's exact byte layout,
  which fights with the "source pristine" contract callers expect
  (compose holding the user's typed text verbatim).

## Decision

Soft wrap moves to render time:

1. **Source is pristine.** `WithWidth` updates the width and the
   underlying textarea size, but does **not** call `Reflow`.
2. **`ansi.Wrap` is the wrap primitive.** Word-aware, hardwraps
   overlong tokens (URLs > budget), preserves ANSI escapes across
   breaks the same way `ansi.Hardwrap` does today. Replaces the
   `ansi.Hardwrap` call in `render.go`'s `softWrap`.
3. **Quote-aware continuation prefix.** For each source line whose
   `LineContext.QuoteDepth > 0`, the renderer strips the prefix
   before wrapping, wraps the body at `width - lipgloss.Width(prefix)`,
   then prepends the styled prefix to every emitted visual row.
4. **Scroll-off in visual-row space.** The 3-row margin operates
   on cursor visual row, computed each paint as
   `Σ visualHeight(line[i], width) for i<cursorRow + visualOffset
   (cursorLine, cursorCol)`. `viewportTop` stays a source row to
   keep the `lines[top:]` slice cheap; only the margin comparison
   converts.
5. **`Reflow` survives as `Model.Reflowed()`.** Already exists.
   `WithWidth` stops calling it. Binding `Reflowed()` to a manual
   command (or to send-time wrap in `mailcompose.AssembleMIME`)
   is a follow-up pass; this pass only stops the auto-call.

## Non-goals

- List-item continuation styling (continuation rows for `- item`
  / `1. item` keep current naked-continuation rendering).
- Code-fence wrap behaviour (`BlockCodeFence` lines still wrap as
  today via `ansi.Wrap` on the chroma-styled output).
- Send-time wrap in `mailcompose.AssembleMIME` and the manual
  `Reflow` keybinding (separate pass).
- Explicit SGR reset across visual breaks. Today's `ansi.Hardwrap`
  doesn't insert one and downstream `tea.View` rendering tolerates
  the open run; `ansi.Wrap` matches. Revisit only if a test
  surfaces tearing.

## Affected surface

### `internal/catkin/render.go`

- `softWrap(line, width)` → `visualWrap(line, width, ctx,
  styles) []string`. Splits prefix from body when
  `ctx.QuoteDepth > 0`, wraps the body via `ansi.Wrap`, prepends
  the styled prefix to every row. Non-quoted lines: direct
  `ansi.Wrap(line, width, "")`.
- New helper `styledQuotePrefix(ctx LineContext, st Styles) string`
  rendering `strings.Repeat("> ", ctx.QuoteDepth)` with `st.Quote`
  / `st.DeepQuote` (matching `styleLine`'s face choice at depth ≥ 2).
- `Render` / `RenderAnnotated` thread `ctxs[i]` into `visualWrap`.

### `internal/catkin/scrolloff.go`

- `applyScrollOff` takes the line list, ctxs, width, and cursor
  row/col already in scope via the Model.
- New helper `cursorVisualRow(lines []string, ctxs []LineContext,
  width, cursorRow, cursorCol int) (visualRow, totalVisual int)`.
  Sums per-line visual heights with the same prefix-stripping
  budget rule as `visualWrap`. Cursor offset within the cursor
  line is computed by wrapping the cursor line's body up to
  `cursorCol` cells and counting newlines.
- `ClampViewport` keeps its signature but is called with the
  visual cursor row and total visual rows. (Rename if needed for
  test clarity; the source-row variant is no longer used.)
- Typewriter mode: same conversion. `clampViewportTypewriter`
  midpoint becomes a visual midpoint.

### `internal/catkin/catkin.go`

- `WithWidth` drops the `Reflow` call. The width still propagates
  to `m.buf.WithWidth(w)` so textarea's internal column math stays
  correct.
- `Reflowed()` stays as-is for callers that want the legacy
  wrap-into-source behaviour.

### Tests

- `render_test.go`: quote-continuation at widths 30/50/80 and
  depths 1/2; overlong-URL row; cursor on continuation row;
  annotation enclosing a break point.
- `scrolloff_test.go`: long quoted paragraph (5+ visual rows)
  with cursor on last visual row → viewport adjusts; typewriter
  mode keeps cursor centred in visual space.
- `wrap_test.go`: visualWrap prefix budget arithmetic, hardwrap
  fallback for unbreakable tokens, non-quoted passthrough.

## Verification

- Live tmux capture at 120×40 stress: Gold Nugget Mailchimp reply,
  cursor moved into the middle of a 4-deep quoted paragraph;
  visible wrap, prefixes on every row, cursor stays inside
  scroll-off band.
- `make check` green.
- Minimum viable width (60×24) capture confirms continuation
  prefix doesn't overflow when budget shrinks.

## ADR

One ADR closes the pass: "Catkin render-time soft wrap (Typora /
iA Writer model)". Supersedes the `softWrap → ansi.Hardwrap`
clause in the catkin invariants block.

## Invariants update

`.claude/rules/catkin-invariants.md` — narrow the soft-wrap clause:

> soft-wrap via render-time `ansi.Wrap` with quoted-line
> continuation prefix (`> ` × `QuoteDepth`); `Reflow` is the
> manual-rewrap operation, not invoked from `WithWidth`.

## Bubbletea conventions check

- No new component; `catkin.Model.View()` still returns a single
  string sized to its width × height.
- Width math uses `lipgloss.Width` for the prefix; `ansi.Wrap`
  uses grapheme widths internally — both honour SPUA cells the
  same way as the existing `ansi.Hardwrap` call.
- No I/O moved into `View()`. The cursor visual-row computation
  is pure.
- `WithSize`/`WithWidth` remain the size contract; the only
  behaviour change is the dropped `Reflow` call.
