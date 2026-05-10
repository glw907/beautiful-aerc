---
title: Pass 13.1.5 — Claude infrastructure v2 prep
date: 2026-05-09
status: ready-to-execute
---

## Goal

Update poplar's Claude-facing binding artifacts (skills, conventions
docs, hooks, pass ritual) to describe **bubbletea/lipgloss/bubbles
v2** before Pass 13.2 lands the code migration. The contracts then
guide Pass 13.2 instead of being retrofit to it.

This is a doc-only pass. No Go code changes. No `make check`. No
`make install`. One commit. STATUS.md gets a one-line pass-table
entry.

## Pre-execution context

Pass 13.2 is the charm.land/v2 migration. Spec at
`docs/superpowers/specs/2026-05-09-pass-13-2-charm-v2-design.md`.
The migration's three architectural reframes (declarative chrome,
cursor hoist, drop AdaptiveColor) plus three coherence moves
(cursored-children-return-tea.View, PasteMsg in compose, native
clipboard) need to be encoded as conventions before code lands.

Read these in full before executing this plan, in this order:

1. `docs/superpowers/specs/2026-05-09-pass-13-2-charm-v2-design.md`
2. `~/.claude/skills/elm-conventions/SKILL.md`
3. `docs/poplar/bubbletea-conventions.md`
4. `.claude/skills/poplar-pass/SKILL.md`
5. `~/.claude/skills/bubbletea-design/SKILL.md` (verify no edits needed)
6. `.claude/rules/ui-invariants.md` (verify left alone)

## Scope discipline

**Prescriptive vs descriptive.** Two classes of doc:

- **Prescriptive** ("how new UI code should be written"). Update
  these now to describe v2 patterns. Pass 13.2 conforms to them.
- **Descriptive** ("what poplar's code currently is"). Leave alone
  until Pass 13.2 ships. Updating descriptive docs to v2 while code
  is still v1 makes them lie about reality.

| File | Class | This pass? |
|---|---|---|
| `~/.claude/skills/elm-conventions/SKILL.md` | prescriptive | yes |
| `docs/poplar/bubbletea-conventions.md` | prescriptive | yes |
| `.claude/skills/poplar-pass/SKILL.md` (step 1b) | prescriptive | yes |
| `~/.claude/skills/bubbletea-design/SKILL.md` | prescriptive | verify only |
| Research docs `2026-04-26-*.md` | historical reference | header note only |
| `.claude/rules/ui-invariants.md` | descriptive | no — Pass 13.2 |
| `docs/poplar/invariants.md` | descriptive | no — Pass 13.2 |
| `docs/poplar/styling.md` | descriptive | no — Pass 13.2 |
| `docs/poplar/system-map.md` | descriptive | no — Pass 13.2 |
| Memory `feedback_no_multikey_sequences.md` | descriptive | no — Pass 13.2 |
| `.claude/hooks/bubbletea-conventions-lint.sh` | enforcement | no — post-13.2 |
| New `.claude/hooks/v1-bubbletea-guard.sh` | enforcement | no — post-13.2 |
| New `docs/poplar/v2-features-used.md` | inventory | no — seeded post-13.2 |

## Tasks

### Task 1 — Update `~/.claude/skills/elm-conventions/SKILL.md`

This is a global skill, ~12KB. Patterns are version-independent;
only API references in code examples need refreshing.

**Edits:**

- **Line ~140** (Rule 4, "Right" example): change
  `case tea.KeyMsg:` to `case tea.KeyPressMsg:`. The interface
  `tea.KeyMsg` still exists in v2 but `KeyPressMsg` is the precise
  match for key presses (excludes releases).
- **Line ~353** (Rule 7, "Right" example): change
  `if k, ok := msg.(tea.KeyMsg); ok` to
  `if k, ok := msg.(tea.KeyPressMsg); ok`.
