---
title: v2 declarative View fields — ProgressBar, ReportFocus, KeyboardEnhancements
status: accepted
date: 2026-05-11
---

## Context

`tea.View` exposes `ProgressBar`, `ReportFocus`, and
`KeyboardEnhancements` as per-frame declarative fields. Pass 30
landed the `tea.View` return shape but only set `AltScreen` and
`WindowTitle`; the other three sat unused while their feature
ROADMAP entry described the consumers.

## Decision

`App.View()` sets all five declarative fields per frame.

- **ProgressBar** follows a fixed priority ladder: attachment
  download > outbox drain > sync. Outbox carries a percentage
  (`OutboxDrainProgress`); attachment and sync ride as
  Indeterminate (no per-byte progress yet). Error state decays
  over ~3s through `progressErrorUntil`. The cache exposes three
  accessors (`AttachmentDownloadProgress`, `OutboxDrainProgress`,
  `SyncProgress`); the App layer composes them in
  `frameProgressBar`.
- **ReportFocus** is on. `tea.FocusMsg`/`BlurMsg` toggle
  `App.focused`. An unfocused-only new-mail toast consumes the
  blur signal: arrivals coalesce across a 1s window, then render
  as `· N new from Foo ·` (single sender) or `· N new in Inbox ·`
  (mixed). `[ui] new-mail-toast = true` by default; setting
  false silences it. `tea.FocusMsg` clears the toast iff the
  active toast is the new-mail variant.
- **KeyboardEnhancements** requests `ReportEventTypes`; Kitty
  disambiguation rides as a default. The negotiated
  `KeyboardEnhancementsMsg` lands in `App.kbdCaps`.
  `uicore.GatedBinding` tags chords whose semantics depend on
  Ctrl+letter disambiguation. `catkin.ChordSet()` returns the
  full six; `catkin.ActiveChords(disambiguates bool)` projects
  the subset that should render. The helppopover Compose context
  and the compose footer hint consume `ActiveChords` to filter.

## Consequences

OS-taskbar progress reflects long-running ops without poplar
drawing a bar. Unfocused users see a transient toast on new
mail; the focus-clear behavior keeps the toast honest. Catkin's
full chord vocabulary becomes honest — visible only on terminals
that can deliver the chords.

The deferred consumers (server-side IDLE pause on blur,
`IsRepeat` for held-key acceleration) are not wired this pass.
The field plumbing — `App.focused` and `App.kbdCaps` — is in
place; a future pass can wire those consumers without re-touching
`App.View`.

The bubbletea v2 `KeyboardEnhancementsMsg` carries `Flags int`
(Kitty bitmask) and exposes `SupportsKeyDisambiguation()` /
`SupportsEventTypes()` accessors; the spec's bool-field shape did
not match the real API. The implementation uses the actual
methods.
