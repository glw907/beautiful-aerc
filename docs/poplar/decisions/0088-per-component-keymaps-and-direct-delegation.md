---
title: Per-component KeyMaps and delegate-then-read accessors
status: accepted
date: 2026-05-01
---

## Context

Pass 4's bubbletea conventions audit landed three structural
deviations from idiomatic bubbletea:

- **A3 / #17.** `Viewer.handleKey` and `AccountTab.handleKey`
  dispatched on `msg.String()` switches. The community norm
  (`key.Matches` against declared `key.Binding` values) is
  documented in conventions §3; only `App.Update` was already
  using it. Raw-string dispatch hides bindings from
  `bubbles/help` introspection and prevents per-state activation
  via `Disable`/`Enable`.

- **A9 / #18.** App kept `viewerOpen`, status-bar mode/scroll,
  footer context, and the link-picker overlay in sync with
  AccountTab/Viewer state by listening for five intra-model
  signal messages (`FolderChangedMsg`, `ViewerOpenedMsg`,
  `ViewerClosedMsg`, `ViewerScrollMsg`, `LinkPickerOpenMsg`)
  emitted by children as zero-latency `tea.Cmd` round-trips.
  The bubbletea source itself flags this pattern: *"there's
  almost never a reason to use a command to send a message to
  another part of your program"* (`tea.go:62-64`). It also
  splits a single logical state transition across two `Update`
  ticks, which complicates reasoning.

- **A10 / #19.** `App.renderFrame` ran a per-line measure-and-pad
  loop over `AccountTab.View()` output before appending the
  right border, masking any width-contract gap inside
  AccountTab. The size contract (conventions §2) requires
  components to honour their assigned width.

## Decision

**Per-component KeyMaps.** All actionable keys are declared as
`key.Binding` values in component-scoped structs in
`internal/ui/keys.go`: `GlobalKeys`, `AccountKeys`, `ViewerKeys`.
Each owning component holds its KeyMap as a `keys` field and
dispatches via `key.Matches`. The nine link bindings are an
array (`Links [9]key.Binding`) so the dispatch loop is plain
range iteration — no per-key switch.

**Delegate-then-read accessors.** Children expose read-only
accessors for state that the parent needs to mirror; parents
read those accessors immediately after delegating an `Update`
call. App.Update centralises this in `deriveChromeFromAcct`:
after every key-driven delegation it re-reads
`AccountTab.ViewerOpen`, `SelectedFolderCounts`,
`ViewerScrollPct`, `SearchState`, and `LinkPickerRequest`, then
propagates to chrome (footer context, status bar mode/counts,
link-picker overlay open state). The five signal message types
are deleted.

**Pending-link-picker as a one-shot field.** Because App cannot
reach across two layers of nesting to read Viewer state, the
link-picker request is mirrored at AccountTab level via a
`pendingLinkPicker []string` field and a forwarding
`LinkPickerRequest()` accessor. Both Viewer and AccountTab
expose pointer-receiver accessors that clear the field on read.
This is a deliberate two-layer mirror: it preserves the
"accessor reads after delegation" norm without requiring App to
breach AccountTab's encapsulation.

**Width contract enforcement.** Every line produced by
`AccountTab.View()` is exactly the assigned width in display
cells.  `App.renderFrame` trusts the contract and appends the
right border without measuring or padding. The contract is
covered by `TestAccountTabView_HonorsAssignedWidth` (normal,
loading, viewer-loading states) and verified by tmux captures at
120×40 and 80×24.

## Consequences

- Bindings are first-class values that can drive ADR-0072's
  help-popover wired-flag rendering when that pass lands.
- App.Update is shorter and more linear: no signal-message arms,
  no zero-latency round-trips, control flow follows the message
  arrow.
- The new norm replaces the older Elm-architecture phrasing
  ("children signal parents via `tea.Msg`") for intra-model
  state mirrors. `tea.Msg` is now reserved for cross-tree
  signals (Cmds returning external state, error banners, etc.).
  Conventions §8 and §10 are updated accordingly.
- Width-contract regressions surface inside the producing
  component, where they belong, instead of being silently
  papered over by App's padding loop.
- Two-layer pending-field mirror at Viewer and AccountTab is a
  documented deviation from "single-source state" — the
  rationale is layering, not laziness, and it is bounded to the
  link-picker request.
