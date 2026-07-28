# ADR-0011: bubbletea v2 root model with a screen registry

Date 2026-07-27. Status: accepted (Phase 4).

## Context

The UI must satisfy UX-1 through UX-9 mechanically: a registry the
grammar test iterates, footers and help derived from keymaps so
they cannot drift, surface state surviving switches, and the
showcase bar (QA-10). bubbletea v2 went stable 2026-02 with the
declarative `tea.View` and new key model; v1 is frozen.

## Decision

bubbletea v2 (`charm.land` paths). One root model owns the active
surface, screen stack, global chrome (footer, status line, toasts,
reminder banner), and account-keyed UI state. Screens implement a
`Screen` interface and register in a package-level registry at
init; keymaps are registry data, and the footer, help overlay,
grammar test, and switch-table test all derive from the same
entries. Components own their size contract (parents pass
dimensions; no parent-side clipping). The reader renders pipeline
markdown through glamour v2 with theme-driven styles; Catkin is
its own model over an incremental goldmark-AST renderer and
imports nothing from poplar. Store reads run as commands;
store-changed messages trigger re-query of visible state.

## Alternatives considered

- **Staying on bubbletea v1**: frozen line; C9 forbids inheriting
  the legacy pin, and v2's pure lipgloss integration is what
  makes compiled themes deterministic.
- **Per-screen ad hoc keymaps** (no registry): UX-1/UX-2's
  acceptance criteria become unenforceable by test; drift between
  footer and behavior was a real legacy-client defect class.
- **glamour for the compose editor**: whole-document re-render
  per keystroke; the survey found no serious app doing live
  editing through it. The goldmark AST walker renders
  incrementally and owns cursor-aware styling.
- **A widget framework abstraction over bubbles**: bubbles v2
  components plus the design language's component vocabulary
  cover the needs; a framework layer would be poplar-only idiom
  with no external maintainer.

## Consequences

The registry is the single source of interaction truth, which is
what makes UX-2's "footer never stale" a theorem instead of a
review item. teatest drives screens headlessly for goldens. The
elm-conventions discipline (state in models, mutation in Update,
I/O in Cmds) is assumed throughout and gated in Phase 5.

## Revision 2 (2026-07-27, post-review)

Two dependency rulings the review demanded be explicit:

- **The list is poplar's own windowed model.** bubbles/list holds
  every item in memory and filters in-process, which cannot meet
  the QA-5 envelope; poplar's list reads keyset-paginated windows
  from the store's read pool. Other bubbles components (viewport,
  spinner, textinput) are used where their contracts fit.
- **huh is ruled out for v1 forms.** The registry-derived footer
  (UX-2) and the UX-8 leave-field model are load-bearing
  constraints a third-party form engine does not satisfy; forms
  are the design language's form component over the shared
  focus-management helper.

Also added: the capability-profile resolver (NO_COLOR, TERM,
COLORTERM, the background-color query, config override) with the
100 ms bounded first-frame wait and default-dark repaint; the
`AccountScoped[T]` wrapper making C4's UI half mechanically
testable; and Catkin's injection contract (its own style struct,
poplar injects theme-derived values; the analyzer exempts
`internal/catkin`; a buffer-mutation API lands each external
mutation as one buffer-undo entry).

## Revision 3 (2026-07-27, build boundary)

Two additions the mouse ruling (ADR-0017) requires, recorded here
because revision 2 described a keyboard-only registry:

- **`LayoutMode` carries per-pane rectangles.** Layout already
  computes once per `WindowSizeMsg` into one struct every
  component consumes. That struct now also carries each pane's
  `image.Rectangle`, so a pointer event resolves to a pane by
  comparison against values the layout already produced. No zone
  library and no render-time registration is involved, which
  matters because bubblezone v2 documents a caveat against the
  lipgloss v2 compositor.
- **Registry entries bind pointer targets alongside keys.** A
  screen's registry entry is already the single source the
  footer, help overlay, grammar test, and switch table derive
  from. Pointer targets join it there, which is what makes UX-6's
  acceptance criterion mechanical: every mouse-reachable action
  has a keymap entry because both hang off one entry, and the
  registry test reads it.
