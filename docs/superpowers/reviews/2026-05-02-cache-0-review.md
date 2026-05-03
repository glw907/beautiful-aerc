# Cache 0 Review (2026-05-02)

Independent multi-angle review of
`docs/superpowers/specs/2026-05-02-cache-0-design.md` and
ADRs 0110 / 0111 / 0112. Conducted in Pass 8.4-review per
`docs/superpowers/plans/2026-05-02-cache-0-review.md`.

## Verdict

**revisions-needed.** The architectural skeleton — per-account SQLite,
unified write path through `cache.QueueOp`, FK-CASCADE outbox by
`messages.id` — survives all four review lenses and should not be
rethought. But the spec has multiple load-bearing gaps that would
turn into v1.0-frozen migration debt if implemented as written:

- The UIDVALIDITY re-key contract is unspecified, and at least three
  failure scenarios depend on it being correct.
- "Sync-first" reconnect ordering contradicts RFC 4549 §6.
- JMAP `cannotCalculateChanges` and IMAP forced full-refetch silently
  CASCADE-delete pending outbox rows — silent data loss.
- The JMAP data model (one Email, N mailboxes) doesn't fit the
  spec's `UNIQUE (folder, protocol_id)` schema.
- Auth errors are misclassified as transient and loop forever under
  `max-attempts = 0`.
- The unified-write-path migration (the spec's own §J.1 admission)
  has a real "double-optimistic-state" intermediate window unless a
  strangler-fig order is prescribed.

Pass 8.4-revise should land all the must-fixes below before Pass 8.4a
opens an editor on `internal/cache/`.

---

## Subagent A — Mail-protocol correctness

Stress-tests the spec's schema, sync model, and conflict matrix
against RFC 4549 (CONDSTORE), RFC 7162 (QRESYNC/VANISHED), RFC 3501
§2.3.1.1 (UIDVALIDITY), and RFC 8620/8621 (jmap-core/mail).

Six findings (4 must-fix, 2 should-fix):

### A1. Sync-first ordering contradicts RFC 4549 §6 (must-fix)

Spec §D.3 runs `ChangeTracker.Changes()` *before* draining the
outbox. RFC 4549 §6 ("Processing Offline Queues") explicitly mandates
the opposite: queued client actions first, then pull server-side
changes. The spec's stated rationale (FK CASCADE will tidy up
remote-removed dependencies) doesn't justify the ordering inversion;
the FK CASCADE protection still works under drain-first because
`notFound` from the backend is already a `done` outcome in the
conflict matrix.

**Recommendation:** Reverse the ordering in §D.3 and ADR-0112.
Drain-first, then `Changes()`. Cite RFC 4549 §6 in the ADR.

### A2. `cannotCalculateChanges` has no defined path (must-fix)

JMAP servers return `cannotCalculateChanges` (RFC 8620 §5.2) when the
client's cached state is too old. The spec treats all `Changes()`
errors as transient (`failed` + backoff). Retrying with the same
stale `SyncToken` will loop forever.

**Recommendation:** Define `ErrCannotCalculateChanges` as a sentinel
error; the syncer's response is wipe-folder + reset `sync_token` +
full re-fetch. **But** see A4 / D4 / B6 below — the wipe must remap
or promote pending outbox rows to `conflict`, not silently CASCADE-
delete them.

### A3. `ChangeSet` drops `hasMoreChanges` (must-fix)

JMAP `Email/changes` may paginate; clients must loop on
`hasMoreChanges = true` until exhausted. The current
`ChangeTracker.Changes() (ChangeSet, SyncToken, error)` signature
doesn't expose this.

**Recommendation:** Loop internally inside `mailjmap` —
implementations of `Changes()` must return a complete delta across
all pages before returning. Document the contract in the interface
godoc.

### A4. JMAP Email-in-N-mailboxes vs per-folder rows (must-fix)

