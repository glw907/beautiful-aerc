---
title: Schedule send — Ctrl+L picker, sidebar Outbox, reschedule + edit-as-draft
status: accepted
date: 2026-05-09
---

## Context

ADR-0183 landed the schema-v10 foundation (`outbox.scheduled_for`,
`outbox.draft_id`) and short-window undo. BACKLOG #35 part 2 is
the long-form schedule path: a user picking a dispatch time
hours or days out, seeing the queued message in their sidebar,
and being able to cancel, reschedule, or open it back in compose.

The picker shape was the open question. Gmail and Fastmail both
ship a curated preset list ("Tomorrow morning", "Monday
morning", …) plus a free-form fallback. Mobile clients lean on
native datepickers; terminal email has no equivalent. The two
honest paths are a sidebar widget for typing or a third-party
dateparser. Vendoring `araddon/dateparse` (the strongest Go
option) added a dependency for one feature; a hand-rolled parser
covering the calibrated test cases is ~175 lines of pure Go and
no upstream risk.

The sidebar question was where the queue surfaces. Placing it in
the chrome row competes with the toast/error banner; a top-of-
list synthetic entry (visible only when non-empty) hits the same
sightline as Inbox and lets `J/K` navigation reach it.

## Decision

**Compose `Ctrl+L` opens the schedule picker.** Three preset
rows (Tomorrow morning 8 AM, Tomorrow afternoon 1 PM, Monday
morning 8 AM) plus a "Custom…" row that expands a textinput
into the modal body. Custom input passes through
`compose.ParseSchedule` (pure function, takes `now`), which
accepts ISO/US/English dates, time-only strings (rolls to
tomorrow if past), keyword shortcuts (`tomorrow`, `tonight`,
`<weekday>`, `next <weekday>`), and offsets (`+Nm/+Nh/+Nd`).
Year defaults to `now.Year()` and past results roll forward by
one unit. ADR-0076 exempts text-entry surfaces from the
modifier-free rule, so `Ctrl+L` coexists with Catkin's existing
`Ctrl+B/I/K/L/Q/Space`. (`Ctrl+S` was the alternative; reserved
for save-draft.)

The picker's send dispatch threads through the existing
`compose.AssembleMIME` → `cache.Account.QueueOutbound` path,
passing `scheduledFor.UnixNano()` instead of zero. ADR-0183's
undo window is bypassed when `userScheduled != 0` — explicit
user intent supersedes the stock 10-second hold.

**Sidebar grows a synthetic Outbox entry.** When
`Account.OutboxDepth().Pending+Failed+Executing+Conflict > 0`,
a render-time injection in `sidebar.Model.effectiveEntries`
splices a virtual `mail.ClassifiedFolder{Canonical:"Outbox"}`
at the top of the Disposal group with the count rendered as
the unread badge. `mail.CanonicalInbox` and `mail.CanonicalOutbox`
constants seal the canonical strings against typos. `J/K`
selection of Outbox routes to the new outbox view in place of
the message list; closing returns to the previous folder.

**Outbox view is a separate `internal/ui/outbox` subpackage.**
Read-only list of `cache.OutboxRow` rows joined to `drafts`
via `outbox.draft_id` (`cache.OutboxScheduled`). Bindings are
`j/k` (cursor), `c` (cancel — `cache.CancelOps`), `s`
(reschedule — opens the schedule picker pre-filled, then
`cache.RescheduleOp`), `e` (edit-as-draft — cancels the op
and reopens compose seeded from the linked draft;
inert when `Draft == nil`), and `Esc/q` (close to previous
folder). The picker is App-owned in this mode (separate from
compose's instance) so the App can route `ScheduleAcceptedMsg`
to `RescheduleOp` instead of `dispatchSend`.

**Cache surface adds two methods.**
`cache.RescheduleOp(ctx, opID, newScheduledFor int64) error`
updates `outbox.scheduled_for` iff the row is `OpPending` and
`scheduled_for > now` (otherwise `ErrNotPending`). The drainer
gate is unchanged from ADR-0183. `cache.OutboxScheduled(ctx)`
returns `[]OutboxRow` joined left to `folders` and `drafts`,
status IN (pending, failed), ordered by `scheduled_for ASC, id
ASC` with NULL last. Subject is decoded from the first 4 KB of
`outbox.payload` via `net/textproto.ReadMIMEHeader`; `To`
addresses come from `SendArgs.Envelope.Rcpts` in `args` JSON.
No MIME parser dependency — the extractor is 12 lines.

The picker's `SchedulePicker` is a value-type sub-model with
preset rows + an inline `bubbles/textinput` for the custom row.
ModalShell-framed; `j/k` or `↑/↓` navigates; Enter on a preset
emits `ScheduleAcceptedMsg{When}`; Enter on Custom expands the
input; Enter again parses and emits the same Msg with the
parsed time, or stores a parse-error string for inline
display. ADR-0163 per-subpackage `Styles` adds a `PickerError`
slot.

## Consequences

- Schema is unchanged from ADR-0183. The whole feature lands
  on top of `outbox.scheduled_for` + `outbox.draft_id`.
- `mail.CanonicalInbox` / `CanonicalOutbox` constants are
  precedent for additional canonical-folder string sealing
  later; this pass touches only the Pass-10b literals.
- The sidebar's `effectiveEntries` allocates a new slice when
  Outbox is visible. Caching it under a dirty flag is a
  measurable-but-tiny pre-1.0 refactor; skipped this pass on a
  premature-optimization rationale (the `/simplify` reviewer
  rated the cost "negligible in practice").
- The hand-rolled date parser commits poplar to a finite
  vocabulary; broader natural-language ("end of next week",
  "in two business days") is out of scope. Adding cases means
  appending to `dateLayouts` or extending `parseKeyword` — the
  test table is the contract.
- Edit-as-draft cancels the op and opens compose; if the cancel
  races the drainer (the row already advanced), the user sees
  `ErrNotPending` from `CancelOps` and the message stays sent.
  This is acceptable: the user clicked "edit" but their message
  was already on the wire.
- App grew a `pendingReschedule{picker, opID}` field that owns
  the picker overlay during reschedule, parallel to compose's
  picker for new sends. The two never overlap because the
  outbox view replaces the message-list pane (compose isn't
  open while in outbox).
- The synthetic Outbox entry uses `icons.Sent` as a stand-in;
  a dedicated icon is post-1.0.
