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
