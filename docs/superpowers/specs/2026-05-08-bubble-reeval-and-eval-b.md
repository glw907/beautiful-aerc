# Bubble Re-eval (post-ansix) + Eval B — Design

A single triage pass that revisits Eval A's verdicts under ansix
and runs Eval B on its four candidates. Output feeds Pass 9z's
consolidation roadmap.

## Context

Pass 9w.1 extracted `internal/ansix/` (ADR-0181) as the
icon-aware width primitive over `charmbracelet/x/ansi`. Eval A
ran pre-ansix; several "Keep + harvest" verdicts cited the
`lipgloss.JoinHorizontal` ban (ADR-0084) or `lipgloss.Width`
miscount under SPUA-A icon mode. ansix changes the seam shape on
poplar's side. It does not change the seam inside upstream
libraries. Some Eval A verdicts may flip; some won't.

Eval B was scoped in the original adoption design
(`2026-05-08-bubble-adoption-design.md`) but never executed. The
post-ansix re-eval is the right moment to land it — same context
load, same rubric, same output shape.

## Output

One spec at
`docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`
(this document evolves into the eval result during execution).
No ADR. Eval A's existing spec
(`2026-05-08-bubble-eval-a-strong-matches.md`) stays as the
pre-ansix baseline; the new spec supersedes its verdicts only
where ansix flips them.

## Candidate set

Nine candidates total.

### Eval A revisit (5)

1. `rmhubbert/bubbletea-overlay`
2. `bubbles/help`
3. `daltonsw/bubbleup`
4. `charmbracelet/huh`
5. `evertras/bubble-table`

### Eval B (4)

6. `bubbles/list` × movepicker / linkpicker / attachpicker
   — batched. One library, three picker sites with similar
   delegate shape. Eval treats them as one verdict with
   per-site notes.
7. `bubbles/list` vs `treilik/bubblelister` for
   `compose.Dropdown`. Two-library compare for the autocomplete
   surface (small list, dynamic suggestions).
8. `bubbles/list` for sidebar folder column. T9 contacts
   groups are out of scope (they are a different shape and
   already settled hand-rolled).
9. `knipferrc/teacup` statusbar for `internal/ui/status_bar`.

## Rubric

Same six dimensions as Eval A:

1. **Feature parity** — does the bubble cover what poplar's
   hand-roll does today, or are there gaps that matter?
2. **Customization seams** — can themes, modifier-free
   single-key bindings, and domain state wire in without
   forking?
3. **Theming integration** — does the bubble accept
   `lipgloss.Style` injection or own its own colors?
4. **Maintenance signal** — last commit, version cadence,
   releases in the past twelve months. Not a veto unless dead.
5. **Code delta estimate** — rough LOC removed from poplar vs
   LOC added (shim + integration). A 50→500 swap is a tell.
6. **License** — MIT, BSD, or Apache only. Hard veto otherwise.

The rubric is the evidence; the verdict rests on the core
question: *does adopting the community bubble make poplar
better?* (At minimum, not worse.)

## Per-candidate process

For each candidate:

1. Read the actual library source — specifically the render
   path that previously gated adoption (typically the function
   calling `lipgloss.JoinHorizontal` or `lipgloss.Width`).
2. Read poplar's current consumer carefully.
3. Decide: does ansix change the verdict? For Eval B, decide
   from scratch.
4. Write the candidate section: lead with the core question
   answered concretely, then rubric evidence, then verdict
   (**Adopt / Adopt-with-fork / Keep + harvest**) with a
   one-line rationale.
5. Close with **Interacts with:** which other candidates'
   swaps this one depends on, blocks, or simplifies.

Per-candidate prose: 300–400 words for Eval A revisits and
single-library Eval B candidates; up to 600 for the
list-vs-bubblelister compare.

## Fork-vs-accept call

