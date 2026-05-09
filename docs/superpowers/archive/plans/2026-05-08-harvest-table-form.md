# Pass 9x.3 — Table/form family harvest implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal.** Produce a table/form-family pattern catalog modeled on
the Pass 9x.1 list-patterns and Pass 9x.2 chrome-family docs,
harvesting design ideas from `charmbracelet/huh` and
`evertras/bubble-table` against poplar's `internal/ui/contacts/form.go`,
`internal/ui/messagelist/`, and `internal/ui/contacts/list.go`.
Land 0–2 concrete diffs per surface where a pattern obviously
improves the consumer.

**Architecture.** Research-shaped pass: read upstream source, read
poplar's consumers, write the catalog doc, then apply any
unambiguous wins inline. One research doc at
`docs/poplar/research/2026-05-08-table-form-patterns.md`. ADR
only if a binding fact changes (none expected — Eval B already
ruled both Keep + harvest with two structural blockers each).

**Tech stack.** `huh@v1.0.0` (in go.mod cache at
`~/go/pkg/mod/github.com/charmbracelet/huh@v1.0.0/`),
`evertras/bubble-table` (not in go.mod — read from GitHub via
WebFetch). Poplar consumers live at
`internal/ui/contacts/form.go` (897 LOC), `internal/ui/messagelist/model.go`
(1095 LOC), and `internal/ui/contacts/list.go` (266 LOC).

**Settled inline (do not re-brainstorm — see brainstorm log in
this conversation):**

- huh's `Theme.Focused FieldStyles` / `Theme.Blurred FieldStyles`
  symmetric pair does not reshape `contacts.Styles`. Three
  state-keyed pairs (`FieldFocus`/`FieldBlur`, `KindOn`/`KindOff`,
  the dead `Border`) is below the threshold where a state-prefix
  sub-struct beats flat naming; huh's pattern fits a many-
  field-type library, not poplar's narrow form. Catalog as a
  reference pattern.
- bubble-table's `WithRowStyleFunc(RowStyleFuncInput → lipgloss.Style)`
  callback does not reshape `messagelist.renderRow` or
  `contacts/list.formatRow`. Both consumers own their predicate
  state directly (`displayRow` flags, cursor index); the
  function-injection seam fits a library that does not know its
  consumers, not a closed render loop with ≤6 row-state
  combinations. Catalog only.
- Inline cleanup: `contacts.Styles.Border` is unused (zero call
  sites under `internal/`). Pre-beta posture removes it as
  adjacent-fix during the form-styles sweep — not a harvest
  pattern, but lands in this pass since the work touches that
  file.

---

### Task 1: Read `charmbracelet/huh` source and capture pattern notes

**Files:**

- Read: `~/go/pkg/mod/github.com/charmbracelet/huh@v1.0.0/theme.go`
- Read: `~/go/pkg/mod/github.com/charmbracelet/huh@v1.0.0/group.go`
- Read: `~/go/pkg/mod/github.com/charmbracelet/huh@v1.0.0/form.go`
- Read: `~/go/pkg/mod/github.com/charmbracelet/huh@v1.0.0/field_input.go`
- Read: `~/go/pkg/mod/github.com/charmbracelet/huh@v1.0.0/field_select.go`
- Read: `~/go/pkg/mod/github.com/charmbracelet/huh@v1.0.0/field_confirm.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/contacts/form.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/contacts/styles.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/contacts/form_test.go` if present
- Scratch notes: kept in conversation context, not committed

- [ ] **Step 1.1.** Read the huh theme + group + form + three field
  source files. Walk the `Theme` struct (`Focused`/`Blurred`
  `FieldStyles`), the `FieldStyles` slot inventory, the
  `TextInputStyles` sub-struct, the `WithTheme(*Theme)` /
  `WithKeyMap(*KeyMap)` / `WithWidth` / `WithHeight` builder
  options, the `Field` interface (`Init`/`Update`/`View`/`Focus`/`Blur`/`Error`/`Skip`/`KeyBinds`/`WithTheme`/etc.),
  the `Group.View()` chrome stack (title + viewport + help row),
  and `selector.Selector[Field]` (the frozen field slice).

- [ ] **Step 1.2.** Read `contacts/form.go` end-to-end. Note the
  `(input, cycler, ★, −)` quartet render path
  (`renderEmailRow`/`renderPhoneRow`), `focusList()` rebuild,
  `applyFocus()` re-anchor, `fromPopover` dual-render branch,
  the kind toggle (Person/Business with cycler), the
  `currentContact()`-vs-initial dirty check, validation in
  `Save()`, and the `ContactSaveMsg`/`ContactCancelMsg`/
  `OpenContactDeleteConfirmMsg` boundary. Note the `Styles`
  slot usage at every `f.styles.X.Render(...)` call site.

