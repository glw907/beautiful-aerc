# Pass 9k.4 — Comment sweep: UI subpackages + catkin

Final slice of the comment voice sweep. Bring the remaining UI
subpackages and `internal/catkin/` into compliance with the §0
rubric and T38–T40 (ADR-0168). T34 (semicolon clause-joiner) is
voice-lens only per ADR-0173 — default to a period, but a
considered semicolon is fine. The density-floor exemption
(ADR-0170) does not apply: none of these packages are header-
shaped.

## Scope

| Package                   | Pre-sweep cmts | Density |
|---------------------------|----------------|---------|
| `internal/ui/sidebar/`    | 132 | 16.3% |
| `internal/ui/helppopover/`|  72 | 16.1% |
| `internal/ui/compose/`    |  67 | 11.4% |
| `internal/ui/movepicker/` |  37 | 10.8% |
| `internal/catkin/`        | 340 | 10.4% |
| `internal/ui/contacts/`   | 199 |  9.6% |

Target: ≤9% density per package.

## Method

Identical to 9k.1–9k.3:

1. Walk every non-test `.go` file in the package.
2. Apply §0(a)/(b)/(c) to every comment. (a)/(b) → delete.
   (c) → keep but rewrite shape if T39/T40 semantically off.
3. Paraphrase test every in-function comment.
4. Compress label-colon godocs and reference-stuffed paragraphs.
   ≤1 ADR/RFC cite per godoc.
5. Don't add new comments.
6. Watch for prior-sweep regression patterns: em dashes (T33),
   label-colon godoc openers (T39), restate-the-code paraphrase.
7. Commit per-package; `make check` green at every commit.

Catkin loads its own invariants rule
(`.claude/rules/catkin-invariants.md`) on file open.

## Tasks

1. `internal/ui/sidebar/` — highest density; calibrate first.
2. `internal/ui/helppopover/` — second-highest density.
3. `internal/ui/compose/` — App-side compose shadow.
4. `internal/ui/movepicker/` — small, fast.
5. `internal/ui/contacts/` — largest by line count after catkin.
6. `internal/catkin/` — last; loads its own invariants rule.
7. Pass-end ritual via `poplar-pass`: invariants / STATUS update,
   archive this plan, commit + push + install. New ADR only if a
   recurring shape surfaces.

## Acceptance

- Per-package density ≤9%.
- Zero paraphrase comments (semantic spot-check).
- Zero label-colon godocs (T40).
- ≤1 ADR/RFC cite per godoc.
- `make check` green at every commit.
