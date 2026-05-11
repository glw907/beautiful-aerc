# Pass 15a — `bubbles/v2/list` adoption for picker surfaces

**Date:** 2026-05-10
**Status:** approved, pending plan
**Pass:** 15a (first of four bubbles-adoption passes before v0.9.0
freeze)

## Goal

Replace hand-rolled cursor, filter, and render code in three
picker surfaces with `charm.land/bubbles/v2/list`. Ship the
upstream component as the load-bearing list primitive in poplar
without regressing visual or behavioral fidelity.

## Context

Poplar shipped Pass 13.2a/b on `charm.land/v2` (ADRs 0189a/b) but
several UI surfaces still hand-roll what `bubbles/v2` now
provides. CLAUDE.md frames poplar as showcase-quality;
`bubbletea-conventions.md` requires ADRs for any bubbles
deviation. None exist for the picker family today, so the
deviation is debt rather than a deliberate call.

Pass 15a is the first of four adoption passes (15a–15d) that land
before Pass 16 (Polish II) and Pass 17 (v0.9.0 prep). Polishing
surfaces about to be rewritten would be wasted motion, so
adoption precedes polish.

## Surface census

Investigation surfaced six picker-shaped surfaces, not four. They
split three ways:

| Surface | Shape | Disposition |
|---|---|---|
| `internal/ui/movepicker/` | Folder list with filter | **In scope (15a)** |
| `internal/ui/reader/linkpicker.go` | URL list, ≤9 items | **In scope (15a)** |
| `internal/ui/reader/attachpicker.go` | Attachment list | **In scope (15a)** |
| `internal/ui/compose/attachpicker.go` | Multi-select TUI file browser, ADR-0179 | Carved out → Pass 15a.5 (`bubbles/v2/filepicker` adoption) |
| `internal/ui/helppopover/` | Static grouped key reference card with `wired bool` flags | Pass 15d (ADR'd deviation — not a list) |
| `internal/ui/compose/schedulepicker.go` | 4 hard-coded rows + `Custom…` textinput | Pass 15d (ADR'd deviation — not a list) |

## Architecture

### Shared style helper

New file `internal/ui/uicore/list_styles.go` exports:

    func NewListStyles(t *theme.CompiledTheme) list.Styles

The function projects poplar palette slots onto `list.Styles`
fields. One source of truth for list theming. Three consumers
call it from their `NewStyles(*theme.CompiledTheme)` constructors
and store the result on the per-subpackage `Styles` struct.

`uicore` is the only existing place outside per-subpackage
`styles.go` files permitted to call `lipgloss.NewStyle()`
(invariants.md "Config & theming"); the helper fits there
without further policy change.

### Per-consumer integration

Each consumer:

- Adds a `list list.Model` field to its `Model` struct.
- Constructs the `list.Model` in `New(...)` with the consumer's
  items, the shared `list.Styles`, and a custom `list.ItemDelegate`.
- Routes `tea.Msg` through `list.Model.Update` in its own
  `Update`. Forwards `tea.WindowSizeMsg` after storing dims.
- Reads cursor and filter state via `list.Model` accessors
  (`Index()`, `SelectedItem()`, `FilterState()`, `FilterValue()`).
- Emits the same external `tea.Msg` types it emits today
  (`movepicker.MovedMsg`, `reader.LinkSelectedMsg`, etc.) so
  callers are unchanged.
- Drops the existing hand-rolled cursor field, filter buffer,
  and render loop.

### Custom item delegates

`list.Item` is `FilterValue() string` — trivial. The visual
fidelity work lives in `ItemDelegate.Render(io.Writer, Model,
int, Item)`. Each consumer keeps its bespoke item shape:

- **movepicker:** folder role badge, unread count, optional
  Nerd Font icon, indent for nested names.
- **linkpicker:** footnote number `[N]`, link text, dim host
  fragment.
- **reader/attachpicker:** filename, humanized size, MIME hint.

Delegates draw colors from the per-subpackage `Styles` struct;
the shared `list.Styles` from `uicore.NewListStyles` covers
chrome (border, scrollbar, filter prompt, status messages).

### ModalShell consumers stay ModalShell consumers

`movepicker`, `linkpicker`, and `reader/attachpicker` all embed
`uicore.ModalShell` for frame chrome (invariants.md "Overlays").
After adoption, `list.Model.View()` becomes the `bodyRows` source
fed to `m.shell.Box(title, bodyRows, footerRows, contentW)`.
Frame chrome, dim-underlay treatment, and overlay cascade are
unchanged.

### Render cache disposition

`movepicker` currently wraps render in a heap-allocated
`*movepickerCache` with a dirty flag (one of three Elm escape
hatches in the tree, ADR-0130). `list.Model` owns its own
viewport and paginator state, which is the upstream answer to
the same problem. The cache field comes out.

Risk: a regression on 1000+-folder accounts. Mitigation: profile
the worst-case account in the tmux harness; if `list`'s scroll
math is materially slower than the cached render, ADR the cache
retention as a poplar-side wrapper. Expectation: no regression.

## Implementation order

1. **`reader/linkpicker.go`** — smallest (251 lines), no filter,
   ≤9 items. Validates the integration shape with the lowest
   risk.
2. **`reader/attachpicker.go`** — similar shape (202 lines), no
   filter.
3. **`movepicker/`** — largest (360 lines), filter + ModalShell
   cache. Most surface area to validate.

Each lands as its own commit so review reads cleanly.

## Tests

Existing table-driven `_test.go` files stay. New cases cover the
`list.Model` integration boundary: cursor advance/wrap, filter
match/clear, item rendering through the custom delegate.
Upstream `bubbles/v2/list/list_test.go` already covers `list`
internals; we don't duplicate.

Live UI verification per `.claude/docs/tmux-testing.md`: capture
each picker at 80×24 and 120×40 before and after, confirm visual
parity (frame chrome, item rendering, filter shelf, scrollbar).

## Bubbletea-conventions checklist alignment

Per `docs/poplar/bubbletea-conventions.md` §10, every changed
surface must pass:

- `View()` returns no lines wider than assigned width, no more
  rows than assigned height — verified by the tmux capture.
- No state mutation in `View()` or `tea.Cmd` closures.
- `WindowSizeMsg` forwarded into `list.Model` after dim store.
- Width math via `lipgloss.Width` / `ansix.Width` — the custom
  delegate honors this.
- Children signal parents via exported `tea.Msg` types — the
  external Msg surface is unchanged.
- Keys via `key.Binding` + `key.Matches` — `list.Model`
  declares its own; consumers map external bindings onto
  `list.Model` keys via `list.Model.KeyMap` overrides.

## ADRs to write at pass end

1. `0194-bubbles-v2-list-adoption.md` — adopt `bubbles/v2/list`
   as the load-bearing list primitive; cite the four-pass
   adoption plan; carve out `helppopover` and `schedulepicker`
   as deviations to be ADR'd in 15d.
2. (Possibly) `0195-list-render-cache.md` if the movepicker
   profile shows a regression worth wrapping.

## Out of scope

- `compose/attachpicker.go` → Pass 15a.5
  (`bubbles/v2/filepicker`).
- `helppopover`, `schedulepicker` → Pass 15d (ADR'd deviations).
- `messagelist` → Pass 15c (custom item delegate for thread-
  prefix walk).
- Sidebar tree → Pass 15b (`Digital-Shane/treeview` candidate
  vs. `list` with custom indentation).

## Acceptance criteria

- [ ] `make check` green.
- [ ] Three picker surfaces use `bubbles/v2/list`; no hand-
      rolled cursor or filter code remains in those files.
- [ ] `internal/ui/uicore/list_styles.go` exists and is the
      sole projection from palette to `list.Styles`.
- [ ] tmux captures at 80×24 and 120×40 confirm visual parity
      for each picker.
- [ ] ADR-0194 written; invariants.md updated to name
      `bubbles/v2/list` as the picker primitive; STATUS.md
      marks 15a done and queues 15a.5 as the next pass.
