# Pass 9x.2 — Chrome family harvest implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal.** Produce a chrome-family pattern catalog modeled on the
Pass 9x.1 list-patterns doc, harvesting design ideas from
`bubbles/help`, `knipferrc/teacup` (statusbar), and
`daltonsw/bubbleup` against poplar's `helppopover`, `status_bar`,
and `toast` surfaces. Land 0–2 concrete diffs per surface where a
pattern obviously improves the consumer.

**Architecture.** Research-shaped pass: read upstream source, read
poplar's consumer, write the catalog doc, then apply any
unambiguous wins inline. One research doc at
`docs/poplar/research/2026-05-08-chrome-family-patterns.md`. ADR
only if a binding fact changes (none expected — Eval B already
ruled all three Keep + harvest).

**Tech stack.** `bubbles/help@v1.0.0` (in go.mod cache at
`~/go/pkg/mod/github.com/charmbracelet/bubbles@v1.0.0/help/help.go`),
`mistakenelf/teacup` and `daltonsw/bubbleup` (not in go.mod — read
from GitHub via WebFetch). Poplar consumers live at
`internal/ui/helppopover/`, `internal/ui/status_bar.go`, and
`internal/ui/toast.go`.

**Settled inline (do not re-brainstorm — see brainstorm log in
this conversation):**

- `bubbles/help`'s `KeyMap` ShortHelp/FullHelp does not replace
  poplar's named-group binding tables. Catalog patterns only.
- teacup's four-slot model does not fit the status-bar's
  six-segment drop-rank shape. Catalog patterns only.
- bubbleup has no queue model — single-alert per-category
  styling, no payload — nothing to add to the App-level cascade
  order.

---

### Task 1: Read `bubbles/help` source and capture pattern notes

**Files:**

- Read: `~/go/pkg/mod/github.com/charmbracelet/bubbles@v1.0.0/help/help.go`
- Read: `~/go/pkg/mod/github.com/charmbracelet/bubbles@v1.0.0/help/help_test.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/helppopover/model.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/helppopover/styles.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/helppopover/model_test.go`
- Scratch notes: kept in conversation context, not committed

- [ ] **Step 1.1.** Read the four `bubbles/help` files end-to-end.
  Walk every exported and unexported symbol. Note the `KeyMap`
  interface (`ShortHelp() []key.Binding`, `FullHelp() [][]key.Binding`),
  the `Styles` struct, the `Model` field set, `ShortHelpView`,
  `FullHelpView`, the column-build loop, the `lipgloss.JoinHorizontal`
  call sites, and `ellipsis` truncation behavior.

- [ ] **Step 1.2.** Read `helppopover/model.go` end-to-end. Note the
  `bindingGroup`/`bindingRow` shape, the `wired bool` per row, the
  named-group grid (Primary / Disposal / Custom), the Go To 3×2
  tile, the bottom hint line, the `Box` cache, and the `dim`
  composition pattern.

- [ ] **Step 1.3.** For each pattern observed in `bubbles/help`,
  decide one of: *applied now in poplar* (cite where), *candidate
  for later* (state the trigger), or *no — with rationale*. Record
  candidate patterns: `KeyMap` as a value type with `key.Binding`
  fields; `Styles` field naming; `ShortHelpView` separator pattern;
  `lipgloss.StyleRunes`-style layered styling if used; the
  ellipsis-on-overflow pattern.

- [ ] **Step 1.4.** Identify 0–2 concrete diffs that obviously
  improve `helppopover` *without* changing the named-group grid
  contract. Examples to test the bar against: extracting a
  `KeyMap` value type that exposes `ShortHelp()` for the bottom
  hint line; renaming `Styles` slots to follow the
  combined-state convention. If neither clears the "obvious win"
  bar, record zero diffs and move on.

### Task 2: Read `mistakenelf/teacup` statusbar source and capture pattern notes

**Files:**

- WebFetch: `https://raw.githubusercontent.com/mistakenelf/teacup/main/statusbar/statusbar.go`
  (the module renamed from `knipferrc/teacup`; main branch is the
  current source)
- Read: `/home/glw907/Projects/poplar/internal/ui/status_bar.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/status_bar_test.go`
- Scratch notes: kept in conversation context, not committed

