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

## `rmhubbert/bubbletea-overlay`

**Does this make poplar better?** No. The library's `Composite` function
is the same algorithm as poplar's `PlaceOverlay` — both derive from the
Superfile `overplace.go` origin, both use `charmbracelet/x/ansi` for
cell-width measurement, and the line-by-line compositing logic is
structurally identical. Swapping in `Composite` saves zero net LOC because
the library's `Model` type does not model the cascade: the nine-level
`if IsOpen()` chain in `App.View` stays unchanged, and every call site
still supplies pre-computed `(x, y)` integers. The library adds a
`Position` enum for semantic placement (Top/Center/Bottom/Left/Right),
but poplar already delegates that arithmetic to per-component `Position()`
methods. The result is a new dependency, no behavior change, and no LOC
reduction.

**Feature parity:** `Composite(fg, bg, xPos, yPos, xOff, yOff)` covers
the same cell-compositing operation as `PlaceOverlay(x, y, fg, bg)` with
a slightly different argument order and an added `Position` enum. The
library's `Model` wraps two `Viewable` values and composites them in
`View()`; it does not implement `Update()` routing, so the cascade
ordering and mutual-exclusion logic poplar needs remain entirely in `App`.
No gap in the library's coverage is the problem — the coverage exactly
matches what poplar already has, hand-rolled.

**Customization seams:** The library exposes no style injection points.
`Composite` works on rendered strings, like `PlaceOverlay`. There is no
hook for border styles, background dimming (poplar's `DimANSI` pass), or
themed color injection. The `Model.Foreground` and `Model.Background`
fields accept any `Viewable`, so domain state threads through naturally,
but that is true of `PlaceOverlay` too.

**Theming integration:** None. `Composite` operates on pre-rendered ANSI
strings; theming is the caller's responsibility in both implementations.
The library owns no colors.

**Maintenance signal:** Last commit 2026-04-20 (dependabot bump of
`charmbracelet/x/ansi`). Releases: v0.6.7 (2026-04-20), v0.6.6
(2026-03-18), v0.6.5 (2026-02-09) — roughly one release per six weeks in
the past twelve months. Actively maintained for Bubble Tea / Lipgloss v1;
the README notes v2 users should use built-in compositing.

**Code delta estimate:** `overlay.go` is 76 LOC, `modal_shell.go` is 66
LOC — 142 total. `PlaceOverlay` alone is ~50 LOC of that. Replacing
`PlaceOverlay` with `overlay.Composite` would remove ~50 LOC and add one
`go.mod` entry; `ModalShell` is unrelated and would not move. Net delta is
approximately −50 LOC at the cost of a new dependency whose implementation
is the same algorithm.

**License:** MIT License, Copyright (c) 2024 Hubby.

**Verdict:** **Keep + harvest**

**Rationale (one line):** Library and poplar share the same Superfile-
derived algorithm — adopting it trades a small hand-roll for a dependency
with no behavior gain and no cascade support.

**Interacts with:**
- No other Eval A candidates depend on this swap landing first.
- No other candidates are simplified by adopting or skipping this one.
- Not blocked by any candidate outside Eval A.
