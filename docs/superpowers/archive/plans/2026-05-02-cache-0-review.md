# Plan — Pass 8.4-review — Cache 0 Independent Review

**Pass:** 8.4-review
**Type:** review pass (no code; produces findings doc)
**Inputs:**
- `docs/superpowers/specs/2026-05-02-cache-0-design.md` (spec)
- `docs/poplar/decisions/0110-cache-storage-architecture.md`
- `docs/poplar/decisions/0111-cache-unified-write-path.md`
- `docs/poplar/decisions/0112-outbox-state-machine-and-conflict-policy.md`
- The poplar codebase (for the Go-fit lens)

## Goal

Independent multi-angle review of the Cache 0 spec. Cache 0 becomes
a v1.0-frozen on-disk format (ADR-0105); errors here turn into
multi-year migration debt, so this pass exists to surface them
before any implementation begins.

## Critical context

**Fresh-session rule.** This pass starts from a clean session. Do
NOT load conversation history from Pass 8.4. Read only the spec,
the three ADRs, and the existing poplar codebase. The whole point is
independent review.

**Open questions in spec section J** are explicit asks but are not
exhaustive. Reviewers should not feel anchored to them.

## Approach

### Step 1: Read first

Read the spec end-to-end, then the three ADRs. Note the scope: this
spec covers Cache 0 (design), Cache I/II/III (implementation passes
8.4a/b/c). Each implementation pass gets its own plan; this review
covers what binds across all of them.

### Step 2: Dispatch four parallel review subagents

Use `general-purpose` subagent type. Each gets clean context (no
conversation history). Cap each at ~1500 words. Run in parallel
(single message, multiple Agent tool calls).

**Subagent A — Mail-protocol correctness.**

Brief:
- Read RFC 4549 (CONDSTORE), RFC 7162 (QRESYNC), RFC 3501 §2.3.1.1
  (UIDVALIDITY semantics), the JMAP `Email/changes` and `Email/set`
  specs (jmap-mail RFC 8621).
- Stress-test the spec's schema and conflict matrix against what
  the protocols actually guarantee. Specifically:
  - Does `messages.protocol_id` + `folders.uidvalidity` + outbox
    design correctly handle every UIDVALIDITY-change scenario?
  - Does `ChangeSet{Added, Modified, Removed}` cover what
    `Email/changes` and CONDSTORE actually report (including
    `vanished` from QRESYNC, `notUpdated` / `created` / `updated` /
    `destroyed` partitions from JMAP)?
  - Are there CONDSTORE / QRESYNC modes the spec doesn't account
    for (e.g., NOMODSEQ servers)?
  - Does the conflict matrix's "apply ours" policy survive RFC-
    legitimate server behaviors (e.g., server-rejected MOVE)?
- Output: list of protocol gaps + recommended schema/policy
  changes.

**Subagent B — Source-level prior art (extension of Pass 8.4 reads).**

Brief:
- Pass 8.4 read FairEmail (`EntityOperation.java`,
  `ServiceSend.java`), Mailspring-Sync (`Task.hpp`,
  `TaskProcessor.cpp`), Thunderbird desktop (`nsImapOfflineSync.cpp`,
  `nsIMsgOfflineImapOperation.idl`).
- Read the ones it didn't:
  - K-9's PendingCommand subclasses
    (`legacy/core/src/main/java/com/fsck/k9/controller/MessagingControllerCommands.java`
    in `thunderbird/thunderbird-android`).
  - Evolution's camel-offline (`gnome/evolution-data-server` repo,
    `src/camel/camel-offline-folder.c` and friends).
  - Claws Mail's offline queue (`claws-mail/claws-mail` repo).
  - Anything else the search surfaces.
- Surface anything that contradicts the FairEmail-derived design
  or reveals a failure mode the design doesn't handle.
- Output: list of patterns from new sources + concrete divergences
  from the spec + recommended adjustments.

**Subagent C — Go-architecture fit.**

Brief:
- Read the existing poplar codebase. Start with: `internal/ui/app.go`,
  `internal/ui/account_tab.go`, `internal/ui/triage*.go`,
  `internal/mail/backend.go`, `internal/mailjmap/jmap.go`,
  `internal/mailimap/imap.go`.
- Identify migration risks for the unified write path (ADR-0111):
  which existing call sites need to change, in what order, and
  what intermediate states could cause bugs.
- Identify concrete API design issues with the proposed
  `cache.Cache` / `cache.Account` method set in spec section B.4.
  Are the signatures right? Do they fit how the UI actually
  consumes data today?
- Identify any place the spec hand-waves over a real wiring
  problem (e.g., how does the drainer goroutine signal the UI for
  re-render? how does the cache get notified when the user adds
  an account at runtime — or is runtime add not supported?).
- Identify abstractions that would not survive the test of writing
  the next-pass code.
- Output: ranked list of concrete API/wiring issues + recommended
  spec changes.

**Subagent D — Failure-mode adversary.**

Brief:
- Given the spec, construct ten concrete user-facing failure
  scenarios — specific event sequences with timing and ordering —
  that test the design's invariants. Each scenario should be
  rooted in something a real user would actually do (close laptop
  mid-action, reconnect on a different network, etc.) or a real
  server behavior (rate-limit, disconnect mid-IDLE, UIDVALIDITY
  bump).
- For each scenario, walk through what the design says happens
  step-by-step, and identify any that result in: data loss, hung
  UI, queue corruption, silent state drift, infinite retry loops,
  or user confusion (e.g., "why does this say synced but the
  server doesn't have it?").
- Output: ten scenarios with verdicts (handled / partial / broken)
  and recommended spec changes for the broken ones.

### Step 3: Synthesize

Produce `docs/superpowers/reviews/2026-05-02-cache-0-review.md` with
this structure:

```
# Cache 0 Review (2026-05-02)

## Verdict
greenlit | revisions-needed | scrap-and-rethink

## Subagent A: Mail-protocol correctness
[raw findings]

## Subagent B: Source-level prior art
[raw findings]

## Subagent C: Go-architecture fit
[raw findings]

## Subagent D: Failure-mode adversary
[raw findings]

## Cross-cutting concerns
[overlapping findings = high-confidence issues]

## Prioritized recommendations
### Must-fix-before-implementation
- [item] (raised by: A,C)

### Should-fix
- [item] (raised by: B)

### Nice-to-have
- [item] (raised by: D)
```

### Step 4: Do not modify the spec

Findings drive the 8.4-revise pass. The spec stays unchanged here
so the diff produced by 8.4-revise is auditable.

## Outputs

- `docs/superpowers/reviews/2026-05-02-cache-0-review.md` (commit).
- This plan archived to `docs/superpowers/archive/plans/`.

## Hand-off

Pass-end ritual:
- /simplify N/A (docs-only).
- Update `docs/poplar/invariants.md` decision index only if review
  findings warrant ADR superseding (probably not at this stage —
  ADRs change in 8.4-revise based on findings).
- Update `docs/poplar/STATUS.md`: mark 8.4-review done; current
  pass becomes 8.4-revise (its plan already exists at
  `docs/superpowers/plans/2026-05-02-cache-0-revise.md`).
- Archive this plan.
- Commit + push. No `make install` (no code change).
