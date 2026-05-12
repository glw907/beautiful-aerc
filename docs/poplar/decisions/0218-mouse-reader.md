---
title: Mouse support in the reader pane
status: accepted
date: 2026-05-11
---

## Context

bubbletea v2 exposes a declarative `tea.View.MouseMode` field and
four pointer-event types (`MouseClickMsg`, `MouseReleaseMsg`,
`MouseWheelMsg`, `MouseMotionMsg`). Pre-Pass-33 poplar set neither
the mode nor any handler — the program ran in a keyboard-only loop.
Pass 33 wires mouse support into the reader pane only; sidebar and
cross-pane mouse is deferred to Pass 34.

Three settled questions framed the design:

1. `CellMotion` vs `AllMotion` mode.
2. Whose `Update` claims the event when overlays + viewer + account
   are all live possibilities.
3. Whether the inline `[^N]` glyph or the `[N]: <url>` ribbon row
   is the click target for footnoted URLs.

## Decision

**Mode.** `tea.View.MouseMode = MouseModeCellMotion` is declared in
`App.view` every frame, unconditional, alongside the other ADR-0217
fields. `AllMotion` would report continuous motion deltas the UI
would drop — there is no hover affordance in v1 — wasting input
bandwidth.

**Dispatch cascade.** `App.Update` grows a `tea.MouseMsg` arm
parallel to `tea.KeyPressMsg`. The arm:

- absorbs silently when any overlay is open (`anyOverlayOpen`
  covers help, confirm, conflict, outbox, link picker, attach
  picker, move picker, compose attach/schedule pickers,
  reschedule, popover-form, popover);
- when no overlay is open and `viewerOpen && acct.ViewerPhase()
  == reader.PhaseReady`, translates global coords to right-pane-
  local coords (subtracting sidebar width + 1 divider for X, the
  topLine row for Y) and forwards via the new
  `account.Model.UpdateViewer(tea.Msg) (Model, tea.Cmd)`;
- ignores everything else. The account view, message list,
  compose surface, sidebar, status bar, and footer do not claim
  mouse in Pass 33.

Overlay close-on-outside-click is rejected: keyboard is the
canonical dismiss path; an accidental click should not tear down
a destructive-action confirm or a half-typed search shelf.

**Reader hit-testing.** `reader.Model` carries two slices —
`chipHits` in chip-row-local coords, `bodyHits` in viewport-local
coords — populated at `layout()` time. `tea.MouseWheelMsg` falling
inside the body rectangle is forwarded to the embedded
`bubbles/v2/viewport`. `tea.MouseClickMsg`:

- on a chip → emits `OpenAttachmentMsg{UID, Att}` (same Msg the
  attach picker emits on `o`/`Enter`/digit).
- on a footnote ribbon row → emits `LaunchURLMsg{URL}` (same Msg
  the digit keys and link picker emit).
- elsewhere → no-op.

**Click target.** The full `[N]: <url>` ribbon row, not the inline
`[^N]` glyph. Reasoning: a 3-4 cell mid-paragraph glyph is a
surprise hazard — terminal text selection, accidental clicks while
reading. The ribbon row is the explicit affordance and aligns with
how the keyboard launches behave (digit keys map 1:1 to the
ribbon's `[N]` numbering).

**Render-time hit tables.** `content.RenderBodyWithFootnotes`
returns a third value, `[]content.FootnoteRow{Row, PickerIndex}`,
locating each ribbon entry's first line in the rendered string and
indexing it into the picker URL slice. SPUA-aware column math
lives co-located with the render that produced it; reader never
re-walks the body string to find ribbon rows.

## Consequences

- Mouse is purely additive. Every action remains keyboard-
  reachable; no key bindings move, shrink, or get advertised in
  help/footer. ADR-0072 wired/unwired flags apply to keys only.
- Pass 34 inherits the dispatch shape. The sidebar will claim
  click when no overlay is open and `viewerOpen == false`; the
  message list will claim click when neither sidebar nor viewer
  does. The translation step in `App.updateMouse` is the seam.
- `account.Model.UpdateViewer` and `account.Model.ViewerPhase`
  are new accessors. `Model.Viewer()` already returned a value-
  type `reader.Model`; the new methods avoid the round-trip of
  `WithViewer(Viewer().Update(...))` at every call site.
- Live mouse verification is bounded: tmux `send-keys` cannot
  inject SGR mouse sequences reliably, so the unit tests carry
  the binding. Reader-level tests assert click → URL launch,
  click → attachment open, wheel → viewport scroll, and out-of-
  hit no-op. App-level tests assert overlay absorb, closed-
  viewer inert, and the `tea.View.MouseMode` declaration.
- The inline `[^N]` glyph could later be promoted to a click
  target without an ADR amendment — its hit zone simply joins
  `bodyHits` at render time. Decision is reversible behind the
  same dispatch.
