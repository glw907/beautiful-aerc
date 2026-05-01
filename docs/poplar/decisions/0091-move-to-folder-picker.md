---
title: Move-to-folder picker — type-to-filter modal, arrow-key nav
status: accepted
date: 2026-05-01
---

## Context

Pass 6 shipped the optimistic-triage primitives (delete / archive /
star / read) with a shared toast + undo bar. The remaining triage
primitive — moving messages to an arbitrary folder — needs a picker
UX. The starter prompt left three open questions: filter UX, folder
ordering, no-match feedback.

## Decision

The move picker (`internal/ui/movepicker.go`) is a modal overlay
owned by App, mirroring the LinkPicker shape (centerOverlay +
DimANSI + PlaceOverlay; ADR-0087). Triggered by `m` from the
account view on a non-empty `ActionTargets` snapshot.

- **Filter:** type-to-filter, implicit input. Letter keys narrow the
  list (substring, case-insensitive). Backspace widens. No separate
  textinput component.
- **Navigation inside the modal:** `↑`/`↓` only. `j`/`k` are filter
  input while the picker is open. The modifier-free keybinding rule
  still holds (arrow keys are modifier-free). This is a documented
  carve-out from "vim-first" navigation, scoped to modal text-input
  contexts.
- **Folder ordering:** sidebar order at open time (Primary →
  Disposal → Custom). Source folder excluded.
- **No-match feedback:** when filter matches nothing, the list area
  renders a single dim hint `no folders match "<filter>"`; Enter is
  inert. No `ErrorMsg`.
- **Dispatch:** picker emits `MovePickerPickedMsg`; App routes it
  back into AccountTab; AccountTab uses the same `buildTriageCmd`
  shape as delete/archive (factored as `buildTriageCmdWithDest`).
  Toast extended with a `dest` field on `pendingAction` and
  `triageStartedMsg` to render `Moved N messages to <Display>`.

## Consequences

- Substring filter scales to Gmail-sized label sets without a new
  textinput dependency.
- Reusing `buildTriageCmd` keeps the optimistic / undo / error
  rollback flow identical to delete and archive.
- Recent-first ordering and folder creation from the picker are
  deferred to post-1.0.
