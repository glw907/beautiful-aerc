# Audit: Invariants vs. Code Drift

> Run with a fresh Claude session on Opus 4.7 (or later). The point
> of this audit is **independent verification** — do not load it
> from the same session that wrote the invariants or the surrounding
> ADRs.

**Goal.** Verify every binding fact in `docs/poplar/invariants.md`
still holds in the current code; flag every contradiction with file
and line references.

**Why now.** Opus 4.7 shipped during poplar development. 69 ADRs
have accumulated, and `invariants.md` is the load-bearing contract
that every pass relies on. A fresh model with no path-dependence
is the best chance to catch claims that drifted as the code evolved
across 20+ implementation passes.

## Scope

Walk `docs/poplar/invariants.md` section-by-section:

1. **Architecture** (~20 claims about package layout, fork policy,
   adapter shape, root model ownership, sidebar/list/viewer contracts).
2. **UX** (~15 claims about keybindings, threading, search, viewer,
   chrome).
3. **Build & verification** (~5 claims about Makefile targets, Go
   module path, skill triggers, pass-end ritual).

For each claim, decide:

- **holds** — confirmed by a specific file/line.
- **drifted** — code says something different (quote both sides).
- **ambiguous** — wording is too loose to verify; suggest a tighter
  phrasing.
- **stale** — refers to deleted or renamed code.

## Out of scope

- Code quality, style, or architectural improvement suggestions.
  This audit is about **doc-vs-code consistency only**. If a real
  bug surfaces, log it via the `log-issue` skill and move on.
- ADR text. ADRs are immutable historical records — drift between
  an ADR and current code is expected and acceptable. Audit only
  the active invariants doc.
- The styling doc, system map, wireframes, keybindings doc. Those
  are separate consistency questions that can have their own audit.

## Inputs to load

- `docs/poplar/invariants.md` (the working list)
- `docs/poplar/system-map.md` (for orientation when grepping)
- The Go source tree under `cmd/poplar/`, `internal/`
- ADRs referenced by the specific claim being checked (lazy-load
  via the decision index at the bottom of `invariants.md`)

## Method

A single Claude session, no subagents needed. Work through the
invariants doc linearly. For each claim, grep or read the relevant
file. Record findings in the output doc as you go — do **not**
batch and synthesize at the end (claims are independent and the
context cost of holding all of them at once is not worth it).

When a claim references "files under X" or "the X struct," grep
for the actual definition. When a claim describes behavior ("Esc
clears the search"), find the keymap and the handler.

## Done

A findings document at
`docs/poplar/audits/2026-04-25-invariants-findings.md` with:

- One entry per checked claim, in the same order as the source.
- File/line refs for every **holds** verdict (so the next reviewer
  can spot-check without redoing the work).
- Both sides quoted for every **drifted** verdict.
- A summary section at the top: counts per verdict + the top three
  drifts ranked by severity.

## Follow-up

If drifts are found, decide for each: **update the doc** to match
the code, or **fix the code** to match the doc. The choice depends
on which side reflects current intent — sometimes the code drifted
from a still-valid invariant, sometimes the invariant ossified
around an old approach.

The output of that decision becomes one or two follow-up
implementation passes (a docs-only commit for invariants updates,
plus a code pass per drift cluster that goes the other way).
