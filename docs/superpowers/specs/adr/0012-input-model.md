# ADR-0012: modifier-free single keys; kitty enhancements declined

Date 2026-07-27. Status: accepted (Phase 4).

## Context

C8 fixes the key model: no chords, no sequences, no modifiers,
vim idiom bent to that constraint. bubbletea v2 makes kitty
keyboard enhancements (disambiguation, key release, repeat
detection) available per view.

## Decision

Poplar binds printable single keys plus Enter, Esc, Space, Tab,
arrows, and PgUp/PgDn, identically on every terminal. Kitty
keyboard enhancements are declined: no `ReportEventTypes`, no
disambiguation, no release events. Text entry follows UX-8's
single model: printable keys are input; Esc is the leave-field
verb exiting to the context's command state; every message-level
verb is a single key in that state. The full verb-to-key grammar
lives in the design-language artifact and binds every surface
through the registry.

## Alternatives considered

- **Adopting kitty enhancements where present**: enables nothing
  poplar binds (modifier combos are out by C8) and forks input
  behavior between enhanced and legacy terminals, which QA-7's
  determinism posture and the "same key, same verb, everywhere"
  rule both disfavor.
- **Esc plus a timeout for bare-Esc detection**: v2's input layer
  handles Esc disambiguation without legacy timeout heuristics;
  nothing to design.
- **Leader-key sequences for surface switching**: sequences are
  excluded by C8 outright; the switching idiom must be single
  keys (design language settles which).

## Consequences

Input behavior is identical across kitty, foot, alacritty, tmux,
and a bare Linux console, which keeps golden-driven UI tests
representative everywhere. If a future feature ever genuinely
needs key-release semantics, this ADR is the record to supersede.
