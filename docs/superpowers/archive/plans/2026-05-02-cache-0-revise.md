# Plan — Pass 8.4-revise — Apply Cache 0 Review Findings

**Pass:** 8.4-revise
**Type:** revision pass (edits spec + ADRs; produces revised spec
and the starter brief for Pass 8.4a)
**Inputs:**
- `docs/superpowers/specs/2026-05-02-cache-0-design.md` (current spec)
- `docs/superpowers/reviews/2026-05-02-cache-0-review.md` (findings)
- ADR-0110, ADR-0111, ADR-0112

## Goal

Apply review findings from Pass 8.4-review to the Cache 0 spec and
ADRs. Produce the implementation-ready spec. Write the detailed plan
brief for Pass 8.4a so implementation can begin from a clean session.

## Critical context

Fresh session. Do NOT load conversation history from Pass 8.4 or
8.4-review. Work entirely from the spec, the review findings, and
the ADRs.

## Approach

### Step 1: Triage findings by priority

Read the review's "Prioritized recommendations" section. Group
findings:

- **Must-fix-before-implementation** — apply to spec inline.
- **Should-fix** — apply unless rationale to defer; record any
  deferral as a backlog item via `/log-issue`.
- **Nice-to-have** — backlog only; do not apply to spec.

If the review identified findings the categorization disagrees with
(e.g., a "should-fix" the revise pass thinks is must-fix), promote
or demote with a one-line justification in the revise plan's
output.

### Step 2: Apply must-fix changes to the spec

For each must-fix finding:

- Edit `docs/superpowers/specs/2026-05-02-cache-0-design.md` inline
  (preserve overall structure).
- If the change reverses or substantively modifies one of the three
  ADRs (0110/0111/0112):
  - Mark the affected ADR's frontmatter `status: superseded by NNNN`.
  - Write a new ADR (next available number; check
    `ls docs/poplar/decisions/`) capturing the revised decision.
  - Add a note in the old ADR's Consequences section pointing at
    the new ADR.
- Update the spec's "Status" line at the top from `unreviewed` to
  `reviewed (YYYY-MM-DD)` and add a "Review notes" section near
  the top pointing at:
  - `docs/superpowers/reviews/2026-05-02-cache-0-review.md`
  - Any new ADRs written this pass.

### Step 3: Apply should-fix changes (or defer with backlog entry)

For each should-fix finding:

- Apply to spec, OR
- Defer with a `/log-issue` entry that names the finding and the
  reason for deferral. Record the backlog issue number in the
  spec's "Review notes" section.

### Step 4: Write the Pass 8.4a starter brief

Create `docs/superpowers/plans/<TODAY>-cache-i-implementation.md`
covering:

- **Goal.** Implement Cache I (schema + envelope/header cache +
  per-account SQLite + `mail.ChangeTracker` interface and impls;
  unified write path migration; online-only behavior).
- **Inputs.** Revised spec, ADRs, relevant codebase paths.
- **Approach.** Ordered task list; rough at this stage but specific
  enough that the implementation pass can begin without ambiguity.
  Suggested order:
  1. New `internal/cache/` package skeleton with `Cache`, `Account`
     types and the SQLite handle plumbing.
  2. Schema migration framework (`schema_version` table, ordered
     migration funcs).
  3. Schema 1 = full schema from spec section A.3.
  4. `mail.ChangeTracker` interface in `internal/mail/`.
  5. JMAP `ChangeTracker` impl (`Email/changes`).
  6. IMAP `ChangeTracker` impl (CONDSTORE/QRESYNC fallback).
  7. `mail.Backend` shrink: collapse `MarkRead`/`MarkUnread`/
     `MarkAnswered`/`Delete` into `Flag`. Two backend impls follow.
  8. Cache reads: implement `(*Account).ListFolders`,
     `QueryFolder`, `FetchHeaders`, `FetchBody` (read from cache;
     populate from backend on miss).
  9. Cache writes: implement `(*Account).QueueOp` for kinds
     `move`, `flag`, `destroy`. Drainer goroutine. Per-op
     dispatcher.
  10. Sync goroutine: on connect, run `Changes()` first; then drain
      the queue.
  11. Wire into `App.NewApp` and `AccountTab` — replace direct
      `mail.Backend` references with `*cache.Account`.
  12. Migrate existing triage `tea.Cmd`s to call `cache.QueueOp`
      instead of backend methods.
  13. Tests: unit tests per package; integration test of cache +
      fake backend.
- **Outputs.** New package, schema migration files, modified UI
  wiring, comprehensive tests.
- **Hand-off.** Cache II (Pass 8.4b) takes over — body cache +
  eviction + CLI.
- **Standard pass-end ritual.** /simplify, ADRs (binding facts now
  realized — invariants.md gets prose updates), STATUS.md update,
  archive plan, commit + push + install, tmux verification of UI
  smoke test.

### Step 5: Update STATUS.md

- Mark 8.4-revise done.
- Replace the current "Next starter prompt" pointer to point at
  Pass 8.4a's plan doc (created in step 4).
- The 8.4-review and 8.4-revise rows in the pass table become
  `done`.

### Step 6: Recommend `/ultrareview` to user

In the pass-end commit message, include a line:

> "Recommend running `/ultrareview` on this branch before starting
> Pass 8.4a — the revised spec is the v1.0-frozen contract for
> Cache 0; one more independent review at this point is cheap
> insurance."

## Outputs

- Revised `docs/superpowers/specs/2026-05-02-cache-0-design.md`.
- New ADR(s) if any decision reversed (next available number).
- New `docs/superpowers/plans/<TODAY>-cache-i-implementation.md`
  (the brief for Pass 8.4a).
- This plan (`docs/superpowers/plans/2026-05-02-cache-0-revise.md`)
  archived at pass-end.

## Hand-off

Pass-end ritual:
- /simplify N/A (docs-only).
- Update `docs/poplar/invariants.md` decision index if any new ADRs.
- Update STATUS.md: 8.4-revise done; current pass becomes 8.4a.
- Archive this plan.
- Commit + push. Recommend `/ultrareview` in the commit message.
- No `make install` (no code change).
