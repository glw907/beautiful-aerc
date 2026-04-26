---
name: poplar-pass
description: >
  Invoke at the start or end of a poplar development pass. Covers the
  pass-end consolidation ritual (ADR writing, invariants update, plan
  archival, commit + push + install) and the starter-prompt format
  for the next pass. Trigger on "continue development", "next pass",
  "finish pass", "ship pass", or explicit invocation.
---

# Poplar Pass

Poplar development proceeds in numbered passes. Each pass has a
starter prompt in `docs/poplar/STATUS.md`, a plan doc under
`docs/superpowers/plans/`, and (usually) a spec under
`docs/superpowers/specs/`. This skill encodes the ritual at both ends
of a pass.

## Starting a pass

1. Read `docs/poplar/STATUS.md` — grab the current pass number and
   the starter prompt.
2. Read `docs/poplar/invariants.md` — the binding facts it points to
   are auto-loaded via CLAUDE.md, but skim them anyway so the pass
   doesn't accidentally contradict them.
3. Read the plan doc for the current pass if one exists. If the
   starter prompt lists "open questions," brainstorm them first
   (invoke `superpowers:brainstorming`) and write a plan doc at
   `docs/superpowers/plans/YYYY-MM-DD-<topic>.md` before touching
   code.
4. **If the pass touches `internal/ui/`** — read
   `docs/poplar/bubbletea-conventions.md`. The plan doc must name
   the bubbles/glamour analogue for each new component (viewport,
   list, table, textinput, textarea, spinner) and call out any
   deviation explicitly. "Custom because X" is fine; "we just
   wrote a custom thing" is not. The size contract, wordwrap +
   hardwrap discipline, and JoinHorizontal trust contract are
   non-negotiable defaults — deviations from them appear in the
   plan with rationale.
5. Execute the plan.

## Ending a pass — the consolidation ritual

This is the anti-drift core. Every pass ends here. No pass is "done"
until every step has been run.

### 1. /simplify

Run the `simplify` skill. Fix anything it flags before continuing.

### 1b. Idiomatic-bubbletea check (only if `internal/ui/` changed)

Open `docs/poplar/bubbletea-conventions.md` and run its **§10
Review checklist** against the pass's UI diff. Each item is
verifiable from the diff or from a tmux capture:

- [ ] Every changed/new component's `View()` returns no lines
      wider than its assigned width and no more rows than its
      assigned height. **Verified with a live tmux capture at
      120×40** (and at the minimum viable width if the pass
      touches layout).
- [ ] No state mutation in `View()` or in any `tea.Cmd` closure.
- [ ] All blocking I/O lives inside `tea.Cmd`.
- [ ] Width math uses `lipgloss.Width` / `ansi.StringWidth` /
      `displayCells` for icon strings, never `len()`.
- [ ] Renderers that take a `width` parameter honor it via
      wordwrap + hardwrap (or equivalent) — wordwrap-only is
      insufficient.
- [ ] No defensive parent-side clipping — a `MaxWidth` on
      `child.View()` output is a sign the child isn't honoring
      its contract; fix the child instead.
- [ ] Children signal parents via `tea.Msg` types, not callbacks
      or parent pointers.
- [ ] `WindowSizeMsg` is forwarded into children after the parent
      stores dims and calls `SetSize`.
- [ ] Keys declared as `key.Binding`; dispatched with
      `key.Matches`. New keys included in the help vocabulary
      per ADR-0072.
- [ ] No deprecated API usage (`HighPerformanceRendering`,
      `tea.Sequentially`, package-level `spinner.Tick`,
      `*Model.NewModel`, `EnterAltScreen`/`EnableMouse*` in
      `Init`).

Any deviation introduced this pass must be named in the ADR
written in step 2 with explicit rationale. "We deviated because X"
is fine; silent deviation is not.

If the pass produced a conventions audit (e.g. when a new component
is introduced and wants validation), link it from the pass entry
in STATUS.md so future passes can find it.

### 2. Write new ADRs for every design decision made this pass

For each design decision made during the pass, write a new file in
`docs/poplar/decisions/` using the next available number:

```markdown
---
title: <short decision title>
status: accepted
date: YYYY-MM-DD
---

## Context

<why the decision came up, what problem it solves>

## Decision

<the decision itself, stated as a binding fact>

## Consequences

<follow-on effects, what this unlocks, what it forecloses>
```

If a new decision **supersedes** a prior ADR, update the old ADR's
frontmatter to `status: superseded by NNNN` and link the new one in
its Consequences section.

### 3. Update invariants.md — edit in place

`docs/poplar/invariants.md` is the single always-loaded doc of
binding facts. For each ADR written in step 2:

- **Add** a new binding fact if one is missing.
- **Rewrite** an existing fact if the decision changed it.
- **Remove** a fact if the decision made it obsolete.

**Never append blindly.** The file is 300 lines max (enforced by
`.claude/hooks/claude-md-size.sh`). If you add without removing, it
grows unbounded. Consider which existing facts this pass made
redundant or wrong.

Update the decision index table at the bottom to include the new
ADR numbers.

### 4. Update STATUS.md

- Mark the current pass `done` in the pass table.
- Replace the current starter prompt with the next one. Use the
  starter-prompt format below.
- Update the "Next steps" list.
- STATUS.md must stay ≤60 lines. If it's growing, prune.

### 5. Archive this pass's plan + spec

Move the plan file from `docs/superpowers/plans/` to
`docs/superpowers/archive/plans/`. Move the spec (if any) from
`docs/superpowers/specs/` to `docs/superpowers/archive/specs/`. Use
`git mv` so history is preserved.

### 6. make check

`make check` must be green before committing.

### 7. Commit, push, install

```
git add -A
git commit -m "Pass <n>: <summary>"
git push
make install
```

Commit message follows the git conventions from `go-conventions`.

## ADR template

See the block in step 2 above. Keep ADRs short — one paragraph per
section is typical. ADRs are a historical record, not design
documentation; the reader already has the current state in
invariants.md and system-map.md.

## Starter-prompt format

The starter prompt for the next pass lives in STATUS.md. Shape:

```markdown
### Next starter prompt (Pass <n>)

> **Goal.** One sentence describing what this pass accomplishes.
>
> **Scope.** What's in, what's out. Reference the relevant
> wireframes, invariants, and ADRs by path.
>
> **Settled (do not re-brainstorm):** Bulleted list of decisions
> already made elsewhere that this pass inherits.
>
> **Still open — brainstorm these:** Bulleted list of questions
> the pass must answer before coding. If empty, the pass is a pure
> implementation pass.
>
> **Approach.** "Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-<topic>.md`, then implement.
> Standard pass-end checklist applies."
```

## When NOT to use

- Mid-pass debugging or single-file edits — use the simplify,
  systematic-debugging, or ship skills directly.
- Purely doc changes (like fixing a typo in wireframes.md) — no
  ritual needed.
- Non-poplar work in this repo (the idle library packages).
