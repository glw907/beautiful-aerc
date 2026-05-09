# Undo Send — Design (Pass 10a)

Pass 10a delivers the headline half of BACKLOG #35: a short
configurable window between **Send** and dispatch during which the
user can press `u` to take it back. Schedule-send (the long-window
variant, plus the sidebar Outbox folder and picker modal) splits
into Pass 10b on top of the same `scheduled_for` foundation laid
here.

## Scope

In: schema column for user-scheduled dispatch time, drainer pickup
gate, compose Send wiring through a configurable undo window, status-
bar countdown, the `u` undo binding, and Draft persistence so that
undo restores the user's text in compose.

Out: schedule-send picker modal, sidebar virtual Outbox folder,
outbox list view, reschedule, edit-as-draft (the foundation lands
here; the user-facing `e` action lives in 10b).

## Settled decisions

- **Two-column schema.** `outbox.scheduled_for` is user intent
  (set at queue time, mutated only by user action). `next_eligible_at`
  stays drainer-managed for transient retry backoff. The drainer's
  pickup gate becomes
  `now >= COALESCE(scheduled_for, 0) AND now >= COALESCE(next_eligible_at, 0)`.
  A backoff retry can never overwrite the user's intended send time;
  the UI can render "scheduled for 3:00 PM" stably while the drainer
  retries underneath.

- **Undo and schedule are one mechanism.** A 10-second undo hold and
  a Tuesday-9am schedule are the same row shape — only the source of
  the timestamp differs. 10a sets the timestamp from
  `now + undo_window`; 10b sets it from a picker.

- **Send persists the Draft until dispatch.** On Send, compose writes
  the Draft to the `drafts` table (Pass 9h shape) and the new outbox
  row carries `draft_id` as a nullable FK. The drainer deletes the
  Draft row on success; `DiscardOp` leaves it. On `u`-undo, App opens
  compose seeded from the still-live Draft. This both restores the
  user's text on undo and gives 10b's edit-as-draft a join-query
  implementation with no MIME→Draft parser anywhere in the codebase.

- **Visibility reuses the existing chrome banner row.** Triage already
  has a `pendingAction` + `tea.Tick` + `u` undo + chrome banner
  toast pattern. Send-undo is the same shape with a longer default
  window and a per-second countdown render. One row above the status
  bar shows `Sending in 8s — u undo`. The sidebar Outbox virtual
  folder is a 10b concern.

- **Undo window default 10s, configurable via `[ui] undo-send-window`.**
  Range 0..5m. Zero disables the hold (immediate dispatch, no `u`
  affordance). 10s matches Gmail's default and what every modern
  client copied.

## Architecture

Three boundaries, no new packages.

### Cache (`internal/cache`)

Schema v10 migration:

```sql
ALTER TABLE outbox ADD COLUMN scheduled_for INTEGER NULL;
ALTER TABLE outbox ADD COLUMN draft_id TEXT NULL
    REFERENCES drafts(draft_id) ON DELETE SET NULL;
DROP INDEX outbox_pickup;
CREATE INDEX outbox_pickup ON outbox(scheduled_for, next_eligible_at, id)
    WHERE status IN ('pending', 'failed');
```

`scheduled_for` is Unix nanoseconds, matching `next_eligible_at`,
`enqueued_at`, and `last_attempt` on the same table. `draft_id` mirrors the `drafts`
table's TEXT primary key. `ON DELETE SET NULL` so a manual draft
delete never breaks an in-flight outbox row.

Drainer pickup query gains one clause:
`now >= COALESCE(scheduled_for, 0) AND now >= COALESCE(next_eligible_at, 0)`.
Drainer success path adds one delete:
`DELETE FROM drafts WHERE draft_id = ?` (no-op when `draft_id` is
NULL). Both run inside the existing op-completion transaction.

`QueueOutbound` signature changes:

```go
func (a *Account) QueueOutbound(
    ctx context.Context,
    sentFolder string,
    env mail.Envelope,
    mime []byte,
    scheduledFor time.Time,
    draftID string,  // empty string = no linked draft
) ([]int64, error)
```

Returns the one (JMAP) or two (IMAP Send + Append-to-Sent) op IDs.
The same `scheduled_for` and `draft_id` apply to every row in the
group. Zero-value `scheduledFor` means "no hold" — drainer picks up
immediately. Empty `draftID` skips the FK. Existing test call sites
of `QueueSend`/`QueueAppend` pass zero/empty.

