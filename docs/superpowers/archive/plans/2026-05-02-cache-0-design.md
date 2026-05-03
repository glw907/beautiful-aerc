# Plan — Pass 8.4 — Cache 0 Design

**Pass:** 8.4
**Date:** 2026-05-02
**Type:** design pass (no code)
**Output:** spec, plan, ADRs

## Goal

Settle the architecture, schema, and behavioral invariants of
poplar's local mail cache. No implementation; the next four passes
(8.4-review, 8.4-revise, 8.4a/b/c) execute against what this pass
produces.

## Approach

Cache 0 becomes a v1.0-frozen on-disk format per ADR-0105. To raise
confidence on a critical architectural underpinning, this pass
splits design from validation:

1. **8.4 (this pass)** — design only. Brainstorm open questions,
   pressure-test against source-level prior art, write spec + ADRs.
2. **8.4-review** — independent multi-angle review of the spec from
   a fresh session.
3. **8.4-revise** — apply review findings to the spec.
4. **`/ultrareview`** — user-triggered final gate (post-revise).
5. **8.4a / 8.4b / 8.4c** — implementation passes.

## What this pass executed

1. Brainstormed the five open questions in STATUS.md's Pass 8.4
   starter prompt:
   - Storage: SQLite per account (vs BoltDB / FS).
   - Cache shape: separate from `mail.Backend`; UI talks to cache.
   - `ChangeTracker`: sibling interface to `Backend`, not folded in.
   - Eviction: bodies only (LRU + size + age); headers preserved.
   - Offline: read-only AND queued triage in Cache III.
2. Surveyed mail-client offline-queue prior art at the docs level
   (Thunderbird, K-9, FairEmail, Mailspring, Outlook, offlineimap,
   mbsync, mutt, aerc, alpine, meli, himalaya).
3. Read source code for FairEmail (`EntityOperation.java`,
   `ServiceSend.java`), Mailspring-Sync (`Task.hpp`,
   `TaskProcessor.cpp`), and Thunderbird desktop
   (`nsIMsgOfflineImapOperation.idl`, `nsImapOfflineSync.cpp`).
   Confirmed meli + himalaya have no offline-write queue (poplar
   would be the first TUI mail client with offline triage queueing).
4. Synthesized findings: queue refs message-row id (FairEmail
   pattern), unified write path (Mailspring pattern), enqueue-order
   replay with no coalescing (Mailspring), apply-ours conflicts
   (OSS majority), `executing` state for crash recovery (FairEmail).
5. Wrote spec (`docs/superpowers/specs/2026-05-02-cache-0-design.md`).
6. Wrote three ADRs:
   - **ADR-0110** — storage architecture (SQLite per account).
   - **ADR-0111** — unified write path (`cache.QueueOp` as single
     action API; `ui_flags` / `ui_hide` columns for optimistic UI).
   - **ADR-0112** — outbox state machine + conflict policy.

## Open questions deferred to review pass

Captured in spec section J for review-pass attention:

1. Unified write path migration cost.
2. `ui_flags` + `ui_hide` split necessity.
3. No drain-time coalescing — confirm dropping doesn't paint JMAP
   backend into a corner.
4. `max-attempts = 0` (retry forever) vs. cap-and-conflict.
5. `!` and `Q` overlay keybindings — discoverability and collision
   with vim conventions.

## Hand-off

Pass-end ritual writes:

- Spec, plan, ADRs (above) committed.
- `docs/poplar/invariants.md` updated with cache-architecture
  binding facts.
- STATUS.md marked 8.4 done; 8.4-review queued with detailed starter
  prompt; 8.4-revise queued; 8.4a/b/c blocked behind 8.4-revise.
- Plan archived to `docs/superpowers/archive/plans/`.
- No code changes, so `/simplify` skipped; `make install` is a
  no-op; commit + push.
