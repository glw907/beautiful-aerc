---
title: App.Update decomposition into per-domain dispatchers
status: accepted
date: 2026-05-11
---

## Context

`internal/ui/app.go` had grown to 1639 lines with an 877-line
`App.Update`. The file mixed rendering helpers, the keypress
cascade, modal/overlay open-close handlers, compose lifecycle,
outbox queue plumbing, contacts mode, and chrome banner logic in
one type-switch. Per-pass diffs increasingly clustered in this
single file, and review fatigue showed up in the Pass 28 audit.

## Decision

`App.Update`'s outer switch shrinks to two special cases
(`tea.WindowSizeMsg` → `updateSize`, `tea.KeyPressMsg` →
`updateKey`) followed by an ordered chain of per-domain dispatchers,
each returning `(App, tea.Cmd, claimed bool)`:

1. `updateChromeMsg` — toast, error banner, notice, triage start,
   undo, backend update, cache event, folder loaded.
2. `updateOutboxMsg` — depth, summary, conflicts, scheduled,
   cancelled, reschedule, conflict resolution, outbox view msgs,
   `OpenConflictsFromOutboxMsg`.
3. `updateComposeMsg` — `uicompose.*` msgs, draft open/restore,
   schedule accept/cancel with outbox-reschedule precedence.
4. `updateModalsMsg` — confirm yes/no/closed, picker open/close,
   unsubscribe confirm, empty-folder confirm.
5. `updateContactsMsg` — popover, form, contacts-mode toggle,
   sync ticker.

Anything unclaimed falls through to `m.acct.Update(msg) +
deriveChromeFromAcct()`.

The receiver stays `App`; no controller types own state. Files
split by message-package domain: `app_view.go`, `app_keys.go`,
`app_modals.go`, `app_compose.go`, `app_outbox.go`,
`app_contacts.go`, `app_chrome.go`. Each file is under ~310 lines;
`app.go` itself holds the model, constructors, accessors, and the
dispatcher.

Chrome runs first in the chain because `backendUpdateMsg` and
`account.CacheEventMsg` fire on every drainer / idle cycle and
would otherwise walk every other dispatcher.

Two duplications surfaced during the split and were unified:
`armToast(action)` for the three pending-toast call sites
(AttachmentSaved, TriageStarted, SentMsg with hold window), and
`openNewCompose(draft)` for SeededMsg / RestoreFromDraftMsg.

## Consequences

- Each pass touches a smaller, narrower file. The audit fatigue
  signal that triggered the split goes away.
- The `(App, tea.Cmd, bool)` claim-bool contract is new vocabulary
  but mirrors the existing `IsOpen()` cascade in `updateKey`.
- Cross-domain msgs that re-use a shared msg type
  (`uicompose.ScheduleAcceptedMsg` flows to both compose and the
  outbox-side reschedule picker) are encoded as explicit precedence
  inside the owning domain file, not as routing logic in the
  dispatcher.
- Two redundant `acct`-delegate cases (`movepicker.PickedMsg`,
  `account.EmptyFolderConfirmedMsg`) collapsed into the default
  fall-through.
