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

## Part 2: user preference and the Charm flagships (same day)

A second dispatch, triggered by two follow-up questions (what do
users prefer; how are flagship bubbletea apps designed) and by the
audience ambition Geoff stated at the review: attractive enough to
tempt GUI mail users.

**User preference (adoption data, GitHub stars verified via API):**
Catppuccin dominates (org ~19.7k stars; its nvim port alone 7.6k),
then Gruvbox 15.7k, Tokyo Night 8.2k, Nord 6.9k, Rosé Pine 3.1k
(Dracula is federated across ~200 repos, so its star count
understates reach). The winners share no hue temperature but do
share desaturated, muted grounds with soft contrast and restrained
pastel accents. Poplar's slate palette sits squarely in that
taste profile.

**Demand signals:** unfocused-pane dimming demand is LOW (zellij
#3373 has 3 reactions after a year; tmux ships it natively).
Terminal-background transparency demand is MODERATE-HIGH and
well-evidenced (OpenCode #11866 with 22 reactions plus two
follow-on requests; a Codex CLI issue; long-standing Windows
Terminal and Warp threads): users who run transparent terminals
resent an opaque paint-over and want an escape hatch, not a rule
that apps never paint. BACKLOG #70 records the escape hatch as a
post-v1 candidate (v1 ships no user configurability by vision).
No aesthetic survey data exists anywhere; absence noted.

**Charm flagships, verified from source:** CharmTone
(charmbracelet/x/exp/charmtone) is 69 named colors: a
near-neon saturated spectrum plus a 12-step neutral ramp. Crush
paints a full opaque app background (`bgBase` set directly),
runs everything through one central token-driven `Styles` struct
(its quickstyle layer is forbidden from hardcoding palette
colors), insets with padding never margin, and styles focus as
paired Focused/Blurred styles per component. No transparency
option exists in its theming code.

*Correction (2026-08-19, adversarial review of the pass 2 design
draft):* this section's first draft said crush gives "panels"
layered grounds, and a design ruling was taken partly on that
reading. Verified deeper: crush's structural panes (sidebar,
chat) share ONE ground and are separated by whitespace gutters
(a 2-cell gap plus 1-cell app insets) and content alignment; its
`bgLessVisible`-family grounds appear only on inline blocks —
dialogs, tool output, popups, buttons. Crush's dialogs are
rounded-bordered in the accent; its selected rows are
accent-backgrounded, not neutral-raised; its help bar carries no
ground; transient status renders as colored pills; pane focus is
a left edge bar that changes glyph weight (`▌` focused, `│`
blurred), always present. Crush is flatter than a ground-ladder
design, not more layered. A layered-ground shell remains
defensible via helix/k9s chrome bands, common-region doctrine,
and the GUI-switcher ambition, but it is poplar's own choice,
not crush's pattern. A second physics fact from the same review:
dark-theme ground steps BELOW a near-black base are compressed
into invisibility (a rail darker than #16181D cannot exceed
roughly 1.1:1 against it), so a dark ladder's legible steps only
go upward from base; downward separation needs whitespace or a
line.
glow/gum/soft-serve ground behavior could not be source-verified
and is left open. Charm's stated philosophy: modern product
thinking in the terminal.

**Synthesis, with the GUI-switcher ambition applied:** the genre
splits into two wings. The infra-tool wing (k9s, lazygit, btop)
is flat and terminal-deferential; the Charm wing, poplar's actual
lineage and showcase target, paints opaque layered grounds on a
token system. Crush's practice plus the GUI-switcher ambition
plus common-region doctrine align: layered opaque grounds are the
right call for poplar, with the broader user taste governing the
palette (desaturated grounds, sparing accents, which slate
already is) rather than CharmTone's neon brand look. Engineering
echo for task 1: crush's three-layer token architecture (semantic
token opts, theme functions, one Styles struct) is the verified
shape exemplar for `internal/theme`.
