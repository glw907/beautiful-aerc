# Bubble Adoption — Design

A multi-pass initiative to evaluate poplar's hand-rolled
`internal/ui/` components against the official and community
bubbletea component ecosystem, adopt where it makes poplar better,
and harvest lessons where the hand-roll stays.

Pre-beta posture (see `CLAUDE.md`) explicitly welcomes refactoring
and breaking changes. ADRs 0173 and 0174 already establish
"idiomatic bubbletea first." This work executes that posture
across the surface area `internal/ui/` accumulated before those
ADRs landed.

## Core question

> **Does using the community bubble make poplar better?**
> If yes, *why* — what concrete gains. If no, *why not* — what
> losses or non-fits.

The rubric below is the evidence each candidate's verdict rests
on. The rubric is not the question.

## Eval rubric

Six dimensions, applied per candidate:

1. **Feature parity** — does the bubble cover what poplar's
   hand-roll does today, or are there gaps? Gaps that matter (vs
   gaps we don't care about) are called out by name.
2. **Customization seams** — can we wire our 15 themes, our
   modifier-free single-key bindings, and our domain state (toast
   triage payload, messagelist threading depth, etc.) without
   forking upstream? Usually the decisive criterion.
3. **Theming integration** — concrete test: does the bubble
   accept `lipgloss.Style` injection, or own its own colors? An
   "owns colors" library is a fork target, not an adopt target.
4. **Maintenance signal** — last commit, version cadence, releases
   in the past twelve months. Not a veto unless the project is
   dead. Informs forking risk.
5. **Code delta estimate** — rough LOC removed from poplar vs LOC
   added (shim + integration). Not decisive on its own; a 50→500
   swap is a tell.
6. **License** — MIT, BSD, or Apache only. Hard veto otherwise.

Two dimensions deliberately excluded:

- **Stars / popularity.** Already factored into prior-art surveys.
  Re-litigating noise.
- **Churn cost.** Pre-beta posture forbids it as a defer rationale
  (`CLAUDE.md`, "Anti-pattern when triaging review findings").
  Including it as a rubric dimension would smuggle it back in.

## Three eval passes by adoption tier

### Eval A — Strong matches

Likely-adopt candidates. Eval confirms feature parity, identifies
seam shape, and ranks swap order.

- `rmhubbert/bubbletea-overlay` — modal & overlay positioning
  across the App overlay cascade.
- `bubbles/help` — replaces `helppopover` key-rendering internals.
- `daltonsw/bubbleup` — toast notification chrome.
- `charmbracelet/huh` — form scaffolding for `contacts/Form` and
  the upcoming first-run wizard (#27).
- `evertras/bubble-table` — flat-list parts of `messagelist` and
  `contacts/List`.

### Eval B — Partial matches

Mixed verdicts expected. Eval reads code closely, sketches the
seam, and may end "Keep + harvest" as often as "Adopt."

- `bubbles/list` for `movepicker`, `reader.LinkPicker`,
  `reader.AttachPicker`.
- `bubbles/list` or `treilik/bubblelister` for `compose.Dropdown`.
- `bubbles/list` for `sidebar` (folder column; T9 contacts groups
  excluded).
- `knipferrc/teacup` statusbar for `status_bar`.

### Eval C — Lesson harvests

Different shape. Not "swap?" but "what to learn from how the
bubble does it?" Targets justified hand-rolls.

- `messagelist` threading layer (depth-prefix walk, fold/unfold).
- `reader.Model` (phase state machine, link harvesting).
- `App` orchestration (overlay cascade, modal lifecycle).
- Domain chrome — `error_banner`, `top_line`, `footer`,
  `confirm_modal`, `conflict_overlay`, `outbox_overlay`.

## Eval pass shape

Each eval pass is **research and writing only — no code changes.**

**Per-candidate process:**

1. Read the bubble's source and docs (actual `*.go` for the
   public surface, not just README).
2. Read poplar's current component carefully.
3. Sketch the integration mentally — seam shape, shims, theme
   hooks, keymap shape.
4. Write the candidate section: lead with the core question
   answered concretely, then rubric evidence, then verdict
   (**Adopt / Adopt-with-fork / Keep + harvest**) with a one-line
   rationale.
5. Close with **Interacts with:** which other candidates' swaps
   this one depends on, blocks, or simplifies.

**Per-pass roadmap section:** every eval pass closes with a
dependency graph (text DAG) and a ranked linearization. Example
sketch for Eval A:

```
bubbletea-overlay  ──┬──>  bubbles/help   (mounts in modal frame)
                     ├──>  bubbleup       (positioning)
                     ├──>  huh            (form mounts in modal)
                     └──>  Eval B pickers (later)

evertras/bubble-table  ──>  (independent — messagelist + contacts/List
                              both consume; one fork serves both)
```

**Output format:** triage spec under
`docs/superpowers/specs/YYYY-MM-DD-bubble-eval-<tier>.md`. No ADR
from an eval pass — eval is research, not architectural decision.
ADRs land when swap passes commit code.

**Eval pass size budgets:**

| Pass    | Candidates | Per-candidate words | Doc length     |
|---------|-----------:|--------------------:|---------------:|
| Eval A  | 5          | 300–400             | ~1500–2000     |
| Eval B  | 4          | 300–400             | ~1200–1600     |
| Eval C  | 4–6 areas  | 150–250 + diff list | ~1000–1500     |

Each fits inside one pass at 5–7 tasks (one task per candidate
plus the roadmap section).

## Consolidation pass

After Eval A, B, and C land, a short consolidation pass produces
the unified roadmap doc. Inputs: the three triage specs and their
dependency graphs. Output: a single ordered list of swap and
harvest passes, ranked by leverage × risk, with cross-tier
dependencies merged.

This is what makes the effort "conclude with a logical plan."
Without it, every swap-pass boundary becomes an ad-hoc "what's
next" decision. With it, the schedule is read off one document.

## Swap pass shape

One swap pass per adopted bubble, sometimes batched when seams
are shared (e.g., the three pickers consuming `bubbles/list` with
the same delegate likely become one pass).

**Plan:** under `docs/superpowers/plans/`, 8–12 tasks.

**Tasks shape:**

1. Introduce seam (interface or accessor that lets the new bubble
   mount where the hand-roll used to).
2. Port one site as the template — smallest, lowest-risk surface.
3. Port remaining sites.
4. Port styles into the per-subpackage `Styles` constructor.
5. Port keymaps into the bubble's keymap shape (or shim).
6. Port tests; add new ones for the seam.
7. Delete dead code.
8. Tmux verification at 80×24 and 120×40.

**ADR:** documents the integration shape — where the bubble plugs
in, what shims exist, what we forked, and which findings from the
eval changed during the swap.

**Mid-swap rollback policy:** if the bubble turns out not to fit
during the swap (typically discovered in task 1 or 2), abandon and
write the ADR explaining why. Pre-beta tolerates this. Eval
reduces this risk; it does not eliminate it.

**Spike before commit:** task 1 of every swap pass is a
spike at the smallest possible site. If the spike fails, abandon
before further investment.

## Harvest pass shape

Output of Eval C. Different from a swap: small targeted
improvements applied to kept-custom components.

- Tasks read like "apply lesson *X* from `bubbles/list`'s delegate
  pattern to `messagelist`'s row renderer."
- Likely batches 2–4 harvests per pass.
- ADR captures "we kept *X* but adopted technique *Y* from *Z*"
  for each lesson.
- **Concrete-diff requirement:** every lesson in Eval C must state
  a concrete diff target — not "consider improving X" but "rename
  *Y* to match `bubble-table`'s convention." A lesson that can't
  be a diff is a vibe; drop it.

## Sequencing & pass numbering

Pre-beta letter sequence is at 9p. Bubble adoption claims `9v`
through `9y` for eval and consolidation; the foundational swaps
interleave into the existing feature queue.

| Pass        | What                                                     |
|-------------|----------------------------------------------------------|
| **9v**      | Eval A — strong matches                                  |
| **9v.1**    | Swap — `bubbletea-overlay` (foundational)                |
| **9q**      | Outbox delivery controls (#35) — modal on overlay        |
| **9v.2**    | Swap — `huh` (foundational)                              |
| **9r–9t**   | List-Unsubscribe (#36), .ics (#37), search (#38)         |
| **9u**      | First-run wizard (#27) + OAuth (#29) — wizard on huh     |
| **9w**      | Eval B — partial matches                                 |
| **9x**      | Eval C — lesson harvests                                 |
| **9y**      | Consolidation roadmap                                    |
| **9y.1+**   | Remaining swap & harvest passes per 9y's roadmap         |
| **10**      | Polish II                                                |
| **11**      | v0.9.0 prep — feature freeze                             |

Notes:

- **Eval A → swap-overlay → 9q** is a two-pass detour before the
  outbox feature. Per the earlier sequencing decision, this is
  acceptable: pre-beta has no shipping clock, and #35 has been
  queued without urgency markers.
- **`bubbles/help`, `bubbleup`, `bubble-table`** have no feature-
  queue dependencies. They're scheduled by 9y, not pre-positioned.
  Only the swaps that *block* feature work jump the queue.
- **Pass numbering after 9y** is set by the consolidation roadmap.
  Eval C's harvest passes use the same `9y.N` namespace as swaps;
  the roadmap orders them together.

## Risks & open items

**Mid-swap rollback discovers the eval got it wrong.** Eval depth
(read source, sketch seam) reduces this. Mitigation already
specified: spike at smallest site as task 1 of every swap.

**`bubbletea-overlay` may not model the cascade.** Poplar's
overlay cascade has explicit ordering — confirm > conflict >
outbox > help > linkpicker > attachpicker > movepicker > form >
popover. If the library only handles single-overlay z-ordering,
the swap shape changes from "delete cascade code" to "thin wrapper
around cascade code." Eval A must answer this before 9v.1
schedules.

**`huh` integration inside Contacts mode.** Contacts mode is a
three-pane layout (sidebar + list + form column). `huh` is
designed as a full-screen form library. Eval A must verify `huh`
exposes a `tea.Model` integration that lets us mount it in a
sub-pane. If not, the verdict shifts to **Adopt-with-fork** and
9v.2 becomes a fork pass.

**License audit.** Every candidate must be MIT, BSD, or Apache.
Skim happens in eval; veto is hard.

**Eval C lesson ambiguity.** Already addressed by the concrete-
diff requirement above.

## Next step

Invoke `writing-plans` to produce the plan doc for **Pass 9v —
Eval A (strong matches)**. That plan turns the per-candidate
process into 5–7 concrete tasks and feeds into pass execution
via `subagent-driven-development` or direct execution per the
poplar-pass starter prompt.
