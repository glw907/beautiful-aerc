---
title: Pass 18 — underlay dim retired; dim = unwired only
status: accepted
date: 2026-05-11
---

## Context

Every overlay branch in `App.View()` ran the underlying account frame
through `uicore.DimANSI` before compositing the overlay box on top.
The intent was a depth cue — the modal is in front, the frame
recedes. Twelve call sites carried the same `dimmed :=
uicore.DimANSI(frame)` boilerplate (help popover, confirm modal,
conflict, outbox, link picker, attach picker (reader + compose),
schedule picker, reschedule picker, move picker, contact form,
contacts popover).

The visual language was overloaded. Dim already meant something else
in the help popover: per ADR-0072, **unwired** keybinding rows
render dim throughout to advertise the future vocabulary without
implying they work today. Using dim for both "this binding isn't
wired yet" and "an overlay is active" forces the reader to
disambiguate by spatial reasoning rather than by color semantics —
exactly the kind of subtlety a TUI cannot afford.

The overlay box itself is already the "modal active" cue. It draws a
border, a title, and footer hints over the frame. The frame's
dimming was a borrowed-from-GUI affordance that didn't carry its
weight.

## Decision

Underlay dim is retired. Overlays composite over the **undimmed**
frame via `uicore.PlaceOverlay`. The `uicore.DimANSI` primitive is
deleted along with its tests.

Dim is reserved for **unimplemented** semantics — wired/unwired
keybinding rows in the help popover, the spinner placeholder, and
the other lipgloss `FgDim` foreground styling that signals "not
loaded yet" or "not yet available." Nothing else dims.

## Consequences

- `App.View()` loses twelve `DimANSI(frame)` calls; the helper
  `m.viewOverlay(box, x, y, frame)` collapses the simple branches
  (eight overlays) to one line each.
- `uicore/dim.go` and `uicore/dim_test.go` are deleted.
- `.claude/rules/ui-invariants.md` Overlays section drops the
  "render underlying frame, dim via `uicore.DimANSI`" clause.
- ADR-0072's wired/unwired vocabulary is now the sole owner of dim
  semantics across the UI. Future ADRs introducing dim must justify
  why the surface they touch is conceptually "unimplemented" or
  "not yet available" — anything else is a re-overload.
- `TestApp_HelpOverlayDimsBg` is replaced by
  `TestApp_HelpOverlayCompositesOverFrame`, which verifies the
  overlay composites without asserting dim ANSI codes.
- If a future surface wants the "modal is in front" depth cue, it
  must use a different primitive (a wash, a subtle border, a
  faint shadow ring) — never dim.
