---
title: Pass 31 — Audit B.2 (general structural integrity)
status: accepted
date: 2026-05-13
---

## Context

Audit phase B.2 per `docs/poplar/audit-plan.md`, triggered by
B.1 returning clean (ADR-0215). The question: does the
post-decomposition structure hold up on non-Elm dimensions —
`app.go` regression, file-size budget, interface count after
`Editor` deletion, package-boundary leaks?

Walked the four focuses against the `internal/ui/` tree.
Findings doc: `docs/superpowers/plans/2026-05-13-audit-b2.md`.

## Decision

Phase B.2 yields one apply-inline finding and one note-only
observation. No new ROADMAP projects; no P1s.

**F-A — `App.updateKey` is 175 lines (>150 threshold).** The
function pulls double duty: lines 16–86 walk the overlay cascade
(eleven `if guard { update; return }` blocks plus the help
special case), lines 87–188 dispatch the App-level shortcut
switch and fall through to `acct.Update`. The seam is exactly
where the audit lens predicted — the cascade is `(claimed bool)`-
shaped already, just inlined. Extracted `routeOverlayKey` (70
lines, claimed-flag return) and `updateGlobalKey` (104 lines, the
shortcut switch); `updateKey` is now a 10-line chain:
NotifyActivity → routeOverlayKey → contacts-mode short-circuit →
updateGlobalKey. Same dispatch order, same semantics.

**Observation — six `internal/ui/` non-test files exceed ~600
lines, each with a named reason.** `messagelist/model.go` (1111,
threading + fold + group→sort→flatten + list delegate),
`compose/model.go` (1066, the inline compose `Model` with
embedded catkin + Dropdown + AttachPicker + SchedulePicker +
Tidy + signature + autosave), `contacts/form.go` (991, the
multi-row edit form), `account/model.go` (873, the account
screen composing sidebar + messagelist + reader), `cmds.go`
(685, the App-side Cmd library — every fallible I/O leaves
through here), `sidebar/model.go` (643, three groups + tree
expansion + tier-driven width). Each is a `Model` (or the App-
side Cmd surface) whose state genuinely justifies the size; the
audit-plan's escape hatch applies. The `ui-all-value` ROADMAP
project (filed by B.1) will re-touch these files as the
all-value conversion lands; if any stays this large after that
work and still feels like more than one responsibility, it's a
future audit's call. No proactive split this pass.

Dispatch chain (`App.Update`) is exhaustive: `WindowSizeMsg` +
`KeyPressMsg` route to dedicated handlers, every other msg walks
chrome → outbox → compose → modals → contacts → `acct.Update`
fall-through. No back-channel coupling between dispatchers; the
domain helpers (`armToast`, `maybeResizeChild`,
`dispatchUnsubscribe`, `confirmYes/No/Closed`) are private to
their file and reached only within it.

Interface count in `internal/ui/` after `Editor` deletion: two —
`compose.CacheStore` (documented test seam; the declaration
comment names the fake-injection use case) and `wizard.section`
(real polymorphism across seven section types). No new
single-impl interfaces, no anonymous indirection.

Package-boundary scan returns zero hits: no subpackage imports
`internal/ui`. Cross-boundary Msgs live in `<subpkg>/msgs.go`
and qualify at the App-side call site.

## Consequences

Audit B.2 returns one apply-inline finding and one note-only
observation. Phase advances to v2-view-fields / mouse / OAuth
work (Passes 32–35) per `docs/poplar/audit-plan.md`. Audit C
runs after that batch lands.

`App.updateKey` now hits 10 lines, `routeOverlayKey` 70,
`updateGlobalKey` 104 — every `internal/ui/` `Update`-side
function is under the 150-line threshold. The cascade order is
codified as a single comment on `routeOverlayKey` and matches
the order documented in `.claude/rules/ui-invariants.md`.

The six >600-line file observations are not gates on beta soak.
The `ui-all-value` project's per-subpackage conversions are the
natural place for any split call to surface; deciding it now
without the all-value work in hand would be speculative.
