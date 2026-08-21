# Pass 2 gate captures, 2026-08-21

The first painted evidence for the pass 2 shell: a real kitty window
(Geoff's kitty.conf, Monaspace Neon 11pt, 150x45) driven by
`scripts/kitty-shot` through the matrix in `scripts/gate-captures`,
108 captures in all, then graded by a fresh-context workflow (7 Opus
vision graders by capture group, 19 adversarial refuters on the
structural findings, one is-it-right auditor). Pass 2c's task 1
turned that spike into the repo's standing tier-4 gate medium; the
files below are the pass 2 run exactly as it happened and are kept
as the historical record the verdict and the plan cite. The method
and the current matrix are described here for anyone re-shooting.

## Files

- `audit.md`: the auditor's report. Section 2A is the help screen as
  eight defects; section 3 reads the 12 gate items off the captures;
  section 5 is what to put in front of Geoff first.
- `findings.json`: all 66 graded findings with class and evidence.
- `manifest.txt`: what every capture in the full run claimed to show.
- The PNGs: the decisive subset (live mail and help in both themes at
  150x45, help at 80x24, calendar and config clusters, the floor at
  59x24, the sketch modal and toast at ANSI-16, the sketch modal and
  banner in truecolor dark, mail at NO_COLOR, and the exemplar's
  standard dark rung painted by the same kitty as the reference).

## Verdict in one paragraph

The live shell is right: grounds byte-exact against the palette,
placeholders centered at every rung, floor clean, pointer clicks and
Esc working. The help screen is uncomposed (flat ground swallowing
the chrome bands, a stranded second column carrying one duplicate
row, a smeared header, two key-column widths, mixed key-name case,
hints that are no-ops). Two degrade defects sit in shipped code
(ANSI-16 muted text at 1.29:1; the ANSI-16 modal pill split in two by
reverse video). `cmd/sketch` omitted `FullRegion` and so every sketch
frame showed a phantom rail, which made it invalid evidence for the
modal, toast, banner, and degrade profiles. Pass 2c
(`docs/superpowers/plans/2026-08-21-pass-2c-real-terminal-verification.md`)
answers all of it.

## The method

`scripts/kitty-shot` drives one real kitty window through a step
list (keys, typed text, resizes, clicks, wheel notches) and captures
each named step as a PNG (`import -window`) plus kitty's own ANSI
text dump of the screen. It is the only gate medium that checks the
painted terminal rather than text: goldens, the gallery, and
`tmux-check` all render or dump text, and the pass 2 gate found
defects (ground bleed past a reset, a phantom rail, an ANSI-16
contrast failure) only the painted terminal showed. It needs X11,
kitty, xdotool, and `import` on PATH, and forces
`background_opacity=1.0` so a light ground composites at its
ratified value regardless of the operator's own `kitty.conf` (the
pass 2 run above used the operator's 0.95, which is why its light
captures measure short of the ratified base ground; see audit item
H).

`scripts/gate-captures` runs the full pass-gate matrix through
`kitty-shot`: the live shell in dark and light kitty themes across
every rung boundary design language section 9 actually names
(columns 60/59, 100/99, 140/139; rows 15/14, 20/19), the pointer
vocabulary, a short spartan window where help overflows the region so
a wheel notch is a falsifiable claim, every `cmd/sketch` fixture
across all four capability profiles, that same ANSI-16 and NO_COLOR
pair again on a light kitty ground, and the ratified exemplar painted
by the same kitty as the reference image. Task 1 changed this matrix
from the pass 2 run above: the `80/79` column pair is gone (80 is not
a design-language boundary, so labeling it one was wrong), the wide-
rung wheel-notch capture that had nothing to scroll is replaced by a
short-window pair where help genuinely overflows the region, and the
light-ground ANSI-16/NO_COLOR sketch set is new (the first pack never
painted a degrade profile on anything but kitty's default dark
ground).

Every capture in `manifest.txt` carries a claim (an active surface,
help over one, the floor, or a sketch fixture and profile), and the
script reads each capture's own `.ansi` dump back to check its claim
before it reports success: the switch bar's first line names the
active surface, help's footer carries `esc back` regardless of scroll
position, the floor notice's `this window is` names the actual size,
and sketch's own status line names its fixture and profile outright.
A mismatch fails the run naming the capture.

## Re-shooting

`scripts/gate-captures <outdir>` on the gate platform with a display
and `FASTMAIL_API_TOKEN` in the environment; a full run takes several
minutes and the output is tens of MB, so it stays out of git. Pair it
with the `tui-visual-verify` skill for the actual grading pass: the
script only checks that a capture shows the surface it claims, never
whether that surface composes correctly, which is what the fresh-
context vision graders are for. The sketch frames in this directory
predate task 2's fix and show the phantom rail on purpose.

## Task 2 re-shots

Six `scripts/kitty-shot` captures (PNG plus `.ansi` beside each),
`task2-`-prefixed, taken after task 2 of pass 2c put `cmd/sketch` on
`ui.ComposeView`, the same composition path `App.View` calls: the
`mail`, `help`, `modal-confirm`, and NO_COLOR `mail` fixtures the
task brief named, re-shot through the fixed binary, plus a rung
150x26 then 100x30 pair proving SK-3's frame-hygiene residue (a stale
larger frame's rows left behind after a shrink) is gone.

- `task2-mail.png`: fixture `mail`, truecolor-dark, 100x30. Full-width
  status line and footer, centered placeholder, no rail and no
  divider column anywhere in the frame.
- `task2-help.png`: fixture `help`, truecolor-dark, 100x30. The
  two-column overlay fills the whole content region edge to edge;
  no rail or divider between the overlay and the frame edge.
- `task2-modal-confirm.png`: fixture `modal-confirm`, truecolor-dark,
  100x30. The status line and footer are both gone; the confirm box
  sits alone on a blank ground, the full-shell wipe StateModal owns.
- `task2-mail-nocolor.png`: fixture `mail`, NO_COLOR, 100x30. No
  SGR background anywhere in the frame (decision 11); the content
  reads as plain text against the terminal's own ground.
- `task2-residue-150x26.png` then `task2-residue-100x30.png`: the
  same fixture (`mail`, truecolor-dark) shot at 150x26, then again
  after resizing down to 100x30. The second capture shows only the
  100x30 frame; none of the wider or taller previous frame's rows
  survive the shrink.
