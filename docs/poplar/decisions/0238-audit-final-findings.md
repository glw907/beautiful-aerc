---
title: Audit Final (Pass 41) findings + Pass 41.1 remediation queue
status: accepted
date: 2026-05-12
---

## Context

Phases A–G of `docs/poplar/audit-plan.md` ran one-per-pass over
Passes 36–40 (ADRs 0221–0233) and returned empty after their
respective remediation sub-passes. Phase Final is the
comprehensive pre-soak sweep — it re-applies every prior lens
plus three not covered upstream: test-infrastructure quality,
security + credential handling, voice + documentation rot.

Pass 41 dispatched four parallel agents covering the three
Final-only lenses + an invariant-drift cross-check between
`docs/poplar/invariants.md` / `.claude/rules/*-invariants.md`
and the code. The Phase A–G re-skim was elided: re-running an
audit lens over code that has not changed since the lens last
ran is exactly the failure mode audit-plan.md "Failure modes for
the audit itself" warns against.

Findings: zero P0, several P1, a small P2 / nit tail. The full
table lives in
`docs/superpowers/archive/plans/2026-05-12-audit-final.md`.

## Decision

Beta soak does not open with Pass 41. Pass 41.1 lands a
remediation batch covering:

1. Security: replace `os.CreateTemp` + follow-up `Chmod` with
   `os.OpenFile(...O_CREATE|O_EXCL, 0o600)` at the three config
   write sites (wizard confirm, repair, discover-folders).
2. Test infrastructure: per-method `*Err error` seams on
   `fakeBackend.Connect`/`QueryFolder`, `fakeBackendWithBody.FetchBody`,
   `mailimap.fakeClient` select/login. One new end-to-end test
   for IMAP cmd-path `ErrAuth` → drainer `OpConflict auth-failure`.
3. Voice: rename `cache.CacheEvent → cache.Event` and
   `compose.CacheStore → compose.Store` (T15 package-doubling);
   em-dash density trim across ADRs 0233–0237; six line-level
   fixes flagged by the codebase-wide voice scan.
4. Doc drift: correct ADR refs at `invariants.md:31`; annotate
   the internal-package list as load-bearing-subset or expand it;
   fix the catkin-invariants.md `muesli/reflow` provenance claim
   (no such dependency; the primitives are clean-room).

Inline in Pass 41 (this consolidation): the two `mailcompose.AssembleMIME`
doc-drift findings — wrong package qualifier and missing
`identities` argument — repaired in `docs/poplar/invariants.md`.

## Consequences

- Pass 41.1 is the soak-gate. Once it lands and a re-skim
  confirms zero P1+, beta soak opens per release-stance.
- The audit method scaled: 4 parallel agents produced
  ~30 findings in roughly 5 minutes of wall-clock per agent.
  Future pre-soak audits should default to that shape rather
  than a serial walk.
- One audit shortcut was taken (no Phase A–G re-skim). The
  rationale is recorded above; if a 41.1 remediation surfaces
  drift that A–G should have caught, the shortcut is wrong and
  the next audit will run the re-skim.
- The codebase-wide voice scan caught two T15 package-doubling
  renames that the per-pass `/simplify` lens does not see
  (it only scans diffs). The Final-only voice lens is therefore
  load-bearing — schedule it for every pre-soak audit, not just
  Pass 41.