`UNIQUE (folder, protocol_id)` allows one JMAP Email id to appear in
multiple folders as multiple rows. JMAP semantics are otherwise: one
Email, one id, an `mailboxIds: Id[Boolean]` set. `Email/set update`
on `mailboxIds` is a "move" without producing a new id, which the
per-folder-row schema can't represent cleanly.

**Recommendation:** For JMAP, treat `protocol_id` as globally unique
per account — one `messages` row per Email, with a primary `folder`
column or a `message_mailboxes` junction table for multi-mailbox
support. Document explicitly in ADR-0110 that the constraint
`UNIQUE (folder, protocol_id)` is correct for IMAP and wrong for
JMAP, and give the JMAP shape.

### A5. NOMODSEQ servers (should-fix)

Spec encodes `SyncToken` as `(uidvalidity, modseq, maxuid)` for IMAP.
Servers without CONDSTORE return `NOMODSEQ`; modseq is unavailable.
Add explicit fallback: full UID+FLAGS listing; encode token as
`(uidvalidity, maxuid)` when NOMODSEQ. Or assert CONDSTORE at
Connect like UIDPLUS and surface a clear error otherwise.

### A6. JMAP `Email/set` partial failures (should-fix)

A single `Email/set` request can return `updated: [M1]` and
`notUpdated: {M1: ...}` for different patches in the same call.
Confirm the JMAP backend sends one object per `Email/set`, or treats
any `notUpdated` entry as a conflict regardless of which patched
property failed.

---

## Subagent B — Source-level prior art

Read K-9/Thunderbird-Android (`MessagingControllerCommands.java`,
`MessagingController.java`, `LocalStore.java`,
`OutboxStateRepository.kt`), Evolution camel-offline
(`camel-offline-folder.c`, `camel-imapx-folder.c`,
`camel-imapx-message-info.c`), Geary (`imap-engine-replay-queue.vala`,
`outbox-folder.vala`), Claws Mail (`imap.c`).

Six findings (1 must-fix, 2 should-fix, 3 nice-to-have).

Confirms meli's position: **poplar would be the first TUI client
with offline triage queueing.** Camel/Geary/Claws all do read-side
cache only; K-9 has a write queue but it's the cautionary case.

### B6. `max-attempts = 0` + permanent errors → infinite loop (must-fix)

Spec §J.4 (open question). K-9 ships `RETRIES_EXCEEDED` as a
distinct terminal state. Move-to-permanently-deleted-destination
where the server returns a 5xx instead of "not found" loops forever
under `max-attempts = 0`.

**Recommendation:** Default `max-attempts = 10`. When
`attempts >= max-attempts > 0`, the drainer transitions `failed →
conflict` with `error.kind = 'max-attempts-exceeded'`. Setting
`max-attempts = 0` (unlimited) remains opt-in.

### B3. Per-op skip semantics not explicit (should-fix)

Drainer query `WHERE status IN ('pending','failed') AND
(last_attempt IS NULL OR last_attempt < now - backoff(attempts))
ORDER BY id` correctly skips a `failed`-but-not-yet-due op and
processes later `pending` ops. K-9 *does* block-on-failure (a real
trap). Make the non-blocking behavior an explicit invariant in §D.3
so a future reviewer can't assume the simpler interpretation.

### B4. Syncer can stomp `ui_flags` on in-flight ops (should-fix)

Camel ships `server_flags`-only (no dual-state). Their model
surfaces a real ordering risk in poplar's spec: when IDLE pushes a
server change for a message that has a `pending` outbox `flag` op,
nothing in the spec prevents the syncer from also updating
`ui_flags` for that row, reverting the optimistic display before
the drainer runs.

**Recommendation:** Add invariant: "The syncer MUST NOT update
`ui_flags` for any message that has a `pending` or `executing`
outbox row. It updates only `flags`. The drainer is responsible for
converging `ui_flags` to `flags` after backend confirmation."

