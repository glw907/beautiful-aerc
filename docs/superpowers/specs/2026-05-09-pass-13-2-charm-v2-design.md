---
title: Pass 13.2 — charm.land/v2 migration
date: 2026-05-09
status: accepted
---

## Goal

Migrate poplar's UI runtime from
`github.com/charmbracelet/{bubbletea,lipgloss,bubbles}` (v1) to
`charm.land/{bubbletea,lipgloss,bubbles}/v2`. The migration is the
substrate for Pass 14's huh integration, which links only against
v2. It is also the single best opportunity poplar will have to
reframe a handful of v1-era architectural workarounds, because v2's
declarative `tea.View` model expresses them as first-class concerns
rather than imperative hacks.

This spec covers the migration itself plus three architectural
reframes that v2 unlocks, three secondary improvements that ride the
same diff, and three named deferrals.

## Background

Poplar runs on bubbletea v1 (`v1.3.10`), lipgloss v1
(`v1.1.1-0.20250404`), and bubbles v1 (`v1.0.0`). The UI tree is
~109 Go files across `internal/ui/` (App + 7 bubbles-shaped
subpackages + `uicore`), `internal/catkin/`, `internal/theme/`,
`internal/ansix/`, `cmd/poplar/`, plus tests. Every `Update`,
`View`, and `Msg` switch in that tree depends on the v1 API surface.

v2 is a coherent rewrite, not a patch release. The core shifts:

- **Declarative chrome.** `tea.WithAltScreen()` / `EnterAltScreen` /
  `tea.SetWindowTitle()` / mouse / focus-reporting all move from
  Program options + Cmds into fields on the `tea.View` struct that
  `Model.View()` returns each frame.
- **`tea.Cursor` on `tea.View`.** Cursor blinking and positioning
  become a parent concern: the focused child exposes a cursor, the
  parent collects it into its `tea.View`. v1's per-textinput
  `cursor.Model` ticker becomes one ticker at the root.
- **Explicit color profile + background detection.** v2 removes
  auto-detection and `lipgloss.AdaptiveColor`. Apps decide once,
  carry the decision through.
- **Mechanical drift.** Import paths, `tea.KeyMsg` →
  `tea.KeyPressMsg`, `tea.Sequentially` → `tea.Sequence`,
  `spinner.NewModel` → `New`, `viewport.New(w, h)` →
  `New(...Option)`, exported `Width`/`Height`/`Cursor` fields →
  methods on textinput/textarea/viewport/help.
- **Paste as dedicated message types.** `PasteMsg` /
  `PasteStartMsg` / `PasteEndMsg` split off from `KeyMsg`.

Pre-beta posture (ADR-0105) governs how we land this: no compat
shims, no incremental adapter layers. Module-wide single-commit
flip; tree compiles only when migration is complete.

## The three architectural reframes (load-bearing, in 13.2)

### 1. Declarative chrome via `tea.View`

Today `cmd/poplar/root.go` configures alt-screen, mouse mode, focus
reporting, and (eventually) window title as imperative Program
options at startup. The App can never adjust chrome without sending
Cmds; chrome state lives in two places (Program options vs. runtime).

**Reframe.** Drop the Program-options chrome configuration entirely.
`App.View()` returns a `tea.View` whose `AltScreen`, `MouseMode`,
`ReportFocus`, and `WindowTitle` fields are computed every frame
from App state. This makes `App.View()` the single source of truth
for chrome, and per-screen chrome (e.g., a future `--print` flag
suppressing alt-screen) becomes a one-line conditional.

Window title: set to `poplar — <account name>` when an account is
selected; `poplar` otherwise. (Cosmetic but free.)

### 2. Cursor management hoisted to `App.View()`

Today every focusable input owns its own `cursor.Model` with its own
blink ticker. The App is unaware of cursor state; visible cursor
position depends on which child rendered last with a fresh blink.

**Reframe.** Adopt v2's `tea.View.Cursor *tea.Cursor` model. Each
focusable subpackage exposes `Cursor() *tea.Cursor` (returns nil
when unfocused). The App, in its `View()`, walks the focused-child
chain and pulls the active cursor up into its own `tea.View.Cursor`.

Implications:

- One cursor blink ticker at the App level; child cursor.Models go
  away. `tea.NewCursor(x, y)` plus `Style`/`Shape`/`Blink` fields
  declare placement.
- `textinput.VirtualCursor`/`textarea.VirtualCursor` set to `false`
  app-wide — we render the cursor declaratively, not as a styled
  rune in the string.
- State-ownership matches the elm-conventions rule that already
  hoists shared state to the root.

### 3. Drop `lipgloss.AdaptiveColor` entirely

Each theme in `internal/theme/themes.go` is already compiled
mono-mode (One Dark is dark; Solarized Light is light; etc.). v1
still routed some palette slots through `AdaptiveColor` "just in
case." v2 removes `AdaptiveColor` from the public API, offering
`compat.AdaptiveColor` as a migration shim.

