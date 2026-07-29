**Verdict: ACCEPT_WITH_MINOR** — both fixes are real and revert-sensitive; one new Important defect ships with the second fix, and the answer to Q3 is that the complexity is now accidental.

## Both fixes: ADDRESSED, revert-proven

| Fix | Location | Revert applied (tests untouched) | Result |
|---|---|---|---|
| `landed` separated from `final` | `/home/glw907/Projects/poplar/internal/outbox/dispatch.go:89`, `:105`, `:256`, `:270`, `:322`, `:492`, `:512`, `:764` | `report` line 512 changed back to `landed: o.final` | `TestUnknownKindSurvivesAFinalizeFailure` FAILS: `intent 1 state = dispatching, want queued` |
| Best-effort backoff replay | `/home/glw907/Projects/poplar/internal/outbox/dispatch.go:131-140` | `revert` restored to `return a.apply(tx, now)` | `TestFinalizeRecoveryFallsBackWhenTheBackoffReplayFails` FAILS on both rows: `state = dispatching, want queued` |

`make check` exits 0 at HEAD (a8fafb2). Worktree removed.

## Q1 — every path into and out of `dispatching`

**In:** exactly one writer, `claimRow` (`store.go:89`), inside `claim`'s single transaction. `resolveClaim`'s error rolls the whole claim back, so a partial claim cannot commit.

**Out**, and every row claimed gets exactly one `finalizeAction` (verified: `finalized` guards the outer loop; `stopped`, `preFailed`, batch, and single paths each produce one outcome per claimed id; `dispatchBatch` always returns `len(moves)+1`):
1. finalize tx commits → `deleteRow` / `requeueRow` / `revertRow`.
2. finalize tx fails, recovery commits → non-landed rows to queued (requeue replay, else bare revert).
3. finalize tx fails, recovery skips → `landed` rows stay dispatching. **By design**, `slog.Error` at `dispatch.go:261`, startup sweep behind it.
4. recovery tx also fails → every row stays dispatching. **By design**, `slog.Error` at `dispatch.go:280` naming all ids, sweep behind it.
5. process death or panic between the two commits → rows stay dispatching, **no log line at all** (nothing runs), sweep behind it.
6. `ReclaimOrphaned` (`reclaim.go:36`), unconditional store-wide `UPDATE ... WHERE state='dispatching'`, called at `cmd/poplar/main.go:107` after the instance lock and before any dispatcher.

**Can a row be unreachable for the process lifetime?** Yes, in cases 3, 4, and 5 — all three by design with the sweep behind them, and no row is unreachable across restarts because the sweep is unconditional and unscoped. No accidental strand found. One forward hazard worth recording for the engine task, not a finding here: the sweep is startup-only, so if a future engine loop `recover()`s a panic inside a pass and keeps running, cases 3-5 become permanent for that process with no sweep and, for case 5, no log. If the engine recovers panics, it must re-sweep or exit.

## Q2 — a new jointly-wrong pair, measured

I found one, and it is the second fix itself.