- [ ] **Step 1.3.** For each pattern observed in huh, decide
  *applied now in poplar* (cite where), *candidate for later*
  (state the trigger), or *no — with rationale*. Record
  candidate patterns: the `Focused`/`Blurred` `FieldStyles`
  symmetric pair; the `WithTheme(*Theme)` propagation through
  Form→Group→Field; the `TextInputStyles` sub-struct
  (`Cursor`/`CursorText`/`Placeholder`/`Prompt`/`Text`); the
  builder-options shape (`WithWidth`/`WithHeight`/`WithKeyMap`);
  the `Field.Skip()` hook; the `selector.Selector[Field]` frozen
  slice as anti-pattern (poplar's `focusList()` rebuild is the
  right shape for dynamic rows).

- [ ] **Step 1.4.** Identify 0–2 concrete diffs that obviously
  improve `contacts/form.go` *without* touching the dynamic-row
  or dual-render contracts. Example to test the bar against:
  introducing a `TextInputStyles`-style sub-struct to group the
  three input-related slots in `contacts.Styles`. The dead
  `Border` removal is not a harvest pattern; it lands inline
  in Task 5 as adjacent cleanup. If neither structural diff
  clears the "obvious win" bar, record zero structural diffs.

### Task 2: Read `evertras/bubble-table` source and capture pattern notes

**Files:**

- WebFetch: `https://raw.githubusercontent.com/Evertras/bubble-table/main/table/view.go`
- WebFetch: `https://raw.githubusercontent.com/Evertras/bubble-table/main/table/row.go`
- WebFetch: `https://raw.githubusercontent.com/Evertras/bubble-table/main/table/model.go`
- WebFetch: `https://raw.githubusercontent.com/Evertras/bubble-table/main/table/options.go`
  (or the equivalent — fetch
  `https://api.github.com/repos/Evertras/bubble-table/contents/table`
  first to confirm filenames)
- Read: `/home/glw907/Projects/poplar/internal/ui/messagelist/model.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/messagelist/styles.go`
- Read: `/home/glw907/Projects/poplar/internal/ui/contacts/list.go`
- Scratch notes: kept in conversation context, not committed

- [ ] **Step 2.1.** Fetch the four bubble-table source files. Walk
  every exported symbol. Note the `RowData map[string]any` shape,
  `StyledCell{Data, Style/StyleFunc}`, `WithRowStyleFunc(func(RowStyleFuncInput) lipgloss.Style)`,
  `WithBaseStyle`, the column definition (`NewColumn`/`NewFlexColumn`),
  fixed-vs-flex column width math, the `renderRowData` `lipgloss.JoinHorizontal`
  call (row.go:243 per Eval B), and the `lipgloss.Width` calls
  in `view.go` and `row.go` (Eval B identified three).

- [ ] **Step 2.2.** Read `messagelist/model.go` end-to-end with
  attention to: `displayRow` shape and flags, `appendThreadRows`
  thread-pipeline, `renderRow` styling switch (read/unread/
  flagged/cursor/visual-marked), `mlFlagWidth = 2` SPUA-A pad,
  `formatRelativeDate`, `ActionTargets()` triage scope,
  `RefreshSource` and `SetMessages`. Read `contacts/list.go`
  end-to-end: `formatRow` fixed four-column layout
  (`metaCol`/`nameCol`), `SetSelectionLetter` letter-jump nav,
  `rebuildViewport`/`syncViewport`, `sortContacts`.

- [ ] **Step 2.3.** For each pattern observed in bubble-table,
  decide *applied now / candidate / no*. Record candidate
  patterns: `WithRowStyleFunc` callback shape; `StyledCell`
  per-cell style override; `RowData map[string]any` indirection;
  `NewFlexColumn` flex-width column model; `WithBaseStyle`
  global theme injection; the column-definition + RowData
  separation. Anti-pattern catch: `lipgloss.JoinHorizontal` in
  `renderRowData` (the (b)-list blocker per Eval B / ADR-0084)
  and internal `lipgloss.Width` calls that bypass ansix.

- [ ] **Step 2.4.** Identify 0–2 concrete diffs against
  `messagelist` and `contacts/list` *each*. Bar: an obvious
  improvement that does not graft the table model onto the
  thread pipeline or onto the letter-jump nav. Most likely
  zero per surface given the consumer-fit gap.

### Task 3: Draft the research doc — per-surface sections

**Files:**

- Create: `/home/glw907/Projects/poplar/docs/poplar/research/2026-05-08-table-form-patterns.md`
- Reference: `/home/glw907/Projects/poplar/docs/poplar/research/2026-05-08-chrome-family-patterns.md`
  (the 9x.2 catalog — copy its section shape exactly)
