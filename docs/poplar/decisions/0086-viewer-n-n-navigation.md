---
title: Viewer n/N walks the visible row set
status: accepted
date: 2026-04-28
---

## Context

The viewer prototype (Pass 2.5b-4) wired `Enter` to open a message
and `q`/`Esc` to close, but had no in-viewer navigation between
messages. Users reading a thread had to close the viewer, move the
cursor, and re-open — three keystrokes per message. BACKLOG #9
called for `n`/`N` (next / previous) inside the viewer that respects
the current folder's filter and threading state.

## Decision

While the viewer is open and in the `viewerReady` phase, `n` advances
to the next visible message in the message list and `N` retreats.
Both reuse the same fetch / mark-read flow as `Enter`. Boundaries are
inert (no wrap-around). During `viewerLoading` `n`/`N` are inert so a
second body fetch isn't queued on top of the first.

Implementation:

- `MessageList.MoveCursor(delta int) (mail.UID, bool)` shifts the
  cursor by `delta` visible rows, skipping folded rows, returning the
  new UID and whether anything moved.
- `MessageList.MessageByUID(uid) (mail.MessageInfo, bool)` is a
  linear lookup over `m.source` (sufficient at folder scale; ~µs
  even for 10k-message folders).
- `AccountTab.openMessage(msg)` extracts the open / fetch / mark-
  read batch previously inlined in `openSelectedMessage`. Both
  `Enter` and `n`/`N` route through it.
- `Viewer.Phase()` exposes `v.phase` so `AccountTab.handleKey` can
  gate `n`/`N` against the loading state.
- The `n`/`N` intercept lives in `AccountTab.handleKey` immediately
  inside the `viewer.IsOpen()` branch, before unconditional
  delegation to `viewer.Update`. The viewer never sees these keys.

The intercept follows the visible-row contract: hidden (folded) rows
are skipped because `MoveCursor` delegates to the existing `moveBy`
walker, which respects `row.hidden`. Search filters and folder-scoped
sort apply by the same path.

## Consequences

- Viewer reading flow becomes single-keystroke: open with `Enter`,
  walk with `n`/`N`, close with `q`. Mark-as-read and body fetch are
  coalesced into the existing optimistic flow.
- The cursor in the underlying message list moves with the viewer.
  When the viewer closes, the cursor is on the last-viewed message,
  not on whatever was selected when the viewer opened.
- Boundary inert behavior matches Pine: pressing `n` past the last
  message does nothing. No bell, no error, no wrap.
- The `n`/`N` intercept uses `msg.String()` rather than `key.Matches`
  + `key.Binding`. This is consistent with the existing viewer-open
  dispatch in `AccountTab.handleKey` and tracked under BACKLOG #17.
- `MessageByUID`'s O(n) scan is acceptable at single-folder scale;
  promote to a UID→index map only if profiles show it dominating.
