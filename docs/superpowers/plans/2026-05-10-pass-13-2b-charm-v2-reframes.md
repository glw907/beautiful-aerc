---
title: Pass 13.2b — charm.land/v2 reframes
date: 2026-05-10
status: pending (blocked on 13.2a)
---

## Source spec

`docs/superpowers/specs/2026-05-10-pass-13-2b-charm-v2-reframes-design.md`.
This plan is the execution shape; rationale lives in the spec.

## Starting state

13.2a closes with `make check` green on the v2 substrate. The
transitional `App.View()` returns `tea.NewView(s)` with no
declarative chrome / cursor wiring. Per-input `cursor.Model`
instances still tick. `tea.WithAltScreen()` still rides the
`tea.NewProgram` call. Catkin's paste handler is deleted; URL-paste
wrapping awaits this pass.

## Bubbletea-conventions binding

This pass changes the architectural claims in
`docs/poplar/bubbletea-conventions.md`: chrome moves from imperative
Program options to declarative `tea.View` fields; cursor moves from
per-input `cursor.Model` to a single ticker hoisted to App. ADR-0189b
refreshes the conventions doc accordingly.

## Tasks

Spec §"Tasks" carries the canonical list.

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
   `view.WindowTitle` in `App.View()` from App state.
4. Compose `PasteMsg` arms: address-field atomic chip emission via
   `content.ParseAddressList`; subject inserts/replaces; body
   delegates to a new catkin PasteMsg handler.
5. Catkin `tea.PasteMsg` handler: bundle paste as one Undo unit;
   restore URL-paste wrapping. Write from scratch as v2-native;
   not a port of the deleted v1 stub.
6. Test fixture sweep for cursored subpackages and PasteMsg
   coverage.
7. `make check` green.
8. tmux capture vs goldens; triage real visual regressions vs.
   cosmetic shifts (cursor rendering may move one cell). Update
   goldens for intentional shifts.
9. ADR-0189b (charm.land/v2 reframes): the three reframes, the
   paste move, the three named deferrals (ansix audit → 13.3/15;
   per-subpackage Styles restructuring → 15; color profile +
   isDark via `term.Resolve` → 14.1/15). Invariants update.
   Refresh `docs/poplar/bubbletea-conventions.md`. STATUS pivot
   to Pass 14; archive 13.2b plan + spec via `git mv`.

## Risks

Inherited from spec §"Risks": cursor coordinate-space surprises,
goldens churn from declarative cursor model, PasteMsg behavior
change in compose addresses.

## Out of scope

Spec §"Out of scope" stands: ansix audit, per-subpackage Styles
restructuring, color profile threading, huh integration.

## Execution recommendation

Subagent-driven, no-scars discipline. Bundle Tasks 1–3 since they
all reach App.View() and splitting leaves intermediate scaffolding.
Tasks 4 and 5 share Catkin internals — sequence them, don't
parallelize.
