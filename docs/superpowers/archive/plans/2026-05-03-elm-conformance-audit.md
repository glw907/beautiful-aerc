# Pass 8.5b — Elm Conformance Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Audit `internal/ui/` for elm-conventions conformance and
substantial refactor opportunities; apply fixes inline per the
pre-beta posture; produce an audit doc as the historical record.

**Architecture:** Two-phase workflow. Phase 1 dispatches four
parallel `Explore` subagents over disjoint file groups in
`internal/ui/`, each producing a structured findings list. Phase 2
consolidates findings into the audit spec and applies fixes in
severity order, with a go/no-go gate before refactor execution.

**Tech Stack:** Go 1.26.1, bubbletea, lipgloss, the
elm-conventions skill, `Explore` subagents, `make check`.

**Spec:** `docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md`

---

## Workflow Notes (read once before starting)

- This is an audit-then-fix pass, not a TDD feature build. Most
  tasks below cannot pre-specify exact diffs — the discovery phase
  determines what code changes. Where this is the case, the task
  describes the workflow + commit shape, and the actual diff is
  produced live against the findings.
- Per pre-beta posture (CLAUDE.md "Release stance"): no compat
  shims, no `// removed` breadcrumbs, no preserved-for-future
  fields, no churn-cost framing. Breaking renames are fine.
- `make check` is the commit gate. Run it before every commit;
  if red, fix before committing.
- Tests that break because of a deliberate architectural change
  get rewritten in the same commit as the production change.

---

## Task 1: Dispatch four parallel discovery subagents

**Files:** None modified.

**Goal:** Launch four `Explore` subagents in a single message,
one per file group from the spec, each returning a structured
findings list against the charter.

- [ ] **Step 1: Compose the shared subagent prompt template**

Construct one prompt body shared across all four subagents
(file list interpolated per agent). The body MUST include:

1. The seven elm-conventions rules verbatim, copied from
   `~/.claude/skills/elm-conventions/SKILL.md` (Rule 1 through
   Rule 7), plus the "Cmd Closures: Capture Values, Not Pointers"
   section. The subagent will not invoke skills itself, so the
   text must be inline.
2. The expanded refactor charter from the spec doc, listing the
   six refactor categories: type-design opportunities, state-shape
   problems, component-boundary problems, Update-method shape,
   duplicated idioms, dead/vestigial code.
3. The output format. Use a markdown table block with one row
   per finding. Columns:
   `file | line | stream | rule_or_category | severity | description | proposed_fix`
   Where:
   - `stream` ∈ {`conformance`, `refactor`}
   - `rule_or_category` is `R1`–`R7` or `cmd-closure` for
     conformance; one of `type-design`, `state-shape`,
     `component-boundary`, `update-shape`, `duplicated-idiom`,
     `dead-code` for refactor
   - `severity` ∈ {`architectural`, `mechanical`, `cosmetic`}
     for conformance; {`major`, `minor`} for refactor
4. Negative-result instruction: if a file has no findings, say
   so explicitly under a "## Files with no findings" heading at
   the bottom of the response.
5. Read-only instruction: do NOT edit any file; the dispatcher
   applies fixes after consolidating.
6. The assigned file list for that subagent (from groups A-D in
   the spec).
7. Length cap: respond in under ~3000 words; if findings exceed
   that, prioritize architectural > mechanical > major-refactor >
   minor-refactor > cosmetic.

- [ ] **Step 2: Dispatch all four subagents in a single message**

Send one message with four `Agent` tool calls, all
`subagent_type: "Explore"`, all in parallel. One per file group:

- Group A: app.go, account_tab.go, top_line.go, footer.go,
  status_bar.go, layout.go, error_banner.go, dim.go, overlay.go
- Group B: sidebar.go, sidebar_search.go, msglist.go, viewer.go
- Group C: help_popover.go, linkpicker.go, movepicker.go,
  confirm_modal.go, toast.go
- Group D: cmds.go, keys.go, styles.go, icons.go, iconwidth.go,
  date_format.go

Use `description` like "Audit Group A (root + chrome)".

- [ ] **Step 3: Collect the four responses**

Save each subagent's response verbatim into a scratch buffer for
Task 2 consolidation. Do not commit anything yet — discovery
output is not a project artifact; the consolidated audit doc is.

---

## Task 2: Consolidate findings into the spec doc

