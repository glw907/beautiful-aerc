# Pass 2 gate captures, 2026-08-21

The first painted evidence for the pass 2 shell: a real kitty window
(Geoff's kitty.conf, Monaspace Neon 11pt, 150x45) driven by
`scripts/kitty-shot` through the matrix in `scripts/gate-captures`,
108 captures in all, then graded by a fresh-context workflow (7 Opus
vision graders by capture group, 19 adversarial refuters on the
structural findings, one is-it-right auditor).

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

## Re-shooting

`scripts/gate-captures <outdir>` on the gate platform with a display
and `FASTMAIL_API_TOKEN` in the environment; the full set is about
32 MB and stays out of git. The sketch frames in this directory
predate task 2's fix and show the phantom rail on purpose.
