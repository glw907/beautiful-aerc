# The shell composition exemplar

**Status:** Ratified and pinned by Geoff at the pass 2 wireframe
review (2026-08-19), after two adversarial review rounds (the
three-lens structural review, then the four-lens polish review)
and six owner iterations. **This is the poplar design language's
visual exemplar: every later screen (help, config, calendar,
contacts, compose) designs from it**, the way cairn sites design
from the cairn system. The pass 2 plan's decisions 1-13 carry the
reasoning; the design-language amendments in the plan's decision 3
make its vocabulary binding.

## Files

- `shell-exemplar.py` — the generator; run it in a truecolor
  terminal (`python3 shell-exemplar.py`) to see the composition
  live. The palette dicts and per-segment role/ground assignments
  are the exact values.
- `render.ansi` — the captured truecolor output; `cat` it in a
  terminal.
- `render.txt` — ANSI-stripped geometry, diffable.

## What it settles

Slate palette on four grounds plus a sub-contrast border role;
chrome bands and the reader card on the elevated panel, content
panes on base; exactly three structural lines (rail divider,
reader header rule, modal frame), all in the border role; focus
and cursor as accent edge bars distinguished by glyph weight
(`▌` focused, `▏` unfocused); selection ground promotes its row's
ink one tier; bold means unread; the prioritized single-row footer
with `? help` pinned right; the mail row anatomy (marker columns,
sender 17, fixed thread-count field, subject `·` italic snippet,
2-inset 8-cell date column); the subject-first reader card with
symmetric margins and padded interior.

Both pin-time items were ruled the same day (Geoff): the toast
keeps its dim `9s` countdown (satisfying UX-9 as written) and the
attachment glyph stays `⊕`. The exemplar is fully ratified with no
open items.

The gallery and freeze stills replace this artifact's role once
pass 2's theme and render seam exist; until then this directory is
the design record.