- **Add Rule 8 (or extend Rule 5) — Cursor hoist:**
  Focused children expose `Cursor() *tea.Cursor` (returns nil when
  unfocused). The parent App walks the focus chain in its `View()`
  and pulls the active cursor up into its own `tea.View.Cursor`.
  `VirtualCursor` is set to `false` on every textinput/textarea so
  the App is the cursor authority. State-ownership matches the
  shared-state-at-the-root rule. Cite as v2 idiom. Include
  Right/Wrong examples (Right: child returns `*tea.Cursor`; Wrong:
  child renders cursor inline as a styled rune).
- **Update the `description:` frontmatter** to mention v2 explicitly:
  "Mandatory rules for the bubbletea v2 UI layer..."

No structural rewrite. Patterns (state-in-models, mutations-in-Update,
I/O-in-Cmd, Msg-driven children, state-ownership, size-contract,
key.Binding) all hold under v2.

### Task 2 — Rewrite `docs/poplar/bubbletea-conventions.md`

This is the most substantial edit. The doc is auto-loaded for UI
work. Sections need varied treatment.

**§1 Component shape** — replace the simple `View() string`
contract with a two-tier contract:

- Non-cursored children: `View() string`. Returns a content string
  honoring the size contract.
- Cursored children + the App root: `View() tea.View`. Returns a
  `tea.View` with `Content` set to the size-contracted string and
  `Cursor` populated when focused.

State which subpackages are cursored (compose, contacts.Form,
search-mode messagelist input). Note `tea.NewView(s)` as the
sugar for non-cursored wrapping.

**§2 Sizing contract** — mostly unchanged. The clipPane idiom
still applies. Update the `viewport.New(width, height)` reference
to mention the v2 options pattern: `viewport.New(viewport.WithWidth(w),
viewport.WithHeight(h))`.

**§4 Key bindings** — minor: examples show `case tea.KeyMsg:` →
update to `case tea.KeyPressMsg:`. The rest (key.Binding,
key.Matches) is unchanged.

**§5 Async I/O and update flow** — add a subsection on **PasteMsg**:
v2 splits bracketed paste from KeyMsg into `tea.PasteMsg` /
`tea.PasteStartMsg` / `tea.PasteEndMsg`. Components handling text
entry must add a PasteMsg arm to their Update for atomic paste
handling (parsing, single Undo unit). Cite compose's address-list
parsing as the canonical example.

**§6 Program setup** — major rewrite. Replace the entire section:

- `tea.NewProgram` no longer takes chrome options
  (`WithAltScreen`, `WithMouseCellMotion`, etc.). These move to
  declarative fields on the `tea.View` returned by the App's
  `View()` method.
- The App computes `view.AltScreen`, `view.MouseMode`,
  `view.ReportFocus`, `view.WindowTitle` every frame from App
  state. Single source of truth.
- `tea.NewProgram(rootModel, tea.WithColorProfile(...))` is the
  only legitimate program-options shape. Color profile, environment,
  initial window size, input/output streams are still program-options
  concerns.
- Spinner pattern: `spinner.New(spinner.WithSpinner(spinner.Dot))`
  unchanged. `m.spinner.Tick` still returned from Init/Update.

Add a **Cursor hoist** subsection: same content as elm-conventions
Rule 8, but more poplar-specific (which subpackages are cursored,
how App.View() walks the focus chain).

Add a **Declarative chrome** subsection: the App.View() pattern;
how to make per-screen chrome decisions (e.g., a future `--print`
mode suppressing alt-screen via `view.AltScreen = false`).

**§7 Layout patterns from reference apps** — unchanged. The
SetSize-on-Component-interface pattern, margin subtraction, root
short-circuits-on-focused-input, theme-as-semantic-tokens — all
version-independent.

**§8 Anti-patterns** — update the "Using deprecated APIs" entry
substantially:

- `viewport.HighPerformanceRendering` — gone in v2.
- `tea.Sequentially` — renamed `tea.Sequence`.
- `spinner.Tick()` (package-level no-arg form) — removed; use
  `m.Tick`.