- [ ] **Step 2.1.** Fetch `statusbar.go` from `mistakenelf/teacup`'s
  main branch. Walk every exported symbol. Note `ColorConfig`, the
  four-column shape (`FirstColumn`…`FourthColumn`), the `SetSize` /
  `SetContent` / `SetColors` API, the truncation rule (`muesli/reflow`,
  30-char cap on first column), the final `lipgloss.JoinHorizontal`
  assembly.

- [ ] **Step 2.2.** Read `status_bar.go` end-to-end. Note the six
  segments (fill rule, ┴ junction at sidebar divider, count /
  scroll-pct toggle, outbox depth + glyph, connection state +
  shape, `─╯` terminator), the `View(width, dividerCol)` signature,
  `renderOutboxSegment`, `buildFill`, the `ConnectionState` type,
  and the `StatusMode` toggle.

- [ ] **Step 2.3.** For each pattern observed in teacup, decide
  *applied now / candidate / no*. Record candidate patterns: the
  `ColorConfig{Foreground, Background lipgloss.AdaptiveColor}`
  pair convention; the per-slot `SetColors` injection seam; the
  `muesli/reflow/truncate` ellipsis rule. Anti-pattern catch:
  teacup's final `JoinHorizontal` is exactly the ADR-0084 ban —
  catalog as a confirming counter-example to poplar's pre-measure +
  fill approach.

- [ ] **Step 2.4.** Identify 0–2 concrete diffs against
  `status_bar.go`. Bar: an obvious improvement that does not
  weaken the SPUA-safe assembly. Most likely zero given the
  domain mismatch.

### Task 3: Read `daltonsw/bubbleup` source and capture pattern notes

**Files:**

- WebFetch: `https://raw.githubusercontent.com/daltonsw/bubbleup/main/alert.go`
- WebFetch: `https://raw.githubusercontent.com/daltonsw/bubbleup/main/bubbleup.go`
  (or the equivalent — list the repo first to confirm filenames)
- Read: `/home/glw907/Projects/poplar/internal/ui/toast.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/toast_test.go`
- Scratch notes: kept in conversation context, not committed

- [ ] **Step 3.1.** Fetch the bubbleup source files from the main
  branch. If the file layout is unclear, WebFetch
  `https://api.github.com/repos/daltonsw/bubbleup/contents/` first
  to list the tree. Walk every exported symbol. Note
  `AlertModel`, `AlertDefinition{Key, ForeColor, Prefix, Style}`,
  the color-lerp tick (≈6 ticks at 100 ms via `go-colorful.BlendLab`),
  the `WithPosition` / `WithMinWidth` / `WithUnicodePrefix` /
  `WithAllowEscToClose` functional options, `RegisterNewAlertType`,
  and `NewAlertCmd`.

- [ ] **Step 3.2.** Read `toast.go` end-to-end. Note `pendingAction`,
  the `op` enum (`TriageOp` from `uicore`), affected message count,
  destination name, the inverse `tea.Cmd` (undo payload),
  `renderToast`, `toastExpireMsg`, the `tea.Tick` deadline, and the
  chrome-banner inline-row contract.

- [ ] **Step 3.3.** For each pattern observed in bubbleup, decide
  *applied now / candidate / no*. Record candidate patterns: the
  functional-options registration pattern; the per-category
  `AlertDefinition` style table; the dismiss-on-Esc affordance.
  Anti-pattern catch: chrome (rounded border + position enum)
  baked into the rendering primitive instead of layered above it.

- [ ] **Step 3.4.** Identify 0–2 concrete diffs against `toast.go`.
  Bar: improves the triage-confirmation bar without grafting the
  category-styled animation model on top. Likely zero given the
  domain mismatch.

### Task 4: Draft the research doc — per-surface sections

**Files:**

- Create: `/home/glw907/Projects/poplar/docs/poplar/research/2026-05-08-chrome-family-patterns.md`
- Reference: `/home/glw907/Projects/poplar/docs/poplar/research/2026-05-08-bubbles-list-patterns.md`
  (the 9x.1 catalog — copy its section shape exactly)

