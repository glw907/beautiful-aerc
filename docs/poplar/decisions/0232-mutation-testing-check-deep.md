---
title: Mutation testing as `make check-deep`
status: accepted
date: 2026-05-12
---

## Context

Audit G (Pass 40, ADR-0230) found 38 useless-test patterns across
22 packages — silent-success fakes, trivially-true lipgloss
assertions, `t.Skip` placeholder stubs, `cmd != nil` assertions
that never invoke the cmd. All passed CI. All sat inside packages
with high line coverage.

The structural problem: coverage measures execution, not
observation. A test that runs every line but asserts nothing
keeps line coverage at 100% while verifying nothing. LLM-authored
test suites reliably produce this shape because the writer's
incentive is "make CI green," and the cheapest way to do that
is to assert what the implementation already returns.

Pass 40 considered three remediation layers: (1) grep-tier checks
in `voice-check.sh` style, (2) skill / write-time conventions,
(3) mutation testing as a measurement gate. Layer 1 turned out to
have weak signal — the regex tells either had zero matches
(because Pass 40.1 fixes haven't landed yet) or high false-
positive rates against legitimate guards. Layer 2 is necessary
but not sufficient — skills shape new code, not legacy. Layer 3
is the only mechanical answer to "is this test useless?"

## Decision

`make check-deep` runs `gremlins unleash` per package across a
curated list of logic-heavy packages, prints efficacy + mutant
coverage, and exits non-zero if any package falls below its
configured threshold. Driver lives at `scripts/check-deep.sh`.

Not part of `make check`. Slow by design (recompile + retest per
mutant; ~20s per small package, multi-minute for `internal/cache`).
Run at pass-end before consolidation, or nightly. The `poplar-pass`
skill will pick it up as a consolidation step in a follow-up pass.

Curated package list:

- `internal/mailcompose` — MIME assembly, parsing, seed
- `internal/mail` — Classify, classifiers, sentinels
- `internal/cache` — drainer state machine, ops, schema
- `internal/content` — body parser, footnote walker
- `internal/filter` — markdown pipeline
- `internal/tidytext` — rewriter
- `internal/mailauth` — token store, refresh
- `internal/config` — strict decoder, validators

Excluded:

- `internal/ui/**` — golden-driven; mutation testing fights
  snapshot tests, and the rendering surface is value-as-bytes
  rather than value-as-property.
- `internal/mailimap`, `internal/mailjmap` — network-heavy and
  fake-driven; gremlins-on-fakes is circular (mutating the fake
  doesn't mutate the contract under test).
- `internal/term`, `internal/ansix` — terminal-probing,
  measurement-stable; mutation-low-signal.

Initial thresholds: 0% efficacy (informational only). The first
real run sets the calibration. A follow-up pass tightens each
package's floor based on observed scores, with `internal/mail` /
`internal/cache` / `internal/mailcompose` targeted at ≥80%.

Tool: `github.com/go-gremlins/gremlins` (actively maintained,
last release 2025-12-06; supports build tags, per-package paths,
diff-mode for incremental runs, exclude-files regex). Installed
via `go install …@latest`; script falls back to `~/go/bin/gremlins`
when not on `$PATH`. `make check-deep` exits 2 if the tool is
missing with the install hint inline.

The `go-conventions` skill grows a new "Assertion Discipline (no
useless tests)" subsection codifying the patterns Audit G found:
silent-success fakes get per-method `*Err error` fields; no
SUT-derived expectations; no `Render("x") != ""` style assertions;
Cmd assertions must invoke and type-switch; no unconditional
`t.Skip` in committed tests.

## Consequences

- Mutation testing answers the question coverage cannot:
  "would any test fail if the code were wrong?" Surviving mutants
  are bug reports against the test suite, not the source. The
  Audit G silent-success-fake cluster would have surfaced as
  dozens of surviving mutants in `mailimap.classifyErr` and
  `cache.drainer.executeOne` before remediation; the post-40.1
  baseline becomes the floor every later pass must hold.
- Smoke test (2026-05-12) on `internal/mailcompose`: 82.28%
  efficacy, 87.78% mutant coverage, 14 surviving mutants across
  scheduleparse + seed paths. Real signal at expected granularity
  for a pure-logic package; the 14 survivors are the queue for
  the calibration pass.
- The grep-tier `voice-check.sh`-style gate is *not* added.
  Patterns flagged by Audit G either have AST-level false-positive
  shapes (`t.Skip` conditional vs. unconditional) or were
  already cleared in code review. The AST-based unconditional-
  `t.Skip` detector is BACKLOG #59 — small `go/parser` + `go/ast`
  walker, wires into `make check` for fast feedback at the
  edit-time end while `check-deep` handles the deep semantic end.
- `make check` stays under ~30s; `make check-deep` is the
  pass-end gate. Two-tier shape mirrors the linting / vetting
  split: cheap mandatory gate on every commit; expensive
  thorough gate at integration boundaries.
- UI testing remains golden-driven. Mutation testing's
  exclusion of `internal/ui/**` is principled (snapshots
  measure bytes, not properties) and matches the recommendation
  in ADR-0230 to keep golden discipline as the UI tier.
- Future passes that add new logic-heavy packages add them to
  the `PKGS` map in `scripts/check-deep.sh` at the same commit
  that introduces the package. No registry, no auto-discovery —
  the curation is the value.
