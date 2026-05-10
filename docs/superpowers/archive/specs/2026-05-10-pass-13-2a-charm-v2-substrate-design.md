---
title: Pass 13.2a — charm.land/v2 substrate
date: 2026-05-10
status: accepted
---

## Goal

Migrate poplar's UI runtime from
`github.com/charmbracelet/{bubbletea,lipgloss,bubbles}` (v1) to
`charm.land/{bubbletea,lipgloss,bubbles}/v2` as a *substrate* pass —
the tree compiles on v2, every test passes, every theme renders, but
the App.View() composition still has its v1 shape (returning the
existing assembled string wrapped in `tea.NewView(s)` with no
declarative chrome / cursor wiring yet).

The three architectural reframes that v2 unlocks (declarative chrome,
hoisted cursor, paste handling) live in Pass 13.2b. Splitting was a
mid-execution call: 13.2 originally bundled both substrate and
reframes per ADR-0189's draft, but mid-pass we discovered the
spec's "Mechanical drift" section was incomplete (200+ KeyPressMsg
field-access call sites — Code/Text/Mod replacing v1's
Type/Runes/Alt/Paste — had to be added as Task 1.5), and bundled
tasks 3+4+5 into a single dispatch because splitting them would
have left intermediate scaffolding inside the pass. Both signals
fired the split-inline rule (CLAUDE.md "Migrations and breaking
changes" + memory feedback "Split passes inline on fatigue
tells"). Splitting now lets each pass tell a coherent story.

## Background

Poplar runs on bubbletea v1 (`v1.3.10`), lipgloss v1
(`v1.1.1-0.20250404`), and bubbles v1 (`v1.0.0`). The UI tree is
~109 Go files across `internal/ui/` (App + 7 bubbles-shaped
subpackages + `uicore`), `internal/catkin/`, `internal/theme/`,
`internal/ansix/`, `cmd/poplar/`, plus tests. Every `Update`,
`View`, and `Msg` switch in that tree depends on the v1 API surface.

v2 is a coherent rewrite, not a patch release. The shifts that 13.2a
absorbs:

- **Mechanical drift.** Import paths, `tea.KeyMsg` →
  `tea.KeyPressMsg`, `tea.Sequentially` → `tea.Sequence`,
  `spinner.NewModel` → `New`, `viewport.New(w, h)` →
  `New(...Option)`, exported `Width`/`Height` fields → methods on
  textinput/textarea/viewport/help.
- **`tea.KeyPressMsg` field layout.** v2's KeyPressMsg embeds the
  `Key` struct (`Code rune`, `Text string`, `Mod KeyMod`,
  `ShiftedCode/BaseCode rune`, `IsRepeat bool`) — fundamentally
  different from v1's `KeyMsg{Type, Runes, Alt, Paste}`. Every
  field-access call site shifts (200+ occurrences, mostly in
  tests). `tea.KeyRunes`, `tea.KeyCtrlO/X/L/T/C/G`, `tea.KeyShiftTab`
  constants are gone — detect via `Mod`/`Code` instead.
- **Color profile + `lipgloss.AdaptiveColor` removal.** v2 removes
  `AdaptiveColor` from the public API. Each poplar theme already
  resolves to one mode at compile time; we drop the abstraction
  outright (no `compat.AdaptiveColor` shim) and let palette /
  themes / per-subpackage styles take concrete `color.Color`.
  `lipgloss.Color(s)` is now a function returning `color.Color`,
  not a struct type — value-comparison sites need updating.

Pre-beta posture (ADR-0105) governs how we land this: no compat
shims, no incremental adapter layers. Module-wide single-commit
flip; the tree compiles only when 13.2a is complete.

## What 13.2a explicitly does NOT do

These belong to Pass 13.2b. Listing them here so the substrate
pass's scope is unambiguous:

- **Declarative chrome.** `tea.WithAltScreen()` and friends stay on
  the `tea.NewProgram(...)` call in `cmd/poplar/root.go` for now.
  `App.View()` returns `tea.NewView(s)` with no `AltScreen` /
  `MouseMode` / `ReportFocus` / `WindowTitle` fields populated. The
  transitional wrapper is the published end state of 13.2a, not
  scaffolding.
- **Cursor hoist.** Per-input `cursor.Model` blink Cmds keep working
  through v2's compatibility surface (or any local indirection that
  keeps them rendering); `VirtualCursor` defaults stand. Cursored
  subpackages still return `View() string`. The single-ticker
  refactor and `tea.View.Cursor` adoption is 13.2b's first task.