- [ ] **Step 4.1.** Write the doc header. One paragraph naming the
  pass, the three libraries surveyed, and the companion eval at
  `docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`.
  State that every verdict was Keep + harvest and that the doc
  catalogs *what to keep in mind*. State the per-entry shape:
  pattern → upstream location (file:line) → why it's good →
  poplar applicability (applied now / candidate for later / no —
  with rationale).

- [ ] **Step 4.2.** Write the `bubbles/help` section. One pattern
  entry per item from Task 1.3 that survived the
  applied-now/candidate filter. Cite each upstream call out with
  `help.go:LINE` precision. Cross-reference 9x.1 patterns by
  number when the pattern is a re-statement (`KeyMap` value type
  → see 9x.1 §9; do not duplicate the entry, link it).

- [ ] **Step 4.3.** Write the teacup statusbar section. Same
  shape. Include the ADR-0084 anti-pattern callout from Step 2.3
  as one of the entries — naming what poplar deliberately *does
  not* do is part of the catalog's value.

- [ ] **Step 4.4.** Write the bubbleup section. Same shape.
  Include the chrome-baked-into-primitive anti-pattern as an
  entry. Note that the cascade-precedence question raised in the
  starter prompt is App-level, not library-level — bubbleup has
  no queue.

### Task 5: Draft the research doc — synthesis sections

**Files:**

- Modify: `/home/glw907/Projects/poplar/docs/poplar/research/2026-05-08-chrome-family-patterns.md`
  (continuing from Task 4)

- [ ] **Step 5.1.** Write the "Top three ways their code beats
  ours" section. Pick the three strongest candidate patterns
  across all three libraries — these feed Pass 9z's adoption
  roadmap. If fewer than three reach the bar, state that
  explicitly and explain why; do not pad. (Pass 9x.1's section
  is the exemplar — three patterns, each with a one-sentence
  rationale tied to the consumer.)

- [ ] **Step 5.2.** Write the "What did NOT survive the harvest"
  section. List patterns considered and rejected with one-line
  rationale each. Examples to test: bubbleup's color-lerp tick;
  teacup's 30-char first-column cap; `bubbles/help`'s
  `Ellipsis` style slot; the four-slot statusbar shape.

- [ ] **Step 5.3.** Write the "Cross-cutting code candidates"
  section. For any helper that two or more chrome surfaces could
  share (per the patterns surfaced), name it and recommend
  *extract now* or *defer until N consumers*. Match 9x.1's
  conservative posture — three call sites is borderline; two is
  too few.

- [ ] **Step 5.4.** Run `wc -l` on the doc. Eyeball it against
  the 9x.1 catalog (~270 lines). If radically shorter, recheck
  Task 1–3 for missed patterns; if radically longer, prune
  redundant entries.

- [ ] **Step 5.5.** Commit the catalog doc:

