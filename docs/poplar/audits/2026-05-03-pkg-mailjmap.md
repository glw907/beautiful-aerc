# Pass 8.5 B — Overengineering Audit: `internal/mailjmap/`

**Date:** 2026-05-03
**Lens:** JMAP backend; provider preset code paths; jmapClient interface.

## Findings

- internal/mailjmap/jmap.go:45 — 3 — `current string` field set in `OpenFolder` and cleared in `Disconnect` but never read in production; only inspected in `jmap_test.go:109`.
  Action: delete
  Rationale: Write-only field carries no information; OpenFolder correctness verifiable by subsequent calls.

- internal/mailjmap/jmap.go:590 — 2 — `firstInReplyTo` has one call site (line 522); two-line body.
  Action: inline
  Rationale: Function adds naming layer over a single slice-index operation.

- internal/mailjmap/errors.go:40 — 2 — `wrapAuth` is a one-line function called exactly once from `classifyErr:32`.
  Action: inline
  Rationale: The `case 401, 403:` arm provides sufficient context.

- internal/mailjmap/errors.go:41 — 2 — `wrapNotFound` is a one-line function called exactly once from `classifyErr:34`.
  Action: inline
  Rationale: Same — `case 404:` provides context.

- internal/mailjmap/jmap.go:864 — 2 — `checkEmailSetCreated` has one call site (line 715, inside `Copy`).
  Action: inline
  Rationale: 12-line body with one caller; should live adjacent to the `Copy` RPC it validates.

- internal/mailjmap/jmap.go:750 — 2 — `checkEmailSetDestroyed` has one call site (line 744, inside `Destroy`).
  Action: inline
  Rationale: Single caller; collapses the `notFound` special-case next to the `Destroy` semantics comment.

- internal/mailjmap/jmap.go:635 — 10 — `Search` comment references "Pass 6 may wire server-side search" but no Pass 6 entry exists in STATUS.md or ROADMAP.
  Action: delete (comment only — stub body must remain to satisfy `mail.Backend`)
  Rationale: Speculative reference without named scheduled pass.
