---
title: Pass 17c — help popover does not adopt bubbles/v2/help
status: accepted
date: 2026-05-11
---

## Context

Pass 17c closes the bubbles-adoption arc that 15a (popovers on
`list`), 17a (sidebar tree), and 17b (messagelist on `list`)
opened. The remaining surface is the modal help popover at
`internal/ui/helppopover/`. The question this pass had to answer:
does `helppopover.Model` lift onto `bubbles/v2/help`, or do its
deviations survive as deliberate?

`bubbles/v2/help` (v2.1.0) renders one of two shapes from a
`KeyMap` interface: a single-line `ShortHelpView` of `key  desc`
pairs joined by ` • `, or a `FullHelpView` of columns where each
column is `JoinHorizontal(keys, " ", descs)`. Width truncation
falls back to an ellipsis. Bindings hidden via
`key.Binding.SetEnabled(false)` drop from the output rather than
dim.

Poplar's popover layers six features on top of that primitive:

1. **Wired vs unwired dimming** — every row carries a `wired bool`.
   Unwired rows render dim throughout; wired rows render
   bright-bold key + dim desc. ADR-0072 binds this distinction so
   the popover advertises the full planned vocabulary, not just
   what is currently dispatchable.
2. **Group headers** — "Navigate", "Triage", "Reply", "Search",
   "Select", "Threads", "Go To" each render as a styled heading
   above their column.
3. **3×2 grid for Go To** — six folder jumps balance into a grid,
   not a tall column.
4. **Embedded title in border** — the popover's top edge is
   `╭─ Message List ─…─╮` drawn as one atomic row so the title
   sits inside the corner box.
5. **Cached render** — `*cache` heap pointer keyed on
   `(context, w, h)` per ADR-0130 (view-stable overlay escape
   hatch from the immutable-model rule).
6. **No `lipgloss.JoinHorizontal`** — ADR-0084 forbids it when
   `spuaCellWidth != 1`. `bubbles/v2/help.FullHelpView` calls it
   internally; adopting `FullHelpView` would regress SPUA-A
   safety on Nerd-Font icon terminals.

## Decision

`helppopover.Model` does not adopt `bubbles/v2/help`. The custom
render pipeline at `internal/ui/helppopover/model.go` is the
binding implementation. The bubbles-adoption arc closes here:
sidebar, messagelist, and the popover-family pickers run on
bubbles; the modal help overlay does not.

The audit also retires the duplicate `account.keys.MsgList*`
bindings — dead since ADR-0199 routed nav through
`messagelist.KeyMap`. `account.Model.handleKey` forwards via
`m.msglist.KeyMap()` directly.

## Consequences

- The help popover stays one of two bubbles-deviating UI surfaces
  in `internal/ui/` after 17c (the other is the compose
  AttachPicker — ADR-0195). Both are ADR-justified.
- Future binding-vocabulary changes touch one place
  (`internal/ui/helppopover/model.go`) without an upstream
  contract to satisfy. The `wired bool` flag stays the slot
  bubbles can't model.
- `account.keys` shrinks by four bindings. `account.Model`
  dispatches list nav through the canonical `messagelist.KeyMap`,
  so help-popover and dispatch read from the same source.
- If `bubbles/v2/help` ever grows a wired/unwired slot and drops
  `JoinHorizontal` internally, revisit.