### B5. `crashed-mid-execute` send → `conflict`, not `failed` (should-fix → upgrade)

A `send` op that keeps crashing mid-execute under the current spec
cycles `executing → failed (crashed-mid-execute) → pending →
executing → ...` forever. Network-failed send (transient) →
`failed` is correct. Crashed-mid-send must require explicit user
action — promote to `conflict` so the user sees it in the `!`
overlay.

### B1 / B2 (nice-to-have)

Cite K-9 as a negative example for UID-in-blob alongside Thunderbird
desktop in §D.4. Spec the user-facing string for the
`crashed-mid-execute` Conflicts-overlay message.

---

## Subagent C — Go-architecture fit

Read the existing poplar codebase (`internal/ui/app.go`,
`account_tab.go`, `triage*.go`, `message_list.go`, `viewer.go`,
`internal/mail/backend.go`, `internal/mailjmap/jmap.go`,
`internal/mailimap/imap.go`, `internal/config/`).

Seven findings (3 must-fix, 2 should-fix, 2 nice-to-have).

### C1. Migration intermediate state — double-optimistic-flip (must-fix)

`dispatchTriage` today does (1) mutate `MessageList` in place AND
(2) fire a backend `tea.Cmd`. The spec replaces (2) with `QueueOp`
but doesn't say what happens to (1). If `MessageList.Apply*` stays
in place while `QueueOp` writes `ui_flags`/`ui_hide`, a folder
reload (which now reads from the cache) sees one state; the
in-memory list sees another.

**Recommendation:** Pass 8.4a plan must prescribe a strangler-fig
order: (a) introduce `cache.Account` and SQLite writes first,
keeping `MessageList.Apply*` and the old backend Cmds alive; (b)
switch `SetMessages`/`AppendMessages` to read from
`cache.Account.QueryFolder`; (c) only then delete `MessageList.Apply*`
and old backend Cmds. The window between (b) and (c) is safe
because the cache is the single source of truth.

### C2. `pendingAction.onUndo` closure breaks under cache (must-fix)

Today `onUndo` closes over `m.msglist` and calls `ApplyInsert /
ApplyFlag / ApplySeen`. Once `MessageList` reads from SQLite on
every `SetMessages`, these in-place mutations have no effect. Undo
must fire a compensating `QueueOp` instead.

**Recommendation:** Spec must state that Cache I removes the
`onUndo func()` field from `triageStartedMsg`. Undo fires only the
`inverse tea.Cmd` (which calls `cache.QueueOp` with the reverse
op). `App.pendingAction` retains the timer and `inverse`; the
synchronous-mutation half disappears.

### C3. Drainer→UI notification path unspecified (must-fix)

Drainer goroutine has no `tea.Program` reference. Spec is silent on
how a confirmed op signals the UI to re-render. Two choices: (a) a
channel on `cache.Account` polled by a `pumpCacheCmd` that mirrors
existing `pumpUpdatesCmd`; (b) leak `tea.Program` into the cache
package. (a) is the only option that preserves the layer boundary
in spec §B.1.

**Recommendation:** Spec must prescribe option (a). `cache.Account`
exposes `Events() <-chan CacheEvent`. New `pumpCacheCmd` in
`internal/ui/cmds.go` mirrors `pumpUpdatesCmd`.

### C4. `Op.Args map[string]interface{}` violates go-conventions (should-fix)

Stringly-typed JSON dispatch is the anti-pattern go-conventions
explicitly rejects. The on-disk `args TEXT` JSON encoding is fixed
by v1.0 freeze regardless — a Go sum type at the queue boundary
doesn't change the on-disk format and gives compile-time safety.

**Recommendation:** Replace with sealed sum:
`type OpArgs interface{ opArgs() }` with `MoveArgs`, `FlagArgs`,
`DestroyArgs` (and reserved `SendArgs`, `AppendArgs`). JSON
encode/decode in `QueueOp` and the drainer's dispatch.

