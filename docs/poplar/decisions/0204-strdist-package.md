---
title: internal/strdist consolidates Levenshtein
status: accepted
date: 2026-05-11
---

## Context

Two two-row-DP Levenshtein implementations coexisted: `internal/
config/accounts.go::levenshtein` (uncapped, used by `suggest
Provider` for typo hints) and `internal/catkin/spellcheck.go::
editDistance` (capped at `limit`, used by the SymSpell candidate-
verification loop). Pass 16b's modernization sweep modernized
both in place but left the duplication. Pre-beta is the cheap
time to dedupe.

## Decision

`internal/strdist/` holds one exported function:

```go
func Levenshtein(a, b string, limit int) int
```

The capped variant is a strict superset of the uncapped — with
`limit ≤ 0` the function treats `limit = math.MaxInt` and the
length-skew + row-min skips never fire. Both call sites route
through it. The `bestDist > 2` post-loop guard at `suggest
Provider` becomes dead (init=3, only `d < bestDist` assigns,
capped at 3) and is deleted.

## Consequences

- One Levenshtein, one home. New string-distance primitives land
  in `internal/strdist/` rather than wherever the first consumer
  lives.
- `internal/config` and `internal/catkin` each shed a private
  helper (and `catkin` sheds its `abs` helper alongside).
- No behavior change at either call site; the capped path was
  already what `catkin` used, and `config` passes `limit=2` so
  the cap is harmless.
