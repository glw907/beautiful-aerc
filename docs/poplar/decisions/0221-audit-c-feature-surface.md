---
title: Audit C — feature surface
status: accepted
date: 2026-05-13
---

## Context

Pass 36 ran Audit C against the four `docs/poplar/audit-plan.md`
§"Phase C" focuses: OAuth refresh against the #53 lens, mouse
hit-test surface, `v.ReportFocus` resume path, and
`v.ProgressBar` lifecycle. The trigger was the close of Passes
32–35 (mouse, v2 declarative chrome, native OAuth).

Findings live in `docs/superpowers/archive/plans/2026-05-13-audit-c.md`.

## Decision

Two P1 findings queue Pass 36.1 remediation before Audit D:

- **A — SMTP OAuth retry gap.** `mailimap.smtpAuth` calls
  `b.oauth.Token(ctx)` once with no `ForceRefresh`-on-`ErrAuth`
  retry. IMAP `dial` already has the retry; SMTP must mirror it,
  and `Backend.Send` must drop and redial the cached SMTP client
  on `mail.ErrAuth` so a refreshed token reaches the next
  attempt.
- **B — Mouse routes to viewer when compose is open over it.**
  `App.updateMouse` keys on `m.viewerOpen`, but `renderFrame`
  shows compose in the right pane whenever `m.compose != nil`,
  regardless of viewer state. Reply/Reply-all/Forward from inside
  a ready viewer (and `c` from any pane) mount compose without
  closing the viewer. Two valid fixes: close the viewer on
  compose open, or branch `updateMouse` on `m.compose != nil`
  ahead of the viewer cases. The remediation pass chooses one.

Two findings apply inline this pass:

- **C — `BeginSync`/`EndSync` not panic-safe.** The two call
  sites in `internal/ui/account/cmds.go` ran the network call
  between two sequential statements rather than under `defer`.
  Wrapped each in an immediately-invoked func with
  `defer c.EndSync()` so a panic inside `SyncFolders` /
  `SyncFolder` cannot strand `syncInFlight` at >0 and orphan the
  OSC-9;4 indeterminate bar. Scope unchanged: `EndSync` still
  fires before the subsequent `QueryFolder` / `ListFolders` so
  the progress bar accurately tracks the network phase only.
- **D — Audit-plan ReportFocus focus item drift.** The Phase C
  focus list cited a pause/resume contract for JMAP push and
  IMAP IDLE that the codebase does not implement.
  `tea.FocusMsg` / `BlurMsg` only toggle `App.focused` and clear
  the new-mail toast (ADR-0217). Dropped the focus item from
  `docs/poplar/audit-plan.md` §"Phase C".

## Consequences

- Pass 36.1 lands before Audit D (Pass 37).
- The SMTP fix needs a fake `smtpClient` that returns an
  `imap.Error`-shaped auth failure so `classifyErr` routes to
  `mail.ErrAuth`; once landed, sent rows survive transient
  server-side token rotation within the 5-minute access-token
  cushion.
- The mouse fix removes a silent miswire — clicks land on the
  surface the user sees.
- The Begin/Sync defer fix removes a latent OSC-9;4-bar orphan
  hazard. Behavior unchanged on the non-panic path.
- The audit-plan edit prevents a future Audit C from chasing a
  non-existent contract. If poplar adopts blur-driven IDLE/push
  pause in a future pass, the focus item is restored against the
  new behavior.
