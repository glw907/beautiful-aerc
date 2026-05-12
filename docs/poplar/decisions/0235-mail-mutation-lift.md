---
title: internal/mail mutation-efficacy lift past 80%
status: accepted
date: 2026-05-12
---

## Context

Pass 40.3 (ADR-0234) calibrated `make check-deep` against the
post-40.1 baseline and parked `internal/mail` at 69.23% efficacy
with a 64% floor. The package was the queue item for a
follow-up pass. The surviving mutants were `mock.go:113`
(`offset >= total` boundary), `mock.go:117` (`end > total`
clamping boundary), and two capacity-expression mutants on
`mock.go:120`. Twenty more mutants sat un-covered, spread across
`backend.go`, `classify.go`, `probe.go`, `types.go`, and the
`mockBodies` literal table.

## Decision

`internal/mail` efficacy is lifted to **94.44%** via test
additions in `backend_test.go`, `classify_test.go`,
`probe_test.go`, `types_test.go`, and `mock_test.go`. The
`scripts/check-deep.sh` floor moves from 64 to 89 (observed minus
the standard 5pp buffer).

Two surviving mutants are accepted as **equivalent mutants**:
mathematically distinct under syntactic mutation but
behaviorally identical to the original code, so no test can
kill them without rewriting the source to a worse shape.

- `mock.go:117:9` — `end > total` ↔ `end >= total` in the clamp
  `if end > total { end = total }`. At the boundary
  `end == total`, both predicates assign `end = total`, which it
  already is.
- `probe.go:28:29` — `iota + 1` arithmetic on the `ProbeStatus`
  constants. Any base shift keeps `ProbeOK` and `ProbeFail`
  distinct, so the enum's only observable property survives.

This codifies the policy: **document equivalent mutants in the
plan + ADR rather than chase them.** The pass-end gate accepts
"<observed minus 5pp> with N documented equivalents" as a valid
clean state for a package.

The fifteen `NOT COVERED` mutants in `mockBodies` (ARITHMETIC_BASE
on `+` string-concatenation operators) are now covered by
`TestMockBackend_FetchBody_SeededUIDs`, but each mutation is a
compile error. Gremlins reports them as not-viable once it
attempts the build. They no longer count against efficacy.

## Consequences

- `make check-deep` ratchets `internal/mail` at 89%. A future
  regression that drops efficacy below 89 fails the gate.
- The ratchet pattern (per-package observed-minus-5pp floor +
  documented equivalent-mutant exemptions) generalizes. Future
  packages that land in the "queue" list follow the same shape:
  lift, document equivalents, raise floor.
- The `mail.MockBackend` is now exercised through the public
  body-fetch path. Future seed-data changes to `mockBodies` must
  preserve the marker substrings asserted by the test (UID 1
  contains `"Q2 launch"`, UID 5 contains `"invoice_id"`, etc.) or
  the markers move with the change.
