# Pass 9v — Bubble Eval A (Strong Matches) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a triage spec evaluating five strong-match
community bubbles against poplar's hand-rolled `internal/ui/`
components, with per-candidate verdicts and a ranked swap roadmap.

**Architecture:** Research-only pass — no code changes. Each
candidate gets a self-contained section in one growing
spec doc. Each section answers the core question, presents rubric
evidence, states a verdict, and lists interactions. The pass closes
with a dependency graph and ranked linearization that authorizes
the foundational swap passes (9v.1 `bubbletea-overlay`, 9v.2
`huh`).

**Tech Stack:** WebFetch for upstream repo content, Bash + grep
for local code reading, Write/Edit for spec doc construction. No
runtime or build changes.

**Reference spec:** `docs/superpowers/specs/2026-05-08-bubble-adoption-design.md`

---

## Guiding principle

Every verdict in this pass answers one balancing question:

> Poplar wants to be a first-class bubbletea app *and* a truly
> excellent mail client. Adopting a community bubble must, at
> minimum, **not make poplar worse**. Ideally it makes poplar
> **better**. When in doubt, lean toward the bubble — being a
> first-class bubbletea app is itself a win. When the bubble
> would compromise mail-client quality, keep the hand-roll.

Each candidate section ends with a verdict
(**Adopt / Adopt-with-fork / Keep + harvest**) and one sentence
naming the specific gain or loss that decided it.

## Eval rubric (applied per candidate)

1. **Feature parity** — does the bubble cover what poplar's
   hand-roll does today? Gaps that matter, called out by name.
2. **Customization seams** — can we wire 15 themes, modifier-free
   single-key bindings, and domain state without forking
   upstream? Usually decisive.
3. **Theming integration** — accepts `lipgloss.Style` injection,
   or owns its own colors? "Owns colors" is a fork target.
4. **Maintenance signal** — last commit, version cadence, releases
   in past twelve months. Informs forking risk.
5. **Code delta estimate** — rough LOC removed from poplar vs LOC
   added (shim + integration).
6. **License** — MIT, BSD, Apache only. Hard veto otherwise.

## File structure

**Created in this pass:**
- `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md` — the eval doc, grown one section per task.

**Read but not modified:**
- `internal/ui/uicore/overlay.go` — `PlaceOverlay`
- `internal/ui/uicore/modal_shell.go` — `ModalShell`
- `internal/ui/app.go` — overlay cascade
- `internal/ui/confirm_modal.go`, `conflict_overlay.go`, `outbox_overlay.go`, `toast.go`
- `internal/ui/helppopover/` — help popover
- `internal/ui/movepicker/`, `internal/ui/reader/linkpicker.go`, `internal/ui/reader/attachpicker.go`
- `internal/ui/contacts/form.go`, `internal/ui/contacts/list.go`, `internal/ui/contacts/popover.go`
- `internal/ui/messagelist/model.go`, `internal/ui/messagelist/styles.go`

---

## Task 1: Eval `rmhubbert/bubbletea-overlay`

**Files:**
- Modify: `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md` (create on first task; append section)

- [ ] **Step 1: Fetch upstream README and source**

```bash
# README and overview
WebFetch https://github.com/rmhubbert/bubbletea-overlay "What does this library do? List its public types, methods, and the example integration pattern. Note license, last commit, version tags."

# Public API source
WebFetch https://raw.githubusercontent.com/rmhubbert/bubbletea-overlay/main/overlay.go "Show the full file contents."
```

Capture: public types, the rendering primitive (does it composite ANSI fg-over-bg?), positioning model, license, maintenance signal.

- [ ] **Step 2: Read poplar's overlay infrastructure**

```bash
# The two pieces this library would replace
cat internal/ui/uicore/overlay.go
cat internal/ui/uicore/modal_shell.go

# Every consumer site
grep -rn "uicore.PlaceOverlay\|uicore.ModalShell\|uicore.NewModalShell" internal/ui/ cmd/

# The cascade
grep -n "overlay\|Overlay\|modal\|Modal" internal/ui/app.go | head -40
```

