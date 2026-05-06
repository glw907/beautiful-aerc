# Pass 9d.4 — Popover edges

## Goal

Verify and fix the spellcheck popover render at the right and
bottom edges of an 80×24 terminal.

## Capture-driven findings

Three render captures (golden tests in
`edge_capture_test.go`, exploratory; deleted before commit) at 80×24
with a misspelling positioned near the right edge, the bottom row,
and the bottom-right corner exposed one real bug.

1. **Right-edge bug.** Misspelling at cols 73..79; `position()`
   correctly computes `col = 80 - 24 = 56`. But `overlay()` calls
   `ansiSpliceAtCol(line, 56, 24, popLine)` against blank or
   short body lines below the cursor. `ansi.Truncate("", 56, "")`
   returns `""`, so the result is `"" + popLine + ""` — the
   popover renders flush left at col 0 instead of col 56.

2. **Bottom-edge flip works.** `position()` flips above the cursor
   when `row+height > viewportH`. The popover overlays editor body
   rows above the misspelling. The misspelling itself stays
   visible on the cursor row. No bug.

3. **Bottom-right corner.** Combination of (1) and (2): vertical
   flip places popover at row 16, horizontal shift gives col 56,
   then short body rows mis-render the popover at col 19 (where
   the body line ends) instead of col 56. Same root cause as (1).

## Open question (settled)

> Should right-edge clipping flip the popover to anchor on its
> right corner, or shift left to fit?

**Settled: shift left.** Convention across major editors (VS
Code, IntelliJ, Sublime, mutt's overlays). Same answer for
vertical: flip up, which is also what the current code does.
No behavior change to `position()`.

## Fix

In `overlay()`, pad the body line with spaces to `col` cells
before splicing the popover slice in. This is the idiomatic
fix at the overlay layer — `ansiSpliceAtCol` is a low-level
ANSI-aware primitive used elsewhere (cursor splice, annotation
splice) where the splice column is always inside the line. The
padding belongs at the overlay boundary, not in the primitive.

## Tasks

1. Pad body lines in `overlay()` to reach `col` before splicing.
2. Unit-test `overlay()` directly: short line → padded; long
   line → spliced in place; out-of-range row → skipped (existing
   behavior, regression-guard it).
3. Render-level test that opens the popover near the right edge
   and asserts the popover row text appears at the expected column
   in `View()` output (not at col 0).
4. Render-level test for the bottom-edge flip (popover row above
   the cursor row).
5. `make check`.
6. Pass-end ritual: ADR (popover overlay padding), invariants
   touch (Catkin section if any binding fact changes — likely
   only an ADR reference in the index), STATUS.md, archive plan,
   commit + push + install.

## Out of scope

- Changing `position()` strategy (shift vs. flip) — convention
  already correct.
- Bubbletea conventions audit beyond this fix.
- The transient destructive-overlay aesthetic (popover hides
  body rows it covers). Standard for terminal popovers; no
  alpha channel.
