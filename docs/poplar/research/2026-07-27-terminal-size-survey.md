# Terminal size survey

**Date:** 2026-07-27
**Phase:** Re-founding Phase 4, gate addendum. Geoff's directive:
the responsive plan starts with research into what terminal
window sizes people actually use. Method: one live research
dispatch (WebSearch/WebFetch over terminal docs, shipped TUI
source and issues, resolution statistics; no model-memory-only
claims), checked against the legacy client's 2026-05 synthesis
(`docs/poplar/responsive-design.md`, reference).

## The evidence picture

**No hard data exists.** No terminal emulator or TUI publishes
column/row telemetry (Warp's disclosures cover UI events, not
dimensions), no usage survey covers window sizes, and no
dotfile-corpus study surfaced. The evidence is defaults, shipped
breakpoints, and screen math, and any responsive design should
hold its thresholds loosely and re-derive them if real data ever
appears.

**Defaults (verified live).** 80×24 remains the near-universal
default: macOS Terminal.app, GNOME Terminal, Konsole. Windows
Terminal ships 120×30, the one major-platform default above the
legacy floor. Alacritty and kitty defer to the window manager or
pixels. No single emulator dominates (Ghostty leads GitHub
adoption, kitty leads an Arch preference survey at 30%).

**The largest real cohort is probably embedded terminals.** VS
Code holds ~76% of the IDE market (Stack Overflow 2025), and its
integrated terminal panel is short and width-constrained by the
editor. Short-height windows (well under 24 rows) are a normal
condition, not an edge case.

**People do not run maximized single panes.** A maximized
1920×1080 terminal yields roughly 200×55 at typical font sizes,
far above every observed cluster, so the missing usage lives in
splits and partial windows: half of ~200 is ~100 columns, a
third is 66-80. The tmux/zellij and HN anecdote base agrees
(several report two 80-column panes over one wide one).

**The modal band is 100-140 columns, 28-36 rows**, anchored by
Windows Terminal's shipped 120×30, btop's "preferred 100×30",
lazygit's 80-column portrait-mode trigger, and anecdote
clustering 88-120. Rows run 24-30 with a tail toward ~50.

**The large tail is small.** Ultrawide displays are ~2-3% of
desktop traffic. Width beyond ~180 columns is a real but minor
audience.

**Shipped-TUI breakpoints.** lazygit stacks vertically below 80
columns; btop hard-blocks below 80×24 and prefers 100×30;
superfile blocks with a too-small error; k9s, gh-dash, and yazi
leave layout to manual config; Crush ships a LayoutEngine
computing layout from terminal size, the one confirmed
automatic-responsive precedent in the Charm ecosystem. Graduated
automatic breakpoints are the exception in the field, which is
an opportunity, not a warning. One legacy claim fell: glow's
"120-column cap" could not be verified (its width is
configurable, defaulting to terminal width or 80).

## Implications adopted by the design language

1. Fully functional at 80×24 (the universal default), degrading
   with grace rather than a hard block down to a 60×15 floor
   state (splits make 60-79 columns real; btop's hard error is
   the anti-pattern).
2. The strip-down boundary sits at 100 columns (btop's preferred
   line, lazygit's neighborhood, the half-split math).
3. The polish center is 100-139 × 24-36: Windows Terminal's
   default and the modal cluster live here.
4. Enhancement earns in at 140 columns (the preview-pane
   precedent band); a distinct ultra tier is not justified by a
   2-3% tail, so capped, centered measures are the wide tier's
   own behavior rather than a fifth class.
5. Short heights get first-class treatment (the embedded-panel
   cohort): chrome compresses below 20 rows before anything
   else degrades.