- Reference: `/home/glw907/Projects/poplar/docs/poplar/research/2026-05-08-bubbles-list-patterns.md`
  (the 9x.1 catalog — sibling reference)

- [ ] **Step 3.1.** Write the doc header. One paragraph naming
  the pass, the two libraries surveyed, and the companion eval
  at `docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`.
  State that both verdicts were Keep + harvest and that the
  doc catalogs *what to keep in mind*. State the per-entry
  shape: pattern → upstream location (file:line) → why it's
  good → poplar applicability (applied now / candidate for
  later / no — with rationale).

- [ ] **Step 3.2.** Write the `charmbracelet/huh` section. One
  pattern entry per item from Task 1.3 that survived the
  applied-now/candidate filter. Cite each upstream call out
  with `theme.go:LINE` or `field_X.go:LINE` precision.
  Cross-reference 9x.1 / 9x.2 patterns by number when the
  pattern is a re-statement (do not duplicate; link). Close
  with a "Diff candidates for Task 5" subsection.

- [ ] **Step 3.3.** Write the `evertras/bubble-table` section.
  Same shape. Include the ADR-0084 anti-pattern callout from
  Step 2.3 as one of the entries — naming what poplar
  deliberately *does not* do is part of the catalog's value.
  Two consumer subsections: messagelist and contacts/list.
  Close with separate "Diff candidates" subsections for each.

### Task 4: Draft the research doc — synthesis sections

**Files:**

- Modify: `/home/glw907/Projects/poplar/docs/poplar/research/2026-05-08-table-form-patterns.md`
  (continuing from Task 3)

- [ ] **Step 4.1.** Write the "Top three ways their code beats
  ours" section. Pick the three strongest candidate patterns
  across both libraries — these feed Pass 9z's adoption
  roadmap together with 9x.1 and 9x.2's lists. If fewer than
  three reach the bar, state that explicitly and explain why;
  do not pad. (Pass 9x.2's section landed at one honest
  recommendation; the same posture applies here.)

- [ ] **Step 4.2.** Write the "What did NOT survive the harvest"
  section. List patterns considered and rejected with one-line
  rationale each. Examples to test: huh's `selector.Selector[Field]`
  frozen slice; huh's group-chrome bake-in; bubble-table's
  `JoinHorizontal` row assembly; bubble-table's `RowData map[string]any`
  indirection; bubble-table's flex-column model.

- [ ] **Step 4.3.** Write the "Cross-cutting code candidates"
  section. Re-evaluate the `uicore.ListBodyRows` extraction
  candidate from 9x.1 against this catalog's surfaces — does
  contacts/list or messagelist add a fourth modal-list
  consumer that crosses the threshold? For any helper that two
  or more table/form surfaces could share, name it and
  recommend *extract now* or *defer until N consumers*. Match
  9x.1 / 9x.2's conservative posture.

- [ ] **Step 4.4.** Run `wc -l` on the doc. Eyeball it against
  the 9x.2 catalog (~590 lines) and 9x.1 catalog. If radically
  shorter, recheck Tasks 1–2 for missed patterns; if radically
  longer, prune redundant entries.

- [ ] **Step 4.5.** Commit the catalog doc:

