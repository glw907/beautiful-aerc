---
title: Undo-send via outbox.scheduled_for + draft-linked rows
status: accepted
date: 2026-05-09
---

## Context

BACKLOG #35 splits across two passes; 10a delivers the headline
short-window undo. The cache outbox already had `next_eligible_at`
for drainer backoff, so we needed a separate user-intent timestamp
that survives transient retries and renders stably in the UI. We
also needed a way to give the user their text back on `u` —
undo without restoration is worse-than-Pine UX.

## Decision

Schema v10 adds `outbox.scheduled_for` (unix nanos, user intent,
mutated only by user action) and `outbox.draft_id` (FK to `drafts`
with `ON DELETE SET NULL`). The drainer pickup gate becomes
`now >= COALESCE(scheduled_for, 0) AND now >= COALESCE(next_eligible_at, 0)`.
`outbox_pickup` is rebuilt with `scheduled_for` as the leading
column so the gate stays index-resolved.

Compose Send persists a drafts row, queues the outbound row(s)
with `scheduled_for` and `draft_id`, and emits
`SentMsg{OpIDs, ScheduledFor, Draft}`. The App reuses the existing
`pendingAction` (`m.toast`) and chrome banner row with an added
1Hz tick that re-renders "Sending in Ns — u undo" until the
window expires. The expiry tick path that already cleared
`pendingAction` for triage now covers send-undo without
modification.

`u` routes by op kind: triage runs the inverse Cmd (existing);
send-undo calls `cache.CancelOps(ctx, opIDs)` and reopens compose
from the in-memory `Draft` carried on `pendingAction`. The drainer
deletes the linked drafts row inside the OpDone transaction on
dispatch success.

`CancelOps` is atomic across one or two rows because IMAP
outbound is two ops (Send + Append-to-Sent) that the user thinks
of as one action. `ErrNotPending` is the sentinel returned when
any named row has already advanced; on that error no row is
deleted. Empty input is a no-op.

`[ui] undo-send-window = "10s"` is the default (Gmail's stock
value, copied by every modern client); range `[0, 5m]`. Zero
disables the hold and dispatches immediately.

## Consequences

- Pass 10b's reschedule and edit-as-draft both build on these
  primitives without further schema work.
- Backoff and user holds are orthogonal — a transient failure
  during a hold can never overwrite the user's intended send
  time.
- The drainer success path now spans two writes (status flip +
  draft delete) and runs inside one transaction;
  non-Send/Append kinds pass empty `draft_id` and pay only one
  extra string compare.
- A crash between Send and dispatch leaves an orphan drafts row
  (the outbox row was inserted with `draft_id` set but the
  drainer never reached OpDone). Pass 10b's reconcile sweep can
  prune these; for 10a the row is harmless and the user can
  delete it from the drafts list.
- `SentMsg.DraftID` is *not* threaded into `pendingAction` in
  10a. The in-memory `Draft` is enough for happy-path restore;
  10b reintroduces a `draftID` seam when `edit-as-draft` and
  reschedule actually consume it.
