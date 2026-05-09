# Table/form family — Pattern Catalog

Patterns harvested from two table/form-family libraries during Pass 9x.3:
`charmbracelet/huh` (form builder) and `evertras/bubble-table` (interactive
table). Companion to the Eval B verdicts at
`docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`.
Both verdicts were Keep + harvest: poplar's contacts/form and messagelist
stay hand-rolled; this doc catalogs what to keep in mind.

Each entry: pattern → upstream location (file:line) → why it's good →
poplar applicability (applied now / candidate for later / no — with
rationale).

Pass 9z consumes the "Top three" section across 9x.1–9x.3 catalogs to
assemble its adoption roadmap.

---

## `charmbracelet/huh`

---

### 1. Symmetric `Focused`/`Blurred` `FieldStyles` pair

**Where.** `theme.go:11–68` (`Theme` struct), `theme.go:86–122`
(`ThemeBase`), `field_input.go:367–376` (`activeStyles()`),
`field_select.go:546–555`, `field_confirm.go:224–233`.

**Why it's good.** Two type-identical structs — `Focused` and `Blurred` —
are the only distinction the theme author needs to think about. The
`activeStyles()` method returns the right one based on a boolean; all render
paths call it once and proceed. No scattered `if focused` guards at each
widget site.

**Applicability.** Candidate, settled as catalog only. `contacts.Styles` has
three state-keyed pairs (`FieldFocus`/`FieldBlur`, `KindOn`/`KindOff`,
`GroupLabel`/`Dim`), each serving a different semantic. Three state-keyed pairs are below the cardinality threshold where a `Focused`/`Blurred` sub-struct buys clarity over flat naming. The `activeStyles()` extraction
pattern applies at the row level already — see pattern §9 below.

---

### 2. `WithWidth`/`WithHeight` propagating into children

**Where.** `form.go:304–328`, `group.go:114–138`, `field_input.go:491–504`.

**Why it's good.** A single root call fans width to all children. Each leaf
field computes its own `frameSize` and `promptWidth` from the received value.
The parent does not need to know the internal layout of each field type.

**Applicability.** Applied — poplar's `Form.SetSize` (`form.go:152–171`)
computes `cw`/`inputW` at the form level and fans out to `textinput.Width`.
Same structural shape.

---

### 3. `activeStyles()` method centralizing focus/blur selection

**Where.** `field_input.go:367–376`, `field_select.go:546–555`,
`field_confirm.go:224–233`.

**Why it's good.** Every render path calls `i.activeStyles()` once; the
method returns a `*FieldStyles` pointer, and the caller uses it without
branching. The guard lives in one place.

**Applicability.** Applied inline at the row level. Each contact row
computes its own `inputStyle := f.styles.FieldBlur; if inputFocused {
inputStyle = f.styles.FieldFocus }` (`form.go:754–758`, `806–810`,
`734–737`). Per-row granularity is correct because each row has four
independent focus bits (input / cycler / star / minus). A single centralized
method returning one style cannot cover that; the inline form is the right
shape.

---

### 4. `WithTheme(*Theme)` propagation Form→Group→Field

**Where.** `form.go:271–281`, `group.go:91–102`, `field_input.go:482–488`.

**Why it's good.** One root call propagates via `selector.Range`; the
`if i.theme != nil { return i }` guard preserves per-field overrides. Theme
authors change one call, not N field constructors.

**Applicability.** Candidate post-1.0. Poplar themes are startup-resolved;
there is no live re-theme path today. A `WithStyles(Styles) Form` would help
theme tests but has no current trigger. Defer until themes become
user-selectable at runtime.

---

### 5. `selector.Selector[Field]` as teaching anti-pattern

**Where.** `form.go:65`, `group.go:21`.

**Why it's flagged.** The frozen slice is appropriate for huh because field
lists are static post-`NewForm`. For a form with dynamic rows (email/phone
add/remove), a frozen slice forces pre-allocate-max plus a `Skip()` flag on
invisible rows. The `Skip()` approach compounds: navigation skips must stay in sync with display; the index contract becomes fragile.

