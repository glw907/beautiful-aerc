---
title: Pass 13.2b — charm.land/v2 reframes
date: 2026-05-10
status: ready
---

## Goal

With Pass 13.2a having delivered the v2 substrate (imports flipped,
KeyPressMsg field-access migrated, AdaptiveColor dropped, palette
concrete, `make check` green), 13.2b lands the three architectural
reframes that v2's `tea.View` model unlocks:

1. **Declarative chrome.** Drop `tea.WithAltScreen()` / mouse mode /
   focus reporting / window title from the `tea.NewProgram` call.
   Set them as fields on the `tea.View` returned by `App.View()`
   each frame. Single source of truth.
2. **Hoisted cursor.** Replace per-input `cursor.Model` blink Cmds
   with a single ticker at the App. Cursored subpackages expose
   their cursor via `Cursor() *tea.Cursor`; App walks the focused-
   child chain and assigns the active cursor to its returned
   `tea.View.Cursor`.
3. **Compose paste handling.** Add `tea.PasteMsg` arms to compose
   Update. Address fields parse the full payload through
   `content.ParseAddressList` and emit chips atomically. Body
   bundles paste as one Catkin Undo unit. Catkin's URL-paste
   wrapping returns as part of the body arm.

The starting point is the transitional `App.View()` from 13.2a:

    func (m App) View() tea.View {
        s := m.assembleFrame()
        return tea.NewView(s)
    }

13.2b's first task replaces that wrapper with the v2-native
declarative composition.

## Background

13.2a's substrate established:

- v2 imports throughout `internal/`, `cmd/`, tests
- `tea.KeyPressMsg.Code` / `Text` / `Mod` as the canonical key-test
  surface (no `Type`/`Runes`/`Alt`/`Paste`)
- Palette and themes concrete (no `AdaptiveColor`)
- `make check` green; tmux goldens unchanged for the substrate's
  v1-shaped App.View()

The reframes turn poplar's UI runtime from "v2 substrate compiling
under v1 architecture" into "v2-native poplar." Together they're
load-bearing for Pass 14's huh integration: huh/v2's modal UX
expects declarative chrome and cursor hoisting to be the host app's
norm.

## The three reframes

### 1. Declarative chrome via `tea.View`

Today (after 13.2a) `cmd/poplar/root.go` configures alt-screen via
`tea.WithAltScreen()` on the `tea.NewProgram` call. The App can't
adjust chrome without restarting; chrome state lives outside
App.View().

**Reframe.** Drop the Program-options chrome configuration entirely.
`App.View()` returns a `tea.View` whose `AltScreen`, `MouseMode`,
`ReportFocus`, and `WindowTitle` fields are computed every frame
from App state. `App.View()` becomes the single source of truth
for chrome.

Window title:

- `poplar — <account name>` when an account is selected
- `poplar` otherwise

A future `--print` flag suppressing alt-screen becomes a one-line
conditional in `App.View()`.

### 2. Cursor management hoisted to `App.View()`

After 13.2a, every focusable input still owns its own
`cursor.Model` with its own blink ticker. The App is unaware of
cursor state.

**Reframe.** Adopt v2's `tea.View.Cursor *tea.Cursor` model. Each
focusable subpackage exposes `Cursor() *tea.Cursor` (returns nil
when no input in that subpackage is focused). The App, in its
`View()`, walks the focused-child chain (modals topmost: confirm >
conflict > outbox > help > linkpicker > attach > move > form >
popover > compose > messagelist search > account-tab default) and
pulls the first non-nil cursor up into its own `tea.View.Cursor`.

Implications:

- One cursor blink ticker at the App level (whatever v2's API
  exposes for that — read `bubbletea/v2/cursor.go` before designing).
  Child `cursor.Model` instances and their `cursor.Blink` Cmds go
  away.
- `textinput.VirtualCursor` / `textarea.VirtualCursor` set to
  `false` everywhere — render the cursor declaratively, not as a
  styled rune in the string.