**Reframe.** Don't take the shim. Delete the abstraction.
`palette.go` and `themes.go` resolve to concrete `color.Color` at
compile time. Per-subpackage Styles take concrete colors; no
runtime light/dark branching. Smaller surface area; one fewer
abstraction; aligns with the "themes are compiled Go values, not
config" decision from earlier passes.

## The three secondary moves (also in 13.2)

### 4. Cursored subpackages return `tea.View`

Natural completion of #2. If cursor lives on `tea.View`, then any
child *with* a focusable cursor hands the parent a `tea.View`
carrying it, not a string the parent has to re-decorate. Surgical
scope:

- `internal/ui/compose/` (To/Cc/Bcc/Subject/body)
- `internal/ui/contacts/` Form
- `internal/ui/messagelist/` search-mode input
- Any future surface with a focusable input

Other subpackages (`sidebar`, `reader`, `account`, `helppopover`,
`movepicker`, `outbox`) keep `View() string`. The App composes
mixed children: tea.Views contribute content + cursor, strings
contribute content only.

### 5. Compose paste handling via `PasteMsg`

v2 splits bracketed paste from `KeyMsg`. Compose's address fields
currently treat a pasted comma-separated list as a stream of
keystrokes — the autocomplete dropdown flickers and parsing is
incremental.

**Move.** Add `PasteMsg` arms in compose Update:

- Address fields: parse the full payload through
  `content.ParseAddressList`, emit N completed `Name <email>, `
  chips atomically.
- Subject field: paste replaces selection or inserts; no parsing.
- Body field: paste is one atomic Undo unit (Catkin already has
  Undo; this just bundles the chunk).

ADR-worthy because it changes user-visible behavior in compose.

### 6. (Deferred — see "Deferrals" below.)

## Mechanical drift

Sed-able across all .go files:

- `github.com/charmbracelet/bubbletea` → `charm.land/bubbletea/v2`
- `github.com/charmbracelet/lipgloss` → `charm.land/lipgloss/v2`
- `github.com/charmbracelet/bubbles` → `charm.land/bubbles/v2`
- `tea.KeyMsg` → `tea.KeyPressMsg` in switch cases
- `tea.Sequentially` → `tea.Sequence`
- `spinner.NewModel` → `spinner.New`

(`github.com/charmbracelet/x/ansi` stays — only the three
top-level libs moved domains.)

Hand-edit:

- `Model.View() string` → `Model.View() tea.View` for the App + the
  cursored subpackages from #4. Returns `tea.NewView(s)` plus any
  declarative chrome / cursor.
- `tea.WithAltScreen()` etc. dropped from `cmd/poplar/root.go`;
  fields set in `App.View()`.
- `viewport.New(w, h)` → `New(viewport.WithWidth(w),
  viewport.WithHeight(h))`.
- `m.Width = x` → `m.SetWidth(x)`; same for `Height`, `Cursor`,
  `KeyMap`.
- `textinput.Styles` field accesses → nested `Styles.Focused` /
  `Styles.Blurred`.
- `lipgloss.AdaptiveColor{...}` → concrete `lipgloss.Color(...)`
  per theme.
- `lipgloss.Color(s)` return type changed to `color.Color` — fix
  any places that compared colors as values.

Tests:

- `tea.KeyMsg{...}` literals → `tea.KeyPressMsg{...}`.
- `m.View()` assertions: call `.String()` on the returned `tea.View`
  for the App + cursored children; non-cursored children unchanged.
- Cursor-blink tests: rewrite to assert App-level cursor state, not
  per-input.

## Deferrals (named, not done in 13.2)

These are real ideas with merit. They aren't in 13.2 because the
mechanical migration is already substantial and these are additive,
not blocking.

- **`internal/ansix/` audit.** Question: does v2's `lipgloss.Width`
  finally handle SPUA codepoints correctly, making `ansix.Width` /
  `ansix.SetSPUACellWidth` obsolete? Answer: requires measurement
  against the SPUA fixture set. Spin off as Pass 13.3 (or fold into
  Pass 15 polish).

- **Per-subpackage `Styles` restructuring.** v2 textinput's nested
  `Styles.Focused` / `Styles.Blurred` is a clean pattern; our per-
  subpackage `Styles` could mirror it. Real ergonomic win; not
  worth thrashing every `styles.go` mid-migration. Pass 15.

- **Color profile + background threading via `term.Resolve`.**
  Extend the existing startup capability resolution
  (`(IconMode, spuaCellWidth)`) to also return
  `(colorprofile.Profile, isDark bool)`, plumb both into
  `ui.NewApp`. Payoff: explicit downgrade decisions on tty,
  first-run could honor terminal background. Additive; works fine
  via lipgloss defaults during 13.2. Schedule for Pass 15 polish or
  Pass 14.1 if first-run wants it.

## Sequencing