### C5. `Op.Folder int64` — name→id translation hidden (should-fix)

UI knows folder names; `Op.Folder int64` is a SQLite row id. Spec
doesn't say where the translation lives. Cleanest: `QueueOp` accepts
folder *names* (canonical strings); the cache resolves to row id
internally in the same transaction. Keeps the UI/cache boundary at
strings, consistent with `mail.Backend`.

### C6 / C7 (nice-to-have)

Add tilde-expansion for `[cache] dir` (no helper exists today —
default to `os.UserCacheDir()`). Add `EROFS`/`ENOSPC` to §I as a
named failure mode.

Clarify the JMAP `Updates()` push-loop role: it nudges the syncer to
call `Changes()`, not apply changes itself. Otherwise push-loop and
poll-`Changes()` may double-process.

---

## Subagent D — Failure-mode adversary

Constructed 10 concrete user-facing failure scenarios. Verdicts: 3
broken, 5 partial, 2 handled.

Headline patterns (high-confidence design gaps because they recur
across multiple scenarios):

### D-pattern-1. UIDVALIDITY re-key is underspecified and load-bearing (must-fix)

Scenarios D1, D2, and D9 all expose the same gap: the spec delegates
to "the IMAP folder sync code" for re-key behavior, but that code's
contract is never defined. Specifically:

- Atomicity boundary: single transaction or chunked? Chunked re-key
  on a 10k-row folder, crashed mid-way, leaves stale `protocol_id`s
  in unprocessed chunks. Drainer reads stale UIDs → silent
  `UID FETCH` on wrong messages.
- Old→new UID mapping: re-key by `UID SEARCH` to build a remap, or
  wipe-and-refetch? If wipe, every outbox row pointing into the
  wiped folder is silently CASCADE-deleted with no notification —
  user's queued moves vanish.
- Mid-drain partial reconnect (D9): IDLE drops + reconnects with new
  UIDVALIDITY while command connection stays live mid-drain. The
  drainer keeps using stale UIDs on the command connection. Spec
  assumes UIDVALIDITY change forces both connections to reconnect;
  invariants don't guarantee this.

**Recommendation:** Define the re-key contract explicitly. Either
(a) re-key is authoritative via `UID SEARCH` to build old→new
mapping inside a single transaction before any CASCADE runs, or (b)
any `messages` row deleted during re-key whose outbox row has
non-`done` status promotes that outbox row to `conflict` with
`error.kind = 'rekey-orphaned'` rather than silently CASCADE-
deleting. UIDVALIDITY change on either connection must fence both
connections until re-key completes.

### D-pattern-2. Forced full-refetch silently destroys pending outbox rows (must-fix)

JMAP `cannotCalculateChanges` (D4) and IMAP wipe-on-UIDVALIDITY-
change (D1) share the same data-loss path: the conflict matrix
treats CASCADE-deleted message rows as "no work," but that's only
correct for *server-driven* removes. A wipe driven by our own
inability to compute changes is not "the message is gone server-
side" — it's "we lost track" — and the user's pending intent should
not be quietly discarded.

**Recommendation:** When `cannotCalculateChanges` (JMAP) or forced
full-refetch (IMAP) drives a wipe, the implementation must attempt
to remap pending outbox rows by matching `messages.protocol_id` after
the re-import before allowing CASCADE. Unremapped pending rows
become `conflict` with `error.kind = 'anchor-lost'`. (Same fix shape
as D-pattern-1 — they should share an implementation path.)

### D-pattern-3. Auth errors misclassified as transient (must-fix)

Scenario D3 (OAuth token expires mid-drain): drainer sees 401, marks
`failed`, backoff, retries with the same expired token, loops
forever. ADR-0108 defers refresh to Pass 9.6. But `max-attempts = 0`
+ no token refresh + no auth-error classification = silent infinite
retry against 401.