- **Paste handling.** `tea.PasteMsg` arms in compose Update are
  13.2b. In 13.2a the catkin paste handler was deleted outright
  (it was a v1 `Paste`-flag check); URL-paste wrapping comes back
  in 13.2b's PasteMsg arm.
- **`internal/ansix/` audit.** Whether v2's `lipgloss.Width` makes
  `ansix.Width` / `ansix.SetSPUACellWidth` obsolete defers to Pass
  13.3 or Pass 15 — needs SPUA-fixture measurement.
- **Per-subpackage `Styles` restructuring.** v2 textinput's nested
  `Styles.Focused` / `Styles.Blurred` is a clean pattern poplar's
  per-subpackage Styles could mirror; defers to Pass 15.
- **Color profile + isDark threading via `term.Resolve`.** Defers
  to Pass 14.1 or Pass 15 (additive, works fine via lipgloss
  defaults until then).

## Mechanical drift in scope

Sed-able across all .go files (Task 1, complete):

- `github.com/charmbracelet/bubbletea` → `charm.land/bubbletea/v2`
- `github.com/charmbracelet/lipgloss` → `charm.land/lipgloss/v2`
- `github.com/charmbracelet/bubbles` → `charm.land/bubbles/v2`
- `tea.KeyMsg` → `tea.KeyPressMsg` in switch cases
- `tea.Sequentially` → `tea.Sequence`
- `spinner.NewModel` → `spinner.New`

(`github.com/charmbracelet/x/ansi` stays — only the three top-level
libs moved domains.)

KeyPressMsg field drift (Task 1.5, complete):

| v1 | v2 |
|---|---|
| `k.Type == tea.KeyEnter` (special key) | `k.Code == tea.KeyEnter` |
| `k.Type == tea.KeyRunes` | `len(k.Text) > 0` |
| `k.Runes[0]`, `string(k.Runes)` | `k.Code`, `k.Text` |
| `k.Alt` | `k.Mod & tea.ModAlt != 0` |
| `k.Paste` | gone — paste is `tea.PasteMsg` (13.2b) |
| `tea.KeyCtrlO` etc. | `k.Mod&tea.ModCtrl != 0 && k.Code == 'o'` |
| `tea.KeyShiftTab` | `k.Mod&tea.ModShift != 0 && k.Code == tea.KeyTab` |
| `tea.KeyPressMsg{Type: tea.KeyRunes, Runes: []rune{'X'}}` (test literal) | `tea.KeyPressMsg{Code: 'X', Text: "X"}` |

Hand-edit (Tasks 4–6 in this pass):

- `Model.View() string` → `Model.View() tea.View` for the **App
  only**. Returns `tea.NewView(s)` — no chrome / cursor fields yet.
  Cursored subpackages keep `View() string` until 13.2b.
- `viewport.New(w, h)` → `New(viewport.WithWidth(w),
  viewport.WithHeight(h))`.
- `m.Width = x` → `m.SetWidth(x)`; same for `Height`.
- `textinput.Styles` field accesses → nested `Styles.Focused` /
  `Styles.Blurred`.
- `lipgloss.AdaptiveColor{...}` → concrete `lipgloss.Color(...)`
  per theme.
- `lipgloss.Color(s)` return type changed to `color.Color` — fix
  any places that compared colors as values.

Tests (Task 6 in this pass):

- `tea.KeyMsg{...}` literals → `tea.KeyPressMsg{...}` (done in
  Task 1.5).
- `m.View()` assertions on the App: call `.String()` on the
  returned `tea.View`. Cursored subpackage tests stay on
  `View() string` until 13.2b.

## The transitional App.View() seam

`App.View()` after 13.2a is:

    func (m App) View() tea.View {
        s := m.assembleFrame() // existing v1-shape composition
        return tea.NewView(s)
    }

This is the **published end state** of 13.2a, not a TODO. ADR-0189a
documents it as the seam to 13.2b. The v1 `tea.WithAltScreen()` on
`tea.NewProgram` continues to handle alt-screen entry. Per-input
cursor.Model blink Cmds continue to render the cursor as a styled
rune. No part of poplar reads `tea.View.Cursor`,
`tea.View.AltScreen`, etc. yet.

13.2b's first commit will replace this wrapper with the v2-native
declarative composition (chrome fields populated from App state,
cursor hoisted from focused child).

## Sequencing

Pre-beta big-bang within 13.2a. The tree won't compile in
intermediate task states (theme drift + bubbles drift + KeyPressMsg
drift all thread through every Update method), but the seam between
13.2a and 13.2b is documented and clean: 13.2a closes with `make
check` green; 13.2b begins from that green substrate.