`CancelOps(ctx, opIDs []int64)` atomically deletes all rows if all
are `OpPending`; returns `ErrNotPending` if any has advanced. The `u`
binding passes the slice from `QueueOutbound`.

### UI (`internal/ui`)

The existing `App.pendingAction` is extended to hold a send-undo
state. The chrome banner row above the status bar already renders
toasts; the same code path renders the per-second countdown:

```go
type pendingAction struct {
    op       string         // existing; "" when none
    inverse  tea.Cmd        // existing; triage compensating op
    expires  time.Time      // existing; tea.Tick fires expiry

    // 10a additions:
    sendOpIDs   []int64        // non-nil iff op == "send-undo"
    sendDraftID string          // linked Draft for compose-restore
}
```

When `op == "send-undo"`, the toast renders `Sending in Ns — u undo`
and a 1Hz `tea.Tick` re-renders each second. Triage's existing
single-shot expiry tick still fires for the natural-window case.
The `u` binding's existing handler routes by `op`: triage runs
`inverse`; send-undo calls `cache.CancelOps(ctx, sendOpIDs)` and
opens compose seeded from `cache.LoadDraft(sendDraftID)`. On expiry
or `CacheEvent` showing any row in `sendOpIDs` left `pending`,
`pendingAction` clears.

Single in-flight send-undo at a time. Compose closes on Send and
can't open again until the App is idle, so the user can't queue a
second send while one is held. (Pass 9h compose lifecycle invariant.)
Long undo windows (`60s < undo_window <= 5m`) display the same
per-second countdown.

### Config (`internal/config`)

`UIConfig.UndoSendWindow time.Duration` with TOML key
`undo-send-window`. Decoded by the existing duration helper.
Validation: `>= 0` and `<= 5 * time.Minute`. Default `10 * time.Second`
when the key is absent.

### Compose

On Send (existing `Ctrl+X` path in `composeSendCmd`):

1. Persist current Draft to `drafts` table via `cache.CreateDraft`
   (one row per send; the draft_id is generated here — distinct from
   any pre-existing 9h draft buffer).
2. Compute `scheduledFor = time.Now().Add(undoWindow)`.
3. Call `QueueOutbound(ctx, sentFolder, env, mime, scheduledFor, draftID)`.
4. Emit `compose.SentMsg{OpIDs []int64, ScheduledFor time.Time, DraftID string}`.

App handles `SentMsg` by closing compose and arming `pendingAction`
(`op = "send-undo"`, `sendOpIDs`, `sendDraftID`, `expires =
ScheduledFor`). On `u`, App calls `cache.CancelOps(opIDs)` and
reopens compose seeded from `cache.LoadDraft(draftID)`.

When `undo-send-window = 0`, step 2 sets `scheduledFor = time.Time{}`
(zero value), `SentMsg.OpIDs` is still populated, but the App skips
arming since `expires <= now`. Draft persistence still runs so a
crash between Send and dispatch doesn't lose the text; drainer's
success path deletes the Draft.

## Data flow

### Send → undo → restore

```
Compose Ctrl+X
  ├─ cache.CreateDraft(draft_id, payload)
  ├─ cache.QueueOutbound(..., scheduledFor=now+10s, draftID)
  │    └─ INSERT 1-2 outbox rows (status=pending, scheduled_for=t, draft_id)
  └─ App closes compose, arms pendingAction (op="send-undo")

Drainer wakes, gate not satisfied → sleeps until t

App ticks 1Hz → StatusBar.SetUndoCountdown(remaining, opID)

User presses u  (within window)
  ├─ cache.CancelOps(opIDs) → DELETE outbox rows
  ├─ Draft row remains (FK ON DELETE SET NULL would have handled
  │   the inverse; here the outbox rows go away cleanly)
  └─ App opens compose seeded from cache.LoadDraft(draftID)
```

### Send → no undo → dispatch

```
At t = scheduled_for, drainer claims row (status=executing).
Backend.Send → Backend.Append.
On success, drainer transaction:
  ├─ UPDATE outbox SET status='done' WHERE id=?
  └─ DELETE FROM drafts WHERE draft_id=?
Status bar clears via the existing CacheEvent → SetOutboxDepth path.
```

### Backoff during undo hold

```
Highly unlikely (the row hasn't been claimed yet during the undo
window), but if some pre-claim error somehow surfaces: row goes to
status=failed with next_eligible_at = backoff_t. Pickup gate is
"now >= max(scheduled_for, next_eligible_at)" — backoff stacks
underneath the user's hold without overwriting it. UI still shows
the user's intended send time on the row.
```