Capture: the cascade order from `app.go` (confirm > conflict > outbox > help > linkpicker > attachpicker > movepicker > form > popover), how `PlaceOverlay` composites at cell offsets, what `ModalShell.Box()` produces.

- [ ] **Step 3: Sketch integration**

Mental model — write a 3-4 sentence sketch into your scratch buffer (no file output):
- Where does `bubbletea-overlay` plug in (replace `PlaceOverlay`, replace `ModalShell.Box()`, both, or neither)?
- Does the library handle a *cascade* (mutual-exclusion ordered stack) or only single-overlay z-ordering? If only single, the cascade logic stays in `App`.
- What shim is needed for the titled `╔═ title ═╗` border `ModalShell` produces?

- [ ] **Step 4: Append the eval section**

Append to `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md`. If the file does not exist, create it with this preamble first:

```markdown
# Bubble Eval A — Strong Matches

## Core question

Does adopting the community bubble make poplar better? At
minimum it must not make poplar worse. Ideally it makes poplar
better. When in doubt, lean toward bubble (first-class
bubbletea-app is itself a win); when adoption would compromise
mail-client quality, keep the hand-roll.

## Rubric

1. Feature parity — does the bubble cover what poplar does today?
2. Customization seams — can we wire themes, keymaps, and domain
   state without forking?
3. Theming integration — accepts `lipgloss.Style` injection?
4. Maintenance signal — last commit, version cadence.
5. Code delta — LOC removed from poplar vs LOC added.
6. License — MIT/BSD/Apache only.

---
```

Then append the candidate section using this template (fill every
bracketed slot — no placeholders survive into the doc):

```markdown
## `rmhubbert/bubbletea-overlay`

**Does this make poplar better?** [One paragraph answering
concretely. If yes, name the specific gain (e.g., "removes
~80 LOC of cell-arithmetic in `PlaceOverlay`, replaces it with a
maintained library that other Charm-ecosystem apps use, and
clarifies overlay positioning as a separate concern from cascade
ordering"). If no, name the specific loss (e.g., "library doesn't
model the cascade and forces App to keep the same logic anyway,
with no LOC win and a new dependency").]

**Feature parity:** [Public types covered, gaps named.]

**Customization seams:** [Theme injection, keymap shape, domain
state. Does it accept `lipgloss.Style`?]

**Theming integration:** [Specific test: does it accept styles or
own colors?]

**Maintenance signal:** [Last commit date, version tags, release
cadence in past twelve months.]

**Code delta estimate:** [Rough LOC removed from poplar vs LOC
added — count `PlaceOverlay` + `ModalShell` lines, estimate shim
size.]

**License:** [Verbatim from LICENSE file.]

**Verdict:** **Adopt** / **Adopt-with-fork** / **Keep + harvest**

**Rationale (one line):** [The single decisive gain or loss.]

**Interacts with:**
- [Which other candidates depend on this swap landing first.]
- [Which other candidates this swap simplifies.]
- [Any blocking dependencies on candidates not in Eval A.]
```

- [ ] **Step 5: Voice check the section**

```bash
scripts/voice-check.sh docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
```

Expected: no flagged tells. If any flag, edit inline before commit.

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
git commit -m "$(cat <<'EOF'
Pass 9v task 1: eval bubbletea-overlay

