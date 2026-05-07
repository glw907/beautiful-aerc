---
title: Decision index extracted from invariants.md to its own file
status: accepted
date: 2026-05-07
---

## Context

`docs/poplar/invariants.md` is auto-loaded into every conversation
and capped at 400 lines by `.claude/hooks/claude-md-size.sh`. The
file's binding-facts sections are stable now that path-scoped
extraction (ADR-0153) has moved cache, catkin, attachments, and UI
out into `.claude/rules/<name>-invariants.md`. The runaway
growth section is the **decision index table** at the bottom: one
row per ADR theme, gaining an entry every pass.

By Pass 9k.2 the file sat at 400 lines exactly, with the index
table contributing ~50 lines on its own. Three back-to-back hook
blocks in one session signaled the cap was now load-bearing
against the *index*, not against the invariants. The index is
also the least hot section: it answers "which ADR justifies X" on
demand, not on every turn.

## Decision

Move the themed decision-index table out of `invariants.md` into
`docs/poplar/decisions/INDEX.md`. Replace the in-file section with
a one-paragraph pointer. CLAUDE.md's on-demand-reading list gains
INDEX.md as a sibling pointer to the existing
`docs/poplar/decisions/` entry, scoped to "load when you need the
themed map."

## Consequences

- `invariants.md` drops from 400 → 359 lines, restoring ~40 lines
  of headroom for the 9k.3/9k.4 sweeps and beyond. The 400-line
  cap stays.
- The themed index becomes a load-on-demand artifact. Auto-loaded
  context shrinks by a corresponding number of tokens on every
  conversation start.
- Adding a future ADR theme grows INDEX.md, not invariants.md.
  Pass-end ritual already updates the index — that step now edits
  the new file path.
- The pass-end ritual in `.claude/skills/poplar-pass/SKILL.md`
  refers to "the decision index table at the bottom" — that line
  is now a pointer to INDEX.md instead.
