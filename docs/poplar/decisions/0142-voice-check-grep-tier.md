---
title: Voice-check grep tier in `make check`
status: superseded by 0148
date: 2026-05-04
---

> Superseded by ADR-0148 (2026-05-05), which widens the grep tier
> from six tells (T4, T10, T14, T16, T27, T28) to nine by adding
> the prose-rhythm tells T33–T35.

## Context

ADR-0141 named three enforcement surfaces for the human-voice
policy: the `go-conventions` skill (write-time), the `/simplify`
voice lens (post-write review), and the `CLAUDE.md` pointer
(in-context reminder). All three are LLM-mediated. None of them
catches drift on a tell that's already shipped, and a contributor
without the skill loaded has nothing between their editor and
master.

A subset of the §7 tells has reliable mechanical signatures —
fixed phrases or anchored grammar that a regex can flag with no
false-positives on the current tree. Those tells should run on
every `make check`, independent of any LLM agent.

## Decision

Add `scripts/voice-check.sh`, wired into the `make check` gate
(between `vet` and `test`). The script greps the tree for the
grep-tier tells and exits non-zero on any finding.

Tells in the grep tier (calibrated against master at end of
Pass 8.10, zero false-positives):

- **T4** — `for now` and bare/stub TODO.
- **T10** — `"failed to "` opener in `errors.New` / `fmt.Errorf`.
- **T14** — `Get*` getter prefix on a method declaration.
- **T16** — `Manager` / `Helper` / `Util` / `Service` type suffix.
- **T27** — narrow set of apologetic phrases (`may not handle`,
  `could be improved`, …).
- **T28** — narrow set of standard-Go-idiom over-explanations
  (`use a goroutine to`, `close the channel when`, `iterate over
  the/all/each`, `loop through the/all/each`).

Tells deliberately not in the grep tier:

- **T15** (package-doubled types) and **T19** (skeleton file
  layout) — pattern is grep-shaped, but T19 would flag
  structurally-meaningful files (`internal/mail/types.go`,
  `internal/mailjmap/errors.go`) and T15 has no current matches
  to calibrate against. Add later if a violation lands.
- **T1, T2, T3, T5, T6, T7, T8, T9, T10b, T11, T12, T13, T17,
  T18, T20, T21, T22, T23, T24, T25, T26, T29, T30, T31, T32** —
  all require semantic context (call-graph, invariants,
  uniformity across symbols, …). Stay with the `/simplify` voice
  lens.

The `/simplify` voice agent (SKILL.md §Agent 4) is updated to
skip the grep-tier tells, freeing its attention for the semantic
ones.

## Why `make check`, not a pre-commit hook

`make check` is poplar's commit gate (CLAUDE.md). Putting
voice-check there makes it version-controlled, runs in CI,
unbypassable without skipping the test gate, and consistent with
`vet` and `test`. A pre-commit hook would scope to changed files
but live in `.git/hooks/` — not tracked, easy to skip with
`--no-verify`, and invisible to CI. Whole-tree grep is
milliseconds on poplar's ~100 .go files; the speed cost is not
real.

A whole-tree scan also means: when the catalogue grows or a new
tell lands grep-tier status, the next `make check` surfaces every
existing instance, not just the diff.

## Consequences

- `make check` now fails on six grep-tier tells. Today the tree
  passes; future drift trips the gate.
- `/simplify` Agent 4 is faster and more focused — fewer redundant
  findings to triage.
- New tells join the script via `scan TELL "regex" "rule"`.
  Calibration rule: a pattern lands only if it returns zero hits
  on the current tree (or all current hits are real and being
  fixed in the same commit).
