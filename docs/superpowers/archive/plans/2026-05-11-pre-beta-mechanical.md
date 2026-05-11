# Pass 19.1 — pre-beta mechanical refactor

Two mechanical pre-beta cleanups bundled into one pass. Pure
implementation; no open questions.

## Tasks

### T1 — close #46 (no-op)

Backlog item #46 (`messagelist.appendThreadRows` to consume
`walkThread` iter.Seq2) already shipped in Pass 17b — commit
`83d2373` ("Pass 17b: thread walker as iter.Seq2 (closes #46)").
`appendThreadRows` (`internal/ui/messagelist/model.go:498`) already
ranges over `walkThread(root)`. The BACKLOG entry was never
checked off.

Action: mark `BACKLOG.md` entry #46 as closed (mirror the #48 /
#45 / #30 format), pointing at Pass 17b. No code touched.

### T2 — #47 strdist consolidation

Two two-row-DP Levenshtein implementations coexist:

- `internal/config/accounts.go:263` `levenshtein(a, b)` —
  uncapped, used by `suggestProvider` for typo hints.
- `internal/catkin/spellcheck.go:145` `editDistance(a, b, limit)`
  — capped at `limit`, returns `limit+1` when the true distance
  would exceed; used by the SymSpell candidate-verification loop.

The capped version is a strict superset. With a generous `limit`
it behaves identically to the uncapped variant; with a tight
`limit` it skips work once the running row-min exceeds the cap.
The config callsite already implements its own cap (`bestDist=3`
initial; threshold of 2), so passing `limit=2` is a clean fit.

**Shape.**

Create `internal/strdist/` with one exported function:

```go
// Package strdist provides string-distance primitives.
package strdist

// Levenshtein returns the edit distance between a and b, computed
// with the standard two-row DP. When limit > 0, the function may
// return limit+1 instead of the true distance once it can prove
// the true distance exceeds limit — callers that only care about
// "is the distance ≤ limit?" get a faster answer. limit ≤ 0
// disables the cap.
func Levenshtein(a, b string, limit int) int
```

Body lifts from `catkin.editDistance` (the capped algorithm),
with `limit <= 0` short-circuiting the cap branches. The `abs`
helper folds into the same file.

**Callsite changes.**

- `internal/config/accounts.go::suggestProvider` calls
  `strdist.Levenshtein(s, c, 2)`. The post-loop `if bestDist > 2
  { return "" }` is now dead (init=3, only `d < bestDist` assigns,
  capped at 3) — delete it; return `bestName` directly.
- `internal/catkin/spellcheck.go::Suggest` calls
  `strdist.Levenshtein(w, candidate, maxEditDistance)`. Delete the
  local `editDistance` and `abs`.

**Tests.**

`internal/strdist/levenshtein_test.go` — table-driven, covering:

- Identity (`"abc","abc" → 0`).
- Empty-string fast paths (`"","abc" → 3`, `"abc","" → 3`).
- Standard cases (`"kitten","sitting" → 3`, `"jmap","imap" → 1`).
- Length-skew skip (`"a","abcdefg",limit=2 → 3`).
- Row-min skip mid-computation (`"abc","xyz",limit=1 → 2`).
- `limit ≤ 0` returns the true distance, no cap.

No callsite tests need new coverage — the existing config and
catkin tests exercise the wrapper functions.

### T3 — pass-end ritual

`/simplify`, then ADR (one — single decision: extract
`internal/strdist/` with the capped signature; document the
cap-disabling sentinel), invariants update (none required — this
is a refactor inside two existing subsystems, not a binding-fact
change), STATUS bump to Pass 20, archive plan, `make check`,
commit + push + install.

## Out of scope

- `mailcompose` rename (#44) and any other pre-beta refactor —
  separate passes.
- Touching the walk implementation in #46 beyond the BACKLOG
  bookkeeping — already done.
