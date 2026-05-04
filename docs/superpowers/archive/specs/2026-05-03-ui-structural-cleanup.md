# Pass 8.5c — UI structural cleanup (queued)

**Date:** 2026-05-03
**Status:** queued
**Predecessor:** Pass 8.5b (Elm conformance audit; ADR-0128)

## Goal

Three structural cleanups in `internal/ui/`, all deferred from Pass
8.5b's audit dispositions or flagged by its `/simplify` pass:

1. **Modal shell extraction** (R-modalshell from ADR-0128). Four
   overlays — `LinkPicker`, `MovePicker`, `ConfirmModal`,
   `HelpPopover` — each duplicate ~30 LOC of `┌─ title ─┐` /
   `├─┤` / `└─┘` box-drawing plus `IsOpen`/`Open`/`Close`/
   `SetSize` lifecycle plus `centerOverlay` delegation. Extract a
   `ModalShell` struct (or free helper) that owns the box frame
   and lifecycle; each overlay composes it and supplies only its
   content rows.
2. **Sidebar-column extraction** (R-sidebarcol, declined in 8.5b
   on out-of-scope grounds — that decline didn't hold up under the
   pre-beta posture). `AccountTab.View` carries 60+ lines of
   sidebar-column assembly: blank rows, account line, folder
   region padding, shelf clamping, row-by-row join with the
   divider. Extract into a `SidebarColumn` helper or component.
3. **Overlay render caching** (efficiency findings from 8.5b
   `/simplify`):
   - `MovePicker.Box` rebuilds `buildListRows` every render frame
     — `[]string` allocation proportional to `len(p.matches)`
     while the picker is open.
   - `HelpPopover.Box` re-renders the entire static layout from
     constant `accountGroups` / `viewerGroups` tables every frame.
     Cache the rendered string per (context, size) on Open /
     SetSize; invalidate only when those change.

## Scope

`internal/ui/` only. No behavior changes — purely structural and
performance.

## Settled (do not re-brainstorm)

- ADR-0128 R-modalshell rationale.
- Existing overlay surfaces and their key contracts (per
  `.claude/rules/ui-invariants.md` "Overlays" section).
- Styles-discipline invariant: `lipgloss.NewStyle()` only in
  `internal/ui/styles.go` and `internal/theme/palette.go`.

## Open questions (brainstorm before planning)

- Is `ModalShell` a struct each overlay embeds, or a free helper
  they call?
- Does the help popover's render cache live on `HelpPopover` itself
  or in a separate render-cache layer?
- Sidebar-column lifecycle: does the extracted component own its
  dims, or stay parameterized at every render?

## Approach

Capture golden-output tests for all four overlays at 80×24 and
120×40 BEFORE refactoring; verify byte-for-byte equality after
extraction. Brainstorm the open questions, write a plan doc at
`docs/superpowers/plans/YYYY-MM-DD-ui-structural-cleanup.md`, then
implement. Standard pass-end checklist applies.