- `*Model.NewModel` constructors — removed; use `New(...)`.
- **(new)** `tea.WithAltScreen()` and similar chrome options to
  `tea.NewProgram` — removed; set on `tea.View` fields.
- **(new)** `tea.EnterAltScreen` / `tea.HideCursor` Cmds — removed;
  declarative on `tea.View`.
- **(new)** `lipgloss.AdaptiveColor` — removed; use concrete
  `lipgloss.Color()` with compile-time-resolved theme. (Note: the
  `compat.AdaptiveColor` shim exists; poplar deliberately does not
  use it — themes are mono-mode.)
- **(new)** `cursor.Model` per textinput/textarea — replaced by
  `tea.Cursor` hoist. Set `VirtualCursor=false` on the inputs.
- **(new)** `viewport.New(w, h)` positional constructor —
  removed; use `viewport.New(viewport.WithWidth(w),
  viewport.WithHeight(h))`.
- **(new)** `tea.KeyMsg{...}` struct literal — `KeyMsg` is now an
  interface; literals must be `tea.KeyPressMsg{...}`.
- **(new)** Exported `Width`/`Height` field assignment on
  textinput/textarea/viewport/help — replaced by `SetWidth(n)` /
  `SetHeight(n)` methods.

Update the "EnterAltScreen() in Init()" entry to "EnterAltScreen
Cmd anywhere" — v2 makes it declarative on `tea.View`.

Add **(new)** anti-patterns:

- **`compat.AdaptiveColor` shim usage.** Poplar themes are
  compiled mono-mode. The shim is a migration aid for apps that
  truly need runtime light/dark; we don't.
- **Cursor rendering as styled rune in input string.** When a child
  is cursored, expose `Cursor() *tea.Cursor` and let the App pull
  it up. Inline cursor rendering bypasses v2's tea.Cursor primitive.
- **`atotto/clipboard` for clipboard access.** v2's
  `tea.SetClipboard` / `tea.ReadClipboard` is the canonical path
  (works over SSH via OSC 52).
- **Imperative chrome via tea.Cmd** (`tea.EnterAltScreen`-style).
  Set fields on the returned `tea.View` instead.

**§9 Planning checklist** — append:

- [ ] If the new component holds a focusable cursor, does it
      expose `Cursor() *tea.Cursor`? Is `VirtualCursor=false` on
      its textinput/textarea?
- [ ] If the pass adds chrome behavior (alt-screen, mouse mode,
      window title, focus reporting), is it set declaratively on
      the App's `tea.View` rather than via Program options or Cmds?

**§10 Review checklist** — update the deprecated-APIs item to
match the new §8 list. Append:

- [ ] Cursored subpackages return `tea.View` with `Cursor`
      populated when focused; non-cursored return `string`.
- [ ] App.View() returns `tea.View` with declarative chrome fields
      (AltScreen, MouseMode, ReportFocus, WindowTitle) computed
      from App state.
- [ ] No imperative chrome Cmds (`tea.EnterAltScreen`,
      `tea.HideCursor`, etc.) — those are removed in v2.
- [ ] No `lipgloss.AdaptiveColor` (or `compat.AdaptiveColor`).
      Themes resolve to concrete `color.Color` at compile time.

**See also** — keep the existing references. The research docs
remain authoritative for bubbletea internals, with the note added
in Task 5 below.

### Task 3 — Update `.claude/skills/poplar-pass/SKILL.md` step 1b

The "Idiomatic-bubbletea check" deprecated-API list (around line
79-82) currently reads:

```
- [ ] No deprecated API usage (`HighPerformanceRendering`,
      `tea.Sequentially`, package-level `spinner.Tick`,
      `*Model.NewModel`, `EnterAltScreen`/`EnableMouse*` in
      `Init`).
```

Replace with the expanded v2 list. The full list lives in
`bubbletea-conventions.md` §8 — the skill's checklist points there
and itemizes the most common offenders:

```
- [ ] No v1-only API usage (see bubbletea-conventions.md §8 for
      the full deprecated list). Common offenders:
      `HighPerformanceRendering`, `tea.Sequentially`,
      `spinner.Tick()` (package-level), `*Model.NewModel`,
      `tea.WithAltScreen()` Program option,
      `tea.EnterAltScreen`/`HideCursor` Cmds,
      `lipgloss.AdaptiveColor`, `cursor.Model` per input,
      `viewport.New(w, h)` positional, `tea.KeyMsg{...}` literal,
      `m.Width = n` field assignment on textinput/textarea/
      viewport/help.
```

Append two new checklist items to step 1b (after the existing
ones):

```
- [ ] App.View() returns `tea.View` with declarative chrome (AltScreen,
      MouseMode, ReportFocus, WindowTitle) — not via Program
      options or Cmds.
- [ ] Cursored children expose `Cursor() *tea.Cursor`; App pulls
      focused child's cursor into `tea.View.Cursor`. `VirtualCursor=false`
      on inputs.
```

### Task 4 — Verify `~/.claude/skills/bubbletea-design/SKILL.md` needs no edits

Read the file. Confirm:

- No references to `tea.KeyMsg`, `tea.WithAltScreen`,
  `lipgloss.AdaptiveColor`, `cursor.Model`, `spinner.NewModel`,
  `viewport.New(w, h)`.
- Visual-design content (color, icons, density, box-drawing,
  spacing) is version-independent.

If clean: no edit. If a stray v1 reference exists: edit.

(Initial scan found no v1 references. Expect this task to be
no-op.)

### Task 5 — Add header note to research docs

Both files:

- `docs/poplar/research/2026-04-26-bubbletea-norms.md`
- `docs/poplar/research/2026-04-26-reference-apps.md`

Add this block immediately after the existing frontmatter or at
the very top of the body if no frontmatter:

```markdown
> **Note (2026-05-09):** Researched against
> bubbletea v1.3.10, lipgloss v1.1.x, bubbles v1.0.0. Pass 13.2
> migrates poplar to charm.land/{bubbletea,lipgloss,bubbles}/v2;
> some upstream APIs cited below have been renamed, restructured,
> or removed in v2. The patterns this research grounds (size
> contract, JoinHorizontal trust, key.Binding declarations,
> Cmd-isolated I/O, etc.) all hold under v2 — only specific API
> names changed. `docs/poplar/bubbletea-conventions.md` supersedes
> for any conflict; load this doc only when the conventions doc is
> silent and primary sources are needed.
```

Don't rewrite the body. The research is historical and remains
useful for understanding how the conventions were derived.

### Task 6 — STATUS.md update

Add a row to the Passes table between 13.1 and 13.2:

```
| 13.1.5 | Claude infrastructure v2 prep (skills, conventions, pass ritual) | done |
```

Mark 13.1.5 done immediately (it's done by the time the commit
lands). The starter prompt for 13.2 stays as-is.

If 13.1.5 doesn't fit on one line under STATUS.md's 60-line limit,
trim verbosity elsewhere in the file. Most likely the table itself
needs no compression — it already lists 13.1, 13.2, etc.

### Task 7 — Commit

Stage only the files touched in tasks 1–6. No `make check` (no Go
code), no `make install`.