```
git add docs/poplar/research/2026-05-08-chrome-family-patterns.md
git commit -m "Pass 9x.2 research: chrome family pattern catalog

Patterns harvested from bubbles/help, mistakenelf/teacup
statusbar, and daltonsw/bubbleup against poplar's helppopover,
status_bar, and toast surfaces. Companion to the Eval B verdicts
(all three Keep + harvest). Top-three list feeds Pass 9z.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

### Task 6: Apply concrete diffs (0–2 per surface)

**Files (conditional on Tasks 1.4 / 2.4 / 3.4):**

- Modify: `/home/glw907/Projects/poplar/internal/ui/helppopover/*.go`
- Modify: `/home/glw907/Projects/poplar/internal/ui/status_bar.go`
- Modify: `/home/glw907/Projects/poplar/internal/ui/toast.go`

- [ ] **Step 6.1.** For each diff identified in Tasks 1.4, 2.4,
  3.4: apply it in a separate commit. Each commit message names
  the upstream pattern and the surface
  ("Pass 9x.2: <surface> — adopt <pattern> from <library>").
  Update the catalog entry in the research doc to flip its
  applicability marker from *candidate for later* to *applied
  now* with the commit SHA when the diff lands.

- [ ] **Step 6.2.** If `internal/ui/` was touched by any diff,
  run the **§10 Review checklist** from
  `docs/poplar/bubbletea-conventions.md` against the diff. Each
  item must verify from the diff or a 120×40 tmux capture.
  Note any deviations in the pass-end ADR (Task 8) if any are
  introduced.

- [ ] **Step 6.3.** Run `make check`. Fix any failures inline
  before proceeding.

- [ ] **Step 6.4.** Live-verify: `make install`, then via tmux
  per `.claude/docs/tmux-testing.md`, capture each touched
  surface at 80×24 and 120×40. Eyeball for visual regressions.

- [ ] **Step 6.5.** If zero diffs landed, this task is empty —
  move to Task 7.

### Task 7: /simplify on the diff

- [ ] **Step 7.1.** Invoke the `simplify` skill against the pass
  diff. Apply genuine wins; ignore noise. If zero code diffs
  landed in Task 6, the simplify pass is over the doc only —
  voice lens applies (tells T4, T10, T14, T16, T27, T28, T33,
  T35, T39, T40 plus T34 voice-only).

### Task 8: Pass-end consolidation ritual

**Files:**

- Possibly create: `/home/glw907/Projects/poplar/docs/poplar/decisions/0182-...md`
  (only if a binding fact moved — not expected)
- Modify: `/home/glw907/Projects/poplar/docs/poplar/STATUS.md`
- Move: `docs/superpowers/plans/2026-05-08-harvest-chrome.md` →
  `docs/superpowers/archive/plans/2026-05-08-harvest-chrome.md`
- Possibly modify: `/home/glw907/Projects/poplar/docs/poplar/invariants.md`
  (only if a binding fact moved)

- [ ] **Step 8.1.** ADR decision. If Task 6 landed any binding-
  fact change (a new component contract, a renamed exported
  type, a swapped library), write a new ADR under
  `docs/poplar/decisions/` with the next number. If not — the
  expected case — skip this step. Add a one-line note to the
  research doc confirming "no ADR" and the pass commit message
  carries the rationale.

- [ ] **Step 8.2.** If an ADR was written, update
  `docs/poplar/invariants.md` in place per the
  consolidation-ritual rules (edit, do not append). Update
  `docs/poplar/decisions/INDEX.md`. If no ADR, skip.

- [ ] **Step 8.3.** Update `STATUS.md`. Mark Pass 9x.2 done.
  Replace the starter prompt with Pass 9x.3's (table/form
  family — `huh`, `bubble-table` against `internal/ui/compose/`
  and `internal/ui/messagelist/`). Keep the file ≤60 lines.

- [ ] **Step 8.4.** `git mv` the plan to the archive:

```
git mv docs/superpowers/plans/2026-05-08-harvest-chrome.md \
  docs/superpowers/archive/plans/2026-05-08-harvest-chrome.md
```

  (No spec to archive — research-only pass.)

- [ ] **Step 8.5.** `make check` — must be green.

- [ ] **Step 8.6.** Commit, push, install:

```
git add -A
git commit -m "Pass 9x.2: chrome family harvest

Catalogs design patterns from bubbles/help, mistakenelf/teacup
statusbar, and daltonsw/bubbleup against poplar's helppopover,
status_bar, and toast. <0–2 diffs landed inline | no diffs —
Eval B's Keep + harvest verdicts hold.> Top-three list feeds
Pass 9z's adoption roadmap.

Co-Authored-By: Claude <noreply@anthropic.com>"
git push
make install
```

---

## Self-review notes

- **Spec coverage.** Three open questions from the starter prompt
  are pre-settled in the brainstorm log; the plan catalogs each
  library × consumer pair (Tasks 1–3), produces the catalog doc
  with the required per-entry shape and synthesis sections (Tasks
  4–5), and applies 0–2 concrete diffs per surface where the bar
  is met (Task 6). The "top three" requirement from the starter
  prompt is Step 5.1.
- **Placeholder scan.** No "TBD" / "implement later" steps; each
  step is one action with concrete file paths and either an
  expected output or a decision rule. The 0–2 diff bar is
  decision-driven, not under-specified — the rule is named in
  Step 1.4 / 2.4 / 3.4 ("obvious improvement that does not change
  the contract").
- **Type consistency.** No code types defined across tasks; the
  research doc is the deliverable.
- **Pass size.** 8 tasks, well under the 12-task budget. ADR
  conditional and expected to be skipped — pre-beta posture
  permits binding-fact changes inline if any pattern adoption
  warrants one.
