---
title: Audit A outcome — bug-fix completeness returns 11 non-blocking findings
status: accepted
date: 2026-05-11
---

## Context

`docs/poplar/audit-plan.md` Phase A is the first gate on the path
to beta soak. It checks whether Passes 23–25 (the bug-fix and
small-refactor sweeps that closed #49, #51, #52, #53) left sibling
hazards behind, missed a callsite, or selected a wrong default
silently. The audit is itself a pass — its outcome decides whether
Batch 2 (Passes 27–29) proceeds or queues a 26.1 remediation pass
first.

The three Phase A focuses ran as parallel audit lenses: mail-infra
regression sweep, config-validator completeness walk, and
defensive-clamp grep. Findings are recorded in
`docs/superpowers/archive/plans/2026-05-11-audit-a.md` with file
lists per focus.

## Decision

Phase A returns **0 blocking findings, 11 non-blocking**. Batch 2
proceeds without a remediation sub-pass.

The non-blocking findings collapse into five BACKLOG entries:

- **#54** — mailjmap's `classifyErr` lacks a network-error branch;
  transport drops never wrap as `mail.ErrConnection`.
- **#55** — `mailjmap.refreshFoldersLocked` holds `b.mu` across an
  HTTP round-trip; readers and the push loop stall together.
- **#56** — `mailimap.Destroy`'s Gmail branch leaves `b.current`
  stale after its internal `Select(trash)`; a subsequent redial re-
  Selects the pre-Destroy folder.
- **#57** — config validator gaps: no strict-mode TOML decoding
  (root typo hazard), unvalidated `oauth-store` / `auth` /
  `smtp.auth` enums, weak `contacts.url` empty error, `port = 0`
  silently accepted for bare `provider = "imap"`, contacts
  credentials not validated after parent-account fallback.
- **#58** — defensive-clamp cleanup sweep across
  `internal/cache/` and `internal/ui/status_bar.go`: seven internal-
  to-internal nil-checks and arithmetic clamps that
  `no-defensive-checks` forbids.

One inline-only item (drainer's implicit `ErrConnection` routing
in `cache/drainer.go:executeOne`) gets a comment in the next pass
that touches that file; no BACKLOG entry.

## Consequences

- **Batch 2 unblocked.** Pass 27 (catkin Elm conformance) becomes
  next-up.
- **No 26.1.** The audit record stands; future Phase A re-runs
  compare against today's "11 non-blocking" rather than an empty
  prior.
- **#56, #57, #58 are pre-beta-friendly schema/refactor work** —
  pre-beta rules welcome them as first-class. #54 and #55 sit in
  the JMAP surface and naturally pair with Pass 35 (Native OAuth)
  prep or earlier opportunistic fixes.
- **The audit lens itself proved out.** Three focuses, three
  parallel agents, ~11 findings in one pass with reproducible file
  walks. Phase B.1 / B.2 / C / Final inherit the same shape.
- **No invariants changed.** This is a read-only audit; no
  binding fact in `docs/poplar/invariants.md` shifted.
