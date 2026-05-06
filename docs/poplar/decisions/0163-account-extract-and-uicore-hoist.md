---
title: account/ extract and uicore chrome hoist
status: accepted
date: 2026-05-06
---

## Context

ADR-0161 split `internal/ui/` into bubbles-shaped subpackages
(compose, helppopover, messagelist, movepicker, reader, sidebar)
and the `uicore` sibling. `AccountTab` — the parent that composes
sidebar/messagelist/reader and threads outbox/triage/sweep state —
was deferred to a follow-up pass because it's the largest extract
by responsibility.

Pass 9h.2 finishes that work. Two aspects needed adjudication
during the extract that ADR-0161's prediction did not anticipate:

1. **Chrome surface area.** Several types referenced by `AccountTab`
   live in package `ui` and are also referenced by the App-level
   chrome. Naively moving them into `account/` would force App to
   import `account` for shared types; leaving them in `ui` would
   force `account` to import `internal/ui/` (forbidden by ADR-0161).
   The list: `ErrorMsg` (produced by reader/compose/account cmds,
   consumed by App banner), `triageOp` + the `op*` constants
   (shared between account's startTriageCmd and App's toast
   renderer), `ComputeLayout` (used by both account.Update and
   App's frame layout math), `NewSpinner` (account spins folder
   loads; pattern reusable elsewhere).

2. **Confirm-modal trio location.** ADR-0161 predicted the trio
   (`OpenConfirmEmptyMsg`, `EmptyFolderConfirmedMsg`,
   `ConfirmModalClosedMsg`) would stay App-level because App owns
   the confirm modal. In practice, only `ConfirmModalYesMsg` and
   `ConfirmModalClosedMsg` belong with the modal. The pre/post-
   request msgs belong to the actor that opened the modal — for
   the empty-folder flow, that's account.

## Decision

`internal/ui/uicore/` is the canonical home for chrome-shared
types. Pass 9h.2 hoists into uicore:

- `ErrorMsg{Op, Err}`. Package `ui` keeps `type ErrorMsg =
  uicore.ErrorMsg` so App-side cmds and the banner consumer keep
  their unqualified spelling. Account cmds emit `uicore.ErrorMsg`
  directly.
- `TriageOp` and `Triage{None,Delete,Archive,Star,Unstar,Read,
  Unread,Move,Empty,SaveAttachment,Sending}` constants. Package
  `ui` keeps `type triageOp = uicore.TriageOp` plus aliases for
  each constant so toast.go reads without a prefix.
- `ComputeLayout(termWidth) LayoutMode` (was in `internal/ui/
  layout.go`). The function moves to `uicore/layoutfn.go`.
- `NewSpinner(*theme.CompiledTheme) spinner.Model`.

`account.Model` lifts:

- Folder cmds (`LoadFoldersCmd`, `OpenFolderCmd`,
  `RefreshFolderCmd`, `LoadMoreCmd`, `QueryFolderCmd`).
- Triage / sweep cmds (`QueueOpsCmd`, `EmptyFolderCmd`,
  `DestroyCmd`, `MarkReadCmd`, `startTriageCmd`).
- Body / attachment cmds invoked from `openMessage`
  (`LoadBodyCmd`, `LoadAttachmentsCmd`).
- Cache pump (`pumpCacheCmd` + `CacheEventMsg`).
- Cross-boundary msgs exported in `account/msgs.go`:
  `TriageStartedMsg`, `CacheEventMsg`, `FolderLoadedMsg`,
  `OpenConfirmEmptyMsg`, `EmptyFolderConfirmedMsg`. The remaining
  account-internal msgs (`foldersLoadedMsg`, `folderAppendedMsg`,
  `sweepCompletedMsg`, `emptyFolderDoneMsg`) stay unexported.

`ConfirmModalClosedMsg` moves into `confirm_modal.go` next to
`ConfirmModalYesMsg`. The empty-folder pre/post msgs go to
account because account is the actionable owner; App imports
account anyway.

`account/keys.go` carries `account.Keys` + `account.NewKeys` (was
`AccountKeys` in `internal/ui/keys.go`). `account/styles.go`
carries `account.Styles` (Dim, PanelDivider) constructed by
`NewStyles(*theme.CompiledTheme)` per the ADR-0161 pattern.
`account.New` no longer takes a `Styles` parameter — it builds
its own from theme.

App-level reader and compose cmds (`loadBodyCmd` is now in
account; `openAttachmentCmd`, `saveAttachmentCmd`, `composeSeedCmd`,
`composeSendCmd`, `envelopeFromDraft`, `resolveSentFolder`,
`sanitizeAttachFilename`, `resolveSaveTarget`) stay in
`internal/ui/cmds.go`. Lifting them into `reader/cmds.go` and
`compose/cmds.go` is queued behind reader/compose work; the App
currently orchestrates those flows directly.

Account exposes test seams `MsgList()`, `SidebarColumnValue()`,
`Viewer()`, `CurrentFolderName()`, `MessageListCount()`,
`SelectedMessage()`, `WithViewer`, `WithMsgList`. These replace
direct unexported-field access in App-level tests.

## Consequences

- `internal/ui/cmds.go` shrinks from 635 lines to 299, holding only
  App-scoped cross-cutting cmds: pump (`pumpUpdatesCmd` +
  `backendUpdateMsg`), URL launcher (`URLOpener`, `launchURLCmd`,
  `xdgOpenURL`), outbox cmds + msgs (`refreshOutboxDepthCmd`,
  `loadOutboxSummaryCmd`, `loadOutboxConflictsCmd`,
  `retryConflictCmd`, `discardConflictCmd`, with their msgs),
  attachment open/save cmds, compose seed/send cmds, plus the
  `ErrorMsg` alias.
- ADR-0161's prediction that `cmds.go` would carry "cmds that emit
  App-level ErrorMsg" narrows: ErrorMsg is now uicore-level, not
  App-level, so the policy is "subpackages that emit
  uicore.ErrorMsg own their cmds; cmds that orchestrate App seams
  (URLOpener, TidyFn) without natural single-subpackage ownership
  stay in cmds.go."
- Account is now the largest single package in the UI tree (~870
  lines model.go + ~295 cmds.go + ~80 msgs.go) but its surface area
  is bounded — folder + outbox + triage + sweep is the full
  responsibility set.
- Reader and compose still don't own their cmds. That gap is the
  next reorg target whenever those subsystems get a feature pass.
