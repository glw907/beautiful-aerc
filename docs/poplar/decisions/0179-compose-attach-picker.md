---
title: Compose-side attach picker — multi-select TUI file browser overlay
status: accepted
date: 2026-05-08
---

## Context

Issue #24 asked for an attachments-richer compose UI: a way to add
and remove file attachments to a draft before sending. The
viewer-side `AttachPicker` (ADR-0140) was the wrong shape — it
lists what arrived on the message, not what to send. A separate
construct was needed.

Two design questions sat open: what kind of picker, and where do
the attached chips live in the compose layout.

For the picker, the alternatives were (a) shelling out to a system
file dialog (xdg-portal / NSOpenPanel), (b) inline path entry, and
(c) an in-process TUI file browser. (a) breaks the single-binary
showcase posture and adds platform skew. (b) is friction-heavy for
multi-attach. (c) keeps everything in bubbletea and matches the
poplar surface vocabulary.

For the chip row, the alternatives were a shelf above Subject, a
chip row below Subject, or a footer-adjacent strip. Below Subject
mirrors aerc and Thunderbird and reads naturally with the
header-block visual rhythm.

## Decision

Compose grows a multi-select TUI file browser overlay
(`internal/ui/compose/attachpicker.go`) modeled on
`bubbles/filepicker` design vocabulary — vim h/l/g/G nav, async
`readDir` with an id-guard so stale results don't clobber a fresh
listing, view-state stack so ascend lands on the dir you came
from. Reimplemented rather than wrapped because filepicker is
single-select. `Ctrl+O` opens the overlay; `Space` toggles a file
into the selection; `a` accepts; `Esc` cancels; `Enter` on a file
with empty selection is a single-attach shortcut.

The picker renders through `uicore.ModalShell.Box` with a footer
hint row that varies by selection state (`a accept` vs
`a accept (N)`) and a path row that left-truncates with `…/`.
Three new picker style fields — `PickerCursor`, `PickerDim`,
`PickerError` — project from the existing palette (BgBase /
AccentPrimary inverse, FgDim, ColorError).

Compose grows a `focusAttach` enum slot between `focusSubject` and
`focusBody`. The Tab cycle skips it when `Draft.Attachments` is
empty, so the slot only joins the rotation once at least one
attachment exists. Inside `focusAttach`, `←/→` move the chip
cursor and `d` / `Backspace` / `Delete` remove the selected chip;
removing the last chip collapses focus back to Subject.

The chip row sits between the Subject and the divider, hidden
when no attachments. Each chip shows the basename plus humanized
size; the focused chip inverts via `AttachChipFocus`. When chips
overflow `width`, `fitChips` greedily includes leading chips and
appends a `+N` overflow chip.

`AttachAcceptedMsg{Paths []string}` and `AttachCancelledMsg{}`
cross the picker→compose boundary; compose appends paths to
`c.attachments` deduped, bumps `localDirty`, kicks autosave, and
remembers `c.attachLastDir` so the next open returns to the same
folder. App routes both msg types in its top-level Update arm
since the picker emits them via `tea.Cmd`.

The footer learns `^O attach` at rank 6 (same rank as `^T tidy`)
in `composeFooterGroups`. App's `View` overlays the picker on top
of compose via `uicore.PlaceOverlay` when
`m.compose.AttachPickerIsOpen()`. The picker's `SetSize` is fed
from compose's `SetSize`, which already runs on `WindowSizeMsg`.

## Consequences

The compose picker is part of compose's input window, so it does
not enter the global modal cascade (`confirm > conflict > outbox
> help > linkpicker > attachpicker > movepicker > form > popover`).
That cascade governs App-level overlays; compose receives keys
only when no global modal is open, and the compose picker rides
inside that.

Existing viewer `AttachPicker` (ADR-0140) is unaffected — the two
pickers share a name pattern but live in different packages
(`internal/ui/uicore` vs `internal/ui/compose`) and have
non-overlapping consumers.

Two deviations from the planning doc, both narrow:

1. The chip row uses no per-chip icon. The "Attach:" label
   already signals attach context; a paperclip on every chip is
   redundant and pushes width math through SPUA-A. Plain text
   chips with `lipgloss.Width` are correct and simpler.

2. Compose keeps its existing `tea.KeyCtrl*` raw type-switch
   pattern for `Ctrl+O` rather than introducing `key.Matches`
   against a `ComposeKeys` field — `compose.Model` has no `keys`
   field today, and the existing pattern is consistent across
   Send/Cancel/Tidy. The footer hint still derives from
   `keys.go`'s `ComposeKeys.Attach`.

The `internal/ui/compose/styles.go` palette grew six new fields
(three picker, three chip). The semantic map in
`docs/poplar/styling.md` is updated.

`internal/ui/uicore.IconSet` grew no new fields — `CustomFolder`
already covers the directory-row icon.

The compose surface now spans 7 plan tasks of file additions
(picker + tests, msgs, styles, model fields/Update arms/View row,
keys, footer, app overlay). It remains within the 8–12 task pass
budget when counted as the user-visible deliverable.
