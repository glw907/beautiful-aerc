# Reading-pane placement and line length: the evidence

**Date:** 2026-08-19. **Trigger:** Geoff's pass 2 wireframe-review
question: does the far-right reader pane make sense from a
readability standpoint, or would the message read better below two
columns? **Method:** one live research dispatch, sources fetched
and graded (peer-reviewed / industry study / vendor claim / design
convention). Feeds the wide-rung layout ruling and pass 3's
rendering-measure decisions.

## Line length (the strong evidence)

- **Dyson & Haselgrove 2001** (IJHCS, controlled, 48 participants,
  25-100 CPL): 55 CPL gave the best comprehension and the fastest
  effective reading; no speed-accuracy tradeoff. Peer-reviewed.
- **Bernard, Fernandez, Hull, Chaparro 2002/2003** (HFES): no
  measurable speed or efficiency difference up to 132 CPL, but
  adults preferred ~76 CPL. Speed and preference explicitly
  diverge. Peer-reviewed.
- **Ling & van Schaik 2006** (IJHCS, 55/70/85/100 CPL): no
  significant speed difference; participants preferred 55 CPL.
  Independent confirmation of the divergence. Peer-reviewed.
- **Baymard Institute** (industry synthesis): 50-75 CPL, upper
  accessibility bound 80. Leans on Ruder's 1967 craft tradition
  plus behavioral observation; not primary research.
- **WCAG 1.4.8**: at most 80 characters per line. Standards body.

**Converged reading:** speed is roughly line-length-invariant
across a broad range; comprehension and preference peak around
55-76 CPL and fall off hard past ~100. The folk claim that long
lines read faster is not supported.

## Pane placement (the gap)

- **No controlled study compares vertical against horizontal
  email preview panes.** This is a genuine research gap.
- Outlook 2003's vertical pane: Jensen Harris (its UX designer)
  says the pane was sized to "~65 characters" citing reading
  research generically; the famous "40% more email" figure is
  Microsoft marketing with no released methodology, and it does
  not even agree with Harris's own "twice as much" figure.
  Vendor narrative, not data.
- NN/g's F-pattern eye-tracking (2006+, 232 users) is the nearest
  relevant industry study: web-content scanning, analogous at
  best.
- Master-detail orientation is documented as a design pattern,
  never comparatively measured.

## What this decides, and what it leaves

The measure question is decided by evidence: prose reads best at
50-75 CPL, so a full-width bottom pane's extra width buys nothing
readable past ~80 cells. The orientation question is decided only
by arithmetic and convention: on a 140+-column terminal, width is
the surplus axis and height the scarce one, so the vertical pane
preserves list rows and message rows at once while landing its
measure inside the evidence band (~65 cells at 150 columns);
Outlook, Apple Mail, and Gmail converged there, but by practice,
not published proof.

Carried implication for pass 3's renderer: the design language
caps reader content at ~100 cells, which sits above the prose
evidence band. The cap is right for code, diffs, and tables
(functional width); prose paragraphs inside the reader should wrap
nearer ~72-78 cells even when the pane offers more, with the full
cap reserved for preformatted content. That is a pass 3 rendering
ruling, recorded here so it is not invented ad hoc.
