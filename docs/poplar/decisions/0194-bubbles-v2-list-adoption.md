---
title: Adopt bubbles/v2/list as the load-bearing list primitive
status: accepted
date: 2026-05-10
---

## Context

Pass 13.2a/b (ADRs 0189a/b) put poplar on `charm.land/v2` but the
picker family — `reader.LinkPicker`, `reader.AttachPicker`,
`movepicker.Model` — still hand-rolled cursor, filter, match-paint,
and render code that `bubbles/v2/list` covers upstream. The
bubbletea-conventions rule (ADR-0083) requires an ADR for any
bubbles deviation. None existed for the picker family, so the
deviation was debt rather than a deliberate call.

CLAUDE.md frames poplar as showcase-quality. v0.9.0 cannot freeze
on a hand-rolled list primitive that the upstream component would
cover.

## Decision

`bubbles/v2/list` is the load-bearing list primitive for picker-
shaped UI. Three surfaces adopted in Pass 15a:

- `reader.LinkPicker` — single-column ≤9-item picker with no filter.
- `reader.AttachPicker` — single-column ≤9-item picker with no
  filter, custom item delegate renders `<icon>[N] filename (size)`.
- `movepicker.Model` — always-on filter, custom delegate uses
  `list.Model.MatchesForItem(index)` to underline filter-match
  runes.

Each picker holds a `list.Model`, routes `tea.KeyPressMsg` through
it after handling picker-specific keys (Enter / Esc / digits / pick),
reads cursor + filter state via `list.Model` accessors, and renders
bespoke item shapes via a custom `list.ItemDelegate`.

`uicore.NewListStyles(*theme.CompiledTheme) list.Styles` projects
the compiled theme onto upstream list chrome so the three consumers
share one chrome projection. `uicore.PickerListSize` and
`uicore.SplitAndPad` factor out the picker box geometry and Box
post-processing that was triplicated across the three pickers.

The pre-adoption render cache on movepicker (an ADR-0130 escape
hatch) is removed — `list.Model` owns its own viewport state.

### Always-on filter friction

`movepicker.Model` runs `list.Filtering` from `Open` and never lets
the user exit. Two upstream-friction workarounds are necessary
under that regime, both documented in `model.go`:

- `Open` calls `SetFilterText("")` *before* `SetFilterState(list.
  Filtering)` so `filteredItems` is seeded synchronously. Without
  this, `VisibleItems()` returns empty until the asynchronous
  `FilterMatchesMsg` would arrive — which it never does, because
  the picker model is queried directly by tests and by the App
  before the next tea-loop tick.
- `Update` diffs `FilterValue()` across `list.Update` and, when
  the text changed, re-applies `SetFilterText` + `SetFilterState`
  + `GoToStart()` synchronously. Same root cause: the list emits
  `filterItems` as a Cmd; in always-on mode there is no tea loop
  to deliver the resulting `FilterMatchesMsg`, and downstream
  consumers (Pick's `SelectedItem`, the footer's match count)
  need `VisibleItems()` to reflect the new filter inside the
  same `Update`. Cost is one extra O(n) filter walk per keystroke
  on a typically-tiny folder list.

Cursor nav under `Filtering` also requires intercepting `up`/`k`
/`down`/`j` before delegating, because upstream `updateKeybindings`
disables the list's own nav bindings during filtering. The picker
configures `l.KeyMap.CursorUp`/`CursorDown` to include the j/k
aliases anyway, for symmetry with the other pickers.

A future upstream `SetFilterSync` method on `list.Model` (or an
opt-out for the async filter pipeline) would eliminate the diff-
and-reapply pattern.

## Consequences

- Three subsystems gain upstream filter, scroll, pagination, and
  status-bar behavior for free; bug fixes upstream land in poplar
  on dependency bumps.
- `list.Model.MatchesForItem` replaces the hand-rolled filter-match
  painting in movepicker.
- `uicore` grows two small picker helpers (`PickerListSize`,
  `SplitAndPad`) and one chrome projection (`NewListStyles`).
- Three deferred surfaces remain: `compose/attachpicker` (Pass
  15a.5, `bubbles/v2/filepicker` — different bubble), `helppopover`
  and `schedulepicker` (Pass 15d — neither is list-shaped, ADR'd
  deviations).
- Pass 15b (sidebar tree) and 15c (messagelist on list) build on
  this foundation.
- The always-on-filter workaround is local; if a future bubbles
  release exposes a sync filter knob, ADR a follow-up to remove it.