**Recommendation:** 4xx authentication errors must be classified
distinctly from network errors. Either promote to `conflict` with
`error.kind = 'auth-failure'` (user-actionable: re-authenticate),
or call `password-cmd` once-per-session on first auth failure
before declaring conflict. Combine with B6: even without a special
auth path, `max-attempts = 10` default would break the loop.

### Other partial scenarios (should-fix)

- **D5** — UI `ResolveConflict` and drainer race on the same row.
  SQLite WAL serializes writes but state-machine transition isn't
  atomic with the drainer's in-progress execution. Add an advisory
  lock or channel-based handoff so `ResolveConflict` signals the
  drainer.
- **D6** — Low disk: eviction runs *after* the insert that pushes
  over `max-size`. SQLITE_FULL fires before eviction can help. Add
  pre-insert disk-space check; trigger eviction at a low-watermark
  on free disk, not just on `max-size`.
- **D8** — GC race with overlay snapshot: `ResolveConflict` on a
  GC'd row silently returns 0 rows affected. Add `RowsAffected == 0`
  guard + define overlay refresh semantics.
- **D10** — Account removed from config while DB has pending ops:
  silent abandonment. Add startup orphaned-DB scan; warn user.

---

## Cross-cutting concerns

Findings raised by ≥2 lenses are highest-confidence:

| Issue | Raised by |
|---|---|
| UIDVALIDITY re-key contract unspecified | A4 (JMAP shape implies it), D1, D2, D9 |
| Forced full-refetch silently destroys pending outbox rows | A2, D4 |
| Sync-first vs drain-first ordering | A1 (vs RFC 4549 §6); B4 partially (syncer-vs-drain coordination) |
| `max-attempts = 0` + permanent errors loops forever | B6, D3 |
| JMAP data model gaps in schema | A3 (pagination), A4 (mailboxIds), C7 (push-loop coordination) |

Note: A1's drain-first conclusion changes the analysis of D9 (mid-
drain partial reconnect). Under drain-first, the drainer completes
all queued ops *before* sync runs, so the IDLE-vs-command ordering
race is narrower — but still real, because the drainer's start
might overlap with an in-flight UIDVALIDITY signal from IDLE.

---

## Prioritized recommendations

### Must-fix-before-implementation

1. **Reverse sync ordering** — drain queue first, then
   `ChangeTracker.Changes()`. Cite RFC 4549 §6 in ADR-0112. (A1)
2. **Define UIDVALIDITY re-key contract** — atomicity boundary (single
   transaction), old→new UID mapping strategy, and mandatory
   promotion-to-`conflict` for any pending outbox row whose
   `messages` row gets dropped during re-key. (A4-related, D1, D2,
   D9, D-pattern-1)
3. **Define `cannotCalculateChanges` / forced full-refetch path** —
   pre-wipe remap by `protocol_id`; unremapped pending rows →
   `conflict` with `error.kind = 'anchor-lost'`. Same implementation
   path as #2. (A2, D4, D-pattern-2)
4. **Fix JMAP data model** — drop `UNIQUE (folder, protocol_id)` for
   JMAP; one row per Email per account, with a primary `folder` (or
   junction table). Spec the JMAP-vs-IMAP shape difference in
   ADR-0110. (A4)
5. **Loop `hasMoreChanges` inside `Changes()` impls** — make the
   contract explicit in the interface godoc. (A3)
6. **Classify auth errors as `conflict`, not `failed`** — `4xx auth`
   errors get `error.kind = 'auth-failure'` and a Conflicts-overlay
   row; do not enter the backoff loop. (D3)
7. **Cap retries by default** — `max-attempts = 10` default;
   `attempts >= max-attempts > 0` promotes `failed → conflict` with
   `error.kind = 'max-attempts-exceeded'`. `max-attempts = 0`
   remains opt-in. (B6, also resolves spec §J.4)