## Pass-budget

Per CLAUDE.md, passes target ~8–12 tasks with one ADR. 13.2a is 9
tasks (including the ADR). Single subsystem (v2 mechanical
substrate); no scope creep candidates.

## Tasks

1. ✅ `go get charm.land/{bubbletea,lipgloss,bubbles}/v2@latest`;
   codemod imports + `KeyMsg`→`KeyPressMsg` + `Sequentially`→
   `Sequence` + `spinner.NewModel`→`New` across all `.go`.
2. ✅ `tea.KeyPressMsg` field-access drift: Code/Text/Mod
   replacements; KeyCtrl* and KeyShiftTab constants gone; test
   literals rewritten; catkin paste handler deleted (13.2b owns
   the PasteMsg arm).
3. ✅ App.View() returns `tea.View` via `tea.NewView(s)` —
   transitional, documented as the seam to 13.2b. Non-cursored
   subpackages keep `View() string`.
4. `bubbles/{textinput,textarea,viewport,help}` field→method:
   `m.Width=x` → `m.SetWidth(x)`; `viewport.New(w,h)` →
   `New(viewport.WithWidth(w), viewport.WithHeight(h))`;
   restructured `textinput.Styles.Focused` / `Blurred` accesses;
   `textarea.SetCursor` and adjacent method drift.
5. Drop `lipgloss.AdaptiveColor` entirely from `palette.go` /
   `themes.go` / per-subpackage `styles.go`. No `compat` shim;
   themes resolve to concrete `color.Color` at compile time. Fix
   any `lipgloss.Color(s)` value-comparison sites for the
   `color.Color` return-type change.
6. Test fixture sweep: any remaining v1 patterns the prior tasks
   missed (App `m.View()` assertions calling `.String()`; theme
   tests that previously compared `lipgloss.Color` values; etc.).
   Cursored subpackage tests stay on `View() string` until 13.2b.
7. `make check` green (gofmt + vet + voice + test).
8. ADR-0189a (charm.land/v2 substrate): document the v2 mechanical
   migration, the KeyPressMsg field-layout drift surprise, the
   AdaptiveColor removal, the transitional `App.View()` seam, and
   the 13.2a/13.2b split rationale. Refresh
   `docs/poplar/bubbletea-conventions.md` deprecated-API list to
   name the v1 specifics now removed at the substrate level
   (`tea.Sequentially`, `spinner.NewModel`, `lipgloss.AdaptiveColor`,
   `viewport.New(w, h)` positional, exported `Width`/`Height`
   fields, `tea.KeyMsg`'s field layout). Invariants update:
   `internal/theme/` palette is concrete; `tea.KeyPressMsg.Code`
   is the canonical key-test field. Chrome / cursor invariants
   land in 13.2b.
9. STATUS pivot to Pass 13.2b; archive 13.2a plan + spec via
   `git mv`.

## Risks

- **`x/ansi` API drift.** Assumes `github.com/charmbracelet/x/ansi`
  stays at its v1 module path. If it pulls a v2 too, `ansix`
  adjusts; the audit/kill decision still defers to 13.3.
- **Catkin.** `internal/catkin/` is bubbletea-shaped (its `Model`
  embeds tea.Model). Migration touches it the same as the UI tree;
  no special handling beyond mechanical drift. Catkin's `Paste`
  flag in v1 has no v2 equivalent on KeyPressMsg — the handler was
  deleted in Task 1.5; URL-paste wrapping returns in 13.2b.
- **Goldens churn.** Substrate-only goldens churn (palette → concrete
  color, occasional rendering shifts from v2's color profile defaults)
  is in scope for Task 7 if `make check` flags it. Any visual shift
  driven by the deferred reframes (chrome, cursor) shows up in 13.2b.

## Out of scope (named, deferred to 13.2b or later)

- Declarative chrome reframe (13.2b)
- Cursor hoist + cursored subpackages return `tea.View` (13.2b)
- Compose `tea.PasteMsg` arms + catkin URL-paste wrapping (13.2b)
- `internal/ansix/` audit (Pass 13.3 or 15)
- Per-subpackage `Styles` restructuring (Pass 15)
- Color profile + isDark threading via `term.Resolve` (Pass 14.1 or 15)

## Execution recommendation

Subagent-driven with the no-scars discipline from CLAUDE.md
"Migrations and breaking changes." Each task lands its slice
cleanly — no `// TODO(pass-13.2-task-N)` markers spanning tasks,
no commented-out v1 logic preserved as "reference." When two tasks
touch a file, the earlier deletes; the later writes the v2-native
replacement.
