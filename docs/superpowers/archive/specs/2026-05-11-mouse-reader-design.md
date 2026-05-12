# Mouse support — reader, attachments, scroll (Pass 33)

**Status:** accepted
**Date:** 2026-05-11
**Pass:** 33
**Related ADRs:** 0189b (v2 input model), 0217 (declarative tea.View
fields), 0185 (footnote ribbon), 0072 (wired/unwired help).

## Goal

Wire bubbletea v2's mouse pipeline so the viewer scrolls on wheel,
attachment chips open on click, and harvested URL runs launch on
click. Mouse is purely additive — every action remains keyboard-
reachable; no keybinding moves or shrinks.

## Scope

- In: reader viewport wheel scroll; click on attachment chip;
  click on `[N]: <url>` footnote ribbon row; `tea.MouseMode`
  declared in `App.View()`.
- Out: sidebar selection, message-list row selection, picker
  navigation, compose, status-bar segment hovers. All of that is
  Pass 34 or later, or never.

## Settled facts

1. **`MouseMode = MouseModeCellMotion`.** AllMotion reports
   continuous motion deltas the UI would drop on the floor (no
   hover state today, no plans for one in v1). CellMotion gives
   the four primitives we need — click, release, wheel, drag —
   without the input-bandwidth tax.
2. **Hit-testing lives in `reader.Model`.** Settled in STATUS.md.
   App routes; reader claims.
3. **Footnote click target = entire `[N]: <url>` ribbon row.**
   Not the inline `[^N]` glyph. Reasoning: the inline glyph is
   3–4 cells mid-paragraph; making it click-to-browser is a
   surprise hazard (terminal selection, accidental clicks during
   read flow). The ribbon row is the explicit affordance and
   already the visual answer to "what would I click."
4. **No close-on-outside-click for overlays.** Mouse outside an
   open overlay's box is absorbed silently. Keyboard is the
   canonical dismiss path (`Esc`/`q`); a stray click shouldn't
   tear down a destructive-action confirm.

## Architecture

### Dispatch

`App.Update` grows a new arm parallel to `tea.KeyPressMsg`:

```go
case tea.MouseMsg:
    return m.updateMouse(msg)
```

`updateMouse` mirrors the existing key cascade:

1. If any overlay is open (per the documented cascade order),
   return `m, nil` — absorb.
2. If `m.viewerOpen && m.acct.Viewer().Phase() == reader.PhaseReady`,
   forward to `reader.Model.Update` through the account model
   (`m.acct, cmd = m.acct.UpdateViewer(msg)`).
3. Otherwise return `m, nil`. Mouse is inert on the account view
   in Pass 33.

`tea.MouseMsg` is the interface; the four concrete types
(`MouseClickMsg`, `MouseReleaseMsg`, `MouseWheelMsg`,
`MouseMotionMsg`) all satisfy it. Reader's handler ignores motion
and release; we never need them.

### tea.View

`App.view` sets `v.MouseMode = tea.MouseModeCellMotion` once,
unconditionally. Declarative, every frame, in line with ADR-0217.

### Reader

`reader.Model` adds an unexported `hits []hitZone` slice rebuilt
on every `layout()` call:

```go
type hitZone struct {
    kind     hitKind  // hitChip, hitFootnote
    index    int      // 0-based into v.attachments or v.links
    rowStart int      // inclusive
    rowEnd   int      // exclusive
    colStart int      // SPUA cells, inclusive
    colEnd   int      // SPUA cells, exclusive
}
```

Two coordinate systems:

- **Chip hits** are frame-local: `rowStart` is row in the chip
  block (0..chipHeight-1), `colStart/End` from
  `ansix.Measurer.Width`-walked offsets of each rendered chip on
  its wrapped line.
- **Footnote hits** are body-local: `rowStart` is the line index
  inside `content.RenderBodyWithFootnotes`'s output string (which
  feeds the viewport). The full row is the hit zone; `colStart=0`,
  `colEnd=contentWidth`.

`Update` translates incoming mouse coords to the appropriate
local frame:

```go
case tea.MouseWheelMsg:
    // Forward to viewport only when click falls within the body
    // rectangle (below panel + invite + chip).
    if v.mouseInBody(m) { return v.forwardToViewport(m) }
    return v, nil

case tea.MouseClickMsg:
    if v.mouseInChipRow(m) {
        if h, ok := v.hitChip(m); ok {
            // emit OpenAttachmentMsg{UID, Item: v.attachments[h.index]}
        }
        return v, nil
    }
    if v.mouseInBody(m) {
        bodyRow := m.Y - v.bodyOriginY() + v.viewport.YOffset()
        if h, ok := v.hitFootnote(bodyRow); ok {
            return v, openURLCmd(v.links[h.index])
        }
        return v.forwardToViewport(m)
    }
```

`forwardToViewport` calls `v.viewport, cmd = v.viewport.Update(m)`
and returns. The `bubbles/v2/viewport` already handles wheel
deltas natively — we just need the events to reach it.

### content package

`content.RenderBodyWithFootnotes` grows a third return:

```go
func RenderBodyWithFootnotes(...) (rendered string, urls []string,
    ribbonRows []int)
```

`ribbonRows[i]` is the 0-based line index of the `[i]: <url>` row
in `rendered`. Walked alongside the existing ribbon emit at the
bottom of the function. Reader stores `ribbonRows` on
`BodyLoadedMsg` arrival and uses it to populate footnote hit
zones in `layout()`.

This keeps SPUA-aware column math co-located with the render that
produced it; reader never re-walks the body string to find ribbon
rows.

### Attachment click → open

Reader emits the existing `OpenAttachmentMsg` (currently emitted
from the attach picker on `o`/`Enter`/digit). Routing it from a
chip click reuses the App-side `openAttachmentCmd` plumbing
verbatim — no new pathways into `xdg-open`.

Actually verify: today `@` opens the picker, then the picker
emits the open msg. If reader can emit the open msg directly on
chip click, we bypass the picker, which is the goal — clicks
should feel like the keyboard `1`–`9` shortcut, not a two-step
ceremony.

### Help / footer

No new bindings advertised; mouse is invisible in `help` and the
footer. ADR-0072 wired/unwired flags don't apply to mouse — the
help popover is the keybinding reference, mouse is an additive
convenience.

## Testing

- `reader_test.go` adds table-driven cases for `hitChip` and
  `hitFootnote` (zone math at narrow + wide widths, with and
  without SPUA glyphs in attachment names).
- `content/render_footnote_test.go` adds an assertion that
  `ribbonRows` indices land on lines that match `^\[\d+\]: `
  after stripping ANSI.
- `app_test.go` adds a dispatch test: `tea.MouseClickMsg` while
  viewer is ready and `links` has 1 entry routes through to
  `openURLCmd` with the expected URL.
- Live tmux verification at 80×24 and 120×40: open a message
  with two attachments and three harvested URLs; verify wheel
  scrolls the body, chip clicks open the right attachment,
  ribbon clicks open the right URL.

## Non-goals (deferred)

- Sidebar folder selection on click → Pass 34.
- Message-list row selection / cursor placement on click →
  Pass 34.
- Compose field focus on click → never (modifier-free single-
  field-at-a-time is the philosophy).
- Hover state, tooltips, drag-to-select → never in v1.
- Click-to-launch on the inline `[^N]` glyph — settled above.

## Open questions

None. All three from STATUS.md are settled inline above.
