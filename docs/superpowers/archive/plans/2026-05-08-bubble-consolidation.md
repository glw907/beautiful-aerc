# Pass 9y — Bubble consolidation verdict

## Goal

Read the 9x.1–9x.3 harvest catalogs together with Eval A and Eval B,
then issue the final keep/swap verdict per bubble. Close the bubble-
eval roadmap so Pass 9z can adopt the surviving harvest items
without re-litigating verdicts.

## Inputs

- `docs/poplar/research/2026-05-08-bubbles-list-patterns.md` (9x.1)
- `docs/poplar/research/2026-05-08-chrome-family-patterns.md` (9x.2)
- `docs/poplar/research/2026-05-08-table-form-patterns.md` (9x.3)
- `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md`
  (Eval A — five candidates, all Keep + harvest)
- `docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`
  (Eval A re-eval post-ansix + Eval B — four more candidates, all
  Keep)

The starter prompt cites Eval A at an `archive/specs/` path; the
file is still in `specs/`. Note the discrepancy and resolve it as
part of this pass: archive Eval A alongside Eval B since both are
now superseded inputs to a closed roadmap.

## The brainstorm question — settled

> Is there a swap candidate where the cumulative "did not survive"
> list now exceeds the "kept" surface area enough to flip the
> verdict?

**No.** Each Keep rests on a structural blocker that cumulative
harvest lessons do not address:

| Library | Blocker that survives consolidation |
|---|---|
| `rmhubbert/bubbletea-overlay` | Same Superfile-derived algorithm, no cascade support — adoption is a dependency for zero behavior gain. |
| `bubbles/help` | Spatial named-group grid layout cannot be expressed as `ShortHelp() []key.Binding` / `FullHelp() [][]key.Binding`; `JoinHorizontal` ban (ADR-0084); loses `wired` dim affordance. |
| `daltonsw/bubbleup` | Animation/categorization chrome is the library's entire value; the triage payload, undo Cmd, and domain verb are hand-rolled content that cannot move into the library. Domain mismatch. |
| `charmbracelet/huh` | `huh.Group` static-only at construction; `Form.View()` has no body-only mode (Contacts mode mounts the form as a right-pane column). |
| `evertras/bubble-table` | `JoinHorizontal` in row + header render (ADR-0084); threading depth-prefix walk + `ActionTargets()` cannot map onto rectangular `RowData`; UID-keyed `marked` map vs check-column. |
| `bubbles/list` (pickers) | Chrome-on-by-default wrong for inline-splice; `FilterState` two-phase machine; paginator math; `Styles` has 13 dead chrome slots. |
| `bubbles/list` vs `treilik/bubblelister` (dropdown) | Both wrong shape for a 7-row positional splice. |
| `bubbles/list` (sidebar folder column) | Three permanent groups + nested-flat display + non-scrolling layout don't fit a single `list.Model`. |
| `mistakenelf/teacup` statusbar | Four positional slots vs poplar's six semantic segments with a left-side fill; `JoinHorizontal` final assembly (ADR-0084). |

The cumulative harvest yielded **one** adoption (movepicker filter-
match rune highlight in 9x.1) and **zero** structural diffs across
the chrome and table/form families. Three small adjacent cleanups
landed (one in 9x.1, one in 9x.3); the harvest catalogs are the
durable record. Nothing in the consolidation suggests a missed
swap.

## One-line argument per Keep (for future-poplar)

These read forward — if a future pass reconsiders one of these
libraries, the line below names what would have to change for the
verdict to flip.

- **bubbletea-overlay** — flip if the library grows a cascade
  ordering primitive (mutual-exclusion with priority); today it
  does not, and `App.View`'s nine-level cascade is the contract.
- **bubbles/help** — flip if helppopover drops the named-group grid
  contract or the `wired` dim affordance, and ADR-0084 narrows.
  None of those are on the table.
- **bubbleup** — flip if poplar adds a separate animated-
  notification surface alongside the triage banner (post-1.0). The
  triage banner itself is the wrong consumer.
- **huh** — flip on the **first-run wizard (Pass 14)** if the
  wizard field set stays static. This eval explicitly does not
  foreclose that. Contacts/form is a separate consumer with a
  dynamic-row blocker that `huh` cannot lift.
- **bubble-table** — flip if `JoinHorizontal` is removed from the
  row + header render path upstream **and** the threading prefix
  walk is replaced (it won't be — threading is the reason
  messagelist exists).
- **bubbles/list** (any of three consumers) — flip if the library
  grows a chrome-off-by-default mode, drops the two-phase
  `FilterState`, and exposes positional splice as a primary
  shape. None of those are upstream priorities.
- **teacup statusbar** — flip if the library grows a fill-absorbs-
  variance segment shape. The four-slot positional contract is
  load-bearing for teacup's file-manager domain; no signal of
  change.

## Tasks

1. **Consolidate the harvest count.** Confirm the three catalogs'
   "Top three" sections together yield one Pass-9z adoption
   (movepicker filter-match highlight, already landed in 9x.1) and
   one "candidate for later" worth carrying forward
   (`bubbles/help` `shouldAddItem` truncation loop in 9x.2).
   Everything else is catalog-only.
2. **Write the verdict doc** at
   `docs/poplar/research/2026-05-08-bubble-consolidation-verdict.md`.
   One section per library with the Keep + one-line flip
   condition. No new analysis — pull rationale from the existing
   evals and catalogs by reference.
3. **Write ADR-0182.** Bubble-eval roadmap closed; nine candidates
   surveyed; one inline adoption; future bubble evals require a
   named flip condition from the verdict doc, not a fresh rubric
   pass. Brief — one paragraph per section.
4. **Update invariants.md** if any binding fact moves. Likely
   none — every Keep matches the current shape of the consumer.
5. **Update the decision index** (`docs/poplar/decisions/INDEX.md`)
   with the new ADR row in the "Bubbletea conventions" theme.
6. **Resolve the Eval A path discrepancy.** Move
   `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md`
   to `docs/superpowers/archive/specs/` so the starter prompt's
   reference path matches reality. Same for
   `docs/superpowers/specs/2026-05-08-bubble-adoption-design.md`
   — its roadmap is now closed; archive it.
7. **Standard pass-end checklist.** `/simplify` (no Go diff this
   pass — voice lens applies to docs), `make check`, commit, push,
   install. Update STATUS.md with the next starter prompt for Pass
   9z (adoption work).

## Out of scope

- Pass 9z adoption work. The verdict doc tells 9z what to adopt;
  9z owns the implementation.
- Re-running any rubric. Eval A and Eval B remain the rubric of
  record; this pass only consolidates their verdicts.
- Touching `internal/ui/` source. The pass is research +
  verdict + housekeeping; no Go changes expected.

## Pass-end checklist

- [ ] `/simplify` (voice lens on the new docs)
- [ ] Verdict doc written
- [ ] ADR-0182 written; INDEX.md updated
- [ ] Invariants checked (likely no change)
- [ ] Eval A spec + adoption-design spec archived
- [ ] Plan archived; STATUS.md updated
- [ ] `make check` green
- [ ] Commit + push + install
