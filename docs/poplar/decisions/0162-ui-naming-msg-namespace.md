---
title: UI naming + per-package Msg-namespace policy
status: accepted
date: 2026-05-06
---

## Context

The Pass 9h.1 reorg (ADR-0161) split `internal/ui/` into bubbles-
shaped subpackages. With multiple subpackages now in play, two
policy questions need binding answers so future passes don't drift:
how subpackage types are named, and how Msg types cross the
parent ↔ child boundary.

The Pass 9h ADRs (0159, 0160) already flagged `ComposeTab` and
`AccountTab` as misleading — there is no tab UI in poplar; the
suffix was a Pass 9g placeholder. The `*Tab` shape also reads
poorly against the bubbles convention (`bubbles/list.Model`,
`bubbles/textinput.Model`), which is the strongest external
authority for poplar's bubbletea idioms.

The Msg question is more subtle. Before the split, every Msg
lived in one `package ui` and every consumer referenced it
unqualified. After the split, a Msg either flows entirely within
one subpackage (private), flows from a subpackage out to App or
back in (cross-boundary), or originates at App and gets routed to
exactly one subpackage. Without a policy, every author would make
a different choice and the package boundary would leak in
inconsistent ways.

## Decision

**Naming.** Every subpackage exposes a single canonical `Model`
type and a `New(...)` constructor. The package name carries the
component identity (`compose.Model`, `sidebar.Model`,
`reader.Model`) and the type name does not duplicate it. The
`*Tab` suffix is dropped — `ComposeTab` is `compose.Model`,
`AccountTab` will become `account.Model` in Pass 9h.2.

A subpackage may export sub-models when they have non-trivial
state of their own — `sidebar.Column`, `sidebar.Search`,
`reader.LinkPicker`, `reader.AttachPicker`. Those keep
descriptive names; the package qualifier already provides the
scope. Single-component subpackages (compose, movepicker,
helppopover, messagelist) export only `Model` + `New`.

Per-subpackage `Styles` lives in that subpackage's `styles.go`
with a `NewStyles(*theme.CompiledTheme) Styles` constructor.
Each `Styles` is a narrow projection of `internal/ui.Styles` —
only the fields that subpackage actually renders. `lipgloss.NewStyle()`
is permitted in `internal/theme/palette.go`,
`internal/ui/styles.go`, and any per-subpackage `styles.go`; no
other site is allowed to construct base styles.

**Msg namespace.** A Msg type lives in the package that *produces*
it. Three patterns:

- **Private Msgs** (subpackage-internal): unexported, defined and
  consumed inside the subpackage. They never cross the boundary.
  Example: `messagelist`'s internal layout msgs.
- **Outbound Msgs** (subpackage → App): exported, defined in
  `<subpkg>/msgs.go`. The parent's `Update` switch arms qualify
  them (`case reader.BodyLoadedMsg:`, `case account.FoldersLoadedMsg:`
  in 9h.2). Subpackage-scoped cmds (the ones that don't emit
  `ErrorMsg`) live in `<subpkg>/cmds.go` next to the msgs they
  produce.
- **Inbound Msgs** (App → subpackage): exported, also defined in
  `<subpkg>/msgs.go`. App fires them by spelling the qualified
  name. `sidebar.ClearSearchMsg` — fired by App's Esc handler,
  consumed by `sidebar.Model.Update` — is the canonical example.

Every Msg type carries the `Msg` suffix. No exceptions. The
suffix is not redundant in the qualified spelling
(`reader.BodyLoadedMsg`) — it's the marker that distinguishes a
type used in `tea.Msg` switches from a regular value type.

**Cmds that emit `ErrorMsg`.** `ErrorMsg` is defined in
`internal/ui/cmds.go` (App owns the error banner). A cmd that
emits `ErrorMsg` on a failure path cannot live in a subpackage
without a circular import. Those cmds stay in `internal/ui/cmds.go`
and accept App-level seams (`URLOpener`, `TidyFn`) as function-
typed parameters. The msgs they produce on the success path can
still live in `<subpkg>/msgs.go` — the cmd just imports the
subpackage to construct its result. Compose's `composeSeedCmd` /
`composeSendCmd` and several reader cmds follow this shape.

**Test boundary.** Subpackage tests prefer `package <name>_test`
(external) so the package's exported API is the only assertion
surface. When a test must touch unexported state, the file uses
`package <name>` (white-box) and the subpackage adds narrow
accessors (`Source()`, `Layout()`, `SelectedCanonical()`). These
accessors are tagged in their godoc as test-only and are
candidates for removal once Pass 9h.2's `account/` extraction
lets parent-package tests in `internal/ui/` move to
`package ui_test`.

## Consequences

- Pass 9h.2 renames `AccountTab` → `account.Model` and lifts the
  account-scoped cmds and msgs. After 9h.2, no `*Tab` types remain.
- `internal/ui/compose` shadows `internal/compose` (the domain
  package). The convention: App-side imports use
  `uicompose "github.com/glw907/poplar/internal/ui/compose"`;
  inside `internal/ui/compose/` the domain package is aliased
  as `mailcompose`. Both aliases are stable.
- A subpackage that needs a primitive currently in `internal/ui/`
  doesn't reach across and import the parent — it asks whether
  the primitive belongs in `uicore` (most do) and the primitive
  hoists. Tasks 2.5 and 4 of this pass are the canonical
  examples; Pass 9h.2 may surface more.
- Compat shims are forbidden in `internal/ui/` after a hoist.
  Tasks 2.5 and 4 had to be re-fixed because they left thin
  type-alias and delegation wrappers behind. Consumers spell
  `uicore.<X>` directly at the call site.
- The `Msg`-suffix rule means types like `BodyLoaded` keep their
  `Msg` suffix even when the qualified spelling
  (`reader.BodyLoadedMsg`) feels redundant. The redundancy is the
  point — it signals "this is part of the tea.Msg switch surface."
