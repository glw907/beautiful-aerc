---
title: Prose-rhythm tells (T33–T35) join the grep tier
status: accepted
date: 2026-05-05
---

## Context

ADR-0142 calibrated `scripts/voice-check.sh` against the tree at
the end of Pass 8.10 and shipped six grep-tier tells (T4, T10,
T14, T16, T27, T28). The lexical audit those tells codify is
effective — `leverage`, `utilize`, `robust`, `Note that`, `failed
to`, `Get*` getters, `Service`-suffixed types, etc. all return
zero hits on master.

But comments in the tree still read like documentation prose. The
8.8/8.9 string audit caught the *vocabulary* tells; it left the
*punctuation rhythm* tells in place. A late-Pass-8.10 catalogue
addition (`~/.claude/docs/go-comment-voice.md` §7, T33–T37) named
the prose-rhythm signature: em-dash clause-joiners, semicolon
clause-joiners, documentation labels (`Preference:`, `Fallback:`,
`Priority:`), long parenthetical asides, and the meta-tell of
multi-clause comment rhythm.

Pass 8.10 calibration on the unmodified tree returned ~211 T33
hits, ~266 T34 hits, and 2 T35 hits. The deferred grep patterns
sat commented in `scripts/voice-check.sh` waiting for a cleanup
pass to bring the tree to zero so the gate could activate without
breaking `make check` immediately.

## Decision

Pass 8.11 ran a tree-wide mechanical sweep across `cmd/` and
`internal/` and rewrote every offending comment:

- T33 (em-dash clause-joiner): rewrite `// X — Y` as `// X. Y.`
  Reserve em dashes for short comma-like asides (still acceptable
  in negligible volume).
- T34 (semicolon clause-joiner): rewrite `// X; Y` as `// X. Y.`
  Real lists (`// a, b; c, d`) keep semicolons.
- T35 (documentation labels): drop the label, write the rule as
  prose.

Five comments hit unavoidable false-positive patterns under the
T34 grep (semicolons inside ANSI escape literals, HTML entities,
quoted user-facing examples, lists in parentheticals). Those were
rewritten to dodge the regex without losing meaning, on the
principle that a regex with no false positives is more valuable
than a comment that can keep its preferred phrasing.

The same commit activates the three deferred `scan` calls in
`scripts/voice-check.sh` so `make check` now fails on T33, T34,
or T35 regressions.

In passing, `internal/mailjmap/jmap.go` flipped 11 `fmt.Errorf`
sites from `%v` to `%w` so the cache drainer's
`errors.Is(err, mail.ErrAuth)` / `mail.ErrNotFound` routing has
an intact wrap chain. (One non-error `%v` formatting an enum
value stayed as-is.)

T36 (long parens) and T37 (multi-clause rhythm) stay in the
`/simplify` voice-lens semantic tier — they cannot be detected
mechanically without false positives. The `/simplify` Agent 4
brief in `~/.claude/skills/simplify/SKILL.md` already excludes
the grep-tier tells from its scope, so the cost is no extra work
per review.

This supersedes ADR-0142 by widening the grep tier from six tells
to nine.

## Consequences

- `make check` enforces nine prose tells mechanically, up from
  six. Future comment drift on T33/T34/T35 trips the gate at
  commit time, not at the next voice audit.
- The mechanical sweep touched 126 files — a significant churn
  surface, but pre-beta posture (ADR-0105) makes that a free move.
- Five comments accept slightly awkward phrasing to keep the
  T34 grep zero-false-positive. Worth it: the gate's value is its
  precision, not the elegance of any single comment.
- The `%v` → `%w` correctness fix in `mailjmap/jmap.go` removes a
  silent class of cache-drainer routing failures (auth and
  not-found errors that previously reached the drainer as raw
  wrapped strings would now match the typed sentinels).
- Catalogue updates to `~/.claude/docs/go-comment-voice.md` §5
  (prose precedence rule) and §7 (T33–T37 entries) are the
  upstream that this ADR formalizes for poplar specifically. The
  go-conventions skill carries the same updates inline for
  write-time enforcement on every Go file.