[One sentence stating the verdict and the decisive rationale,
e.g., "Verdict: Adopt — replaces PlaceOverlay's ANSI compositing
with a maintained library while leaving cascade logic in App."]

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Eval `bubbles/help`

**Files:**
- Modify: `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md` (append section)

- [ ] **Step 1: Fetch upstream README and source**

```bash
WebFetch https://github.com/charmbracelet/bubbles/tree/master/help "What does the help component do? List public types, KeyMap interface, and the short/full view rendering."

WebFetch https://raw.githubusercontent.com/charmbracelet/bubbles/master/help/help.go "Show the full file contents."
```

Capture: `KeyMap` interface shape (`ShortHelp() []key.Binding`, `FullHelp() [][]key.Binding`), `Model` width handling, style hooks, license.

- [ ] **Step 2: Read poplar's helppopover**

```bash
ls internal/ui/helppopover/
cat internal/ui/helppopover/*.go

grep -rn "helppopover\." internal/ui/ cmd/ | head -20

# How keys are declared elsewhere — sample one consumer
cat internal/ui/keys.go | head -80
```

Capture: how the two `Context` values (`Account`, `Viewer`) build their key lists today, the modal frame lookup, the styles used.

- [ ] **Step 3: Sketch integration**

Write a 3-4 sentence sketch:
- Two `KeyMap` implementations (one per `Context`) feed `help.Model`.
- The titled modal frame stays — `help.Model` mounts inside `ModalShell` (or `bubbletea-overlay` if 9v.1 lands first).
- Style injection: does `help.Styles` accept our palette?

- [ ] **Step 4: Append the eval section**

Append to `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md` using the same template as Task 1, with section heading `## bubbles/help`. Fill every bracketed slot from the steps above. The "Does this make poplar better?" paragraph should specifically address whether replacing the two-`Context` enum branches with two `KeyMap` impls reduces or just relocates the code.

- [ ] **Step 5: Voice check**

```bash
scripts/voice-check.sh docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
```

Expected: no flagged tells. Edit inline if needed.

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
git commit -m "$(cat <<'EOF'
Pass 9v task 2: eval bubbles/help

[Verdict + one-line rationale.]

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Eval `daltonsw/bubbleup`

**Files:**
- Modify: `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md` (append section)

- [ ] **Step 1: Fetch upstream README and source**

```bash
WebFetch https://github.com/daltonsw/bubbleup "What does this library do? List public types, the timing/animation model, positioning model, and how callers attach domain payloads."

WebFetch https://raw.githubusercontent.com/daltonsw/bubbleup/main/bubbleup.go "Show the full file contents."
```

Capture: how toasts are queued, animation/timing primitives, whether toasts can carry caller-defined payloads, license, maintenance.

- [ ] **Step 2: Read poplar's toast**

```bash
cat internal/ui/toast.go
cat internal/ui/toast_test.go

# Triage payload shape
grep -n "TriageOp\|TriageUndo" internal/ui/toast.go internal/ui/uicore/*.go

# Producers
grep -rn "toast\." internal/ui/ | grep -v _test.go | head -20
```

Capture: `triageOp` payload, undo-deadline semantics, dismissal triggers, how the toast is positioned and styled.

- [ ] **Step 3: Sketch integration**

- Does `bubbleup` allow the toast `View()` to be supplied by the
  caller, or does it own rendering? If owns, can we still attach
  a `triageOp` and read it on `u` to fire undo?
- Positioning: bottom-right? bottom-center? Does it match
  poplar's current placement?

- [ ] **Step 4: Append the eval section**

Append section `## daltonsw/bubbleup` using the Task 1 template.
The "Does this make poplar better?" paragraph specifically
addresses whether the timing/animation chrome is worth importing
when the *content* (triage payload + undo countdown) stays
custom — i.e., are we adopting a small chrome library or paying
for one we don't need?

- [ ] **Step 5: Voice check**

```bash
scripts/voice-check.sh docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
```

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
git commit -m "$(cat <<'EOF'
Pass 9v task 3: eval bubbleup

[Verdict + one-line rationale.]

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Eval `charmbracelet/huh`

**Files:**
- Modify: `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md` (append section)

- [ ] **Step 1: Fetch upstream README and source**

```bash
WebFetch https://github.com/charmbracelet/huh "What does this library do? List the field types (Input, Text, Select, Confirm, etc.), the Group/Form structure, validation hooks, and the embedded tea.Model integration pattern."

WebFetch https://raw.githubusercontent.com/charmbracelet/huh/main/form.go "Show the full file contents — focus on Form struct, View, Update, embedded usage pattern."

WebFetch https://raw.githubusercontent.com/charmbracelet/huh/main/group.go "Show the full file contents."
```

Capture: field set, validation API, theming hook (`*huh.Theme`), key bindings shape, embedded-vs-fullscreen integration, license, maintenance.

- [ ] **Step 2: Read poplar's contacts/Form and the layout it lives in**

```bash
cat internal/ui/contacts/form.go
cat internal/ui/contacts/types.go
cat internal/ui/contacts/keys.go
cat internal/ui/contacts/styles.go

# How Form mounts inside the contacts three-pane layout
grep -n "form\|Form" internal/ui/contacts/sidebar.go | head -30
grep -n "form\|Form" internal/ui/app.go | head -30

# Key invariant — the dynamic per-row email/phone add/remove
grep -n "addEmail\|removeEmail\|addPhone\|removePhone\|primary\|focusList" internal/ui/contacts/form.go
```

Capture: the field set (kind toggle, name fields, email/phone quartets `(input, cycler, ★, −)`, note, save destination), the dynamic add/remove pattern, the dual render mode (`fromPopover=true` modal vs `fromPopover=false` embedded), `ContactSaveMsg`/`ContactCancelMsg` shape.

- [ ] **Step 3: Sketch integration**

Two open questions to answer in the verdict:

1. **Sub-pane mount.** Contacts mode is sidebar + list + form-column. Does `huh.Form` expose an embedded `tea.Model` integration that mounts in a fixed-width sub-pane, or only full-screen? If only full-screen, this is **Adopt-with-fork** at minimum.

2. **Dynamic field set.** Email/phone rows are dynamic — user can add/remove. Does `huh.Group` support adding fields at runtime, or only static field lists declared at construction? If static, the dynamic rows are a fork target.

- [ ] **Step 4: Append the eval section**

Append section `## charmbracelet/huh` using the Task 1 template.
The "Does this make poplar better?" paragraph must address both
sub-questions above. The verdict's one-line rationale should
name whichever of the two is decisive.

Also note: this candidate has the **highest swap stakes** of
the five — it lands as 9v.2 before 9u's first-run wizard.
A wrong verdict here costs the most. Be specific.

- [ ] **Step 5: Voice check**

```bash
scripts/voice-check.sh docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
```

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
git commit -m "$(cat <<'EOF'
Pass 9v task 4: eval huh

[Verdict + one-line rationale.]

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Eval `evertras/bubble-table`

**Files:**
- Modify: `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md` (append section)

- [ ] **Step 1: Fetch upstream README and source**

```bash
WebFetch https://github.com/Evertras/bubble-table "What does this library do? List public types — Table, Row, Column, Cell — the filtering, sorting, pagination, frozen-column, and per-cell styling APIs."

WebFetch https://raw.githubusercontent.com/Evertras/bubble-table/main/table/table.go "Show the full file contents."

WebFetch https://raw.githubusercontent.com/Evertras/bubble-table/main/table/options.go "Show the full file contents."
```

Capture: column model, row model, filter API, sort API, frozen column support, per-cell styling, license, maintenance.

- [ ] **Step 2: Read poplar's two consumers**

```bash
cat internal/ui/messagelist/model.go
cat internal/ui/messagelist/styles.go

cat internal/ui/contacts/list.go
cat internal/ui/contacts/styles.go
```

Capture for each:
- `messagelist`: the threading depth-prefix walk (`├─`, `└─`, `│ `), fold/unfold, thread-root tracking, date column, unread/flag/answered icon cells, cursor-preserving UID refresh. Note: the threading layer is **out of scope** for `bubble-table` adoption — it's the flat list parts (cell rendering, sort, scroll) that are candidates.
- `contacts/List`: three-column layout (Name 22, Email 30, Phone 16), sort-mode toggle (`SortFirstName`/`SortLastName`), filter on browse list.

- [ ] **Step 3: Sketch integration**

Two distinct integration sketches:

1. **`messagelist`:** Can the depth-prefix string be rendered as the *first column* of a `bubble-table` row, with thread fold state managed externally? The flat-rendering parts (date column, icon cells, sort, scroll) move into `bubble-table`; the threading walk stays. If `bubble-table` mandates rectangular rows with no per-row prefix injection, this is **Adopt-with-fork**.

2. **`contacts/List`:** Direct fit. Does `bubble-table`'s filter API match poplar's needs?

- [ ] **Step 4: Append the eval section**

Append section `## evertras/bubble-table` using the Task 1 template.
The "Does this make poplar better?" paragraph addresses both
consumers. The verdict can split: e.g., "Adopt for `contacts/List`,
Adopt-with-fork for `messagelist`." Be specific in
**Interacts with:** about whether one fork serves both consumers
or each takes its own integration.

- [ ] **Step 5: Voice check**

```bash
scripts/voice-check.sh docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
```

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
git commit -m "$(cat <<'EOF'
Pass 9v task 5: eval bubble-table

[Verdict + one-line rationale.]

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Roadmap section — dependency graph and ranked linearization

**Files:**
- Modify: `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md` (append final section)

- [ ] **Step 1: Re-read every candidate's "Interacts with" subsection**

```bash
grep -A 8 "Interacts with:" docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
```

Capture: every dependency edge between Eval A candidates, plus
edges that point forward to Eval B (e.g., "pickers in Eval B
depend on bubbletea-overlay landing first").

- [ ] **Step 2: Draw the dependency graph as text DAG**

Append section `## Swap roadmap` to the eval doc. Start with the
DAG. Use this shape (fill from actual interactions):

```markdown
## Swap roadmap

### Dependency graph

\`\`\`
bubbletea-overlay  ──┬──>  bubbles/help        (mounts in modal frame)
                     ├──>  bubbleup            (positioning, if shared)
                     ├──>  huh                 (form mounts in modal)
                     └──>  Eval B pickers      (bubbles/list)

evertras/bubble-table  ──>  messagelist        (flat layer; threading stays)
                       └──>  contacts/List     (direct consumer)
\`\`\`

(Adjust edges based on actual verdicts. If a candidate is
**Keep + harvest**, omit it from the graph and note the harvest
target.)
```

- [ ] **Step 3: Ranked linearization with rationale**

Append the linearization. This is what authorizes 9v.1 and 9v.2
ahead of feature work and informs 9y consolidation later.

```markdown
### Ranked swap order

1. **`bubbletea-overlay`** (Pass 9v.1) — foundational. Lands
   before 9q so 9q's schedule-send modal builds on it. Rationale:
   [from the verdict].
2. **`huh`** (Pass 9v.2) — foundational. Lands before 9u so the
   first-run wizard builds on it. Rationale: [from the verdict].
3. **`bubbles/help`** — scheduled by 9y consolidation. Depends on
   `bubbletea-overlay`. Rationale: [from the verdict].
4. **`daltonsw/bubbleup`** — scheduled by 9y. [Independent or
   depends on `bubbletea-overlay` — fill from interactions.]
5. **`evertras/bubble-table`** — scheduled by 9y. Independent of
   the overlay/form swaps. Rationale: [from the verdict].

(Omit any **Keep + harvest** candidates from this list. If a
candidate is **Adopt-with-fork**, note the fork target inline.)
```

- [ ] **Step 4: Read the whole doc top to bottom**

```bash
cat docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md | wc -w
cat docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
```

Expected: ~1500-2000 words. Each candidate has every rubric
dimension answered, every section ends in a verdict, every
verdict has a one-line rationale, every "Interacts with" appears
in the dependency graph.

If a candidate's verdict in the body contradicts its placement in
the roadmap, fix it now.

- [ ] **Step 5: Voice check the whole doc**

```bash
scripts/voice-check.sh docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
```

Expected: no flagged tells.

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md
git commit -m "$(cat <<'EOF'
Pass 9v task 6: swap roadmap and dependency graph

Closes the Eval A triage spec with the dependency DAG and ranked
linearization. Authorizes 9v.1 (bubbletea-overlay) and 9v.2 (huh)
ahead of 9q and 9u; remaining swaps queued for 9y consolidation.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Pass-end ritual

**Files:**
- Modify: `docs/poplar/STATUS.md` — mark 9v done, set 9v.1 as next.

- [ ] **Step 1: Update STATUS.md**

Open `docs/poplar/STATUS.md` and edit:

1. Change the top line `**Current pass:** Pass 9q next — ...` to
   `**Current pass:** Pass 9v.1 next — bubbletea-overlay swap.`
2. Add a row to the passes table between 9p and 9q:

```markdown
| 9v | Bubble Eval A (strong matches) — triage spec + roadmap | done |
| 9v.1 | Swap — `bubbletea-overlay` | pending |
```

3. Replace the "Next starter prompt (Pass 9q)" block with the
   starter prompt for 9v.1. Use the verdict and integration
   sketch from the Eval A doc to fill it. Format:

```markdown
## Next starter prompt (Pass 9v.1)

> **Goal.** Adopt `rmhubbert/bubbletea-overlay` for poplar's
> overlay positioning, replacing `uicore.PlaceOverlay` and
> [`uicore.ModalShell.Box()` if also adopted; otherwise leave it].
> The cascade ordering [stays in `App` / migrates to library;
> per Eval A verdict].
>
> **Scope.** [List of files from Eval A's interactions section.]
>
> **Approach.** Spike at smallest site (likely `confirm_modal`)
> as task 1. If the spike succeeds, port remaining sites; port
> styles; port tests; delete dead code; tmux verify at 80×24
> and 120×40. If the spike fails, ADR the discovery and abandon.
> Standard pass-end ritual via `poplar-pass`.
```

4. Move the original "Next starter prompt (Pass 9q)" content
   into the queued-passes section below, so it's preserved for
   when 9q resumes after 9v.2.

- [ ] **Step 2: Run the full check pipeline**

```bash
make check
```

Expected: PASS (no code changed; this is a sanity check that the
voice-check script accepts the new doc).

- [ ] **Step 3: Push and install**

```bash
git push
make install
```

- [ ] **Step 4: Invoke poplar-pass for end-of-pass consolidation**

The `poplar-pass` skill handles ADR consolidation, invariants
update, and plan archival. For this pass:

- **No ADR.** Per the design spec, eval passes don't write ADRs;
  swap passes do. Skip the ADR step.
- **No invariants update.** No code changed.
- **Plan archival.** Move
  `docs/superpowers/plans/2026-05-08-pass-9v-bubble-eval-a.md`
  to `docs/superpowers/archive/plans/`.

```bash
mkdir -p docs/superpowers/archive/plans/
git mv docs/superpowers/plans/2026-05-08-pass-9v-bubble-eval-a.md docs/superpowers/archive/plans/
git add -u docs/poplar/STATUS.md
git commit -m "$(cat <<'EOF'
Pass 9v: archive plan + STATUS to 9v.1

Eval A done; triage spec at docs/superpowers/specs/. Foundational
swap 9v.1 (bubbletea-overlay) authorized for next pass.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git push
```

---

## Self-review

Spec coverage check (against `docs/superpowers/specs/2026-05-08-bubble-adoption-design.md`):

- ✅ Eval A covers `bubbletea-overlay`, `bubbles/help`, `bubbleup`, `huh`, `bubble-table` — Tasks 1-5.
- ✅ Per-candidate process: read upstream, read poplar, sketch, write — Steps 1-4 of each task.
- ✅ Per-candidate output format: core question, rubric evidence, verdict, interactions — Step 4 template.
- ✅ Per-pass roadmap section: dependency graph + ranked linearization — Task 6.
- ✅ No ADR from eval pass — Task 7 step 4 explicitly skips.
- ✅ Output filename matches spec convention — `2026-05-08-bubble-eval-a-strong-matches.md`.
- ✅ Foundational swaps (9v.1, 9v.2) authorized by ranked roadmap — Task 6 step 3.
- ✅ Pass numbering matches spec — 9v for eval, 9v.1 for next.
- ✅ Pre-beta posture honored — eval informs adoption; pre-beta tolerates breaking changes in subsequent swap passes.

Placeholder scan: bracketed slots in Step 4 templates and STATUS
prompts are *fill-in-during-execution* placeholders, not plan
placeholders — the executing engineer fills them from the
research output. The plan itself contains no TBD/TODO/vague
instructions.

Type consistency: filenames, section headings, and pass
numbering are consistent across tasks. The eval doc filename
is the same string in every task.