## Error handling

- **Drainer dispatch fails after `scheduled_for`.** Existing buckets
  apply unchanged: transient → `OpFailed` + `next_eligible_at`
  backoff; auth → `OpConflict auth-failure`; permanent → `OpConflict`.
  Conflict overlay surfaces it. Draft row stays alive (drainer only
  deletes on success), so the user can retry-as-draft via the
  existing conflict-overlay flow once that lands in 10b.
- **Crash during `OpExecuting`.** Existing recovery rule unchanged:
  `Send` → `OpConflict crashed-mid-execute`. Draft row stays.
- **Crash during the undo hold (before `OpExecuting`).** Row is still
  `pending` after restart. Drainer evaluates the gate; if `now >=
  scheduled_for`, dispatches immediately. Effectively "the undo
  window is over" — matches Gmail's behavior on tab close.
- **Cancel semantics for pending rows.** `DiscardOp` today rejects
  non-conflict rows with `ErrNotConflict`. The undo path needs to
  cancel `pending` rows, which doesn't fit. We add `(*Account).
  CancelOps(ctx, opIDs []int64) error` — accepts only when *all*
  named rows are `OpPending`, deletes them in one tx, signals the
  drainer. Returns `ErrNotPending` if any row has advanced;
  `OpConflict` rows still flow through `DiscardOp`. The chrome
  banner clears when `pendingAction` expires or when a `CacheEvent`
  reports a `sendOpIDs` row left pending. Race: the user presses `u`
  in the same tick the drainer claims one of the rows; `CancelOps`
  returns `ErrNotPending`, App emits a transient `ErrorMsg{Op: "undo
  send", Err: ...}`. Acceptable; the window is the contract, not a
  guarantee.
- **`undo-send-window` out of range.** `config.LoadUI` returns a
  validation error at startup with the offending value, same shape
  as other UI validation errors.
- **Draft persistence fails.** Send fails atomically — no outbox
  row, no draft row. Compose stays open with an error toast. No
  partial state.

## Testing

- **Cache unit tests** (table-driven, no mocks):
  - schema v10 migration adds both columns + rebuilds the index;
    backward-compatible read of v9 rows.
  - pickup gate respects `scheduled_for` (row not picked before t).
  - `QueueSend` with `scheduledFor`/`draftID` round-trips.
  - drainer success deletes the linked draft row in the same tx.
  - `CancelOps` on rows with `draft_id` leaves the draft row.
  - co-existence of `scheduled_for` and `next_eligible_at`.
- **Drainer test**: row with `scheduled_for = now + 100ms` is not
  picked up before that time, is after.
- **Config test**: `undo-send-window` parsed from TOML; range
  validation; default applied when absent.
- **Compose test**: Send path persists Draft, sets `scheduledFor`,
  emits `SentMsg`.
- **UI test**: status-bar countdown formatting at 10s/1s/0s;
  `u` keybinding registered/unregistered with arm state.
- **Live tmux capture** at 80×24 and 120×40: status-bar countdown
  visible during a 10s window; undo restores compose with the
  draft text intact.

## Pass-size budget

8 tasks, comfortably under the 12-task cap:

1. Schema v10 migration + tests (`scheduled_for` + `draft_id` FK +
   index rebuild).
2. Drainer pickup gate update; drainer success deletes linked
   draft; tests.
3. `QueueSend` signature change + call-site updates.
4. `UIConfig.UndoSendWindow` parse/validate/default.
5. Compose Send wires Draft persist + `scheduledFor`.
6. Status-bar countdown + tick + `SentMsg` handling.
7. Global `u` discard + compose-from-Draft restore.
8. Live tmux verification + ADR + invariants + STATUS update.

## ADRs to write at pass-end

- ADR-0183 (or next free) — outbox `scheduled_for` and Draft-linked
  outbox rows: schema rationale, drainer gate, the
  user-intent-vs-drainer-state separation. Names Pass 10b as the
  consumer of the foundation primitives that don't fully land here
  (just the FK + the gate; no `RescheduleOp` yet — 10b adds it
  with its consumer).

## Out of scope, follow-ups

- Pass 10b — schedule-send picker, sidebar virtual Outbox folder,
  outbox list view, `RescheduleOp`, `OutboxScheduled`, edit-as-draft
  user-facing key.
- BACKLOG entry to file at 10a pass-end — not "edit-as-draft"
  (covered by 10b) but a tracker for any post-10a polish surfaced
  during live tmux verification.
