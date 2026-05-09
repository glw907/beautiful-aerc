# Pass 9x.1 — Harvest from `bubbles/list`

## Goal

Walk poplar's five list-shaped surfaces against the `bubbles/list`
source and produce concrete diffs where the library's design teaches
something poplar isn't doing. Vibes get a one-line skip note; only
diffs land. Verdict from the eval was Keep + harvest at every site
— the harvest is the deliverable.

## Settled (open-question resolution)

The starter prompt names three open questions. All three resolve
from the source without a brainstorm round.

**Q1 — Does `ItemDelegate` simplify any current row renderer?**
No. Skip. The pattern (separate row formatter from list mechanics)
is already present in `buildListRows` / `formatRow` / `renderRow`.
The library's signature `Render(w io.Writer, m Model, idx int,
item Item)` is the wrong shape — wants an `io.Writer` sink and pulls
cursor + filter state from the parent `Model` via `m.Index()` /
`m.FilterState()`. Poplar renderers return strings and take the
cursor as a parameter; switching to writer-based delegate would add
a sentinel-item type plus an `io.Writer` adapter to recover the
string the call site wants. Net: indirection without code removal.
This is also at odds with the value-type sub-model contract from
`elm-conventions` (children expose `View() string`, not writer
plumbing).

**Q2 — Worth mirroring `list.Styles` field shape?** No. Skip.
`list.Styles` enumerates 17 fields; ~13 (TitleBar, Title, Spinner,
Pagination*, StatusBar*, NoItems, HelpStyle) describe chrome
poplar's pickers don't render — that's the whole reason the eval
chose Keep over Adopt. The four that map (FilterPrompt,
FilterCursor, FilterMatch, SelectedTitle/NormalTitle) are already
covered by per-subpackage `Styles` (movepicker has `{Dim, Cursor}`
which is the minimum the surface needs). Mirroring would import
slots with no consumers, contrary to `feedback_optimize_for_claude`
("don't carry dead fields").

**Q3 — Filter / cursor / scroll machinery overlap missed?**
Partial yes — one harvest. The library's filter pipeline carries
two things poplar's substring-match doesn't: (a) `MatchesForItem`
returns matched-rune indices that the default delegate paints with
`lipgloss.StyleRunes(title, indices, matched, unmatched)`, and
(b) a two-phase `Filtering` vs `FilterApplied` state machine. (b)
is over-engineered for movepicker's per-keystroke `recompute` (one
phase, no commit step) — skip. (a) is concrete UX polish on the
one surface poplar has that filters interactively (movepicker).
Worth a diff. The other scroll/cursor machinery (paginator math,
page-up / page-down, GoToStart / GoToEnd) is keys poplar deliberately
doesn't bind (`feedback_no_modifier_keybindings`,
`feedback_no_multikey_sequences`); no harvest there.

## Pass shape

Five surface visits + one cross-cutting task. STATUS allows 0–2
diffs per surface. Each surface task produces two deliverables:

1. **Concrete diff(s)** — code change(s) where the library teaches
   something poplar isn't doing. 0–2 per surface.
2. **Pattern notes** — design choices in `bubbles/list` that struck
   me as good ideas regardless of whether they apply at this surface
   today. Captured as bullet jottings under the surface entry in
   the pass commit body, then consolidated.

Honest expectation: one real diff (movepicker match-rune highlight),
one adjacent cleanup (compose dropdown `key.Matches`), four skip
notes carrying pattern jottings each, plus a research doc
consolidating the catalog at the cross-cutting task. No ADR — no
binding facts change.

The cross-cutting task lands a research doc at
`docs/poplar/research/2026-05-08-bubbles-list-patterns.md` so
Passes 9x.2 / 9x.3 (chrome family / table-form family) and any
later UI work can reference the catalog without re-reading the
library. Format: pattern name → where in `bubbles/list` →
why it's a good idea → poplar applicability (now / later /
no — with rationale).

## Tasks

### 1. movepicker — match-rune highlight

Add filter-match rune highlighting via `lipgloss.StyleRunes`. On
each `recompute()`, capture the byte-index range of the filter
substring within each matched folder's display name; convert to
rune indices once. In `buildListRows`, when `p.filter != ""`,
paint matched runes with `Styles.FilterMatch` (new style slot,
underline-only at first cut to compose cleanly with the cursor
row's full-row inverse-video background).

Notes:
- Substring match only — no fuzzy matching, no regex. The library's
  `DefaultFilter` is fuzzy; movepicker's match is `strings.Contains`
  on lowercased display name. The rune set to highlight is the
  contiguous run starting at the case-folded match position.
- Compose-with-cursor concern: the cursor row wraps the whole
  padded line in `p.styles.Cursor.Render(...)`. `lipgloss.StyleRunes`
  emits per-rune ANSI sequences inside the string before the outer
  wrap; lipgloss styles compose. Verify in tmux.
- Add `Styles.FilterMatch` slot. App-side `NewMovePickerStyles`
  sources from `theme.CompiledTheme` (probably a single underline
  bool — no new color slot needed at first cut).
- Tests: extend `model_test.go` with a case asserting matched-rune
  positions in the rendered row contain the underline SGR.

If the cursor-row composition turns out ugly (background overrides
the underline in some terminals), fall back to highlighting only
on non-cursor rows. Document the call in the commit.

### 2. linkpicker — skip note

No diff. The picker has no filter (URL set is fixed at open),
the cursor / offset / clamp pattern is already shared via
`uicore.ClampScrollOffset`, and the library's chrome (paginator,
status bar, help) is exactly what the eval excluded. The library's
`MatchesForItem` is filter-only — irrelevant. The only library
pattern not mirrored is the `[][]key.Binding` short / full help
shape, which is `bubbles/help` territory, not `bubbles/list`, and
sits on the (b)-list per Eval A.

### 3. reader/attachpicker — skip note

Same shape as linkpicker. `visibleLinkRows` is shared in the
package; cursor/offset uses `uicore.ClampScrollOffset`. No filter.
Row format is icon + `[N] filename (size)` via `ansix.PadOrTruncate`
— delegate adoption would reproduce that exactly via writer plumbing
without LOC reduction. The digit-quick-launch + `o` / `s` split is
not addressable through `ItemDelegate`.

### 4. sidebar — skip note

The three-group blank-line separator is the structural blocker
the eval already named. The library has no inter-item separator
hook. Cursor + selection + unread-count rendering all live in
`renderRow` and need bg-style composition with `ApplyBg` /
`FillRowToWidth` that no library seam reaches. Search shelf is
sibling, not child of the list. Nothing to harvest.

### 5. compose dropdown — skip note (with adjacent cleanup)

The chromeless inline splice is the blocker named in the eval —
no chrome budget, ≤7 rows, value-type sub-model. No diff from the
library.

**Adjacent cleanup found mid-pass** (per pre-beta inline-fix rule):
`Dropdown.Update` switches on `tea.KeyMsg.Type` (`tea.KeyDown` /
`tea.KeyUp`) instead of `key.Matches` against declared
`key.Binding` values. `bubbles/list` and every other poplar sub-
model use `key.Binding` + `key.Matches`. Adding a `dropdownKeys
struct { Up, Down key.Binding }` to the value type and switching to
`key.Matches` aligns with `elm-conventions` / idiomatic-bubbletea
without changing behavior. Land in the same commit as the skip note.

### 6. Cross-cutting

Two deliverables.

**(a) Patterns research doc** at
`docs/poplar/research/2026-05-08-bubbles-list-patterns.md`.
Consolidates pattern notes from tasks 1–5 plus any cross-cutting
patterns the per-surface walks exposed. One section per pattern:
name, where in `bubbles/list` (file:line), why it's a good idea,
poplar applicability (apply now → cite the diff; apply later →
cite the surface; do not apply → cite the rationale). Aimed at
Passes 9x.2 / 9x.3 reviewers.

**(b) Code-level cross-cutting candidates.** Decide on at most
one. Anticipated but unconfirmed:

- **`uicore` helper for "rows = boxHeight − reservedChrome".**
  movepicker has `visibleRows(h int)` (reserve 7), reader has
  `visibleLinkRows(total, h int)` (reserve 7, with an early return
  when total < max), compose/AttachPicker has `viewportRows()`
  (reserve 5). All three compute the same shape with different
  reserve constants. Candidate: `uicore.ListBodyRows(boxHeight,
  reserved int) int`. Three call sites — borderline. Skip unless
  a per-surface task surfaces a fourth consumer.

## Pass-end

Standard checklist applies. No ADR expected (binding facts
unchanged). STATUS rolls forward to Pass 9x.2 (chrome family
harvest: helppopover, statusbar, toast).
