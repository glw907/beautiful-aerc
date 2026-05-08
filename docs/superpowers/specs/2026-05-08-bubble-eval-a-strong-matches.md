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

---

## `bubbles/help`

**Does this make poplar better?** No. The two `Context` enum branches
map cleanly onto two `KeyMap` implementations — the concept translates
— but the structural layout does not. `FullHelpView` renders a flat
key-column + desc-column per `[]key.Binding` group; poplar renders
three named groups per row with `joinColumnsRow`, a custom `renderGotoGrid`
for the six-cell Go To block, and a bottom hint line outside any group.
That shape cannot be expressed through `[][]key.Binding` without encoding
layout intent in grouping, making the KeyMap impls as bespoke as the
current `accountGroups` slice. Additionally, `FullHelpView` calls
`lipgloss.JoinHorizontal` internally — which poplar bans when
`spuaCellWidth != 1` (ADR-0084). The renderer cannot be patched without
forking. Finally, `bubbles/help` has no `wired bool` concept: disabled
bindings are invisible (`kb.Enabled() == false` skips them), not
dim-but-visible. The planned-binding affordance — show unwired keys
dimmed to advertise future bindings — is a first-class poplar feature and
would be lost.

**Feature parity:** `KeyMap.ShortHelp()` / `FullHelp()` cover the
conceptual shape. Width-aware truncation with ellipsis matches what the
current `Box` does for the `tooNarrow` fallback. What is missing: the
multi-row grouped grid layout, the `wired` dim affordance, the `ModalShell`-
compatible box return (the library renders inline text, not a bordered
box), and SPUA-safe column joining.

**Customization seams:** `help.Styles` has `FullKey`, `FullDesc`,
`FullSeparator`, `ShortKey`, `ShortDesc`, `ShortSeparator`, and `Ellipsis`
— all `lipgloss.Style` fields. Poplar's palette maps onto them cleanly
(`HelpKey` → `FullKey`, `Dim` → `FullDesc`, etc.). The seams exist; they
just cannot recover the layout or `wired` semantics.

**Theming integration:** Clean. All style fields are `lipgloss.Style`;
`NewStyles(*theme.CompiledTheme)` can populate them in one assignment
block. No color hardcoding in the render path; `New()` default colors can
be overridden.

**Maintenance signal:** `bubbles/help` ships inside `charmbracelet/bubbles`
v1.0.0 — pinned in poplar's `go.mod`. Stable API; well-maintained. No
maintenance risk.

**Code delta estimate:** Replacing `helppopover` with `bubbles/help`
would delete ~380 LOC (model.go + styles.go) but require reimplementing
the grid layout either in `KeyMap.FullHelp()` (encoding layout in group
ordering — fragile) or by calling `FullHelpView` as a sub-renderer and
wrapping it in the existing `Box` frame. The `JoinHorizontal` blocker
and `wired` loss mean adoption requires a fork of help.go (~200 LOC) on
top of whatever layout scaffolding stays. Net delta is negative only if
forking is counted as "free"; in practice the total owned code changes by
less than ±50 LOC.

**License:** MIT License, Copyright (c) 2020-2026 Charmbracelet, Inc.

**Verdict:** **Keep + harvest**

**Rationale (one line):** `bubbles/help` cannot render poplar's multi-
column grid, uses `JoinHorizontal` (ADR-0084 banned), and drops the
`wired` dim affordance — the hand-roll is the right shape; the `Styles`
field names are useful terminology to mirror.

**Interacts with:**
- `wired` dim pattern is not affected by any other Eval A candidate.
- `JoinHorizontal` ban (ADR-0084) applies equally to any candidate
  whose render path calls it internally.
- No other Eval A candidate depends on this evaluation's outcome.
