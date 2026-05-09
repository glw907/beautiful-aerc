---
title: Background body sync + status indicator (Pass 13)
status: accepted
date: 2026-05-09
---

## Context

Pass 13.1 will deliver search (BACKLOG #38). Search must feel
snappy and operate over locally cached messages. Today the cache
fetches bodies lazily on viewer open — a sample of the active
Fastmail account at design time showed 33,644 messages indexed
(headers) but only 5 bodies cached, because only 5 messages had
ever been opened. Body-text search across that cache would miss
≥99% of mail.

The substrate Pass 13.1 needs is universal local body coverage.
This pass delivers it: a per-account background worker that
fills the body cache newest-first, suspends on user activity,
and surfaces progress in the status bar.

Prior-art surveys drove the design and live at:

- `docs/poplar/research/2026-05-09-mail-client-search-survey.md`
- `docs/poplar/research/2026-05-09-background-body-sync-survey.md`

Key matrix findings:

- **Newest-first ordering is universal.** Thunderbird, K-9,
  Geary, Fastmail, Evolution all walk UIDs descending.
- **Watermark-resumed is universal**, but every implementation
  uses an explicit column or token. Poplar's implicit-via-SQL
  approach (`LEFT JOIN bodies WHERE bytes IS NULL`) is
  functionally equivalent without the column, and self-heals on
  cache eviction.
- **Thunderbird stands alone on user-activity gating**
  (`nsIUserIdleService` → explicit `Pause()` / `Resume()`). For
  a TUI sharing the IMAP cmd connection between user-driven
  fetches and backfill, this pattern transfers directly.
- **Apple Mail's "no in-app gating" path** is a cautionary
  tale — the public signal is users complaining about runaway
  background activity. Poplar lacks the OS-level power
  management that catches Apple's failure mode.
- **Geary's #130 bug** (20 GB runaway sync from an unrespected
  prefetch window) is the headline cost of skipping
  client-side throttle.

The default cache cap (`max-size = 2 GB`) was unmeasured — set
in ADR-0122 without derivation. A typical 30k-message mailbox
runs ~2.5 GB at 75 KB/body; the existing cap would force
eviction churn during initial fill. Thunderbird, Apple Mail,
Outlook, Geary all default to no cap on bodies; the cap is a
power-user opt-in. Pass 13 aligns with that consensus.

## Decision

### Worker

A new file `internal/cache/backfill.go` adds a per-account
background worker:

```go
type Backfiller struct {
    acct    *Account
    backend mail.Backend
    rate    time.Duration  // base interval between batches
    pause   atomic.Bool    // user-activity / connection / cap
}

func (b *Backfiller) Run(ctx context.Context) error
func (b *Backfiller) NotifyActivity()             // tea.KeyMsg hook
func (b *Backfiller) NotifyConnState(s ConnState) // online/offline
```

Lifecycle: `Account.Open` constructs and starts the worker;
`Account.Close` cancels its context. One Backfiller per
`*cache.Account`.

### Work queue

Implicit. Each tick issues:

```sql
SELECT m.uid
FROM messages m
LEFT JOIN bodies b ON b.uid = m.uid
WHERE b.bytes IS NULL
ORDER BY m.sent_at DESC
LIMIT 1
```

State lives in the cache itself. Crash, quit, restart, eviction
— next launch's first tick picks up where work stopped. No
schema migration. No watermark column.

New mail from `pumpUpdatesCmd` arrives via the existing header-
fetch path; backfill picks it up automatically on the next tick
because `sent_at DESC` puts new UIDs at the top.

### Throttle shape

Thunderbird-shape, adapted for poplar:

- **Batch ceiling**: 2 MB cumulative bytes per batch
  (`maxBatchBytes`). After a batch, sleep `rate` (default 500
  ms) before the next.
- **Idle gate**: `lastActivity` timestamp updated on every
  `tea.KeyMsg` via `NotifyActivity`. Worker checks
  `time.Since(lastActivity) >= 5 * time.Second` before each
  fetch; if not, sleep `rate` and re-check.
- **Connection gate**: `NotifyConnState` flips `pause` on
  any non-`Online` state. Worker checks `pause.Load()` before
  each fetch. Reconnect resumes naturally on the next tick.
- **Cache-cap gate**: when `[cache] max-size` is set non-zero,
  worker checks `SELECT SUM(LENGTH(bytes)) FROM bodies` against
  90% of cap before each batch. At-cap → sleep `rate` and
  re-check; eviction is the user's signal to read more or raise
  the cap.

