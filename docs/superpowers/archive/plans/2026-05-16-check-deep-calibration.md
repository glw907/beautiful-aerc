# Pass 40.3 — `make check-deep` calibration + AST skipcheck

## Goal

Run `make check-deep` for the first time against the post-40.1 test
suite, set per-package mutation thresholds from the observed
efficacy, and land the AST-based unconditional-`t.Skip` detector
queued as BACKLOG #59.

## Why

ADR-0232 landed the `check-deep` driver with all thresholds at 0%
(informational). Without calibration the gate has no teeth — a
package can regress to 50% efficacy and CI stays green. The
calibration target is "observed efficacy minus a small buffer" so
the floor tracks current quality without flapping on incidental
variance.

ADR-0230 surfaced `internal/contacts/sync_test.go` as four
unconditional `t.Skip("stand up httptest server")` stubs. Pass 40.1
replaced those with real tests, but the failure mode — committed
placeholder skips passing CI — is grep-resistant: legitimate
guarded skips (env-gated integration tests, GOOS-conditional,
missing-fixture short-circuits, ~10 valid sites in-tree) share the
same `t.Skip(...)` token. AST-level shape detection is the right
substrate: walk every `func Test*(t *testing.T)`, find bodies whose
first executable statement is `t.Skip(...)` with no enclosing `if`,
flag.

## Scope

In:

- `scripts/check-deep.sh` — replace 0% thresholds with calibrated
  per-package floors (observed - 5pp buffer).
- `scripts/skipcheck/main.go` — Go AST walker; ~50 LOC plus a
  fixture or two if needed.
- `Makefile` — new `skipcheck` target; chain into `check`.
- ADR-0234.
- Invariants update — `make check` line gains the new step.

Out:

- New tests for the mutation survivors found during calibration.
  Surviving mutants surface as the queue for a follow-up pass;
  this pass only sets the floor.
- Tightening thresholds beyond observed - 5pp. The buffer is the
  margin; raising the floor without writing new tests is the next
  pass's job.
- Coverage for the mutation-low-signal packages
  (`internal/term`, `internal/ansix`, `internal/ui/**`,
  `internal/mailimap`, `internal/mailjmap`). ADR-0232 already
  ruled them out.

## Plan

1. **Capture baseline.** Run `make check-deep` against the eight
   curated packages. Record each package's efficacy + mutant
   coverage. `internal/cache` is the slow one; budget ~10 min.
2. **Calibrate.** For each package, set the threshold to
   `floor(observed - 5)`. If observed is under ~30%, document
   the gap in the ADR — that package's suite needs work before
   the floor tightens, but the calibrated number prevents
   further regression.
3. **Write the skipcheck walker.** `scripts/skipcheck/main.go`:
   - `filepath.WalkDir` from `.`, filtering to `*_test.go`,
     skipping `vendor/`, `.git/`, build artifacts.
   - `parser.ParseFile` with `parser.SkipObjectResolution` for
     speed.
   - For each `*ast.FuncDecl` whose Name starts with `Test` and
     whose first param is `*testing.T`, check the function body's
     first statement. Hit if it's an `*ast.ExprStmt` wrapping a
     `t.Skip(...)` or `t.SkipNow()` call at the body's top level
     (not inside an `if`, `switch`, `select`).
   - Exit 1 with `file:line: SK1 unconditional t.Skip` on hit.
4. **Wire into make.** `Makefile`: add `skipcheck:` target
   running `go run ./scripts/skipcheck`. Append to `check`
   between `modern-go-check` and `test`.
5. **Calibration sanity.** Re-run `make check-deep` after
   threshold edits to confirm green.
6. **Pass-end ritual.** ADR-0234, invariants update, STATUS
   bump, archive plan, `make check`, commit, push, install.

## Risks

- **Skipcheck false positives.** Legitimate top-level
  `t.Skip("integration only")` will trip the rule. The right
  shape for those is `if testing.Short() { t.Skip(...) }` or a
  build-tag fence — both make the skip visible at the right
  layer. If a legitimate site exists, fix the site (preferred)
  rather than weakening the detector.
- **Calibration drift.** Gremlins reports float between runs by
  a few percent (mutant ordering, timing). The 5pp buffer
  absorbs this; if the floor still flaps the buffer widens.

## References

- ADR-0230 — Audit G findings, the placeholder-skip motivation.
- ADR-0232 — check-deep driver decision, the 0% floor lineage.
- ADR-0233 — Audit G remediation, the post-fix baseline.
- BACKLOG #59 — skipcheck queue entry.