A closing section names every **(b)** candidate — bubbles whose
render path calls `lipgloss.Width` internally, where ansix in
poplar's call sites doesn't help because the library does its
own width math. From the queued STATUS entry these are at least
`bubbles/help`, `bubble-table`, and `glamour` (the third is
speculative — not currently a consumer; included only if the
fork would unlock its future use).

The decision is binary:

- **Fork** — `go.mod replace` for either `charmbracelet/x/ansi`
  or `charmbracelet/lipgloss`, swapping `ansi.StringWidth` /
  `lipgloss.Width` for an ansix-equivalent. Permanent rebase
  cost. Explicitly supersedes ADR-0002 and ADR-0075. Unlocks
  every gated bubble.
- **Accept** — these bubbles stay hand-rolled. The fork cost
  exceeds the adoption value.

Decision factors to weight:

- **Breadth of bubbles unlocked** by the fork. If only one
  candidate flips to Adopt under fork, the math favors accept.
- **Upstream PR path.** Memory note
  `reference_displaywidth_contribution_patterns.md` records
  PR-first, Copilot-reviewed contribution culture for
  `clipperhouse/displaywidth`. If x/ansi or lipgloss accepts
  a configurable width hook upstream, the fork becomes
  temporary instead of permanent. Worth checking before
  defaulting to accept.
- **Maintenance burden** of the rebase. x/ansi cadence is
  weekly; lipgloss is monthly. Both are charmbracelet-owned
  with stable APIs.
- **What "accept" forecloses.** Any future bubble whose render
  path uses `lipgloss.Width` internally is ineligible by
  default. Worth naming the cost concretely.

The verdict is one of two strings, with a paragraph of
rationale and any conditional follow-ups (e.g., "accept
unless upstream PR lands by Pass 9z").

## Task budget

11 tasks:

1. Re-read ansix capabilities — what `ansix.Width` /
   `ansix.Truncate` do, where they're used now, why they
   diverge from the upstream `ansi.StringWidth`.
2. Re-eval `rmhubbert/bubbletea-overlay`.
3. Re-eval `bubbles/help`.
4. Re-eval `daltonsw/bubbleup`.
5. Re-eval `charmbracelet/huh`.
6. Re-eval `evertras/bubble-table`.
7. Eval `bubbles/list` × picker sites (batched).
8. Eval `bubbles/list` vs `treilik/bubblelister` for compose
   dropdown.
9. Eval `bubbles/list` for sidebar folder column.
10. Eval `knipferrc/teacup` statusbar.
11. Fork-vs-accept synthesis.

Fits the 8–12 task budget.

## Out of scope

- **Eval C (lesson harvests)** — different shape (concrete-diff
  requirement against kept-custom code); stays as Pass 9y.
- **Glamour full eval** — not currently a poplar consumer. The
  fork-vs-accept call names it only if the fork option would
  unlock its future use.
- **Code changes** — research pass; no `internal/` edits.
- **ADRs** — eval produces no architectural decisions; only
  Pass 9z's consolidation roadmap and the subsequent swap
  passes write ADRs.

## Risks

**The re-eval finds nothing flips.** Possible. ansix's call
sites are inside poplar; library-internal width calls are
unaffected. If every Eval A "Keep" verdict survives and every
Eval B candidate also fails, the pass output is "fork-vs-accept
is the only meaningful lever." The fork-vs-accept synthesis
becomes load-bearing. This is fine — the pass still concludes
the bubble effort's open question.

**The fork option requires upstream investigation.** Checking
whether x/ansi or lipgloss would accept a width-hook PR is a
real research task, not a paragraph guess. Budgeted inside
task 11.

**Eval B `bubbles/list` for sidebar may bog down.** Sidebar is
custom for principled reasons (T9 contacts groups, three fixed
folder groups, search shelf composition). The eval may end
"Keep + harvest" quickly. Worth not over-investing.

## Next step

Invoke `writing-plans` to produce the plan doc for Pass 9w.2.
The plan turns the 11 tasks into concrete steps and feeds into
pass execution per the poplar-pass starter prompt.
