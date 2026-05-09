---
title: internal/ansix — vendored SPUA-aware width adapter
status: accepted
date: 2026-05-08
---

## Context

ADR-0084 introduced a runtime probe for the rendered cell width of
SPUA-A glyphs (Nerd Font icons) and a `uicore.DisplayCells` family
that adjusts `lipgloss.Width` accordingly. ADR-0180 closed the
upstream `clipperhouse/displaywidth` override-hook path: the
maintainer prefers wrappers, and lipgloss now routes through that
library via `charmbracelet/x/ansi`. The width-math helpers were
sitting in `uicore` next to unrelated chrome — `ModalShell`,
`PlaceOverlay`, `Spinner`, `ComputeLayout`. Conflating "the width
adapter we vendor" with "shared UI chrome" obscured the package
boundary the wrap-not-fork stance assumes: a small, clearly-scoped
shim that future passes can swap or extend without touching
unrelated code.

## Decision

A new package `internal/ansix/` owns the SPUA-aware width layer.
It exports `Width`, `Truncate`, `TruncateEllipsis`,
`PadOrTruncate`, `SpuaCount`, `SPUACellWidth`, and
`SetSPUACellWidth`. Implementations move from
`internal/ui/uicore/iconwidth.go` unchanged in semantics; the
runtime cell-width probe still calls `ansix.SetSPUACellWidth`
exactly once at startup. `uicore.FillRowToWidth` and
`uicore.ApplyBg` stay in `uicore` (they are row-assembly helpers,
not width math) but route their measurements and truncations
through `ansix`.

The JoinHorizontal ban from ADR-0084 stands. The shim measures
correctly, but lipgloss's internal `lipgloss.Width` calls inside
`JoinHorizontal` still undercount SPUA-A glyphs, and `internal/
ansix/` cannot reach those without forking lipgloss. Manual row-
by-row joins remain mandatory under `spuaCellWidth != 1`.

## Consequences

- Sixteen callsites across `cmd/poplar/`, `internal/ui/account/`,
  `compose/`, `messagelist/`, `reader/`, `sidebar/`, and the
  associated tests now import `ansix` instead of using the
  `uicore.Display*` family. The renames are mechanical:
  `DisplayCells → Width`, `DisplayTruncate → Truncate`,
  `DisplayTruncateEllipsis → TruncateEllipsis`,
  `DisplayPadOrTruncate → PadOrTruncate`, plus `SpuaCount` /
  `SPUACellWidth` / `SetSPUACellWidth` keeping their names.
- The override-hook fork prototype at
  `~/Projects/displaywidth/add-overrides` stays parked. If a
  future signal makes upstream-injection tractable, `ansix` is the
  one place that would absorb the change — its public surface is
  shaped to allow swapping the implementation without rippling
  back through callers.
- ADR-0084 stays in force; ADR-0161 (uicore composition) loses
  the width-math helpers from its inventory. Future invariant
  edits that touch the helper list reference `ansix` directly.
