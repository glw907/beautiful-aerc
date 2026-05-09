# `bubbles/list` — Pattern Catalog

Patterns harvested from `bubbles/list@v1.0.0` during Pass 9x.1, the
list-family lesson-harvest sweep. Companion to the eval at
`docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`,
which decided every list-shaped surface stays hand-rolled. This doc
captures *what to keep in mind* from the library's design, organized
by pattern rather than by surface.

Each entry: pattern → where in `bubbles/list` (file:line) → why it's
a good idea → poplar applicability (applied now / candidate for
later / no — with rationale).

Loaded into context on demand. Intended for the Pass 9x.2 (chrome
family) and 9x.3 (table/form family) reviewers, plus any later UI
pass that touches a list-shaped surface.

---

## 1. Filter-match rune highlight via `lipgloss.StyleRunes`

**Where.** `defaultitem.go:191-202`. The library's default delegate
extracts matched-rune indices from the parent model
(`m.MatchesForItem(index)`, `list.go:483-498`) and paints them with
`lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)`.

**Why it's good.** Rune-level visual cue tells the user *why* a
filtered row is in the result set without re-reading. Substring
matches are obvious; fuzzy matches benefit even more. Costs nothing
when the filter is empty.

**Applicability.** Applied now in movepicker (Pass 9x.1 diff —
`internal/ui/movepicker/model.go` `buildListRows`). Underline-only
style so it composes with the row's base foreground; non-cursor
rows only at first cut to avoid the SGR-composition fuss the
library sidesteps with `Inline(true).Inherit(...)`. See pattern §2
for the composition trick if cursor-row highlight is wanted later.

linkpicker / reader/attachpicker / sidebar do not filter
interactively — pattern does not apply.

---

## 2. `Inline(true).Inherit(other)` for layered styles

**Where.** `defaultitem.go:191-194`. To make selected-and-matched
runes carry both styles (selected fg + match underline) without one
clobbering the other, the library composes:

```go
unmatched := s.SelectedTitle.Inline(true)
matched := unmatched.Inherit(s.FilterMatch)
title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
```

`Inline(true)` strips block-level attributes (padding, borders) so
the style works mid-string; `Inherit` layers the second style on
top of the first.

**Why it's good.** The naive approach — wrap the whole rendered
string in an outer style after `StyleRunes` has emitted internal
SGR resets — produces partial coloring (the resets clear the outer
fg for the rest of the row). `Inline(true).Inherit(...)` applies the
outer style to *every* rune (matched and unmatched) inside the
StyleRunes call, so the row paints uniformly.

**Applicability.** Candidate for later. Movepicker's first cut
skips highlight on the cursor row to avoid this. If the cursor row
should also carry match underline, switch to this composition
trick. Documented here so the next person doesn't have to
re-derive it.

---

## 3. Per-state delegate styles, named by combined state

**Where.** `defaultitem.go:14-31`. `DefaultItemStyles` declares
six fields named by combined state (Normal/Selected/Dimmed × Title/Desc),
not by attribute (Foreground / Border / Padding).

**Why it's good.** When a row can be in three or more states
(normal, selected, dimmed-during-empty-filter), per-attribute style
tables fragment ("which fg for selected? which border for dimmed?").
Named combined-state styles make the matrix explicit and the call
site obvious (`s.SelectedTitle.Render(title)`).

**Applicability.** Sidebar already does this partially
(`SidebarFolder` / `SidebarUnread` / `SidebarSelected`) but the
combined cursor-on-unread state composes via `ApplyBg` rather than
a fourth named style. Worth keeping in mind for any future surface
where state-combinations grow past 3. Not a current refactor target
— sidebar's two-state composition is still legible.

---

## 4. Row renderer separated from list mechanics

**Where.** `list.go` (Model: cursor, scroll, filter state) +
`defaultitem.go` (DefaultDelegate: row presentation). The
`ItemDelegate` interface (`list.go:191-198`) is the seam.

**Why it's good.** The list mechanics (cursor motion, page nav,
filter state machine, viewport scroll) are general; row presentation
is per-application. Separating them lets the same list shell host
many row types.

**Applicability.** Already mirrored in poplar at the per-surface
level: `movepicker.buildListRows`, `linkpicker.formatRow`,
`attachpicker.formatRow`, `sidebar.renderRow`. The library's
*specific* shape — `Render(w io.Writer, m Model, idx int, item Item)`
with cursor + filter state pulled from the parent — is the wrong
shape for poplar (writer sink + parent-pull contradicts the
value-type-with-`View() string` convention from `elm-conventions`).
The *principle* is sound and already in use.

---

## 5. Chrome-on-by-default with suppression flags

**Where.** `list.go:277-375`. Title / filter / pagination / status /
help all render unless `SetShowTitle(false)` etc. is called.

**Why it's good (in the library).** Caller can pick what they want
without subclassing. Sane defaults for the standalone use case.

