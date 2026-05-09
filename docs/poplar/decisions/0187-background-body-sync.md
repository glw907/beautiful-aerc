---
title: Background body sync + status indicator
status: accepted
date: 2026-05-09
---

## Context

Body fetch was lazy-only: a message body landed in the cache the
first time the user opened it. That shape was fine for the
viewer but blocks Pass 13.1 search (#38), which needs the body
text indexed before the user types a query. A search that has
to fetch each candidate body over the network is unusable.

The matrix split cleanly. Thunderbird's `nsAutoSyncManager`
runs a per-account background fetcher with a 2 MB batch ceiling,
timer-slack between batches, and an idle gate driven by user
input events. K-9 holds an explicit watermark UID per folder
and walks backward. Geary and Evolution prefetch headers
eagerly and bodies opportunistically. aerc stays demand-only.
Fastmail's JMAP server tracks its own modseq — clients can ask
for "everything since X" — but the local cache still has to
write the bytes.

Settled in brainstorm before the pass: newest-first ordering
(matches user expectation that recent mail is the search hit
most often); implicit SQL queue (no watermark column —
`LEFT JOIN bodies WHERE bytes IS NULL ORDER BY sent_at DESC`
already names the work); Thunderbird-shape throttle (idle gate
+ batch ceiling, not a token bucket); status-bar sibling
segment for progress (not a glyph overload on the connection
icon); `[cache] max-size = 0` reinterpreted as unlimited
(matrix-aligned default — Thunderbird, Geary, Evolution all
default to no cap).

## Decision

Pass 13 lands a per-account `Backfiller` worker in
`internal/cache/backfill.go`. One goroutine per `*cache.Account`,
constructed in `Open` and stopped by canceling the context
threaded into `Run`. The work queue is implicit:
`SELECT m.protocol_id FROM messages m LEFT JOIN bodies b ON
b.message = m.id WHERE b.bytes IS NULL ORDER BY m.sent_at DESC
LIMIT 1`. Each tick (500ms) checks gates, then drains a batch
up to 2 MB before sleeping. Gates: `connOnline` (Backfiller
pauses on `Reconnecting` and `Offline`); `idle` (5s threshold
on `lastActivity` driven by `tea.KeyMsg`); `atCap` (90% of
`acct.maxSize`, skipped when `maxSize <= 0`).

Server back-pressure (IMAP `[THROTTLED]`, JMAP rate-limit, HTTP
429) is detected by substring on `err.Error()` — the underlying
libraries surface these as opaque strings without typed
sentinels at this layer. Throttle hits route through the
shared `internal/backoff.Exponential` helper with `initial=1s,
max=60s`, mirroring the outbox drainer's curve. A successful
fetch resets `throttleAttempts`.

`[cache] max-size` default flips from 2 GB to 0 (unlimited).
The runtime gate in `bodies.go:storeBody` already treats
`a.maxSize > 0` as "cap enabled" — only the config default + the
template comment changed.

Status bar grows a sibling segment between the outbox depth
chunk and the connection indicator: `↓ N/M` in the Full and
Intermediate tiers (width >= 90), bare `↓` glyph in the Spartan
tier (width < 90). Substates: `↓ paused` / `↓⏸` while offline or
mid-activity; `↓ ⚠` / `↓⚠` reserved for persistent throttle (the
warn signal is wired through the API but always passed `false`
in Pass 13 — Pass 13.1 lights it up alongside the search
surface, when the cache→UI signaling shape for throttle state is
designed holistically).

App wires `m.acct.NotifyActivity()` at the top of every
`tea.KeyMsg` and `m.acct.NotifyConnState(cs == Connected)` after
`SetConnectionState`. `account.CacheEventMsg` and
`backendUpdateMsg` both call `App.refreshBackfillSegment()`,
which queries `BackfillProgress`, derives `paused` from the
status bar's connection state, and short-circuits when nothing
changed (so the COUNT queries don't fire on every drainer event
once the segment is steady).

## Consequences

**ADR-0122 partially superseded.** That ADR established the
2 GB body-cache default. The cap mechanism stays; only the
default value changes. Users who want a cap opt in via
`[cache] max-size = "5GB"` in `config.toml`.

Bodies populate ahead of demand. By the time Pass 13.1 lands
search, the FTS5 index can read directly from local rows for
the entire mailbox newest-first, with the tail filling as
sync continues. The user sees `↓ N/M` ticking down during the
initial sync after a fresh install or after pulling a new
account.

The 500ms blind-tick when caught up costs two atomic reads + one
indexed SQL query per second. Detectable on a profiler, not
user-perceptible. An adaptive ticker (5s when caught up, 500ms
when work is pending) is a future cleanup if power becomes a
concern; no measurement justifies it today.

`atCap` re-runs `SUM(LENGTH(bytes))` per fetch when `maxSize >
0`. With the new default of 0 the query is skipped entirely. If
opt-in caps see wide use, the right fix is a rolling counter
maintained by `storeBody` — not blocking on it for the substrate
pass.

The status-bar segment was unit-tested at both tiers (8 cases
in `TestStatusBar_BackfillSegment`, covering hidden / active /
paused / warn × full / spartan). No live tmux capture during
the consolidation pass: the segment only renders when there is
real cache+sync activity, which Pass 13.1 will exercise during
search. The unit-test coverage is the substrate guarantee.

Idiomatic-bubbletea check (§10): the new segment is a pure
`View()` formatter, no state mutation, no I/O. Width math uses
the existing `lipgloss.Width` infrastructure already in `View`.
No new key bindings, no `WindowSizeMsg` plumbing changes. The
App's `refreshBackfillSegment` reads cache state via the same
`Cache().BackfillProgress` accessor used by tests. Conformant.