**Applicability.** No. Poplar's `focusList()` rebuild (`form.go:505–532`)
re-derives the ordered focusable-widget sequence on each mutation. It
eliminates stale-index bugs entirely by treating the sequence as a derived
value, not mutable state. The anti-pattern comparison is the catalog value:
frozen slice is the wrong default when the form's row count changes at
runtime.

---

### 6. Group `viewport` chrome — teaching anti-pattern

**Where.** `group.go:360–378`; `buildView()` at `group.go:323–327` feeds
`viewport.SetContent`.

**Why it's flagged.** The viewport absorbs overflow when a group has more
fields than fit the terminal; header/footer stay fixed. Necessary for
variable-depth wizard forms where total field count is unknown at render
time.

**Applicability.** No. Poplar's `Form.View()` (`form.go:649–661`) builds a
flat `[]string` and joins. The form fits within a fixed pane height by
construction (height is set by the parent layout, not by field count). A
viewport would add scroll complexity and an extra model to thread — overhead
without benefit.

---

### 7. `updateFieldMsg` Eval-binding fan-out — teaching anti-pattern

**Where.** `group.go:166–175`, `group.go:252–285`, `field_input.go:281–324`.

**Why it's flagged.** Dynamic title/description re-eval fires per Update via
a hash-keyed cache. The mechanism is correct when field labels or options
must change in response to other fields' values — a wizard that adjusts
downstream questions based on earlier answers.

**Applicability.** No. Poplar's form fields have static labels; only the
row structure is dynamic. The `focusList()` rebuild handles structural
dynamism without eval-binding machinery. This pattern applies only when
field *content* (not field *presence*) changes at runtime.

---

### 8. `Field.Skip()` hook — teaching anti-pattern

**Where.** `form.go:139–192` (`Field` interface, `Skip()` at line 161),
`group.go:200–213`, `group.go:218–248`.

**Why it's flagged.** Fields self-declare as non-interactive without being
removed; navigation centralizes the skip logic. Correct in combination with
a frozen-slice model where removal is not an option.

**Applicability.** No. The `focusList()` rebuild pattern supersedes `Skip()`
entirely. When a row is removed, it leaves the slice; the focus list
re-derives from current state. No stale `Skip()` flag can drift out of sync.
`Skip()` solves a problem `focusList()` doesn't have.

---

### 9. `FieldPosition` context injection

**Where.** `form.go:195–212`, `field_input.go:512–518`.

**Why it's good.** Fields receive their position (first / middle / last /
only) and self-configure navigation hints — "Enter to continue" vs "Enter to
complete". No parent needs to special-case the last field.

**Applicability.** No. Poplar's form uses `key.Binding` at the App and tab
level; the footer is a static key-hint string, not a per-field nav prompt.
No current consumer.

---

### 10. `TextInputStyles` sub-struct grouping

**Where.** `theme.go:71–77`, `field_input.go:385–390`,
`field_select.go:751–755`.

**Why it's good.** Bundles the five styles flowing into `bubbles/textinput`'s
internal API (Cursor / CursorText / Placeholder / Prompt / Text) in one
assignment block per theme.

**Applicability.** No. Poplar's form does not assign into `textinput`'s
internal style slots — it wraps the already-rendered `ti.View()` in
`FieldFocus`/`FieldBlur` lipgloss styles (`form.go:737–740`, `754–758`). No
coupling between theme assignment and textinput's internal API. No benefit.

---

### 11. `selector.Range` iteration over field list

**Where.** `form.go:271–281`, `group.go:91–102`.

**Why it's good.** A typed Range helper on the generic selector encapsulates
the iteration pattern and keeps the propagation code terse.

**Applicability.** Candidate, settled as catalog only. Poplar's form row
slice is a plain `[]formRow`; range over it is idiomatic Go. A custom Range
wrapper at current scale is ceremony.

---

### Diff candidates for Task 5 (contacts/form)