**Why it's bad (for inline-splice consumers).** Compose dropdown,
movepicker, linkpicker all want **zero chrome by default**. The
library's suppression flags work but leave layout residue
(`SetShowFilter(false)` still leaves a one-row filter slot in some
configurations). Chrome-on-by-default is the wrong default for
embeddable components.

**Applicability.** Catalog as an anti-pattern for poplar's
component design. Compose dropdown's `View()` returns bare
`\n`-joined rows — chrome-off by default. Keep this discipline
for any future inline-splice component.

---

## 6. Index-stable digit dispatch (poplar-original)

**Where.** Not in `bubbles/list`. Poplar's linkpicker
(`internal/ui/reader/linkpicker.go` `linkPickerKeys.Digits`) and
attachpickers (reader + compose) bind `1`-`9` to "activate item N".

**Why it's good.** For modal lists with ≤9 items, single-key
direct activation beats arrow-then-Enter. Discoverable via the
visible `[N]` index column. Costs one keymap entry per slot.

**Applicability.** Not a `bubbles/list` harvest — listed here for
catalog completeness. Could apply to movepicker (folder picker)
where a filter narrows to ≤9 results: bind `1`-`9` to picked-folder
shortcuts. Candidate for a later UX polish pass; not landed in
9x.1.

---

## 7. Preview row below the fold (poplar-original)

**Where.** Not in `bubbles/list`. Linkpicker's `previewLines`
(`internal/ui/reader/linkpicker.go:220-236`) renders 2 wrapped lines
of the cursor row's full URL below the list when row truncation
hides the tail.

**Why it's good.** Truncated row text is opaque ("…") — the preview
gives the user the full content for the row they're considering
without expanding the modal width.

**Applicability.** Could apply to movepicker for long nested
folder paths (`Receipts/2026/Q1/January` truncated to `Receipts/…`).
Candidate for a later pass.

---

## 8. Per-row action verbs as keybindings (poplar-original)

**Where.** Not in `bubbles/list`. reader/attachpicker binds `o`
(open), `s` (save), `Enter` (open default), `1`-`9` (open Nth)
against the cursor-selected attachment.

**Why it's good.** Two or more per-row actions don't need a sub-menu
or a verb-then-row sequence. Each verb is one key. Discoverable via
the footer hint row.

**Applicability.** Library's `ItemDelegate.UpdateFunc` is tied to
delegate state, not row-keyed verbs — no equivalent pattern. Worth
keeping in mind when a future modal grows a second action verb (e.g.
movepicker grows "preview folder contents" — bind `p` not a
sub-menu).

---

## 9. KeyMap as a dedicated value type

**Where.** `keys.go:7-31`. `KeyMap` is a struct of `key.Binding`
fields, separated from the model. `DefaultKeyMap()` returns the
canonical set; `Model.KeyMap` field lets consumers swap the whole
table.

**Why it's good.** Bindings are inspectable, swappable, and feed
directly into `bubbles/help`'s ShortHelp/FullHelp. The model's
Update branches on `key.Matches(msg, m.KeyMap.CursorUp)` — declarative,
not magic-string'd.

**Applicability.** Already idiomatic across poplar UI. Pass 9x.1
adjacent cleanup in compose dropdown (`internal/ui/compose/suggest.go`)
swapped `tea.KeyMsg.Type` switch to `key.Binding` + `key.Matches` to
match this norm. Worth re-checking other sub-models for stragglers.

---

## What did NOT survive the harvest

- **Paginator math.** The library's `paginator.Model` integration
  prints page dots and computes per-page offsets. Poplar's pickers
  use simple `offset` + `uicore.ClampScrollOffset`. No paginator
  consumer in the codebase; PageUp/PageDown are deliberately
  unbound (`feedback_no_modifier_keybindings` constraints push
  away from extra nav keys).
- **`FilterState` two-phase state machine** (Filtering /
  FilterApplied / Unfiltered, `list.go:118-138`). Movepicker's
  per-keystroke `recompute` is one-phase; the commit step buys
  nothing.
- **Status bar item count + singular/plural item-name mode**
  (`list.go:346-352`). Poplar shows item counts in the modal title
  ("Move to (N)"), not in a status bar. Cleaner.
- **`bubbles/list.Styles` field shape** (17 fields, 13 chrome
  slots). Importing the shape would import dead slots.

---

## Cross-cutting code candidates

Considered, declined for now:

- **`uicore.ListBodyRows(boxHeight, reservedChrome int) int`.**
  Three near-duplicate helpers exist (movepicker `visibleRows`,
  reader `visibleLinkRows`, compose/AttachPicker `viewportRows`)
  with reserve constants 7, 7, 5 respectively. Three call sites
  is borderline; extracting now is premature without a fourth
  consumer. Re-evaluate when a 9x.2 or later pass adds another
  modal list.
