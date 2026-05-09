---
title: Upstream displaywidth override hook — investigation closed; vendored shim chosen
status: accepted
date: 2026-05-08
---

## Context

ROADMAP entry `spua-width-upstream` framed a one-line override
of `runewidth.DefaultCondition.RuneWidth` as the lever that
would collapse ADR-0084's machinery — the runtime SPUA cell-
width probe, the `uicore.DisplayCells` family, the
`lipgloss.JoinHorizontal` ban, and the manual row-by-row joins.
Pass 9w.0's investigation
(`docs/superpowers/archive/specs/2026-05-08-spua-width-upstream-investigation.md`)
walked the resolved dependency chain and found the seam had
moved: lipgloss now routes through `clipperhouse/displaywidth`
via `charmbracelet/x/ansi`, and `displaywidth.Options` exposes
no override hook. The runewidth path is dead for the default
`GraphemeWidth` mode that lipgloss uses.

Pass 9w prepared an upstream PR adding `Options.Overrides
[]OverrideRange` to `clipperhouse/displaywidth`: implemented on
a local fork, tests + fuzz + benchmarks + README — zero-
allocation preserved on the nil-overrides path, ~349 ns
worst-case on the printable-ASCII gate. Before pushing, a
review of the repo's external-contribution record showed zero
external-contributor merges in the prior seven months (19
merged PRs, all by `clipperhouse`). A maintainer comment on the
unrelated PR #23, dated 2026-05-02, stated the position
directly: *"The external overrides, I can imagine the use case …
But I don't love injecting into this library, I'd rather
someone wrap this library instead."*

## Decision

**The upstream override-hook path is closed.** The PR was not
filed. The fork prototype is parked locally
(`~/Projects/displaywidth/add-overrides`) as a record of the
design and to preserve optionality if the upstream picture
shifts. Poplar will instead vendor a thin SPUA-aware width
adapter at `internal/ansix/`, queued as Pass 9w.1.

The choice between (a) re-engaging the maintainer with a
wrapper-can't-work argument and (b) wrapping silently was
decided by the asymmetry: a rejected PR burns review goodwill
that we may need later for a different conversation (the
lipgloss-internal-call problem the override hook does not
solve anyway). Wrapping silently keeps that goodwill intact
and lets future evidence — multiple downstream consumers, a
sharper articulation of the layer-mismatch problem — drive any
re-engagement, not the override hook itself.

## Consequences

- ADR-0084 stays in force. The JoinHorizontal ban + manual row
  joins in `AccountTab.View` / `App.renderFrame` *stay*; the
  shim does not unlock them (those are inside `lipgloss`,
  upstream of any package poplar can vendor without forking
  lipgloss too).
- The shim's deletion budget is the four wrappers in
  `internal/ui/uicore/iconwidth.go` plus 16 callsite renames.
  Pass 9w.1 lands those changes and writes a separate ADR
  codifying the `internal/ansix/` package boundary.
- The override-hook design is preserved in the parked fork
  branch. If `clipperhouse/displaywidth` later signals interest
  (e.g. enough downstream wrappers accumulate that the layer-
  mismatch argument lands), the prototype can be revived as an
  issue linking that signal — not as a cold PR.
- The investigation specs move to
  `docs/superpowers/archive/specs/`; the plan moves to
  `docs/superpowers/archive/plans/`. Future passes that
  rediscover the SPUA width problem find this ADR first via
  the decision index.