```
git add docs/poplar/research/2026-05-08-table-form-patterns.md
git commit -m "Pass 9x.3 research: table/form family pattern catalog

Patterns harvested from charmbracelet/huh and
evertras/bubble-table against poplar's contacts/form,
messagelist, and contacts/list surfaces. Companion to the
Eval B verdicts (both Keep + harvest). Top-three list feeds
Pass 9z together with 9x.1 and 9x.2 catalogs.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

### Task 5: Apply concrete diffs (0–2 per surface)

**Files (conditional on Tasks 1.4 / 2.4):**

- Modify: `/home/glw907/Projects/poplar/internal/ui/contacts/form.go`
- Modify: `/home/glw907/Projects/poplar/internal/ui/contacts/styles.go`
  (always: remove the dead `Border` field — adjacent cleanup
  per pre-beta posture)
- Possibly modify: `/home/glw907/Projects/poplar/internal/ui/messagelist/model.go`
- Possibly modify: `/home/glw907/Projects/poplar/internal/ui/contacts/list.go`

- [ ] **Step 5.1.** Remove the dead `contacts.Styles.Border`
  field. Confirm with `grep -rn "styles.Border\b" internal/`
  — must return zero hits (verified in plan; re-verify before
  edit). One commit:
  "Pass 9x.3: contacts — drop dead Styles.Border field".

- [ ] **Step 5.2.** For each diff identified in Tasks 1.4 / 2.4:
  apply it in a separate commit. Each commit message names the
  upstream pattern and the surface ("Pass 9x.3: <surface> —
  adopt <pattern> from <library>"). Update the catalog entry
  in the research doc to flip its applicability marker from
  *candidate for later* to *applied now* with the commit SHA
  when the diff lands.

- [ ] **Step 5.3.** If `internal/ui/` was touched by any diff,
  run the **§10 Review checklist** from
  `docs/poplar/bubbletea-conventions.md` against the diff.
  Each item must verify from the diff or a 120×40 tmux
  capture. Note any deviations in the pass-end ADR (Task 7)
  if any are introduced.

- [ ] **Step 5.4.** Run `make check`. Fix any failures inline
  before proceeding.

- [ ] **Step 5.5.** Live-verify: `make install`, then via tmux
  per `.claude/docs/tmux-testing.md`, capture each touched
  surface at 80×24 and 120×40. Eyeball for visual regressions.
  (Border removal alone needs no capture — it is a dead-field
  delete with no render impact.)

### Task 6: /simplify on the diff

- [ ] **Step 6.1.** Invoke the `simplify` skill against the pass
  diff. Apply genuine wins; ignore noise. If only the
  Styles.Border removal landed in Task 5, the simplify pass is
  mostly over the doc — voice lens applies (tells T4, T10, T14,
  T16, T27, T28, T33, T35, T39, T40 plus T34 voice-only).

### Task 7: Pass-end consolidation ritual

**Files:**

- Possibly create: `/home/glw907/Projects/poplar/docs/poplar/decisions/0182-...md`
  (only if a binding fact moved — not expected)
- Modify: `/home/glw907/Projects/poplar/docs/poplar/STATUS.md`
- Move: `docs/superpowers/plans/2026-05-08-harvest-table-form.md` →
  `docs/superpowers/archive/plans/2026-05-08-harvest-table-form.md`
- Possibly modify: `/home/glw907/Projects/poplar/docs/poplar/invariants.md`
  (only if a binding fact moved)

- [ ] **Step 7.1.** ADR decision. If Task 5 landed any binding-
  fact change (a new component contract, a renamed exported
  type, a swapped library), write a new ADR under
  `docs/poplar/decisions/` with the next number. The
  Styles.Border removal does not qualify — internal field on
  a per-package style struct, no binding-fact effect. If no
  ADR — the expected case — skip. Add a one-line note to the
  research doc confirming "no ADR" and the pass commit
  message carries the rationale.

- [ ] **Step 7.2.** If an ADR was written, update
  `docs/poplar/invariants.md` in place per the consolidation-
  ritual rules (edit, do not append). Update
  `docs/poplar/decisions/INDEX.md`. If no ADR, skip.

- [ ] **Step 7.3.** Update `STATUS.md`. Mark Pass 9x.3 done.
  Replace the starter prompt with Pass 9y's (bubble
  consolidation — decide if any swap work remains across the
  three catalogs and Eval B verdicts). Keep the file ≤60
  lines.

- [ ] **Step 7.4.** `git mv` the plan to the archive:

```
git mv docs/superpowers/plans/2026-05-08-harvest-table-form.md \
  docs/superpowers/archive/plans/2026-05-08-harvest-table-form.md
```

  (No spec to archive — research-only pass.)

- [ ] **Step 7.5.** `make check` — must be green.

- [ ] **Step 7.6.** Commit, push, install:

```
git add -A
git commit -m "Pass 9x.3: table/form family harvest

Catalogs design patterns from charmbracelet/huh and
evertras/bubble-table against poplar's contacts/form,
messagelist, and contacts/list. <0–2 diffs landed inline | no
diffs — Eval B's Keep + harvest verdicts hold.> Top-three list
feeds Pass 9z's adoption roadmap together with 9x.1 and 9x.2.

Co-Authored-By: Claude <noreply@anthropic.com>"
git push
make install
```

---

## Self-review notes

- **Spec coverage.** Two open questions from the starter prompt
  are pre-settled in the brainstorm log; the plan catalogs each
  library × consumer pair (Tasks 1–2), produces the catalog doc
  with the required per-entry shape and synthesis sections
  (Tasks 3–4), and applies 0–2 concrete diffs per surface where
  the bar is met (Task 5). The "top three" requirement from the
  starter prompt is Step 4.1.
- **Placeholder scan.** No "TBD" / "implement later" steps; each
  step is one action with concrete file paths and either an
  expected output or a decision rule. The 0–2 diff bar is
  decision-driven, not under-specified — the rule is named in
  Step 1.4 / 2.4 ("obvious improvement that does not change the
  contract").
- **Type consistency.** No code types defined across tasks; the
  research doc is the deliverable.
- **Pass size.** 7 tasks, well under the 12-task budget. ADR
  conditional and expected to be skipped — pre-beta posture
  permits binding-fact changes inline if any pattern adoption
  warrants one.