**Files:**
- Modify: `docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md`

**Goal:** Append a "Findings" section to the spec doc that merges
all four subagent outputs, deduplicates, resolves conflicts by
re-reading the source, identifies cross-cutting themes, and
records negative results.

- [ ] **Step 1: Merge the four findings tables**

Append a new top-level section `## Findings` to the spec.
Subsections, in this order:

1. `### Conformance findings` — one combined table, sorted by
   severity (architectural → mechanical → cosmetic), then by
   file, then by line.
2. `### Refactor proposals` — one subsection per proposal (not a
   table; each proposal needs prose). Header format:
   `#### Refactor N: <short title>`. Body fields: **Motivation**,
   **Files touched**, **Risk**, **Rough size**, **Recommendation**
   (apply this pass / queue as own pass / decline).
3. `### Cross-cutting themes` — patterns that recur across two or
   more components and want a single shared fix. Each theme lists
   the files it spans and the proposed shared fix.
4. `### Negative results` — explicit "no findings of category X"
   statements where applicable, plus the union of subagent
   "Files with no findings" lists.

- [ ] **Step 2: Resolve conflicts by re-reading source**

If two subagents disagree about a file (one flags it, one says
"no findings"), `Read` the file directly and decide. Note the
resolution in the audit doc only if non-obvious.

- [ ] **Step 3: Commit the findings**

```bash
git add docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md
git commit -m "Pass 8.5b D-1: audit findings — consolidated discovery output

Four parallel Explore subagents produced findings against the
seven elm-conventions rules + cmd-closure capture rule + the
expanded refactor charter. Consolidated into the spec doc.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

The `D-N` prefix (D for Discovery / Decision) follows the
established pass-numbering convention from prior passes.

---

## Task 3: Apply architectural conformance fixes

**Files:** Determined by Task 2 findings. Each fix touches one or
more files in `internal/ui/`.

**Goal:** Fix every conformance finding with severity
`architectural` (Rules 1-5 violations). One commit per logical
theme; `make check` green between commits.

- [ ] **Step 1: Order findings by theme**

Group architectural findings by theme (e.g., "package-level state
in cmds.go", "callback-based child→parent in linkpicker", "I/O
in Update of viewer"). Within a theme, group by file. Each theme
gets its own commit.

- [ ] **Step 2: For each theme, apply the fix**

For each theme:
1. `Read` every file the theme touches.
2. Apply the fix(es) via `Edit`.
3. If a test breaks because of the architectural change, rewrite
   the test in the same commit. Do not rewrite tests to be lenient
   — rewrite them to reflect the corrected architecture.
4. Run `make check`. If red, fix before committing.
5. Commit with message:
   ```
   Pass 8.5b A-N: <theme description>

   <2-4 sentence body explaining the violation and the fix.
   Reference the elm-conventions rule number(s).>

   Co-Authored-By: Claude <noreply@anthropic.com>
   ```
   Where `A-N` is the architectural-fix sequence (A-1, A-2, …).

- [ ] **Step 3: If no architectural findings exist, skip this task**

Note in the audit doc under "Negative results" that no
architectural findings were identified, and proceed to Task 4.

---

## Task 4: Apply mechanical conformance fixes

**Files:** Determined by Task 2 findings.

**Goal:** Fix every conformance finding with severity `mechanical`
(Rules 6-7 + cmd-closure capture). Same commit-per-theme shape as
Task 3, with prefix `M-N`.

- [ ] **Step 1: Order findings by theme**

Likely themes: "switch msg.String() → key.Matches", "MaxWidth
parent-side clipping → child clipPane", "len() in width math →
displayCells", "Cmd closure pointer capture → value extraction".

- [ ] **Step 2: For each theme, apply the fix**

Same workflow as Task 3 Step 2. Commit prefix: `Pass 8.5b M-N:`.

- [ ] **Step 3: Run make check**

```bash
make check
```

Expected: `ok  github.com/glw907/poplar/...` for every package.
If red, fix before continuing to Task 5.

---

## Task 5: Apply trivial cosmetic fixes

**Files:** Determined by Task 2 findings.

**Goal:** Apply only the cosmetic fixes that are trivial
(single-line edits, mechanical renames). Skip any cosmetic finding
that would expand the diff non-trivially.

- [ ] **Step 1: Filter cosmetic findings to trivial-only**

For each cosmetic finding, decide: trivial (single-line edit, no
risk of meaning change) → apply; non-trivial → leave a note in
the audit doc that it was deferred and why.

- [ ] **Step 2: Apply trivial cosmetic fixes**

One commit covering all of them, no need for per-theme split.
Commit prefix: `Pass 8.5b C-1:`.

```bash
git add internal/ui/...
git commit -m "Pass 8.5b C-1: trivial cosmetic conformance fixes