- State-ownership matches the elm-conventions rule that already
  hoists shared state to the root.
- Coordinate-space: confirm whether `tea.View.Cursor` position is
  global or local before designing the App-side composition. If
  local, cursored children expose a `CursorOffset()` accessor or
  return their cursor in their `View()` and the App offsets during
  the JoinHorizontal/Vertical assembly.

### 3. Cursored subpackages return `tea.View`

Natural completion of #2. If cursor lives on `tea.View`, then any
child *with* a focusable cursor hands the parent a `tea.View`
carrying it, not a string the parent has to re-decorate.

Surgical scope:

- `internal/ui/compose/` (To/Cc/Bcc/Subject as bubbles textinputs,
  body as catkin)
- `internal/ui/contacts/` Form (name/email/phone/note inputs)
- `internal/ui/messagelist/` search-mode input — confirm during
  reading whether the focusable input lives in messagelist or
  `internal/ui/sidebar/search.go`; whichever has it is the
  cursored package.

Other subpackages (`sidebar`, `reader`, `account`, `helppopover`,
`movepicker`, `outbox`, `uicore`) keep `View() string`. App.View()
composes mixed children: tea.Views contribute content + cursor,
strings contribute content only.

### 4. Compose paste handling via `PasteMsg`

v2 splits bracketed paste from `KeyMsg` into its own message types
(`PasteMsg`, `PasteStartMsg`, `PasteEndMsg`). Compose's address
fields currently treat a pasted comma-separated list as a stream of
keystrokes — the autocomplete dropdown flickers and parsing is
incremental.

**Move.** Add `PasteMsg` arms in compose Update:

- Address fields: parse the full payload through
  `content.ParseAddressList`, emit N completed `Name <email>, `
  chips atomically. Single-address paste (no commas) falls through
  as one chip — matches today's behavior for that case.
- Subject field: paste replaces selection or inserts; no parsing.
- Body field: paste is one atomic Catkin Undo unit. Restore the
  URL-paste wrapping logic that 13.2a deleted (single
  whitespace-free token starting with `http://` / `https://` /
  `mailto:` wraps the cursor word in `[...](url)` — VS Code's
  pragmatic test).

The body arm is the new home of `internal/catkin/`'s former
`handlePaste` function. Write it as a v2-native PasteMsg handler
from scratch; do not resurrect the v1-shaped stub. Helpers like
`looksLikeURL` and `wordAt` come back if and only if the new
implementation needs them.

ADR-worthy because it changes user-visible behavior in compose.

## Mechanical drift in scope

This pass touches the App + cursored subpackages + tests. The
mechanical surface is small compared to 13.2a:

- `App.View()` from `tea.NewView(s)` wrapper to a fully populated
  `tea.View` (chrome + cursor).
- `compose.Model.View()`, `contacts.Form.View()`, messagelist
  search-mode `View()` from `string` to `tea.View`.
- Per-input `cursor.Model` removal across cursored subpackages.
- `VirtualCursor=false` on every textinput/textarea constructor in
  cursored subpackages.
- `tea.WithAltScreen()` and adjacent Program-option chrome flags
  removed from `cmd/poplar/root.go`.
- Cursored subpackage tests: `m.View()` assertions call `.String()`
  on the returned `tea.View`. Cursor-blink tests rewrite to assert
  App-level cursor state, not per-input.
- Compose / catkin tests: PasteMsg arm coverage — atomic chip
  emission for address fields; URL-wrap for body.

## Sequencing

13.2b is incremental in spirit (each task is a coherent slice that
compiles + tests on its own where possible) but constrained by the
fact that the chrome reframe (Task 3) and the cursor reframe
(Task 2) both rewrite `App.View()`. We bundle Tasks 1–3 into one
slice — the App.View() composition reaches its final v2 shape
once, with cursor + chrome wired together. The compose subpackage
View() rewrite (Task 1) lands first because it's a self-contained
input change.

## Pass-budget

Per CLAUDE.md, passes target ~8–12 tasks with one ADR. 13.2b is 9
tasks (including the ADR). Single subsystem (v2-native architecture
reframes); coherent.