**0 structural diffs.** Every huh pattern either matches what poplar already
does (P2 `SetSize` cascade, P3 per-row `activeStyles` inline) or is
inapplicable because poplar's form is dynamic-row (frozen-slice patterns P5,
P8 don't fit) or because poplar's form fits a fixed pane by construction
(P6 viewport). The `TextInputStyles` sub-struct (P10) has no consumer
because poplar wraps rendered `ti.View()` output rather than setting
textinput's internal styles. `WithTheme` propagation (P4) is a post-1.0
candidate. Nothing clears the bar for a Task 5 structural diff.

---

## `evertras/bubble-table`

---

### 1. `RowData map[string]any` indirection — teaching anti-pattern

**Where.** `row.go:15–18`, `renderRowColumnData` (`row.go:~56–130`).

**Why it's flagged.** Decouples row construction from column schema; hidden
metadata survives sort/filter. For a generic table widget that cannot know
its row schema at compile time, this is the right tradeoff.

**Applicability.** No. Messagelist's `displayRow` is a typed struct
(`model.go:66–74`); contacts/list holds typed `contacts.Contact` values.
The map adds runtime key-lookup and type assertions (`fmt.Sprintf("%v",
entry)`) for no benefit on a uniform, compile-time-known schema. Typed
structs are cheaper and catch schema drift at compile time.

---

### 2. `StyledCell{Data, Style, StyleFunc}` per-cell override — teaching anti-pattern

**Where.** `cell.go` (full type), dispatched in `renderRowColumnData`
(`row.go:~87–103`).

**Why it's flagged.** Allows one cell per row to carry a different style
without a per-row permutation. Useful when rows are heterogeneous and cells
can be individually highlighted (e.g., an error cell rendered red
regardless of row state).

**Applicability.** No. Messagelist applies row-level background via
`uicore.ApplyBg` and per-segment styles from a fixed `Styles` struct. Both
surfaces have uniform schema; per-cell allocation adds no value and a
non-trivial per-frame cost.

---

### 3. `WithRowStyleFunc` callback — settled as catalog only

**Where.** `options.go:28–34`, consumed in `renderRow` (`row.go:~139–158`).

**Why it's good.** Externalizes row-level conditional styling from the table
model; keeps the model free of domain predicates (unread, flagged, selected,
etc.). The callback receives a `RowStyleFuncInput` with row data and state.

**Applicability.** Candidate, settled as catalog only. Messagelist owns its
predicates (`isUnread`, `isSelected`, `marked`) directly in `renderRow`
(`model.go:795–865`). Caller and model share a compilation unit. There is no seam benefit from externalizing the predicate into a closure. The pattern
matters when the table is truly generic and the caller is a different
package. Not poplar's shape.

---

### 4. `lipgloss.JoinHorizontal` in row assembly — canonical ADR-0084 anti-pattern

**Where.** `renderRowData` (`row.go:~243`, final row join).

**Why it's flagged.** `lipgloss.JoinHorizontal` calls `lipgloss.Width`
internally on each cell string. `lipgloss.Width` undercounts SPUA-A glyphs
(Private Use Area icons) by `(spuaCellWidth - 1)` per glyph, producing
1-cell misalignment per icon-bearing cell. On a Nerd Font terminal with
double-wide icons, every row with an icon column shifts right.

**Applicability.** No — poplar's row assembly uses per-segment string
concatenation with `ansix.Width` for icon-bearing strings and
`uicore.ApplyBg` for background application. `JoinHorizontal` is forbidden
when `spuaCellWidth != 1` (ADR-0084). This is the library-side instance of
the exact anti-pattern poplar codified the ADR against.

---

### 5. `lipgloss.Width` on cell strings

**Where.** `row.go:~210` (overflow check), `view.go:~47`, `view.go:~50`
(footer-width computation).

**Why it's flagged.** On `row.go:~210` the width check is used to decide
whether a cell value overflows the column budget. For icon-bearing cells
this produces a wrong budget — the cell appears shorter than its rendered
width on a Nerd Font terminal.

**Applicability.** No. Poplar uses `ansix.Width` for icon-bearing strings
(documented in ADR-0181). `lipgloss.Width` is permitted only on strings
whose content is known to contain no SPUA-A glyphs. The `view.go` footer-
width sites are lower-risk (operating on full assembled row strings) but
are still potentially wrong when icons are present.

---