Pre-beta posture: big-bang single commit. The tree won't compile in
intermediate states because `tea.Msg` / `tea.Cmd` types thread
through every Update method. No incremental subpackage flip is
possible without per-subpackage adapter shims, which the no-shims
rule forbids.

Implication for review: this commit is large by design. The ADR
will name the override explicitly so future readers don't reach for
"why didn't they split this?"

## Pass-budget note

Per CLAUDE.md, passes target ~8–12 tasks. This pass is 13. The 13th
task is documentation (ADR + invariants + STATUS), so the
implementation-task count is 12. The work is mechanically large but
single-subsystem (v2 runtime flip with three coherent
architectural moves riding it). Pass-budget rule splits when
subsystems are *unrelated*; here they aren't.

The split-inline tells (per-task review fatigue, hot-file churn,
two unrelated subsystems) don't apply: the work is uniform and
every file changes for the same reason. ADR-0189 will record the
override.

## Tasks

1. `go get charm.land/{bubbletea,lipgloss,bubbles}/v2@latest`;
   codemod imports + `KeyMsg`→`KeyPressMsg` + `Sequentially`→
   `Sequence` + `spinner.NewModel`→`New` across all `.go`. Don't
   try to compile yet.
2. App + non-cursored subpackages: `View()` returns `tea.NewView(s)`
   wrapping the existing string. Verify each subpackage compiles
   in isolation (or as close as the inter-package dependencies
   allow).
3. Cursored subpackages (`compose`, `contacts.Form`, search-mode
   input): `View()` returns a `tea.View` with `Cursor` populated.
   Set `VirtualCursor=false` on every textinput/textarea.
4. App pulls focused-child cursor up: focus-aware composition in
   `App.View()`. Single cursor ticker at the App level. Drop
   per-input `cursor.Model` blink Cmds.
5. Declarative chrome: drop `tea.WithAltScreen`/etc. from
   `cmd/poplar/root.go`; set `view.AltScreen`/`view.MouseMode`/
   `view.ReportFocus`/`view.WindowTitle` in `App.View()`.
6. `bubbles/{textinput,textarea,viewport,help}` field→method:
   `m.Width=x` → `m.SetWidth(x)`; `viewport.New(w,h)` →
   `New(viewport.WithWidth(w), viewport.WithHeight(h))`;
   restructured `textinput.Styles` accesses.
7. Drop `lipgloss.AdaptiveColor` entirely from `palette.go` /
   `themes.go` / per-subpackage `styles.go`. No `compat` shim;
   themes resolve to concrete `color.Color` at compile time.
8. Compose paste handling: `PasteMsg` arms in compose Update;
   address fields parse via `content.ParseAddressList`; body field
   bundles paste as one Catkin Undo unit.
9. Test fixture sweep: `tea.KeyMsg{...}` → `tea.KeyPressMsg{...}`;
   `m.View()` assertions call `.String()` on the returned
   `tea.View` for App + cursored children.
10. `make check` green.
11. tmux capture: 80×24 + 120×40 mainline screens (sidebar +
    messagelist + reader, compose, search results, contacts) vs.
    existing goldens. Update goldens for intentional shifts
    (cursor rendering may move one cell).
12. ADR-0189 (charm.land/v2 migration): document the three
    architectural reframes, the three secondary moves, the three
    deferrals, the big-bang single-commit sequencing override.
    Invariants update: `internal/ui/` chrome is declarative;
    cursor is hoisted; `AdaptiveColor` is gone. Refresh
    `docs/poplar/bubbletea-conventions.md` deprecated-API list to
    name v1 specifics now removed (`tea.WithAltScreen`,
    `tea.Sequentially`, `spinner.NewModel`, `cursor.Model` per
    input, etc.).
13. STATUS pivot to Pass 14; archive plan + spec via `git mv`.

## Risks

- **Goldens churn.** Any cursor shift, color downsample change, or
  alt-screen behavior change shows up in tmux goldens. Budget time
  in task 11 to triage real visual regressions vs. cosmetic shifts.
- **`x/ansi` API drift.** We assume `github.com/charmbracelet/x/ansi`
  stays at its v1 module path. If it pulls a v2 too, ansix needs
  adjusting — but the audit / kill decision still defers to 13.3.
- **Catkin.** `internal/catkin/` is bubbletea-shaped (its `Model`
  embeds tea.Model). Migration touches it the same as the UI tree;
  no special handling needed beyond the mechanical drift.
- **Compose `PasteMsg`.** New behavior in address-field parsing
  could surprise users who paste single addresses. Mitigation: the
  parse path falls back gracefully when input has no commas (treats
  as one chip) — matches today's behavior for the single-address
  case.

## Out of scope

- Pass 14 (first-run wizard) — runs against the migrated tree
  without re-planning.
- huh integration — Pass 14.
- ansix audit — Pass 13.3 or Pass 15.
- Per-subpackage Styles restructuring — Pass 15.
- Color profile / background threading via `term.Resolve` — Pass
  14.1 or Pass 15.
