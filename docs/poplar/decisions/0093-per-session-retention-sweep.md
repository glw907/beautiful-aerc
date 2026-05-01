---
title: Per-session retention sweep on Disposal folders
status: accepted
date: 2026-05-01
---

## Context

Provider retention policies vary: Gmail Spam auto-purges at 30 days
but Trash is configurable; Fastmail enforces per-account policy.
Users want a tighter, deterministic local cap independent of the
provider — particularly useful when the backend doesn't enforce
server-side retention or when the user wants a strict floor.

## Decision

Two new opt-in `[ui]` config knobs: `trash_retention_days` and
`spam_retention_days`. Both default to 0 (disabled). Clamped on
parse to `[0, 365]` (negative → 0).

When non-zero and the user enters the corresponding Disposal folder
(role: `trash`, `junk`, or `spam`) for the first time in the
session, the sweep dispatches a single `destroyCmd` over messages
loaded by the initial `headersAppliedMsg` whose `SentAt` is older
than `now - retention_days * 24h`. Messages with zero `SentAt` are
skipped (partial sweep is by design — fixtures and unparseable
dates do not block the sweep).

The sweep is keyed off `headersAppliedMsg` (not folder selection)
so the loaded message slice is non-empty when the cutoff is
applied. `AccountTab.swept[folder.Name]` is set on first attempt
regardless of outcome — failures land in the error banner; we do
not retry-loop within a session. `IsSwept(folder)` is exposed as a
test accessor; the UI does not consume it.

## Consequences

- Sweep is silent on success (no toast). The destroyed UIDs flow
  back via `sweepCompletedMsg` and are applied to the visible
  message list via `ApplyDelete`.
- "Per-session" is reset on every poplar launch — no persistent
  state, no on-disk sweep ledger.
- Background or interval-based sweeping is explicitly not done.
  The trigger is human-driven (folder visit) so the sweep cost
  attaches to a load the user already initiated.
