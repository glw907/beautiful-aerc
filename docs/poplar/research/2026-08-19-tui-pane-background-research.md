# Pane backgrounds and TUI aesthetics: the evidence

**Date:** 2026-08-19. **Trigger:** Geoff's pass 2 wireframe-review
question: could pane background colors be used more helpfully or
aesthetically, and what do research and modern TUI practice say
about a design that is productive and pleasing? **Method:** one
live research dispatch, sources graded (peer-reviewed / doctrine /
standard / project practice / craft convention). Feeds the pass 2
theme and chrome rulings and every later screen pass.

## The theory (graded)

- **Common region (Palmer 1992, Cognitive Psychology).**
  Peer-reviewed and foundational: elements sharing a closed region
  are perceived as grouped, strongly enough to override proximity
  and similarity. The often-repeated corollary that background
  fills group more strongly than border lines is designer
  inference layered on top, not something Palmer measured.
- **Material Design dark elevation** (doctrine): surfaces lighten
  by a white overlay scaling with elevation (roughly 5% at 1dp,
  8% at 3dp, 12% at 8dp, 16% at 24dp) over one base color, not a
  hand-picked palette per level.
- **Apple HIG dark mode** (doctrine): two tiers only, base and
  elevated; the distinction carries depth information, so custom
  per-panel colors can erase real signal.
- **Preattentive processing (Ware)** (research-grounded
  inference): luminance and chrominance are preattentive
  channels. A panel ground near the base in luminance groups
  content without competing for attention; a saturated or
  high-contrast panel ground is itself a preattentive signal and
  fights the content.
- **When it hurts:** vibrating boundaries between saturated
  near-complementary colors of similar value (classical color
  doctrine); and WCAG 4.5:1 / 3:1 is a hard standard that must be
  checked against the tint itself, not just the base background.

## The field (project practice, 2024-2026)

- **Focus is border color, not background**, near-universally:
  lazygit (`activeBorderColor`), zellij (pane frames; native
  unfocused-bg dimming requested and not shipped, issue #3373),
  k9s (`frame.border.focusColor`).
- **Background-fill budget goes to the selected row, not the
  pane**: yazi, k9s, btop, helix all highlight the active
  row/item and leave the pane on the default ground.
- **Chrome bands do get contrasting grounds**: helix's statusline
  (with a distinct inactive variant) and k9s's header are the one
  place these apps deliberately leave the base ground.
- **superfile** is the decorated exception: per-panel themed
  grounds, at the ornamental end of the genre.
- btop and k9s often leave the main background at
  terminal-default. Poplar diverges here deliberately: it paints
  its own `bg` token because QA-7 determinism and the UX-7
  contrast assertions need a known ground (pass 2 plan,
  decision 3). The divergence is recorded, not accidental.
- crush and opencode: semantic token systems confirmed, exact
  panel-ground choices unverifiable this session; no claim made.

## Actionable synthesis (each tagged)

1. Pane focus by border accent, never a whole-pane ground shift
   (project practice; poplar's focusedBorder/heavy-set already
   matches).
2. Spend background fill on the cursor/selected row (near-universal
   practice; poplar's selectedBg already matches).
3. If panels differentiate at all, keep the tint within a few
   luminance points of base (doctrine + Ware inference; the slate
   `bgPanel` candidates #1C2027 / #E8ECF1 follow this and pass
   contrast against themselves).
4. Chrome bands (status line, footer) are the field-backed place
   for a contrasting ground (helix/k9s pattern); a raised sidebar
   is the decorated end (superfile), doctrine-defensible via
   common region but against the genre's minimalist grain.
5. Elevation, if adopted, follows Material's overlay logic or
   Apple's two-tier model, never an invented per-panel palette
   (doctrine).
6. Check every text role against the tint it renders on (WCAG,
   hard requirement; wired into the pass 2 contrast test).
7. Screen accent-on-panel pairs for vibrating-boundary risk when
   themes change (doctrine).
8. The genre's border-for-structure, row-fill-for-selection
   consensus is craft convention, not a refutation of Palmer;
   poplar may choose ground-grouping deliberately, but should
   know it is choosing against the genre's grain.

The ruling this feeds: flat panes, chrome-band ground, or raised
sidebar, decided by eye in the pass 2 preview with this brief as
the weight behind each option.
