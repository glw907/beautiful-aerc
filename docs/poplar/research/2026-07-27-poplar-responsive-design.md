# Responsive Design

Durable reference for poplar's responsive-layout model. The
authoritative implementation is `internal/ui/layout.go`
(`ComputeLayout`); this doc covers the *theory* — when to make
things responsive, where breakpoints come from, and how to extend
the model when adding new UI components.

## The three-tier model

Every responsive UI surface in poplar resolves to one of three
tiers driven by terminal width:

| Tier | termWidth | Identity |
|------|-----------|----------|
| **Spartan** | 80–89 | Polish-bar floor. Minimum viable chrome — no flag column, no date, no sidebar icons. Subject and sender get every available cell. |
| **Intermediate** | 90–107 | Information density returns: flag column, compact (3-cell) or short (5-cell) date, but no sidebar icons. The "tiled-pane laptop" target. |
| **Full** | 108+ | All chrome on. Sidebar icons, flag column, short date. The "modal cluster" (100–140 cols, peaking around 113–120) lives here. |

Tiers are *emergent* from formulas + thresholds, not configured
directly. Tasks should consume `LayoutMode` from `ComputeLayout`,
not branch on tier names.

## The formula primitive

Two continuous slopes plus three discrete thresholds govern the
message-list and sidebar:

```
sidebar = clamp(round(14 + (W - 80) * 0.2),  14, 30)
sender  = clamp(round(22 + (W - 80) * 0.125), 22, 32)
flags   = (W >= 90)
date    = 0 if W < 90 else (3 if W < 100 else 5)
icons   = (sidebar >= 20)        # ≈ W >= 108
```

**Slope choice rationale:**
- Sidebar slope **0.2** (1 cell per 5 cols of W): floor 14 fits
  `Archive` (7 chars) with icons off; ceiling 30 gives breathing
  room for nested custom folder names.
- Sender slope **0.125** (1 cell per 8 cols of W): hits the three
  data-derived sender-name coverage cliffs (22 → 86%, 28 → 92%,
  32 → 95%) at natural widths.
- Subject column is **derived**: it absorbs ~67% of every new
  cell of width (`1 - 0.2 - 0.125`). Appropriate because subject
  is the most cell-hungry column (real-world median 39 cells).

## How to add responsive behavior to a new UI component

1. **Find the data.** Sample a representative corpus of whatever
   the component displays (sender names, folder labels, contact
   names, header field values). Compute the cumulative coverage
   distribution. The cliffs in that distribution are your
   candidate breakpoints — every cell below a cliff is wasted,
   every cell above is buying small marginal gains until the
   next cliff.

2. **Decide: continuous slope or discrete threshold?**
   - **Continuous slope** when the value adapts smoothly with
     terminal width and there's a meaningful range (sender,
     sidebar). Choose slope so the value hits the next coverage
     cliff at a memorable width (round-100, round-120).
   - **Discrete threshold** when the value is a small enum
     (date format: none/3/5; icons: on/off; chrome: visible/
     omitted). Place the threshold at a width where the cell
     budget can absorb the new chrome without subject falling
     below ~30 cells (the "preview-readable" floor).

3. **Use round numbers for thresholds.** Memorable thresholds
   (80, 90, 100, 110, 120, 160) are easier to reason about and
   debug than `87.3` or `113`. Round to the nearest 10 unless
   the math demands otherwise.

4. **Bundle related transitions.** The msglist flag column and
   sidebar icons could have separate thresholds, but bundling
   them at sidebar≥20 (≈ W=108) keeps the conceptual model
   small: "icons appear together." Don't multiply transitions
   for the sake of optimization.

5. **Write a `LayoutMode`-style struct.** Add fields to the
   existing `LayoutMode` if the new component participates in
   the global layout (most do). Resist the temptation to make a
   per-component `XLayoutMode` — there is one `LayoutMode` for
   the whole UI; the consumers pick which fields they care about.

6. **Test at boundaries.** Table-driven tests covering each
   threshold width minus 1, exact, plus 1 (e.g., 89/90/91).
   Most layout bugs hide at the boundary.

## Terminal-size research

The breakpoints **80 / 100 / 120** are evidence-backed by 2026
research (no published hard-data survey of actual runtime sizes;
synthesized from emulator defaults + TUI source + anecdote):

- **80×24** is every Unix emulator's default. It is the *legacy
  floor*, not the modal usage size.
- **Windows Terminal** ships **120×30** as its default — explicit
  signal that 80 cols is undersized in 2022+.
- **btop** uses W=100 as its only structural feature-reveal
  threshold — the strongest precedent in TUI source.
- **glow** caps render width at 120 cells.
- Anecdotal modal cluster: **100–140 cols**, peaking around
  113–120 (developers report this size repeatedly).
- Tail to **150–220 cols** for ultrawide / fullscreen 27"+
  displays. Rows much less variable: 24–30 modal, up to ~50 on
  ultrawide.

**Implication for breakpoint placement:** poplar's polish bar is
80×24, but the experience peak should be the 100–140 modal
cluster. Spartan tier handles the floor (graceful, never
embarrassing); Intermediate handles 90–107 (laptop tiled-pane
case); Full handles the modal cluster and everything above
without needing a separate "Wide" tier — the formulas just keep
flowing.

## Anti-patterns

- **Don't make things responsive that don't need it.** A modal
  popover with fixed natural dimensions doesn't need a slope —
  just clamp it against the terminal size and center.
- **Don't hardcode column widths in renderers.** Always consume
  `LayoutMode` (or pass widths in via parameters). Constants
  like `mlSenderWidth = 22` were the original sin that #26 fixed.
- **Don't add per-user configuration for breakpoints.** v1 is
  opinionated; tuning thresholds is a v2+ conversation.
- **Don't introduce CSS-style media queries.** The model is
  formulas + thresholds in pure Go, computed once per
  `WindowSizeMsg`. Anything more elaborate than that is over-
  engineering.
- **Don't reuse the date-format selection pattern for things
  that aren't formats.** The 0/3/5 enum on date width is special
  because date strings have natural "compact / short / full"
  forms. Most other responsive choices are continuous (size) or
  binary (visible/hidden).

## Reference

- Source: `internal/ui/layout.go`
- Decision: `docs/poplar/decisions/0109-polish-i-msglist-layout.md`
- Sender-name distribution data: 2000-message Fastmail sample,
  2026-05-02 (see `docs/superpowers/specs/2026-05-02-polish-i-design.md`)
