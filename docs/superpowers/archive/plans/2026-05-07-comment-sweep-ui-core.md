# Pass 9k.3 — Comment sweep: UI core

Third slice of the comment voice sweep. Bring `internal/ui/`
(App layer + chrome) and the four most-trafficked UI subpackages
into compliance with the §0 rubric and T38–T40 (ADR-0168). Apply
the density-floor exemption (ADR-0170) only where the package is
truly header-shaped — none of these are.

## Scope

| Package                   | Pre-sweep cmt | Density |
|---------------------------|---------------|---------|
| `internal/ui/`            | 365 | 11.4% |
| `internal/ui/uicore/`     | 179 | 28.1% |
| `internal/ui/account/`    | 208 | 15.6% |
| `internal/ui/messagelist/`| 240 | 19.6% |
| `internal/ui/reader/`     | 143 | 14.2% |

Target: ≤9% density per package. uicore is the worst in the
tree; 9k.4 covers the remaining UI subpackages + catkin.

## Method

Identical to 9k.1/9k.2:

1. Walk every non-test `.go` file in the package.
2. Apply §0(a)/(b)/(c) to every comment. (a)/(b) → delete.
   (c) → keep but rewrite shape if T39/T40 semantically off.
3. Paraphrase test every in-function comment.
4. Compress label-colon godocs and reference-stuffed paragraphs.
   ≤1 ADR/RFC cite per godoc.
5. Don't add new comments.
6. Watch for the 9k.2 regression patterns: em dashes (T33),
   semicolon clause-joiners (T34), label-colon godoc openers
   (T39).
7. Commit per-package; `make check` green at every commit.

UI passes load `docs/poplar/bubbletea-conventions.md` before
touching `View()` or layout godocs — phrasing in render comments
must match the size-contract / wordwrap discipline language.

## Tasks

1. `internal/ui/uicore/` — highest density; do first to get
   shape calibration before the bigger packages.
2. `internal/ui/messagelist/` — second-highest density.
3. `internal/ui/account/` — App-owned account model.
4. `internal/ui/reader/` — body + linkpicker + attachpicker.
5. `internal/ui/` — App layer; largest line count, lowest
   density of the slice. Do last so prior calibration carries.
6. Pass-end ritual via `poplar-pass`: invariants / STATUS
   update, archive this plan, commit + push + install. New ADR
   only if a recurring shape surfaces.

## Acceptance

- Per-package density ≤9% (no header-shaped exemptions in this
  slice).
- Zero paraphrase comments (semantic spot-check).
- Zero label-colon godocs (T40).
- ≤1 ADR/RFC cite per godoc.
- `make check` green at every commit.
