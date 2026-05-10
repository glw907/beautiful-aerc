---
title: Pass 13.2a — charm.land/v2 substrate
date: 2026-05-10
status: ready
---

## Source spec

`docs/superpowers/specs/2026-05-10-pass-13-2a-charm-v2-substrate-design.md`.
This plan is the execution shape; rationale lives in the spec.

## Why this is 13.2a, not 13.2

The original Pass 13.2 plan bundled substrate and architectural
reframes into one 13-task pass. Mid-execution two signals fired
the split-inline rule:

- **Scope discovery.** Task 1's codemod missed v2's KeyPressMsg
  field-layout drift (200+ call sites — Code/Text/Mod replacing
  v1's Type/Runes/Alt/Paste). Task 1.5 was added to absorb it.
- **Cross-task scaffolding.** Tasks 3+4+5 had to be bundled into a
  single subagent dispatch because splitting them would have left
  intermediate transitional state (cursored child returning
  tea.View that the App didn't yet read; per-input cursor.Model
  removed before the App-level ticker was wired). Per the
  no-scars rule (CLAUDE.md "Migrations and breaking changes"),
  that's not allowed.

The substrate (this pass) and the reframes (13.2b) are now
separated. The natural seam — "v2 imports + v2 mechanical drift,
App.View() still v1-shaped via tea.NewView(s)" — is documented as
the published end state of 13.2a, not transitional scaffolding.

## Bubbletea-conventions binding

`docs/poplar/bubbletea-conventions.md` and the `elm-conventions`
skill bind. 13.2a's substrate work doesn't touch the convention
docs' architectural claims (size contract, JoinHorizontal trust,
etc.); ADR-0189a refreshes the deprecated-API list at the
substrate level. The reframes that the conventions doc currently
*should* describe but doesn't (declarative chrome, hoisted cursor)
land in 13.2b's invariants update.

## Tasks

Spec §"Tasks" carries the canonical list. Reproduced here so the
plan is executable standalone:

1. ✅ `go get charm.land/{bubbletea,lipgloss,bubbles}/v2@latest`;
   codemod imports + `KeyMsg`→`KeyPressMsg` + `Sequentially`→
   `Sequence` + `spinner.NewModel`→`New` across all `.go`. Don't
   try to compile yet.
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
8. ADR-0189a (charm.land/v2 substrate): the v2 mechanical
   migration, the KeyPressMsg field-layout drift surprise, the
   AdaptiveColor removal, the transitional `App.View()` seam,
   the 13.2a/13.2b split rationale. Refresh
   `docs/poplar/bubbletea-conventions.md` deprecated-API list.
   Invariants update: palette concrete; `tea.KeyPressMsg.Code`
   canonical key-test field. Note that chrome / cursor invariants
   land in 13.2b.
9. STATUS pivot to Pass 13.2b; archive 13.2a plan + spec via
   `git mv`.

## Risks

Inherited from spec §"Risks":

- `x/ansi` API drift — assumes v1 module path; if not, ansix
  adjusts; audit/kill decision still defers to 13.3.
- Catkin follows the UI tree's mechanical drift; paste handler
  deleted, returns in 13.2b.
- Substrate goldens churn (palette → concrete color, color profile
  defaults) is in scope for Task 7. Reframe-driven shifts (chrome,
  cursor) are 13.2b territory.

## Out of scope

Spec §"Out of scope" stands: declarative chrome, cursor hoist,
cursored-children return tea.View, PasteMsg arms, ansix audit,
per-subpackage Styles restructuring, color profile threading.

## Execution recommendation

Subagent-driven, no-scars discipline. Bundle Tasks 4 and 5 if a
fresh implementer can hold both in context — they share theme +
per-subpackage styles.go files. Task 6 sweeps the leftover after
4+5 are confirmed compiling. Task 7 verifies and surfaces any
leftover. Task 8 is documentation. Task 9 is the close-out.