<short list of what changed>

Co-Authored-By: Claude <noreply@anthropic.com>"
```

- [ ] **Step 3: Run make check**

```bash
make check
```

---

## Task 6: Refactor proposals — present and gate

**Files:**
- Modify: `docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md`

**Goal:** Present the refactor proposal list to the user with
recommendations; receive go/no-go per proposal; record dispositions
in the audit doc.

- [ ] **Step 1: Present the proposal list to the user**

Format the user-facing message as a numbered list. For each
proposal: title, motivation (one sentence), risk + size, my
recommendation. Ask the user for go/no-go per proposal.

If a proposal is large enough that it warrants its own pass,
recommend "queue as own pass" and offer a draft starter prompt
for STATUS.

- [ ] **Step 2: Record dispositions in the audit doc**

For each proposal in `### Refactor proposals`, append a
**Disposition** field with one of: `applied this pass` /
`queued as Pass <N>: <starter title>` / `declined: <one-line
reason>`.

- [ ] **Step 3: Commit the disposition update**

```bash
git add docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md
git commit -m "Pass 8.5b R-0: refactor proposals — dispositions

Per pre-beta posture, all proposals reviewed; dispositions
recorded.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Execute approved refactors

**Files:** Determined by Task 6 dispositions.

**Goal:** Apply each `applied this pass` refactor as its own
commit. `make check` green between commits.

- [ ] **Step 1: For each approved refactor**

1. `Read` every file the refactor touches.
2. If the refactor is large, write a short scratch outline of
   the steps before editing — refactors are not as well-bounded
   as conformance fixes.
3. Apply edits.
4. Update or rewrite tests as needed.
5. `make check`.
6. Commit with prefix `Pass 8.5b R-N:` and a body describing
   what changed and why.

- [ ] **Step 2: If no refactors were approved, skip this task**

Note in the audit doc that all refactor proposals were declined or
queued as their own passes.

---

## Task 8: /simplify on the cumulative diff

**Files:** Whatever the simplify skill flags.

**Goal:** Run the `simplify` skill against the diff produced by
Tasks 3-7 and apply any genuine wins.

- [ ] **Step 1: Invoke simplify**

Use the `Skill` tool with `simplify`. The skill dispatches three
review agents (reuse, quality, efficiency) in parallel against the
recent diff.

- [ ] **Step 2: Aggregate findings and apply genuine wins**

Per the skill's own discipline: not every suggestion is a win.
Apply the ones that are; skip the ones that aren't. Commit any
applied changes:

```bash
git add internal/ui/...
git commit -m "Pass 8.5b S-1: simplify pass

<short list of what changed>

Co-Authored-By: Claude <noreply@anthropic.com>"
```

- [ ] **Step 3: Run make check**

```bash
make check
```

---

## Task 9: ADR + invariants.md update (only if needed)

**Files:**
- Possibly create: `docs/poplar/decisions/0128-<title>.md` (and
  successive numbers if more than one decision)
- Possibly modify: `docs/poplar/invariants.md`

**Goal:** Write new ADR(s) only if a refactor codified a *new*
binding decision (a pattern that wasn't already in an existing
Elm-architecture ADR). Update `invariants.md` only if a binding
fact changed.

- [ ] **Step 1: Decide whether any ADR is warranted**

Conformance fixes do NOT need ADRs — Rules 1-7 are already
covered by ADR-0023, 0035-0037, 0042, 0044, 0054, 0088 plus
ADRs 0077-0084 for bubbletea conventions. A refactor that codifies
a previously-unspecified pattern (e.g., a new convention for
how a particular kind of state should be modeled) earns one.

If no ADR is warranted, skip to Task 10.

- [ ] **Step 2: Write the ADR(s)**

Use the next available number after 0127 (i.e. 0128, 0129, …).
ADR template from the `poplar-pass` skill:

```markdown
---
title: <short decision title>
status: accepted
date: 2026-05-03
---

## Context
<why the decision came up, what problem it solves>