## Tasks

1. Cursored subpackages return `tea.View` with `Cursor` populated
   (or expose `Cursor() *tea.Cursor` accessor — pick consistently
   after reading v2 source). `VirtualCursor=false` on every
   textinput/textarea in compose, contacts.Form, messagelist
   search-mode input.
2. App pulls focused-child cursor up: focus-chain walk in
   `App.View()`, single cursor ticker at App level. Drop every
   per-input `cursor.Model` and `cursor.Blink` Cmd. Confirm
   coordinate-space; do offset math at composition.
3. Declarative chrome: drop `tea.WithAltScreen()` (and any other
   chrome-config Program options) from `cmd/poplar/root.go`. Set
   `view.AltScreen` / `view.MouseMode` / `view.ReportFocus` /
   `view.WindowTitle` in `App.View()` from App state. Window title
   `poplar — <account>` when selected, else `poplar`.
4. Compose `PasteMsg` arms in `compose/model.go`'s Update:
   address-field atomic chip emission via
   `content.ParseAddressList`; subject inserts/replaces; body
   delegates to a new catkin PasteMsg handler.
5. Catkin `tea.PasteMsg` handler: bundle paste as one Undo unit;
   restore URL-paste wrapping (cursor word + http/https/mailto
   single-token detection). Write from scratch as a v2-native
   handler; not a port of the deleted v1 stub.
6. Test fixture sweep: cursored subpackage `m.View()` assertions
   call `.String()` on returned `tea.View`. Cursor-blink tests
   rewrite to assert App-level state. PasteMsg arm coverage in
   compose + catkin.
7. `make check` green.
8. tmux capture vs goldens: 80×24 + 120×40 mainline screens
   (sidebar + messagelist + reader, compose, search results,
   contacts). Triage real visual regressions vs. cosmetic shifts
   (cursor rendering may move one cell with the declarative
   model). Update goldens for intentional shifts.
9. ADR-0189b (charm.land/v2 reframes): document the three reframes
   (declarative chrome, hoisted cursor, cursored-children return
   tea.View), the paste move, and the three named deferrals
   (`internal/ansix/` audit → 13.3/15; per-subpackage Styles
   restructuring → 15; color profile + isDark threading via
   `term.Resolve` → 14.1/15). Invariants update: `internal/ui/`
   chrome is declarative; cursor is hoisted to App; cursored
   subpackages return `tea.View`. Refresh
   `docs/poplar/bubbletea-conventions.md` to name the v1
   specifics now removed at the architectural level
   (`tea.WithAltScreen`, per-input `cursor.Model`, virtual cursor
   rendering). STATUS pivot to Pass 14; archive 13.2b plan + spec
   via `git mv`.

## Risks

- **Cursor coordinate-space surprises.** v2's `tea.View.Cursor`
  position semantics (global vs. local) may not match what the
  spec assumes. Read the v2 source before designing the App-side
  composition; if the assumption breaks, document the actual
  behavior in the ADR rather than papering over it.
- **Goldens churn.** Cursor rendering may shift one cell when
  moving from styled-rune cursor to declarative `tea.View.Cursor`.
  Triage real regressions vs. cosmetic shifts in Task 8.
- **PasteMsg behavior change.** Atomic address-list emission
  changes user-visible compose behavior; the address-list parser
  must fall back gracefully on no-comma input so single-address
  paste matches today's behavior.

## Out of scope

- `internal/ansix/` audit (Pass 13.3 or 15)
- Per-subpackage `Styles` restructuring (Pass 15)
- Color profile + isDark threading via `term.Resolve` (Pass 14.1
  or 15)
- huh integration (Pass 14)

## Execution recommendation

Subagent-driven, no-scars discipline. Bundle Tasks 1–3 into one
dispatch since they all reach App.View() and splitting would leave
intermediate scaffolding. Tasks 4 and 5 can run in parallel only
if they don't share Catkin internals — confirm before dispatching.
