---
title: check-deep threshold calibration + AST skipcheck
status: accepted
date: 2026-05-16
---

## Context

ADR-0232 landed `make check-deep` with every per-package floor at
0%: the driver was wired but had no teeth. ADR-0230 also surfaced
the unconditional `t.Skip("stand up httptest server")` placeholder
shape — four committed test bodies in `internal/contacts/` whose
first executable statement was `t.Skip`, passing CI with zero
coverage. Pass 40.1 replaced those bodies with real tests, but the
shape itself is grep-resistant: ~10 legitimate guarded skips
(`testing.Short`, `runtime.GOOS`, env-gated integration) share the
`t.Skip(...)` token. BACKLOG #59 queued the AST detector.

## Decision

**Per-package mutation floors.** `scripts/check-deep.sh` carries
calibrated efficacy thresholds equal to the observed baseline
minus a 5pp buffer:

| Package              | Observed | Floor |
|----------------------|---------:|------:|
| `internal/mailauth`     | 76.00% | 71% |
| `internal/content`      | 78.75% | 73% |
| `internal/filter`       | 82.14% | 77% |
| `internal/mail`         | 69.23% | 64% |
| `internal/cache`        | 77.54% | 72% |
| `internal/tidytext`     | 79.07% | 74% |
| `internal/mailcompose`  | 83.76% | 78% |
| `internal/config`       | 83.93% | 78% |

The 5pp buffer absorbs the 1–3pp run-to-run drift gremlins shows
from mutant ordering. Raising a floor requires writing new tests
that lift the observed number — closing the buffer is forbidden.
`internal/mail` at 69% is the queue: `classifyErr` sentinel
routing carries the most surviving mutants. A follow-up pass
lifts it past 80%; the 64% floor is the no-regression line, not a
target.

**AST `skipcheck`.** `scripts/skipcheck/main.go` walks every
`*_test.go` under the working directory, parses with
`go/parser`, and for each `func Test*(t *testing.T)` whose body's
first executable statement is a top-level `t.Skip` / `SkipNow` /
`Skipf` call (the actual `*testing.T` parameter name is matched,
not just `t`), prints `file:line: SK1 …` and exits 1. Wired into
`make check` between `modern-go-check` and `test`. Legitimate
guarded skips sit inside an `if` and pass cleanly; build-tag-
fenced files (the IMAP integration test) stay out of the default
compilation set.

## Consequences

- `make check-deep` is now a real gate. The next regression in
  any of the eight curated packages fails CI at the
  pass-end consolidation step rather than slipping past coverage.
- `make check` gains a new fast-path detector for the
  placeholder-skip shape. The fast end (skipcheck, ~50ms over the
  whole tree) and the deep end (gremlins, multi-minute) compose:
  unconditional skips fail the cheap gate, useless-but-non-empty
  tests fail the expensive gate.
- The `internal/mail` 69% number is the visible queue item for a
  follow-up pass. `classifyErr` is the dominant survivor bucket;
  the work is symmetric to the Audit G mailimap/mailjmap fixes
  but on `mail.WrapSentinel`'s caller-side routing.
- 5pp buffer is the load-bearing convention. The next pass that
  edits `scripts/check-deep.sh` must not tighten any floor by
  closing the buffer; the buffer is what keeps gremlins'
  non-determinism from flapping the build red.
- Skipcheck's identifier-name match (uses the declared
  `*testing.T` parameter name, not a hardcoded `t`) handles the
  `func TestX(tb *testing.T)` and `func TestY(t *testing.T)`
  shapes uniformly. A future move to `*testing.TB` for
  shared-helper test signatures would need the matcher widened.
- BACKLOG #59 closes.