### 6. `NewFlexColumn` / `updateColumnWidths` GCD-based flex math

**Where.** `column.go:32–43`, `dimensions.go:22–65`.

**Why it's good.** Proportional flex absorption across multiple flex
columns; GCD prevents int-div drift when many columns share the residual
width.

**Applicability.** No. Messagelist has one flex column (subject, residual
after fixed-width columns); contacts/list has one flex column (meta,
residual). Single-flex-column shape means GCD math is overengineering. The
residual is just `totalWidth - sumOfFixedWidths`.

---

### 7. `WithBaseStyle(lipgloss.Style)` global theme injection

**Where.** `options.go:168–172`, consumed via
`cellStyle.Inherit(m.baseStyle)` in `renderRowColumnData` (`row.go:57`).

**Why it's good.** One call themes the entire table. New columns inherit
automatically.

**Applicability.** No. Messagelist's `Styles` struct populated by
`NewStyles(*theme.CompiledTheme)` is the equivalent — with named semantic
slots. Replacing it with one opaque base collapses the named-slot semantics
that `docs/poplar/styling.md` and the invariants depend on.

---

### 8. `style.Copy()` proliferation — teaching anti-pattern

**Where.** `row.go:57`: `cellStyle := rowStyle.Copy().Inherit(column.style).Inherit(m.baseStyle)`.

**Why it's flagged.** Per-cell style allocation: every cell in every row
allocates on every frame. `lipgloss.Style` is a struct with a pointer to a
`css.Rules` map; `.Copy()` allocates a new map per call.

**Applicability.** No. Poplar pre-constructs `Styles` at startup. `uicore.ApplyBg` builds a concrete style from pre-constructed pieces without intermediate copies. The per-frame allocation pattern is avoided by design.

---

### 9. `WithTargetWidth` + `recalculateWidth`

**Where.** `options.go:150–157`, `dimensions.go:4–20`.

**Why it's good.** One call adapts all columns to terminal resize.

**Applicability.** No. Poplar drives layout via `SetLayout(uicore.LayoutMode)`
per ADR-0109's three-tier responsive model. `SetSize` fans out to children.
A single target-width integer is less expressive than the layout mode + size
pair.

---

### 10. `SelectableRows` check-column + `SelectedRows()`

**Where.** `options.go:88–108`, `row.go:Selected()`.

**Why it's good.** Visible checkbox column; typed selection retrieval.

**Applicability.** No. Messagelist uses a UID-keyed `marked map[mail.UID]struct{}`
plus `ActionTargets()`. Lighter; no display-column intrusion; selection
semantics are thread-aware in ways a prepended check column is not.

---

### 11. `WithFilterFunc(FilterFunc)` / `WithFuzzyFilter()`

**Where.** `options.go:263–278`.

**Why it's good.** Arbitrary filter predicates on row data.

**Applicability.** No. Messagelist's `filterBuckets` filters at thread level
(`model.go`), not row level. A row-level `FilterFunc` would break thread
semantics by potentially hiding thread-root rows while showing replies.

---

### Diff candidates for Task 5 (messagelist)

**0 structural diffs.** All eleven bubble-table patterns are either
inapplicable (typed struct vs `RowData` map, single-flex column, UID-keyed
selection vs check column, thread-level filter) or active anti-patterns
(`JoinHorizontal`, `lipgloss.Width`, `style.Copy()` proliferation). No
pattern improves messagelist's current shape.

---

### Diff candidates for Task 5 (contacts/list)

**0 structural diffs.** Contacts/list has a static four-column schema, a
single flex column, and typed `contacts.Contact` rows. None of the table
patterns (dynamic schema, multi-flex, per-cell override, check column)
apply. The row assembly and width math are already correct per ADR-0084 and
ADR-0181.

---

## Top three ways their code beats ours

No pattern across these two libraries reaches the bar.

The honest tally: every huh pattern that could apply either already matches
what poplar does (`SetSize` cascade, per-row `activeStyles` inline) or is
explicitly wrong for a dynamic-row form (`selector.Selector[Field]` frozen
slice, `Skip()` hook, `Group` viewport). Every bubble-table pattern is
either inapplicable to typed-struct row schemas or is a named anti-pattern
(ADR-0084's `JoinHorizontal`, ADR-0181's `lipgloss.Width`).

