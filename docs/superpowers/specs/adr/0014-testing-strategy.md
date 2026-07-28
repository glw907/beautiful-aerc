# ADR-0014: test economy matched to layer shape

Date 2026-07-27. Status: accepted (Phase 4).

## Context

The requirements bind tests to acceptance criteria (registry-
driven grammar tests, golden renders, kill harness, synctest
scenarios, perf harnesses). The strategy must make those cheap
enough to run on every commit (QA-10) and honest at the QA-5
envelope.

## Decision

- Pure logic (render rules, JWZ, query and time parsers, iTIP,
  recurrence splicing): table-driven tests over fixture corpora;
  every render rule ships at least one fixture; the license-clean
  specimen corpus is a Phase 5 pass-3 artifact, the private
  corpus a local supplemental run.
- Store: transaction tests, the randomized mutation+search
  consistency script, migration-from-N-1 tests, and the QA-6
  seeded kill harness (30-action script, 200 SIGKILL points,
  three seeds) in CI.
- Engines: `testing/synctest` scenarios over scriptable backend
  fakes (state resets, 412s, push drops, throttled first sync).
  The fakes are the seam's second implementation, which keeps the
  interface honest.
- UI: teatest goldens per screen state and capability profile
  (profiles are test inputs, never sniffed); registry-driven
  grammar/footer/switch tests; scripted keystroke tests for
  optimistic-paint criteria.
- Performance: QA-1/2/3 harnesses land with their subjects
  (build-order step 1) with CI thresholds recorded separately
  from gate-platform numbers.
- Live account: a small tagged manual suite for server behavior
  (EventSource auth, draft replacement, RSVP once the token
  exists); CI never touches the live account.

## Alternatives considered

- **Mock-heavy unit testing of engines**: mocks assert call
  shapes, not behavior; scriptable fakes plus synctest test the
  actual loop under virtual time, which is what SY-2's recovery
  numbers need.
- **End-to-end tmux tests as the primary harness**: kept for
  Phase 5's verification layer where a real terminal matters, but
  goldens through teatest are deterministic and fast enough to
  gate every commit.
- **Deferring perf harnesses to a hardening pass**: the legacy
  client died of deferred performance; the spine (section 15 of
  the requirements) forbids it.

## Consequences

The build machine (Phase 5) tunes gates around these harness
shapes. Fixture corpora are the shared currency between the
improve loop, the grading harness, and CI.

## Revision 2 (2026-07-27, post-review)

Added to the store suite: the three SY-8 tests (forced
corruption, failed migration, full disk), the FTS5
integrity-check inside the QA-6 restart assertions and at the end
of the SR-1 randomized script (row-count equality does not detect
term rot), and `EXPLAIN QUERY PLAN` goldens for every hot query.
Added to the engine suite: idempotent-replay tests per intent
kind. Added to the gate outputs: the QA-10 artifacts (conventions
gates in CI from the first build pass, `internal/ui` package
documentation, README and architecture map at 1.0). Named risk:
teatest is under the experimental x/exp path; goldens are plain
files, so a harness swap is mechanical.
