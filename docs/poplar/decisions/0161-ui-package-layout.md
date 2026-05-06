---
title: internal/ui/ package layout — bubbles-shaped subpackages + uicore
status: accepted
date: 2026-05-06
---

## Context

By the end of Pass 9h, `internal/ui/` was a flat ~17k-LOC package:
one `package ui` namespace holding the App, every screen, every
overlay, every render helper, every Msg type, and every cmd. The
Pass 9h consolidation ADRs flagged the names (`AccountTab`,
`ComposeTab`) as placeholders pending a structural pass, and the
cmds.go file had become a kitchen sink that grew with every new
feature.

Three forces argued for a split before v1.0:

- **External convention.** The bubbles ecosystem ships
  `package list`, `package viewport`, `package textinput` — each a
  bubbles-shaped leaf with `package.Model` + `package.New`.
  Poplar's flat layout reads strangely against that norm.
- **Future-feature accommodation.** Post-1.0 work (CardDAV
  contacts, calendar surfaces, neovim companion) needs a clear
  place to plug in. A flat `package ui` with one App type doesn't
  give those new surfaces a natural shelf.
- **Per-screen reasoning.** A screen's render code, its cmds, and
  its msgs all live in different files within one package. Moving
  those into a per-screen subpackage co-locates the artifacts and
  makes the boundary visible to readers (and to subagents
  executing focused tasks).

Pre-beta posture made the breaking renames first-class. The risk
was a cycle: subpackages need shared chrome (modal frames, render
primitives, layout enums) but cannot import their parent package.

## Decision

`internal/ui/` is split into seven bubbles-shaped subpackages plus
a sibling `uicore` for shared chrome:

```
internal/ui/
  app.go, cmds.go, keys.go, layout.go, top_line.go, styles.go,
  status_bar.go, footer.go, error_banner.go, toast.go,
  confirm_modal.go, conflict_overlay.go, outbox_overlay.go,
  account_tab.go, date_format.go, golden_test.go, cache_helpers_test.go

  uicore/
    modal_shell.go, overlay.go, dim.go, render.go,
    iconwidth.go, layout.go (LayoutMode/IconSet/icon tables),
    search.go (SearchMode)

  compose/    — compose.Model, compose.SeedKind, SeedReply/All/Forward
  movepicker/ — movepicker.Model + Msgs
  helppopover/— helppopover.Model
  messagelist/— messagelist.Model
  sidebar/    — sidebar.Model, sidebar.Column, sidebar.Search,
                sidebar.ClearSearchMsg
  reader/     — reader.Model, reader.LinkPicker, reader.AttachPicker,
                reader.LoadBodyCmd / MarkReadCmd / LoadAttachmentsCmd /
                OpenAttachmentCmd / SaveAttachmentCmd, plus
                BodyLoadedMsg / AttachmentsLoadedMsg / etc.
```

`internal/ui/uicore/` holds the cycle-breaking primitives —
`ModalShell`, `PlaceOverlay`, `DimANSI`, render helpers
(`PadOrTruncate`, `TruncateToWidth`, `DisplayCells`,
`DisplayTruncate`, `DisplayTruncateEllipsis`, `DisplayPadOrTruncate`,
`CenterOverlay`, `ClampScrollOffset`, `ApplyBg`, `FillRowToWidth`,
`SpuaCount`, `SPUACellWidth`, `SetSPUACellWidth`), the `LayoutMode`
+ `IconSet` types and the `SimpleIcons`/`FancyIcons` tables, and
`SearchMode` + constants. Both `internal/ui/` and every subpackage
import `uicore` directly. Per-subpackage `Styles` structs are
narrow projections of `ui.Styles` — each subpackage's `styles.go`
holds its `NewStyles(*theme.CompiledTheme)` constructor; this is
the only place the `lipgloss.NewStyle()` policy admits beyond
`internal/ui/styles.go` and `internal/theme/palette.go`.

`mail.FolderEntry` lives in `internal/mail` (it carries
`mail.ClassifiedFolder` + display projections; `mail.Group` is
already there), removing a former translation loop in
`account_tab.go` between `ui.FolderEntry` and `movepicker.FolderEntry`.

The split is two passes. Pass 9h.1 (this pass) extracted the
leaves: compose, movepicker, helppopover, messagelist, sidebar,
reader, and uicore. The two structurally hardest pieces —
extracting `account/` (the AccountTab parent) and the residual
`cmds.go` decomposition — are deferred to Pass 9h.2 with their
own plan, ADR, and live tmux verification, per the pass-size
budget rule. `AccountTab` is still the parent that composes
sidebar + messagelist + reader in `internal/ui/`; renaming it to
`account.Model` is the work of 9h.2.

## Consequences

- Subpackages cannot import `internal/ui/`. Anything every screen
  needs lives in `uicore`. Future shared primitives (e.g., a
  reusable input validator) follow the same rule.
- `cmds.go` shrunk substantially this pass — reader, sidebar, and
  compose cmds + msgs moved out — but still holds App-level
  cross-cutting Cmds (`pumpUpdatesCmd`, `pumpCacheCmd`,
  `launchURLCmd`, the confirm-modal trio, and the cmds whose error
  paths emit `ErrorMsg`). Pass 9h.2 will audit and finalize what
  belongs at the App level.
- `URLOpener` and `TidyFn` (App-level seams from ADRs 0128/0160)
  stay in `internal/ui/`. Reader and compose cmds accept them as
  function-typed parameters at call time, avoiding any reverse
  import.
- Each subpackage Model carries a small set of test-boundary
  accessors (`Layout()`, `Source()`, `SelectedCanonical()`, etc.)
  because parent-package tests in `package ui` run on the
  AccountTab integration; once `account/` extracts in 9h.2 most of
  those accessors collapse into `package <name>_test` external
  tests.
- The `internal/ui/compose` import shadows the existing
  `internal/compose` domain package. App-side imports use
  `uicompose "github.com/glw907/poplar/internal/ui/compose"` to
  disambiguate; inside `internal/ui/compose/` the domain package
  is aliased as `mailcompose`. Both aliases are stable.
- Live tmux verification at 80×24 and 120×40 is deferred to the
  pass-end of 9h.2 (when the AccountTab parent rename and cmds.go
  decomposition complete) so the captures cover the full reorg.
  Pass 9h.1's regression net is `make check` — 24 packages,
  ~12 seconds, all green.