The contacts/form and messagelist surfaces are full-pane, hand-rolled
components with typed models. Both libraries' design strengths — generic
schema decoupling, configurable row callbacks, flex column math — address
the cost of being a reusable widget with an unknown consumer. Poplar's
surfaces are not that; the consumer-fit gap is the reason the verdicts were
Keep and not Replace.

No pattern from this catalog feeds Pass 9z's adoption roadmap. The
`bubbles/list` and chrome-family catalogs (9x.1, 9x.2) carry the one honest
recommendation: the `shouldAddItem` loop shape from `bubbles/help`.

---

## What did NOT survive the harvest

- **huh `selector.Selector[Field]` frozen slice**: wrong shape for
  dynamic-row models; forces pre-allocate-max plus `Skip()` flags to
  simulate removal.
- **huh group `viewport` chrome**: unnecessary when the form fits a
  fixed pane by construction; adds scroll complexity with no benefit.
- **huh `updateFieldMsg` Eval-binding fan-out**: only pays when field
  *content* (labels, options) changes in response to user input; poplar's
  form labels are static.
- **huh `Field.Skip()` hook**: superseded by `focusList()` rebuild;
  `Skip()` solves a stale-index problem the rebuild pattern doesn't have.
- **huh `WithTheme` propagation**: post-1.0 candidate; themes are
  startup-resolved and there is no live re-theme trigger today.
- **huh `TextInputStyles` sub-struct**: no consumer; poplar wraps
  `ti.View()` output in lipgloss, not textinput's internal style slots.
- **huh `FieldPosition` context injection**: no consumer; footer is a
  static key-hint string.
- **bubble-table `lipgloss.JoinHorizontal` row assembly**: canonical
  ADR-0084 anti-pattern; undercounts SPUA-A glyphs on icon-bearing cells.
- **bubble-table `lipgloss.Width` on icon-bearing cell strings**: wrong on
  Nerd Font terminals; poplar uses `ansix.Width` per ADR-0181.
- **bubble-table `RowData map[string]any` indirection**: type-erased;
  runtime key-lookup and `fmt.Sprintf("%v", entry)` per cell per frame;
  typed structs are correct.
- **bubble-table `StyledCell` per-cell override**: per-frame allocation;
  no heterogeneous cell schema in either surface.
- **bubble-table `WithRowStyleFunc` callback**: no seam benefit when
  caller and model share a compilation unit.
- **bubble-table `NewFlexColumn` GCD flex math**: both surfaces have one
  flex column; residual is a one-liner.
- **bubble-table `WithBaseStyle` global theme**: collapses named semantic
  slots that invariants and `styling.md` depend on.
- **bubble-table `style.Copy()` proliferation**: per-cell per-frame
  allocation; poplar pre-constructs styles at startup.
- **bubble-table `SelectableRows` check-column**: UID-keyed `marked` map
  is lighter and thread-aware.
- **bubble-table `WithFilterFunc`/`WithFuzzyFilter`**: row-level filter
  breaks thread semantics.
- **bubble-table `WithMultiline` wordwrap**: neither surface renders
  multi-line rows.
- **bubble-table `WithHorizontalFreezeColumnCount`**: neither surface has
  horizontal scroll.

---

## Cross-cutting code candidates

The `uicore.ListBodyRows` extraction candidate from 9x.1 remains at three
call sites (movepicker `visibleRows`, reader `visibleLinkRows`,
compose/attachpicker `viewportRows`) — still below the threshold for
extraction. The question is whether contacts/list or messagelist adds a
fourth modal-list consumer.

They do not. Both are full-pane lists, not modal lists. Messagelist's
height derives from `ComputeLayout` applied to the app's available space;
contacts/list computes content height the same way — `ComputeLayout` applied to available space. Neither uses the
`boxHeight - reservedChrome` subtraction pattern that the three modal lists
share. Adding them as "consumers" of a `ListBodyRows` helper would be a
false unification — the subtraction constant differs and the layout path
differs.
