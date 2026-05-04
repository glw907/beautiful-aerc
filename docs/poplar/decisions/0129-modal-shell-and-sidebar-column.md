---
title: ModalShell and SidebarColumn — overlay frame + sidebar composite extraction
status: accepted
date: 2026-05-03
---

## Context

Pass 8.5c targeted three structural cleanups deferred from Pass 8.5b
(ADR-0128). Two of them — modal frame duplication across overlays
and the 60-line sidebar-column assembly inlined in `AccountTab.View`
— were declined in 8.5b on out-of-scope grounds; under the pre-beta
posture (ADR-0105) that decline didn't hold. Each modal overlay
(`LinkPicker`, `MovePicker`, `ConfirmModal`, `HelpPopover`)
duplicated `┌─ title ─┐` / `├─┤` / `└─┘` box-drawing plus
`open`/`width`/`height` lifecycle fields. `AccountTab` carried
account-line + folder region + spacer + search shelf assembly inline
on top of holding the children directly.

A 1-cell asymmetry surfaced during ModalShell adoption: the inline
top border in `LinkPicker.Box` and `MovePicker.Box` rendered as
`boxW + 1` cells (the `rest = boxW - 2 - lipgloss.Width(title)`
formula already accounted for the title's surrounding spaces but
ignored the right-border cell), while the bottom border was `boxW`.

## Decision

**`ModalShell` (internal/ui/modal_shell.go).** A small value-typed
shell each Box-rendering overlay embeds via a *named* field
(`shell ModalShell`, never anonymous embed — preserves grep-ability
and avoids method-name collisions). Carries `open bool` and
`width, height int`. Lifecycle accessors: `IsOpen`, `WithOpen`,
`SetSize`, `Width`, `Height`. The `Box(title string, bodyRows,
footerRows []string, contentW int) string` method renders the shared
frame; callers pre-pad each row to exactly `contentW` cells.
`HelpPopover` renders through `lipgloss.Style.Render` with a rounded
border, not box-drawing characters, and is therefore *not* a
ModalShell consumer. ConfirmModal, LinkPicker, MovePicker are.

**`SidebarColumn` (internal/ui/sidebar_column.go).** Owns the
left-hand column composition: account header rows, embedded
`Sidebar` and `SidebarSearch`, spacer padding, and shelf placement.
`AccountTab` holds one `SidebarColumn` field instead of separate
`sidebar` + `sidebarSearch` fields. SidebarColumn renders the column
content *without* the right-edge divider; `AccountTab` still owns
the row-by-row join with the message-list pane (preserves the
SPUA-A-safe assembly invariant — see ADR-0084).

**SidebarColumn API shape: verbose-explicit With\* chains.** Reads
go through `Sidebar()` / `SidebarSearch()` accessors; mutations
follow the local-var → pointer-receiver mutation → `WithSidebar` /
`WithSidebarSearch` pattern, matching how App threads chrome through
AccountTab. SidebarColumn does not propagate `SetSize` to its
children internally; `AccountTab.WindowSizeMsg` calls
`SetLayout`/`SetSize` on local Sidebar/SidebarSearch copies, then
re-wraps. SidebarColumn.SetSize stores its own column dims for
`View()` only. This keeps SidebarColumn unaware of `Layout` and
preserves the existing chrome-threading shape.

**Top-border asymmetry.** `ModalShell.Box` produces matching widths
for top, separator, and bottom borders (all `boxW` cells). Migrating
LinkPicker and MovePicker to `ModalShell.Box` fixes the pre-existing
1-cell-wider top border; goldens were updated.

## Consequences

- Adding a new Box-rendering overlay requires only the per-overlay
  bodyRows/footerRows assembly plus a `m.shell.Box(...)` call. ~30
  LOC of duplicated frame chrome per overlay collapsed.
- `LinkPicker` / `MovePicker` top borders now match bottom borders;
  the centered overlay is symmetric.
- `AccountTab.View()` shrank from ~60 lines of column assembly to a
  single `m.sidebarColumn.View()` call; child accessor pattern is
  consistent with how App threads other components.
- The verbose-explicit `WithSidebar` / `WithSidebarSearch` chains
  add boilerplate at every mutation site (~30 sites in AccountTab),
  but preserve the Elm immutable-model contract and keep
  SidebarColumn's API narrow.
- The `Box(title, bodyRows, footerRows, contentW)` shape carries the
  body/footer split (ConfirmModal, MovePicker, LinkPicker each
  render a `├─┤` separator above an action-hint footer); a single
  `contentRows` slice would have leaked separator chrome to the
  caller.