## Decision
<the decision itself, stated as a binding fact>

## Consequences
<follow-on effects, what this unlocks, what it forecloses>
```

If the new ADR supersedes a prior one, update the prior ADR's
frontmatter to `status: superseded by NNNN`.

- [ ] **Step 3: Update invariants.md if a fact changed**

Edit `docs/poplar/invariants.md` in place. Add the new ADR(s) to
the decision-index table at the bottom. Stay under the 300-line
cap (the `claude-md-size.sh` hook enforces this).

- [ ] **Step 4: Commit**

```bash
git add docs/poplar/decisions/ docs/poplar/invariants.md
git commit -m "Pass 8.5b: ADR + invariants — <summary>

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10: Update STATUS.md

**Files:**
- Modify: `docs/poplar/STATUS.md`

**Goal:** Mark Pass 8.5b done; promote Pass 8.4c to current.

- [ ] **Step 1: Edit STATUS.md**

1. In the pass table, change the 8.5b status from `pending` to
   `done`.
2. Update the "Current pass" header line to reflect 8.4c next.
3. Replace the "Next starter prompt (Pass 8.5b)" section with the
   "Queued after 8.5b (Pass 8.4c)" content (which is already
   drafted at the bottom of STATUS.md), promoting it to the
   "Next starter prompt (Pass 8.4c)" position.
4. Draft the next "Queued after 8.4c" section if a natural
   successor is obvious from the table (Pass 8.6 — Attachments I).

- [ ] **Step 2: Verify STATUS.md is ≤60 lines**

```bash
wc -l docs/poplar/STATUS.md
```

If over 60, prune.

- [ ] **Step 3: Commit**

```bash
git add docs/poplar/STATUS.md
git commit -m "Pass 8.5b: STATUS — mark 8.5b done; queue 8.4c

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 11: Archive plan + spec

**Files:**
- Move: `docs/superpowers/plans/2026-05-03-elm-conformance-audit.md`
  → `docs/superpowers/archive/plans/2026-05-03-elm-conformance-audit.md`
- Move: `docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md`
  → `docs/superpowers/archive/specs/2026-05-03-elm-conformance-audit-design.md`

**Goal:** Preserve git history while moving the pass artifacts to
the archive.

- [ ] **Step 1: git mv the plan and spec**

```bash
git mv docs/superpowers/plans/2026-05-03-elm-conformance-audit.md \
       docs/superpowers/archive/plans/2026-05-03-elm-conformance-audit.md
git mv docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md \
       docs/superpowers/archive/specs/2026-05-03-elm-conformance-audit-design.md
```

- [ ] **Step 2: Commit**

```bash
git commit -m "Pass 8.5b: archive plan + spec

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 12: Final make check, push, install

**Files:** None modified.

**Goal:** Final gate, publish, and install the new binary.

- [ ] **Step 1: make check**

```bash
make check
```

Expected: every package green.

- [ ] **Step 2: Push**

```bash
git push
```

- [ ] **Step 3: Install**

```bash
make install
```

Expected: `~/.local/bin/poplar` updated.

- [ ] **Step 4: Smoke test**

```bash
poplar --version
```

Expected: prints version without error. (If a richer smoke test
is warranted because of the scope of architectural changes, run
`poplar` against a real account and verify the UI still launches
and renders. The pre-beta posture means architectural breakage is
allowed but UI breakage isn't acceptable on master — verify
before declaring the pass done.)

---

## Self-review checklist (already run; recorded for the executor)

- **Spec coverage:** Every spec section maps to a task. Phase 1 →
  Task 1. Phase 2 → Task 2. Phase 3 (conformance) → Tasks 3-5.
  Phase 3 (refactor) → Tasks 6-7. Phase 4 → Tasks 8-12.
- **Placeholder scan:** Tasks 3, 4, 5, 7 cannot pre-specify exact
  diffs because their content is determined by Task 2 findings.
  This is acknowledged at the top of the plan and is intrinsic to
  audit-then-fix work, not a placeholder failure.
- **Type consistency:** Commit-prefix scheme (`D-N`, `A-N`, `M-N`,
  `C-N`, `R-N`, `S-N`) is used consistently across tasks.
- **Pass-end ritual coverage:** Every step from `poplar-pass`
  (simplify, ADR, invariants, STATUS, archive, check, commit,
  push, install) has a task.