8. **Promote `crashed-mid-execute` send → `conflict`, not `failed`**
   — prevents an infinite executing/failed/pending loop on a
   reproducible crash. (B5)
9. **Spec the migration order for the unified write path** — Pass
   8.4a plan must prescribe the strangler-fig: cache writes →
   cache-backed reads → delete `MessageList.Apply*` and old backend
   Cmds. (C1, resolves spec §J.1)
10. **Eliminate `onUndo` synchronous mutation** — undo fires only
    the `inverse tea.Cmd` (compensating `QueueOp`). Spec the change
    in ADR-0111. (C2)
11. **Spec the drainer→UI notification path** — `cache.Account`
    exposes an `Events()` channel; new `pumpCacheCmd` in
    `internal/ui/cmds.go` mirrors `pumpUpdatesCmd`. No
    `tea.Program` ref in the cache package. (C3)
12. **Add syncer/drainer coordination invariant** — syncer MUST NOT
    update `ui_flags` for any message with a `pending`/`executing`
    outbox row. Drainer is responsible for `ui_flags → flags`
    convergence post-confirmation. (B4)

### Should-fix

13. NOMODSEQ fallback for IMAP `Changes()` — full UID+FLAGS listing,
    token = `(uidvalidity, maxuid)`. Or assert CONDSTORE at Connect
    like UIDPLUS. (A5)
14. JMAP `Email/set` partial-failure handling — confirm one object
    per `Email/set` call, or treat any `notUpdated` entry as
    conflict regardless of which property failed. (A6)
15. Replace `Op.Args map[string]interface{}` with sealed sum
    (`MoveArgs`, `FlagArgs`, `DestroyArgs`, reserved
    `SendArgs`/`AppendArgs`). JSON encoding in `QueueOp` and
    drainer dispatch; on-disk format unchanged. (C4)
16. `QueueOp` accepts folder *names*, not row ids; cache resolves
    name→id internally. (C5)
17. UIDVALIDITY-change fence on either connection (covered by #2).
    (D9)
18. Make per-op skip semantics explicit in spec §D.3 — a `failed`
    op whose backoff hasn't elapsed is skipped by `ORDER BY id`;
    later `pending` ops proceed. (B3)
19. Pre-insert disk-space check + low-watermark eviction trigger.
    Add `EROFS`/`ENOSPC` to §I failure modes. (D6)
20. `ResolveConflict` must check `RowsAffected == 0` and return a
    user-visible error; define overlay refresh semantics. (D8)
21. Startup orphaned-DB scan: warn when a cache DB exists for an
    account no longer in config and has non-`done` outbox rows.
    (D10)
22. Advisory-lock or channel handoff between `ResolveConflict` and
    drainer to make state transitions atomic with execution. (D5)

### Nice-to-have

23. Tilde expansion in `[cache] dir`; default via
    `os.UserCacheDir()`. (C6)
24. Clarify JMAP push-loop role: nudges the syncer to call
    `Changes()`; doesn't apply changes itself. (C7)
25. Cite K-9 as a negative example for UID-in-blob alongside
    Thunderbird desktop in spec §D.4. (B1)
26. Spec the user-facing `crashed-mid-execute` string for the
    Conflicts overlay (Pass 9 input). (B2)
27. Add a "migrating" badge state distinct from "offline (network)"
    in the status bar. (D7)

---

## Hand-off

This review's findings drive Pass 8.4-revise, whose plan already
exists at `docs/superpowers/plans/2026-05-02-cache-0-revise.md`.

The spec stays unchanged in this pass so 8.4-revise produces an
auditable diff. ADRs 0110 / 0111 / 0112 may need supersedes-or-
narrows treatment in 8.4-revise depending on which must-fixes are
adopted as written.

After 8.4-revise, recommend `/ultrareview` on the revised-spec
branch (per STATUS.md notes) before opening Pass 8.4a.