```
git add ~/.claude/skills/elm-conventions/SKILL.md \
        docs/poplar/bubbletea-conventions.md \
        .claude/skills/poplar-pass/SKILL.md \
        docs/poplar/research/2026-04-26-bubbletea-norms.md \
        docs/poplar/research/2026-04-26-reference-apps.md \
        docs/poplar/STATUS.md \
        docs/superpowers/plans/2026-05-09-pass-13-1-5-claude-infra-v2-prep.md

git commit -m "$(cat <<'EOF'
Pass 13.1.5: Claude infrastructure v2 prep

Update prescriptive contracts (skills, conventions doc, pass
ritual) to describe bubbletea/lipgloss/bubbles v2 patterns ahead
of Pass 13.2's code migration. Pass 13.2 conforms to the new
contracts instead of being retrofit to them.

- elm-conventions skill: KeyMsg→KeyPressMsg in examples; new
  Rule 8 (cursor hoist).
- bubbletea-conventions.md: tea.View return contract for
  cursored children + App; declarative chrome; PasteMsg
  handling; expanded deprecated-API list; cursor hoist +
  declarative chrome added to planning + review checklists.
- poplar-pass step 1b: deprecated-API list updated; two new
  checklist items (declarative chrome, cursor hoist).
- Research docs: header note flagging v1-era research.

Descriptive docs (invariants.md, ui-invariants.md, styling.md,
system-map.md) are unchanged this pass — they describe current
v1 reality and update during Pass 13.2's pass-end ritual.

Plan: docs/superpowers/plans/2026-05-09-pass-13-1-5-claude-infra-v2-prep.md

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"

git push
```

No `make install` (no binary changes).

### Task 8 — Archive this plan

After commit lands, archive the plan via `git mv`:

```
git mv docs/superpowers/plans/2026-05-09-pass-13-1-5-claude-infra-v2-prep.md \
       docs/superpowers/archive/plans/
git commit -m "Archive Pass 13.1.5 plan

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
git push
```

(Or fold the archive move into Task 7's commit. Either works.)

## Verification

This pass has no `make check` step. Verify by:

1. **Reading each touched file end-to-end** after edits. The
   prescriptive docs must be internally consistent: no remaining
   v1 API references in places that describe how to write new code.
2. **Cross-reference check.** `bubbletea-conventions.md` references
   the research docs — those references still work.
   `poplar-pass` step 1b points to `bubbletea-conventions.md` §8 —
   that section now exists in the expanded form.
3. **STATUS.md line count** ≤ 60 (enforced by hook on
   `.claude/hooks/claude-md-size.sh` if it covers STATUS; check).

If a deferred file (descriptive class) was accidentally touched,
revert that change before committing.

## Out of scope

- Any Go code change.
- Any update to `internal/ui/`, `internal/theme/`, etc.
- Hook updates (`bubbletea-conventions-lint.sh` v2 rules,
  `v1-bubbletea-guard.sh` new hook). Those land post-13.2 because
  they'd block legitimate WIP during 13.2 itself.
- Seeding `docs/poplar/v2-features-used.md`. That's an inventory
  of *shipped* v2 features; populate as 13.2+ land them.
- Updating `.claude/rules/ui-invariants.md`, `docs/poplar/invariants.md`,
  `docs/poplar/styling.md`, `docs/poplar/system-map.md`. These are
  descriptive — they update during Pass 13.2's pass-end ritual.
- Updating memory file `feedback_no_multikey_sequences.md` (KeyMsg
  reference). Same reasoning — it describes current behavior;
  Pass 13.2 updates it.

## Notes for execution

- The pass is mechanical. No design decisions remain — all settled
  in the conversation that produced this plan and in the Pass 13.2
  spec.
- Read the Pass 13.2 spec (`docs/superpowers/specs/2026-05-09-pass-13-2-charm-v2-design.md`)
  before starting — it's the source of truth for what v2 patterns
  the conventions doc describes.
- This pass does NOT trigger `poplar-pass` end ritual. No ADR (the
  plan + commit message are the record). No invariants.md update
  (descriptive class). No `make install`.
- Total work estimate: small. The largest edit is the
  bubbletea-conventions.md rewrite, which is section-by-section
  surgical not full-rewrite.
- One commit covers tasks 1–6. Task 8 (archive) is a separate
  trivial commit or folded in.

## Pass-budget note

This is a small prep pass — 7 working tasks, no Go code, one ADR's
worth of doc work. Outside the normal 8–12 task budget on the low
end, but appropriate for a doc-only prep pass that exists to set
the contract for Pass 13.2.