**Shared precondition: a store-write failure that reaches `requeueRow`.** Two individually correct behaviours:
- The recovery replays a requeue as a requeue, so a persistent finalize failure does not spend every pass on another live backend attempt (8c08083's stated purpose).
- Reaching `queued` matters more than the backoff, so the requeue falls back to a bare state write (a8fafb2).

Jointly wrong, because the fallback's trigger condition *is* the harm's condition: the fallback is only reachable when a store write fails, and the no-backoff hot loop only bites when a store write persistently fails. I measured what the fallback leaves on the row (probe test in the scratch worktree, since the shipped test asserts only `state == queued`):

```
state=queued attempts=0 next_attempt_at=now eligible_now=true failure_class=<null>
```

Three consequences, none pinned by a test:
- `next_attempt_at` is already expired → the row is eligible on the **very next pass**, no backoff. This is verbatim the failure `revert`'s own doc comment (`dispatch.go:121-126`) says it exists to prevent.
- `attempt_count` never increments → even a later successful requeue restarts `backoff(0)` at 1s.
- `failure_class` is NULL → `shouldLogFailure` (`dispatch.go:526`) sees `lastClass != class.String()` every pass, so `uerr.New` fires a fresh error every pass forever, defeating ADR-0013 rev 2's construction-is-the-surfacing-event gate.

Also honest about the production reachability: with modernc/sqlite in WAL mode there is no store condition I can name that fails `requeueRow` and permits `revertRow` on the same row, except a `failure_detail` long enough to force an overflow-page allocation under a full disk. The shipped test reaches the branch only with a `CREATE TRIGGER ... RAISE(ABORT)`. So the branch is close to speculative, and it costs a real invariant to hold.

Pairs I tried and could not construct (reporting honestly): recovery 5s bound versus a saturated writer lane (bulk chunks are ~50ms by design under ADR-0003's 50ms ceiling, so 5s is not close); `WithoutCancel` versus shutdown (`enqueue` selects on `w.stop`, so a closed writer fails the recovery cleanly and the sweep covers it); `landed` versus `retry` (landed ⟹ final ⟹ !retry holds at both current setters); `landed` versus `Undo` (a stranded landed row is simply not annihilated, which `Undo`'s contract already returns to the caller).

## Q3 — the complexity is now accidental

`outcome` carries `failed`, `final`, `landed`: eight combinations, four meaningful, and the invariant `landed ⟹ final ⟹ failed` is enforced nowhere — `report` only propagates `o.landed` on the `!retry` branch, so a future setter of `landed` without `final` silently drops it and the recovery requeues a landed row into a duplicate mailbox. The same three-valued fact is then re-encoded as `finalizeVerb` plus a `landed` bool where `landed` only ever co-occurs with `finalizeDelete`.

Simpler structure that satisfies the same invariants, named:
1. **One `disposition` enum end to end** — `delivered | retry | terminal | landed` — replacing `failed/final/landed` on `outcome` and the `landed` bool on `finalizeAction` (a fourth verb, `finalizeDeleteLanded`). `retry := isRetriable(o.class) && !o.final` becomes a switch, the recovery's `if a.landed { continue }` becomes a case, and the unenforced invariant closes by construction.
2. **Delete the fallback.** Make the recovery's requeue write only the non-growing columns — `state`, `attempt_count`, `next_attempt_at`, `failure_class` — and drop `failure_detail`, the one unbounded value in the statement. That removes the branch, its log line, and the finding below, while keeping the backoff and the `shouldLogFailure` gate.
3. **Keep the detached-context recovery.** That one is essential: ctx expiry during the backend calls is the ordinary cause of the finalize failure, so the recovery cannot carry the same context.

Net: `outcome` loses two fields, `finalizeAction` loses one, `revert` loses its branch and its `slog.Warn`.

## Findings

**IMPORTANT — `/home/glw907/Projects/poplar/internal/outbox/dispatch.go:131-140`.** The fallback re-opens the no-backoff replay loop under the only precondition that reaches it, and additionally drops `failure_class`, which re-arms the `uerr` surfacing gate every pass (measured above: `attempts=0`, `next_attempt_at` expired, `failure_class` NULL). Fix: adopt Q3.2 — in the recovery, requeue with `state`, `attempt_count = attempt_count + 1`, `next_attempt_at`, `failure_class`, omitting `failure_detail`, and delete the fallback branch entirely. If the branch is kept instead, it must at minimum preserve `next_attempt_at` and `failure_class`.

**IMPORTANT — `/home/glw907/Projects/poplar/internal/outbox/dispatch.go:137`.** The `slog.Warn` asserts an outcome that has not happened: it is emitted before `revertRow` on line 139, whose error the caller may still surface as "claimed intents stranded in dispatching". Under any store failure that poisons the transaction rather than just the statement, the log claims a row was reverted when it was stranded — a log line that lies about a user-visible outcome, which is exactly what the project's logging seam exists to prevent. Fix: run `revertRow` first and log only on its success, or include the revert result in the line.

**MINOR — `/home/glw907/Projects/poplar/internal/outbox/failure_test.go:395-433.** `TestFinalizeRecoveryFallsBackWhenTheBackoffReplayFails` asserts only `state == queued`. It does not pin what the fallback costs (`attempt_count`, `next_attempt_at`, `failure_class`), and no test covers the new `slog.Warn`. The test therefore passes against an implementation with the Important finding above. Fix: assert the three columns and the warn line.

**MINOR — `/home/glw907/Projects/poplar/internal/outbox/dispatch.go:489-495`.** `failUnresolvedBatch` marks the moves `landed` too, so they also wait for the startup sweep and are then replayed into the duplicate mailbox the create makes. `unresolvedCreate`'s doc (`:749-761`) describes the window for the create and for a *queued* dependent, not for a move stranded in the same batch. Fix: extend that doc paragraph to name the batched moves.

**Design-language and ADR conformance:** clean. The diff touches no UI, theme, catkin, key registry, or import boundary; the store is reached only through `store.Writer`, and both new failure paths reach `slog`.

**Suppression audit:** clean. The diff adds no `//nolint` and no `//poplar:allow-unicode`.

**Rationale:** both claimed fixes are real and revert-proven, but the second one buys a stranded row back at the price of the backoff and the log-suppression gate under the very condition that triggers it, and the `final`/`landed` pair is now two unenforced booleans encoding one four-valued fact.