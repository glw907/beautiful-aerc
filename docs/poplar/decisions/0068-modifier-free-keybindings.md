---
title: Modifier-free keybindings — no Ctrl/Alt/Meta in user bindings
status: accepted
date: 2026-04-25
---

## Context

ADR 0024 established vim-first single-key bindings; ADR 0051
banned multi-key sequences. A complementary rule had been
operating only as developer memory: poplar binds no Ctrl/Alt/Meta
chord. The msglist still routed `ctrl+d/u/f/b` and the wireframes
still listed those bindings, leaving the rule unenforced.
Pass 2.5b-4 was the first pass where the inconsistency mattered
(viewer needed scroll bindings; the obvious answer is `Ctrl-d/u`
which the rule prohibits).

## Decision

User-facing keybindings use no modifier keys. The msglist drops
`ctrl+d/u/f/b` and `pgup/pgdown`. The viewer uses `j/k/Space/b/g/G`
for scroll/navigation. `Ctrl-c` remains as a terminal-kill alias
on the Quit binding (the keybinding library lists it second after
`q`) but is never advertised in footers, help popovers, or docs.

## Consequences

- The discoverable vocabulary is now smaller and easier to memorize
  — `Space`/`b` for page nav across viewer + future modals,
  `g/G` for jumps everywhere.
- Some power users may miss `Ctrl-d/u` half-page semantics. They
  can use repeated `Space`/`b` or migrate to a different client.
  Poplar is opinionated (see invariant in §UX); this is the kind
  of opinion it ships with.
- The keybindings doc and wireframes are the canonical reference;
  they were updated in this pass to drop every `C-` row.
- Future passes that add chords (e.g. for visual-select multi-row
  triage) must use single-key bindings only.
