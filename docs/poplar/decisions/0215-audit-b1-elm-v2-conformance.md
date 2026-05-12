---
title: Pass 30 — Audit B.1 (Elm + bubbletea v2 conformance)
status: accepted
date: 2026-05-12
---

## Context

Audit phase B.1 per `docs/poplar/audit-plan.md`, triggered by
Passes 27–29 shipping (catkin all-value, Editor wrapper deletion,
`App.Update` decomposition). The question: does the UI tree
conform to the Elm and bubbletea-v2 contracts after the structural
changes?

Walked every `internal/ui/`, `internal/catkin/`, and
`internal/ui/wizard/` source against the focus list in
`audit-plan.md` §"Phase B.1". Findings doc:
`docs/superpowers/plans/2026-05-12-audit-b1.md`.

## Decision

Phase B.1 yields four findings. Three remediate inline this pass;
one becomes a ROADMAP project.

**F2 — wizard `writeConfig` runs file I/O inline in `Update`.**
`section_confirm.go:61` performed `os.MkdirAll`, `os.Stat`,
`os.CreateTemp`, `Write`, `Sync`, `Chmod`, `Rename` inline on
form completion. Lifted into a `tea.Cmd` returning a
`writeConfigDoneMsg{Path, Err}`. `Update` arms on the message
and flips state when it arrives.

**F3 — catkin's wrapped textarea missed `SetVirtualCursor(false)`.**
ADR-0189b's "every textinput/textarea calls `SetVirtualCursor(false)`"
was literally false for `catkin.NewBuffer`. In practice harmless
(catkin owns rendering and never calls `b.ta.View()`), but the
claim has to be true to be useful to future audits. Added the call
inside `NewBuffer`.

**F4 — `WindowSizeMsg` not forwarded into embedded list / viewport.**
`messagelist.Model.Update` and `reader.Model.Update` both matched
only the events they consumed, dropping `WindowSizeMsg` on the
floor. The embedded `bubbles/v2/list.Model` and `viewport.Model`
therefore never saw it. Bubbles v2 components don't currently
consume `WindowSizeMsg`, but the conventions doc requires the
forward and the cost is two case-arms; restored.

**F1 — pointer-receiver mutators across UI subpackages.** Pass 27
converted catkin; the other eight subpackages still ship the
straddle (value `Update`/`View`, pointer mutators called by the
parent). Multi-pass remediation: filed as ROADMAP project
`ui-all-value`. Sequencing in the ROADMAP entry: `messagelist`
first, then `sidebar`, then `compose`, then the rest.

The stale `catkin-all-value` ROADMAP project (Pass 27 closed it
but the entry never moved) is moved to Done.

## Consequences

Audit B.1 returns three apply-inline findings and one ROADMAP
project. Phase advances to B.2 (general structural integrity)
per audit-plan §triggers. The B.2 starter prompt lands in
`STATUS.md`.

The conventions claim that bubbles components consume
`WindowSizeMsg` (`bubbletea-conventions.md` §2, citing
ref-apps §4 / §8 avoid #6) is currently aspirational for v2 —
no `bubbles/v2/{list,viewport,textinput,textarea,help}` Update
matches `WindowSizeMsg`. The forwards added in F4 are forward-
compatibility for a future bubbles version that adds such an arm.
The doc claim stands; an explicit "as of bubbles/v2 v2.1.0 this
is forward-compat only" caveat in the conventions doc is left for
a future doc pass.

ADR-0189b's literal claim is now backed by code. Future audits
re-checking the claim won't find a hole.

The `ui-all-value` project doesn't gate beta soak; the straddle
works. Sequence between feature passes.
