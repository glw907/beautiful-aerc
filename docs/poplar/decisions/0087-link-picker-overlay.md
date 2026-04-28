---
title: Link picker overlay
status: accepted
date: 2026-04-28
---

## Context

The viewer's `1`-`9` numeric quick-launch (ADR-0067) opens up to 9
harvested URLs but provides no affordance for messages with more, no
preview of the destination, and no keyboard navigation between
candidates. `Tab` was reserved as a placeholder.

A modal picker was the natural fit: similar shape to the help popover
(ADR-0082), invoked from a single key, dismissible with `Esc`/`Tab`.
The bubbles `list` component is the obvious analogue, but its
deviation is justified — the picker needs a custom row format
(right-aligned indices, leading-space pad, 50-cell URL truncation,
2-row preview footer) that doesn't fit `list.Item`'s rendering
contract without significant override.

## Decision

`Tab` while the viewer is open and ready opens a modal `LinkPicker`.
App owns the open state and overlay composition (mirrors help
popover). The picker model lives in `internal/ui/linkpicker.go`.

Key vocabulary inside the picker:
- `j`/`k` (or `down`/`up`) — move cursor.
- `Enter` — launch the URL under the cursor and close.
- `1`-`9` — quick-launch by index; in-range → launch + close, out-of-
  range → inert.
- `Esc`/`Tab` — close without launching.
- `q` and any other unbound key — swallowed.

Visual contract:
- Box width capped at 70 cells, floor of 20, otherwise `width-4`.
- Index column right-aligned with leading-space pad so closing `]`
  aligns: `␣[1]` ... `[12]` for a 12-link picker.
- Inline URL truncated to 50 cells via `displayTruncate`.
- Preview footer is 2 rows: full URL of the cursor row wrapped via
  `ansi.Hardwrap(ansi.Wordwrap(...))`, with `…` truncating row 2 if
  the URL exceeds two rows worth of cells.
- Centered on screen via `centerOverlay` (shared with help popover).
- Background dimmed via `DimANSI`, composited via `PlaceOverlay`.

Communication:
- `LinkPickerOpenMsg{Links []string}` — viewer emits on `Tab` when
  `len(v.links) > 0`. App handles by calling `linkPicker.Open(...)`.
- `LinkPickerClosedMsg{}` — picker emits on close. App calls
  `Close()`.
- `LaunchURLMsg{URL string}` — picker emits on Enter / numeric. App
  responds with `launchURLCmd(url)`. Errors swallowed (xdg-open
  detaches; exit status unreliable).

Shared helpers:
- `parseLinkKey(s, count) (int, bool)` — decodes a `1`-`9` keypress
  into a link index. Used by both viewer and picker.
- `centerOverlay(box, totalW, totalH) (int, int)` — centered top-
  left coordinates with non-negative clamp. Used by both popover and
  picker.

While the picker is open, `App.Update` short-circuits all keys into
`linkPicker.Update` (mirrors the `helpOpen` short-circuit pattern).
`WindowSizeMsg` is threaded into the picker via `SetSize`.

## Consequences

- Tab is now a wired binding in the viewer help vocabulary.
- The picker is the second App-owned modal overlay. Help and picker
  cannot both be open via the UI (help opens from any context;
  picker requires viewer ready) but are mutually exclusive by
  ordering in `App.Update` regardless.
- `LinkPicker.Box` mutates `p.offset` on a value receiver as a
  render-time scroll clamp; this is intentional since `Update`
  manages cursor only and offset is recomputed each frame.
- The picker's effectiveness is gated on bare-URL autolinking
  (BACKLOG #22). For markdown-formatted bodies and any body where
  the body parser produces `Link` spans, the picker works as
  designed. For plaintext bodies with bare URLs, `v.links` is empty
  and `Tab` is inert.
- Bubbles `list` was rejected as the analogue. ADR-0070 anticipates
  per-screen prototype passes where deviations are named — this is
  one. Custom because: row format requires column alignment that
  doesn't map onto `list.Item.Render`, and the preview footer is
  outside `list`'s vocabulary.