### Server-side back-pressure

`mail.Backend.FetchBody` errors classified as throttle
(`[THROTTLED]` IMAP response, JMAP rate-limit response, HTTP
429) trigger exponential back-off: 1s, 2s, 4s, …, cap 60s.
Identical to the outbox drainer's curve — same helper in
`internal/cache/`. Other errors (NOT FOUND, transient network)
log and skip the UID; the next tick picks up the next eligible
row.

### Configuration

`[cache] max-size = 0` is reinterpreted as **unlimited**
(matrix-aligned default). The eviction logic in `storeBody`
treats 0 as "skip the size short-circuit" — when a user sets a
non-zero cap, behavior is unchanged.

Default in `config.Template()` and the docs example flips from
`2GB` to `0` (with a comment naming the unlimited semantics and
noting it's a guard, not a target).

No new config keys. Backfill enabled implicitly when the
account is open.

### Status indicator

The status bar gains a sibling segment between connection-state
and outbox-depth, hidden when there's no work:

| Tier | Caught up | Active | Paused | Warn |
|---|---|---|---|---|
| ≥90 cells | (hidden) | `↓ 1843/33644` | `↓ paused` | `↓ ⚠` |
| 80–89 cells (Spartan) | (hidden) | `↓` | `↓⏸` | `↓⚠` |

Glyphs use `FgDim` for active/paused (informational), `ColorWarning`
for warn. Numbers come from two `SELECT COUNT(*)` queries against
`messages` and `bodies`, cached for 1 second to avoid per-frame
SQL. Updates piggyback on the existing cache-event tick.

The "Paused" substate is shown when:
- User activity within 5s, OR
- Connection state non-Online, OR
- Cache at ≥90% of explicit cap.

The "Warn" substate is shown when:
- Persistent server back-pressure (≥3 consecutive 429 / `[THROTTLED]`
  responses with backoff at the 60s cap), OR
- Persistent fetch failures (≥10 consecutive non-throttle errors).

### Activity wiring

`App` holds `acct account.Model`, which holds the
`*cache.Account` handle. `account.Model` gains a thin
`NotifyActivity()` accessor that forwards to its cache handle's
Backfiller. `App.Update` calls it on every `tea.KeyMsg` before
delegating to children — one line in the existing switch.

Connection state already flows through `App.connState`; on
state-change, dispatch `NotifyConnState` through the same
account.Model accessor before emitting the existing chrome
notice.

## Consequences

**Unlocks Pass 13.1.** Search-via-cache becomes meaningful
because bodies actually exist locally. FTS5 (planned for 13.1)
can be backfilled from already-stored bodies in one pass.

**Bandwidth visibility.** Users on metered or slow connections
see exactly what poplar is doing via the status segment, and
can disable backfill by setting `max-size` to a small value
(rather than a separate toggle). One knob covers the spectrum
from "lazy" to "full sync."

**Schema is unchanged.** No migration in this pass. The
`bodies` table grows naturally; the size short-circuit in
`storeBody` already enforces caps.

**No new sentinel errors.** Throttle classification reuses
`mail.ErrAuth` / `ErrNotFound` patterns; back-off helper
reused from outbox.

**Apple Mail's failure mode is locked out.** The user-activity
gate plus the explicit cap interpretation prevent runaway
background activity, even on accounts with hundreds of
thousands of messages.

**ADR-0122 is partially superseded.** The 2 GB default → 0
unlimited shift will be recorded in a new ADR at pass-end and
0122 marked `partially superseded`.

**Cost.** One new file (`backfill.go`), one new accessor
(`Account.BackfillProgress`), one App-level wire-up, one
status-bar segment, one config-template tweak, an ADR. Tests
cover the queue query, the throttle decision tree, the segment
formatter, and a fake backend's idle-gate behavior. Roughly 10
tasks within the 8–12 budget.

## Open follow-ups (defer to live use)

- **Idle-threshold tuning.** 5s is a defensible default but
  unmeasured. If real use shows backfill never running (always
  active typing) or too-aggressive interruption (jitter on
  user fetches), revisit in a follow-up patch. The constant
  is one named value in `backfill.go`.
- **Per-folder opt-out.** A user with a 10 GB list-server
  folder might want to exclude it from backfill. YAGNI for
  v1; add via `[ui.folders.<name>] background-body-fetch =
  false` if real demand surfaces.
- **Attachment policy.** Apple Mail offers
  All/Recent/None for attachments. Pass 14's first-run wizard
  is the natural place to surface this; not in Pass 13.
