---
title: ConfirmModal overlay + manual empty-folder action
status: accepted
date: 2026-05-01
---

## Context

The retention sweep runs silently and is bounded by config. Users
also want an immediate "empty Trash now" affordance, distinct from
configured retention. Because the action is irreversible (Destroy
has no inverse), it must be guarded by a confirmation step rather
than the optimistic toast/undo pattern that delete/archive/move
share.

## Decision

New `E` key on the account view, advertised in the help popover's
Triage group. Inert outside Disposal folders (role: `trash`,
`junk`, `spam`).

A new `ConfirmModal` component (`internal/ui/confirm_modal.go`) is
the generic destructive-action confirmation overlay, App-owned and
composited via the same `centerOverlay` + `DimANSI` +
`PlaceOverlay` pattern as `LinkPicker` (ADR-0087) and `MovePicker`
(ADR-0091). The component takes a `ConfirmRequest{Title, Body,
OnYes}`; key bindings are fixed: `y` confirms (emits the request's
`OnYes` Cmd plus `ConfirmModalClosedMsg`), `n`/`Esc` dismiss
(`ConfirmModalClosedMsg` only), `q` is swallowed. The confirm
modal is the **topmost** overlay — Update key-routing and View
overlay rendering both check it before link/move pickers.

`E` flow: AccountTab emits `OpenConfirmEmptyMsg{Folder, Total,
Source}`; App opens the modal with body `"<N> messages will be
permanently deleted."`; on `y`, `EmptyFolderConfirmedMsg` flows
back to AccountTab which fires `emptyFolderCmd` (paginates
`QueryFolder` then `Destroy`). `emptyFolderDoneMsg` clears all
loaded rows and emits a `triageStartedMsg{op:"empty", n, dest:
display}`. The toast renders `Emptied <Folder> (<N>)` and
suppresses `[u undo]` (toast detects this via `op == "empty"`).

## Consequences

- `ConfirmModal` is reusable for future destructive actions
  (drop-account, purge-cache, etc.) without per-call overlay code.
- The undo bar's no-undo render path is one line in `renderToast`,
  keyed off the op string — no new pendingAction field.
- Manual empty paginates `QueryFolder` in batches of 1000 so very
  large folders are handled without a single jumbo request.
- Disposal folder detection lives in `AccountTab.dispatchEmpty`
  and `maybeRetentionSweep`, both keyed on `folder.Role` from the
  classifier — the role string set (`"trash"`, `"junk"`, `"spam"`)
  is the same set already used by `internal/mail/classify.go`.
