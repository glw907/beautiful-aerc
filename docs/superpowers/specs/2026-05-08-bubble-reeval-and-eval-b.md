# Bubble Re-eval (post-ansix) + Eval B — Design

A single triage pass that revisits Eval A's verdicts under ansix
and runs Eval B on its four candidates. Output feeds Pass 9z's
consolidation roadmap.

## Context

Pass 9w.1 extracted `internal/ansix/` (ADR-0181) as the
icon-aware width primitive over `charmbracelet/x/ansi`. Eval A
ran pre-ansix; several "Keep + harvest" verdicts cited the
`lipgloss.JoinHorizontal` ban (ADR-0084) or `lipgloss.Width`
miscount under SPUA-A icon mode. ansix changes the seam shape on
poplar's side. It does not change the seam inside upstream
libraries. Some Eval A verdicts may flip; some won't.

Eval B was scoped in the original adoption design
(`2026-05-08-bubble-adoption-design.md`) but never executed. The
post-ansix re-eval is the right moment to land it — same context
load, same rubric, same output shape.

## Output

One spec at
`docs/superpowers/specs/2026-05-08-bubble-reeval-and-eval-b.md`
(this document evolves into the eval result during execution).
No ADR. Eval A's existing spec
(`2026-05-08-bubble-eval-a-strong-matches.md`) stays as the
pre-ansix baseline; the new spec supersedes its verdicts only
where ansix flips them.

## Candidate set

Nine candidates total.

### Eval A revisit (5)

1. `rmhubbert/bubbletea-overlay`
2. `bubbles/help`
3. `daltonsw/bubbleup`
4. `charmbracelet/huh`
5. `evertras/bubble-table`

### Eval B (4)

6. `bubbles/list` × movepicker / linkpicker / attachpicker
   — batched. One library, three picker sites with similar
   delegate shape. Eval treats them as one verdict with
   per-site notes.
7. `bubbles/list` vs `treilik/bubblelister` for
   `compose.Dropdown`. Two-library compare for the autocomplete
   surface (small list, dynamic suggestions).
8. `bubbles/list` for sidebar folder column. T9 contacts
   groups are out of scope (they are a different shape and
   already settled hand-rolled).
9. `knipferrc/teacup` statusbar for `internal/ui/status_bar`.

## Rubric

Same six dimensions as Eval A:

1. **Feature parity** — does the bubble cover what poplar's
   hand-roll does today, or are there gaps that matter?
2. **Customization seams** — can themes, modifier-free
   single-key bindings, and domain state wire in without
   forking?
3. **Theming integration** — does the bubble accept
   `lipgloss.Style` injection or own its own colors?
4. **Maintenance signal** — last commit, version cadence,
   releases in the past twelve months. Not a veto unless dead.
5. **Code delta estimate** — rough LOC removed from poplar vs
   LOC added (shim + integration). A 50→500 swap is a tell.
6. **License** — MIT, BSD, or Apache only. Hard veto otherwise.

The rubric is the evidence; the verdict rests on the core
question: *does adopting the community bubble make poplar
better?* (At minimum, not worse.)

## Per-candidate process

For each candidate:

1. Read the actual library source — specifically the render
   path that previously gated adoption (typically the function
   calling `lipgloss.JoinHorizontal` or `lipgloss.Width`).
2. Read poplar's current consumer carefully.
3. Decide: does ansix change the verdict? For Eval B, decide
   from scratch.
4. Write the candidate section: lead with the core question
   answered concretely, then rubric evidence, then verdict
   (**Adopt / Adopt-with-fork / Keep + harvest**) with a
   one-line rationale.
5. Close with **Interacts with:** which other candidates'
   swaps this one depends on, blocks, or simplifies.

Per-candidate prose: 300–400 words for Eval A revisits and
single-library Eval B candidates; up to 600 for the
list-vs-bubblelister compare.

## ansix capability summary

`internal/ansix/` exports seven symbols. `SetSPUACellWidth(w int)`
records the per-glyph cell count measured at startup (1 or 2);
`SPUACellWidth()` reads it back. `Width(s string) int` is the
primary measurement primitive: it calls `lipgloss.Width` then adds
`(spuaCellWidth-1) * SpuaCount(s)` to correct for the extra cells
SPUA-A glyphs occupy when `spuaCellWidth == 2`. `SpuaCount(s int)`
counts SPUA-A runes with a fast ASCII-bail path. `Truncate(s string,
n int) string` is the SPUA-aware analogue of `ansi.Truncate`:
because `ansi.Truncate` uses runewidth internally and undercounts
SPUA-A glyphs by `(spuaCellWidth-1)` cells each, `Truncate`
iteratively tightens the runewidth budget until `Width(result) <= n`.
`TruncateEllipsis(s, n)` delegates to `Truncate` with a one-cell
ellipsis reserve; a one-cell budget returns bare `"…"`.
`PadOrTruncate(s, n)` fits a string to exactly n cells, padding with
spaces or truncating without an ellipsis. These replace the former
`uicore.Display*` family (ADR-0181) and are the active width layer
across sixteen call sites in `account`, `compose`, `messagelist`,
`reader`, `sidebar`, and their tests.

**The seam.** ansix sits at poplar's call sites. It corrects width
math in code poplar owns. It cannot reach inside upstream libraries:
when `lipgloss.JoinHorizontal` or a bubbles render function calls
`lipgloss.Width` on its own strings, those measurements bypass
ansix entirely. The ADR-0084 ban on `JoinHorizontal` under
`spuaCellWidth != 1` remains in force for exactly this reason.

**Implication for the re-eval.** A prior "Keep + harvest" verdict
that cited poplar's own width-math as the blocker may flip to
Adopt under ansix — poplar can now pass correctly-measured
strings into the library. A verdict that cited the library's
*internal* width calls (its own `lipgloss.Width` inside its render
path) is unaffected. ansix does not help there; only a fork of
`charmbracelet/x/ansi` or `lipgloss` would. That binary is the
fork-vs-accept section's subject.

## Fork-vs-accept call

A closing section names every **(b)** candidate — bubbles whose
render path calls `lipgloss.Width` internally, where ansix in
poplar's call sites doesn't help because the library does its
own width math. From the queued STATUS entry these are at least
`bubbles/help`, `bubble-table`, and `glamour` (the third is
speculative — not currently a consumer; included only if the
fork would unlock its future use).

The decision is binary:

- **Fork** — `go.mod replace` for either `charmbracelet/x/ansi`
  or `charmbracelet/lipgloss`, swapping `ansi.StringWidth` /
  `lipgloss.Width` for an ansix-equivalent. Permanent rebase
  cost. Explicitly supersedes ADR-0002 and ADR-0075. Unlocks
  every gated bubble.
- **Accept** — these bubbles stay hand-rolled. The fork cost
  exceeds the adoption value.

Decision factors to weight:

- **Breadth of bubbles unlocked** by the fork. If only one
  candidate flips to Adopt under fork, the math favors accept.
- **Upstream PR path.** Memory note
  `reference_displaywidth_contribution_patterns.md` records
  PR-first, Copilot-reviewed contribution culture for
  `clipperhouse/displaywidth`. If x/ansi or lipgloss accepts
  a configurable width hook upstream, the fork becomes
  temporary instead of permanent. Worth checking before
  defaulting to accept.
- **Maintenance burden** of the rebase. x/ansi cadence is
  weekly; lipgloss is monthly. Both are charmbracelet-owned
  with stable APIs.
- **What "accept" forecloses.** Any future bubble whose render
  path uses `lipgloss.Width` internally is ineligible by
  default. Worth naming the cost concretely.

The verdict is one of two strings, with a paragraph of
rationale and any conditional follow-ups (e.g., "accept
unless upstream PR lands by Pass 9z").

## Task budget

11 tasks:

1. Re-read ansix capabilities — what `ansix.Width` /
   `ansix.Truncate` do, where they're used now, why they
   diverge from the upstream `ansi.StringWidth`.
2. Re-eval `rmhubbert/bubbletea-overlay`.
3. Re-eval `bubbles/help`.
4. Re-eval `daltonsw/bubbleup`.
5. Re-eval `charmbracelet/huh`.
6. Re-eval `evertras/bubble-table`.
7. Eval `bubbles/list` × picker sites (batched).
8. Eval `bubbles/list` vs `treilik/bubblelister` for compose
   dropdown.
9. Eval `bubbles/list` for sidebar folder column.
10. Eval `knipferrc/teacup` statusbar.
11. Fork-vs-accept synthesis.

Fits the 8–12 task budget.

## Out of scope

- **Eval C (lesson harvests)** — different shape (concrete-diff
  requirement against kept-custom code); stays as Pass 9y.
- **Glamour full eval** — not currently a poplar consumer. The
  fork-vs-accept call names it only if the fork option would
  unlock its future use.
- **Code changes** — research pass; no `internal/` edits.
- **ADRs** — eval produces no architectural decisions; only
  Pass 9z's consolidation roadmap and the subsequent swap
  passes write ADRs.

## Risks

**The re-eval finds nothing flips.** Possible. ansix's call
sites are inside poplar; library-internal width calls are
unaffected. If every Eval A "Keep" verdict survives and every
Eval B candidate also fails, the pass output is "fork-vs-accept
is the only meaningful lever." The fork-vs-accept synthesis
becomes load-bearing. This is fine — the pass still concludes
the bubble effort's open question.

**The fork option requires upstream investigation.** Checking
whether x/ansi or lipgloss would accept a width-hook PR is a
real research task, not a paragraph guess. Budgeted inside
task 11.

**Eval B `bubbles/list` for sidebar may bog down.** Sidebar is
custom for principled reasons (T9 contacts groups, three fixed
folder groups, search shelf composition). The eval may end
"Keep + harvest" quickly. Worth not over-investing.

## Next step

Invoke `writing-plans` to produce the plan doc for Pass 9w.2.
The plan turns the 11 tasks into concrete steps and feeds into
pass execution per the poplar-pass starter prompt.

## `rmhubbert/bubbletea-overlay`

**Does this make poplar better?** No. The library's `Composite`
function is the same algorithm as poplar's `PlaceOverlay` — both
derive from the Superfile `overplace.go` origin and both reach
for `charmbracelet/x/ansi` for cell-width measurement. Post-ansix,
the seam question is whether ansix unlocks something here it
couldn't before. It doesn't. The prior "Keep + harvest" verdict
rested on structural grounds, not width-math grounds: `Composite`
handles exactly two layers; the nine-level `if IsOpen()` chain in
`App.View` is the cascade, and no two-layer compositor touches it.
ansix is irrelevant to that gap. Adopting the library would add a
dependency, replace ~50 LOC with ~50 equivalent LOC, and change
nothing about App behavior.

**Feature parity:** `Composite(fg, bg, xPos, yPos, xOff, yOff)`
covers the same cell-compositing operation as `PlaceOverlay(x, y,
fg, bg)` with a different argument order and an added `Position`
enum (Top/Right/Bottom/Left/Center). The library's `Model` wraps
two `Viewable` values and calls `Composite` in `View()`; `Update`
is a no-op pass-through. It does not implement cascade ordering,
mutual-exclusion logic, or `DimANSI` dimming — all of which stay
in `App` regardless. There is no feature gap in the library; the
coverage is exactly coextensive with the hand-rolled compositor.

**Customization seams:** `Composite` works on pre-rendered ANSI
strings, the same as `PlaceOverlay`. No style injection, no border
hooks, no theme parameters. `Model.Foreground` and
`Model.Background` accept any `Viewable`, so domain state would
thread naturally — but that is true of `PlaceOverlay` too, via
direct argument passing.

**Theming integration:** None. The library owns no colors and
applies no styles. Theming is entirely the caller's responsibility
in both implementations.

**Maintenance signal:** Last commit 2026-04-20 (dependabot bump of
`charmbracelet/x/ansi` to 0.11.7). Release cadence: v0.6.7
(2026-04-20), v0.6.6 (2026-03-18), v0.6.5 (2026-02-09), v0.6.4
(2026-01-19) — roughly one release per six weeks over the past
twelve months. Actively maintained against current Charm versions.
The README notes that bubbletea v2 users should prefer built-in
compositing; poplar is on v1, so no compatibility concern.

**Code delta estimate:** `overlay.go` is 76 LOC; `modal_shell.go`
is 66 LOC — 142 total in the uicore overlay surface. `PlaceOverlay`
accounts for roughly 50 of those. Replacing it with `overlay.Composite`
removes ~50 LOC, adds one `go.mod` entry, and leaves `DimANSI`,
`ModalShell`, and the entire `App.View` cascade untouched. Net delta:
roughly neutral on LOC, negative on dependency count.

**License:** MIT License, Copyright (c) 2024 Hubby. Clear.

**Verdict:** Keep + harvest

**Rationale (one line):** The library is the same algorithm poplar
already owns; ansix changes nothing about the structural blocker
(two-layer compositor vs nine-level cascade), and the adoption
delta is a new dependency for zero behavior change.

**Interacts with:** None. The overlay compositor is a leaf utility
with no downstream dependency on other evaluated bubbles. The
`Position` enum vocabulary (Top/Center/Bottom/Left/Right) is worth
mirroring in any future overlay whose placement becomes semantic
rather than fixed-coordinate; nothing in the current cascade
qualifies.

## `bubbles/help`

**Does this make poplar better?** No. The two blockers that drove the original "Keep + harvest" verdict are both structural — ansix does not touch either. `FullHelpView` calls `lipgloss.JoinHorizontal` twice (building each column, then joining all columns) and `lipgloss.Width` once per column inside the library's own render path. ansix sits at poplar's call sites; it cannot correct measurements the library makes on its own strings. The ADR-0084 ban stays in force. Beyond the width blocker, the library has no concept of a "wired" row: `bubbles/help` suppresses rows by disabling a `key.Binding`, but poplar's popover renders unwired rows dim throughout — a positive affordance advertising planned bindings that has no analogue in the upstream model. Both gaps would require a fork to close.

**Feature parity:** `bubbles/help.Model` renders two modes — `ShortHelpView` (one line, truncated to width) and `FullHelpView` (flat columns, one `[]key.Binding` slice per column). Poplar's popover renders three named groups per row in a two-row grid, then a standalone Go To grid in a 3×2 tile, then a bottom hint line outside any group. `KeyMap` (`ShortHelp() []key.Binding`, `FullHelp() [][]key.Binding`) provides no named group titles. The layout contract is fundamentally different: a list of columns vs a named-group grid with a hint line. Mapping poplar's `bindingGroup` slices onto `[][]key.Binding` would produce an un-named column layout and lose the bottom hint row entirely.

**Customization seams:** `Styles` exposes six fields (`ShortKey`, `ShortDesc`, `ShortSeparator`, `FullKey`, `FullDesc`, `FullSeparator`, `Ellipsis`). These can be injected. No hook for per-row conditional dimming; no title rendering seam. Key bindings are `key.Binding` values via the `KeyMap` interface — compatible with the existing `key.Binding` declarations in `helppopover`, though poplar's `bindingRow` carries `wired bool` alongside the key string, not a `key.Binding`.

**Theming integration:** Full style injection via the `Styles` struct, same pattern as other Charm bubbles. Usable as-is with a `NewStyles(*theme.CompiledTheme)` constructor, for the styles it exposes.

**Maintenance signal:** v1.0.0 shipped 2026-02-10; v2.0.0 shipped 2026-02-24; v2.1.0 shipped 2026-03-26 — three releases in twelve months, active cadence. Poplar pins v1.0.0. The v2 line exists but the upgrade is not required; v1 is stable and the help API is unlikely to break.

**Code delta estimate:** `helppopover/model.go` is 380 LOC; `helppopover/styles.go` is 25 LOC. Adopting `bubbles/help` would not replace most of that: the `Box` + `Position` + cache machinery, the `bindingGroup` table, `joinColumnsRow`, `renderGotoGrid`, and the bottom hint line are all poplar-domain logic with no upstream analogue. The net replacement is effectively zero — any integration would wrap `bubbles/help.ShortHelpView` in the hint bar only, a use case poplar does not currently have.

**License:** MIT (charmbracelet/bubbles). Clear.

**Verdict:** Keep + harvest

**Rationale (one line):** Both blockers survive ansix — internal `JoinHorizontal` in `FullHelpView` is unreachable from poplar's call sites, and the `wired` dim affordance has no upstream analogue — leaving poplar's hand-rolled layout as the only shape that fits the grid + hint contract.

**Interacts with:** Fork-vs-accept (Task 11). `bubbles/help` is a (b)-list candidate — its blocker is the library's internal `lipgloss.Width` + `JoinHorizontal`, not poplar's own width math. It belongs on the fork-vs-accept list alongside `bubble-table`. If the fork option is accepted, `FullHelpView`'s column layout becomes width-safe, but the named-group-grid contract gap remains; the verdict would likely soften to "Adopt-with-fork for hint bar only, keep grid hand-rolled."

## `daltonsw/bubbleup`

**Does this make poplar better?** No. Bubbleup is a category-styled animated notification overlay (Info/Warn/Error/Debug, color lerp from black to a target hue over ~6 ticks at 100 ms). Poplar's toast is a different thing: a triage-confirmation bar that carries an `op` enum, an affected message count, an optional destination name, and an inverse `tea.Cmd` (the undo action). The two primitives do not share a domain. ansix is irrelevant here — bubbleup's color lerp uses `go-colorful.BlendLab` and `lipgloss.Width` only for its own dynamic-width clamp; there is no SPUA gating and no call site poplar could fix with `ansix.Width`. The domain gap is the entire verdict.

**Feature parity:** Bubbleup renders a rounded-border floating overlay at six configurable positions (Top/Bottom × Left/Center/Right) with a fixed or dynamic width, dismissible via `Esc`. Poplar's toast renders as a plain one-row bar inlined into App's chrome-banner slot (shared with the error banner); it has no border, no position enum, no animation, and no dismiss key — it expires on a `tea.Tick` deadline and clears on any folder change or error. The overlay approach would break the existing chrome-banner contract and require a complete rearchitecture of `chromeBannerRow`.

**Customization seams:** `AlertDefinition{Key, ForeColor, Prefix, Style}` registers new alert types. `WithPosition`, `WithMinWidth`, `WithUnicodePrefix`, `WithAllowEscToClose` are functional-options-style mutators. No seam for a countdown timer, no seam for an undo action, no seam for op-keyed verb text. The customization surface covers visual style, not payload.

**Theming integration:** Bubbleup owns its color pipeline: hex strings via `go-colorful` parsed at `RegisterNewAlertType` time, blended on every tick. Colors are not `lipgloss.AdaptiveColor` values and do not reference poplar's palette. Wiring into `theme.CompiledTheme` would require replacing the lerp target and background with palette slots and rebuilding the interpolation per tick — there is no injection seam for this.

**Maintenance signal:** 41 stars, 3 forks, created 2024-10-01, last pushed 2026-05-01. MIT license. Active but small; the `go.mod` still pins `charmbracelet/bubbletea v1.1.1` and `lipgloss v0.13.0` while poplar tracks the current release line. There are deprecated constants in `alert.go` (`InfoUniPrefix`, `WarningUniPrefix`, etc.), suggesting the API is still settling.

**Code delta estimate:** Adoption would require replacing `pendingAction` + `renderToast` + the `toastExpireMsg` tick with bubbleup's `AlertModel`, inventing a new `AlertDefinition` for each triage op, mapping the op-verb-count-dest tuple into a flat string before passing to `NewAlertCmd`, and rearchitecting `chromeBannerRow` to overlay rather than inline. The current `toast.go` is 128 lines and handles exactly the domain; bubbleup would add ~300 LOC of animation machinery in exchange for eliminating it while making the feature harder to extend.

**License:** MIT (`go.dalton.dog/bubbleup`). Clear.

**Verdict:** Keep + harvest

**Rationale (one line):** Domain mismatch — bubbleup is a category-styled animated notification overlay, not a triage-state bar with undo payload; ansix does not touch the gap, and nothing in the library's render or theming model maps onto `pendingAction`.

**Interacts with:** None. Toast is a leaf feature in `internal/ui/toast.go` with no dependencies on other evaluated libraries. The `position` vocabulary bubbleup introduces (Top/Bottom × Left/Center/Right) is conceptually reusable for any future floating overlay, but no current overlay in poplar's cascade requires positional semantics beyond the fixed `centerOverlay` placement.

## `charmbracelet/huh`

**Does this make poplar better?** No. Two structural blockers survive the ansix re-eval unchanged. First, huh always renders its own chrome — title bar, group borders, help row, field separator — in its `View()` path; there is no body-only rendering mode. Poplar's `Form` serves two contexts: `fromPopover=true` wraps body rows in a `ModalShell` box, `fromPopover=false` emits raw rows so the Contacts-mode frame supplies the borders. huh's chrome is baked into `Group.View()` via the `viewport` and `titleFooterHeight()` layout; no seam exposes body rows alone. Second, `NewGroup(fields ...Field)` freezes the field slice at construction. Poplar's `focusList()` rebuilds the ordered widget list on every mutation — when the user adds an email row, four new widgets (input, cycler, star, minus) are inserted mid-sequence and `applyFocus()` re-anchors. huh has no equivalent of a dynamic field list; the `selector.Selector[Field]` it wraps is fixed once the group is created. ansix corrects width math at poplar's call sites; neither blocker touches width math, so ansix changes nothing here. Both blockers would require forking huh to close.

**Feature parity:** huh provides `Input` (wraps `bubbles/textinput`), `Text` (wraps `bubbles/textarea`), `Select`, `MultiSelect`, `Confirm`, and `FilePicker` field types — adequate primitives for a contact form. What is missing is the composite `(input, cycler, ★, −)` quartet that poplar renders per email/phone row. That quartet is not a field type; it is a custom render unit with four focus stops and inline row-mutation semantics (add, promote to primary, remove). No huh field type models it, and the `Field` interface (`Init`, `Update`, `View`, `Blur`, `Focus`, `Error`, `Skip`, `WithTheme`, `WithKeyMap`, `WithWidth`, `WithHeight`, `WithPosition`) does not expose the hooks needed to implement it without owning the entire group rendering path anyway.

**Customization seams:** `WithTheme(*Theme)` accepts a `Theme` struct built entirely from `lipgloss.Style` fields — `FieldStyles.Title`, `.Description`, `.TextInput`, `.Cursor`, and so on across `Focused` and `Blurred` variants. This is the strongest seam in huh and maps cleanly onto poplar's `NewStyles(*theme.CompiledTheme)` pattern. Key bindings are set via `WithKeyMap(*KeyMap)`; the `KeyMap` struct uses `key.Binding` values, compatible with poplar's existing binding declarations. Neither seam helps with the dynamic-row or chrome-mode blockers.

**Theming integration:** Full `lipgloss.Style` injection throughout. `WithTheme` propagates from `Form` to `Group` to each `Field`. No hardcoded hex values; the built-in themes (`ThemeCharm`, `ThemeDracula`, `ThemeCatppuccin`, `ThemeBase16`) are all replaceable. This is the one dimension where huh is genuinely strong and worth harvesting as a design reference — per-field blurred/focused style pairs are the right pattern for any multi-field form.

**Maintenance signal:** huh is charmbracelet-maintained, 6 857 stars, 242 forks. Recent cadence: v0.8.0 (2025-10-14), v2.0.0 (2026-03-09), v2.0.3 (2026-03-10), last push 2026-04-22. The v2 jump (bubbletea v2 + accessible mode overhaul) lands a significant API break; poplar is on bubbletea v1, so v2 is not a viable target. v0.x / v1.x is the last compatible line. Actively maintained but the v2 split adds adoption risk.

**Code delta estimate:** `form.go` in poplar is 898 LOC. huh would not reduce that meaningfully: the `(input, cycler, ★, −)` quartet, `focusList()` rebuild, `applyFocus()`, `fromPopover` branch, and the `contactRow`/`phoneContactRow` render functions all have no huh analogue. Any integration would wrap huh for the static fields only (kind toggle, name, note, save-to) while keeping the dynamic email/phone section hand-rolled — a chimera that buys nothing over the current single-model design.

**License:** MIT (charmbracelet/huh). Clear.

**Verdict:** Keep + harvest

**Rationale (one line):** Both blockers are architectural — no body-only render mode and fields fixed at group construction — and ansix does not touch either; the blurred/focused `lipgloss.Style` pair pattern is the one thing worth mirroring in poplar's own `NewStyles` constructors.

**Interacts with:** None for the adoption question. The `fromPopover` dual-render pattern and the `focusList()` dynamic-rebuild are pre-1.0 architectural choices poplar owns; no other evaluated bubble depends on them. The Pass 14 first-run wizard is a separate question: a static, sequential form with fixed fields is exactly huh's strength, and the wizard shape may fit `NewGroup(fields...)` without triggering either blocker. That evaluation belongs in Pass 14's planning, not here.

## `evertras/bubble-table`

**Does this make poplar better?** No. The core render path calls `lipgloss.JoinHorizontal(lipgloss.Bottom, columnStrings...)` in `renderRowData` (row.go:243) and `lipgloss.Width` three times across `view.go` and `row.go`. ansix corrects width at poplar's call sites; library-internal calls are unreachable from those sites. The ADR-0084 ban therefore survives the re-eval. Neither poplar consumer — `messagelist` or `contacts/list` — maps cleanly onto the table model anyway, for reasons unrelated to width. This is a (b)-list entry for Task 11 (fork-vs-accept).

**Feature parity:** `bubble-table` provides a scrollable, filterable, sortable table with column definitions, `RowData map[string]any` per row, `WithRowStyleFunc` for conditional row styles, `StyledCell` for per-cell style overrides, horizontal scrolling, multi-select, and pagination. `messagelist` is not a table: it owns a thread-grouping pipeline, box-drawing prefix computation, fold state, and visual-mode selection semantics. None of those are `bubble-table` features — they live in `appendThreadRows`, the `displayRow` builder, and the keyboard handler. `contacts/list` (137 LOC) is a `bubbles/viewport`-backed list with four fixed columns rendered by `formatRow`; `SetSelectionLetter` does letter-jump navigation without cursor arithmetic that the library exposes. The library's column-width model (fixed-width columns set at construction) conflicts with `messagelist`'s responsive column math derived from `ComputeLayout`.

**Customization seams:** `WithRowStyleFunc(RowStyleFuncInput → lipgloss.Style)` and `StyledCell{Data, Style/StyleFunc}` are strong per-row and per-cell injection points. `WithBaseStyle` lets callers push a background. These seams are real, but they don't reach the SPUA compensation path: `mlFlagWidth = 2` pads the flag cell in `messagelist` specifically because SPUA-A icon glyphs expand to two terminal cells under Nerd Font mode. `renderRowColumnData` uses `lipgloss.Width(cellStr)` for its own column-width accounting, so the flag pad would double-count.

**Theming integration:** No `CompiledTheme`-aware path exists. All styles arrive via `WithRowStyleFunc`/`StyledCell`/`WithBaseStyle` as raw `lipgloss.Style` values, compatible with the per-subpackage `NewStyles(*theme.CompiledTheme)` pattern poplar uses. Integration is mechanical but not zero-effort — every style passed in must be wired to the theme constructor.

**Maintenance signal:** 569 stars, 35 forks, 21 open issues, not archived. Latest release v0.19.2 on 2025-09-06; five commits in the week before that release (global metadata for style/filter funcs, `Row` in `StyleFuncInput`, filter improvements). Active development cadence. MIT license, no API breakage risk at the current Go 1.x line.

**Code delta estimate:** `contacts/list.go` is 267 LOC; replacing `formatRow` + `rebuildViewport` with a `bubble-table.Model` would not reduce that — `SetSelectionLetter`, `syncViewport`, `sortContacts`, and the `metaCol`/`nameCol` projection helpers are domain logic with no library analogue. `messagelist/model.go` is the larger consumer; the threading pipeline and fold state machinery are not table concerns. Adoption would add a dependency and a workaround layer (pre-padded cells to route around `JoinHorizontal`) while eliminating no material code.

**License:** MIT (evertras/bubble-table). Clear.

**Verdict:** Keep + harvest; (b)-list for Task 11

**Rationale (one line):** `JoinHorizontal` in `renderRowData` is unreachable from ansix call sites, and neither consumer fits the fixed-column table model — `messagelist` needs the threading pipeline and `contacts/list` needs letter-jump nav — so adoption saves no code and adds a width-safety workaround.

**Interacts with:** Fork-vs-accept (Task 11). `bubble-table` joins `bubbles/help` on the (b) list — its blocker is internal `JoinHorizontal` in the library's core render path, not poplar's own width math. If the fork option is accepted and the fix lands in lipgloss or x/ansi, the width blocker dissolves, but the consumer-fit gap (threading pipeline, letter-jump nav) remains, and the verdict would still be Keep + harvest.

## bubbles/list × pickers (movepicker / linkpicker / attachpicker)

**Does this make poplar better?** No. The library's chrome — title bar, filter input, status bar, paginator, help footer — is built into `View()` and stacked with `JoinVertical`; all of it ships whether callers want it or not. Suppression flags (`SetShowTitle(false)`, `SetShowFilter(false)`, `SetShowPagination(false)`, `SetShowStatusBar(false)`, `SetShowHelp(false)`) exist but leave layout residue and fight `ModalShell`'s border-plus-footer contract. Critically, `JoinHorizontal` does not appear in the list or defaultitem render paths — the row delegate writes to an `io.Writer` and the library uses `JoinVertical` to stack sections — so there is no (b)-list entry here. The width concern is irrelevant; the chrome entanglement is the real blocker.

**movepicker.** The closest fit. `bubbles/list` is a scrollable, filterable list of typed items — exactly the folder-selection shape. But movepicker owns its filter inline (printable rune → `p.filter` → `recompute()`), renders rows via `buildListRows` → `ModalShell.Box`, and clamps scroll with `uicore.ClampScrollOffset`. Replacing that with a `list.Model` would mean delegating filter keystrokes to the library, losing the per-rune `recompute()` path, and fighting `ModalShell.Box` for border ownership. The custom delegate pattern does map onto `buildListRows` shape, but the net delta is negative: more code surface, less control over the modal frame.

**linkpicker.** Poor fit. The list is 1–9 URLs with digit quick-launch, a two-row URL preview below the fold, and a tight `linkPickerMaxWidth = 70` cap. `bubbles/list`'s paginator and status bar consume vertical space that linkpicker reclaims for the preview rows. The digit bindings (`1`–`9` → `LaunchURLMsg`) are not a `list.Model` concept — they require custom key handling that bypasses the delegate entirely. Adoption would add a paginator that activates only when links overflow 9 (the practical ceiling of quick-launch) and remove the two-row full-URL preview that makes the picker usable on long URLs. Net: worse.

**attachpicker.** Poor fit for the same structural reasons as linkpicker. `formatRow` renders icon + `[N] filename (size)` using `ansix.PadOrTruncate` for SPUA-safe width; the custom delegate pattern would reproduce that exactly since `DefaultDelegate` renders a title + optional description with `ansi.Truncate` (not ansix-aware). The open-vs-save split (`o`/`s`/`Enter`/digit) is also delegate-external. No gain from the library's fuzzy filter (attachment lists are short and named; substring movepicker-style filter would be dead weight).

**Feature parity:** `bubbles/list` provides fuzzy filtering, pagination, multi-style selection, delegate-driven row rendering, and built-in help. Movepicker uses a subset (scroll, filter, cursor highlight); linkpicker and attachpicker use a smaller subset still (scroll, cursor highlight, digit dispatch). The library's added features — fuzzy rank ordering, paginator dots, status bar item count — are either duplicated by movepicker's own title line or irrelevant to the other two.

**Customization seams:** `SetDelegate(ItemDelegate)` replaces row rendering entirely. Style injection via `list.Styles` (title, filter prompt, selected item, dimmed item, pagination) provides broad control. Neither seam reaches `ModalShell.Box` border compositing or `uicore.PadOrTruncate` row padding, which poplar controls directly. Seams are real but don't eliminate the chrome-conflict problem.

**Theming integration:** Mechanical — all styles arrive as raw `lipgloss.Style` values and can be wired to `NewStyles(*theme.CompiledTheme)`. Not zero-effort given the per-style-slot mapping, but not a blocker.

**Maintenance signal:** `bubbles/list` ships with the main charmbracelet/bubbles module (v0.21.x active, v1.0.0 stable). Well-maintained, MIT. Not a concern.

**Code delta estimate:** Adopting `bubbles/list` for movepicker would add the library dep (already in go.mod), replace ~150 LOC of `buildListRows`/`clampOffset`/`recompute` with a delegate + a thinner Update handler, but require wrapper glue for `ModalShell` border, filter routing, and chrome suppression. Net change is roughly neutral in size, negative in clarity. Linkpicker (252 LOC) and attachpicker (203 LOC) see no reduction — their digit dispatch, URL preview, and SPUA-safe row format are not delegate addressable.

**License:** MIT (charmbracelet/bubbles). Clear.

**Verdict:** Keep + harvest

**Rationale (one line):** The library's title/filter/paginator/status chrome is structurally incompatible with `ModalShell.Box` ownership, no `JoinHorizontal` blocker exists, and no picker sees a meaningful LOC reduction — the harvest is the delegate pattern itself, already mirrored in movepicker's `buildListRows` shape.

**Interacts with:** Task 9 (bubbles/list × sidebar), which evaluates the same library against a non-modal, non-overlay consumer. The pickers' verdict does not carry over — sidebar has no `ModalShell` conflict and a different chrome budget.

## `bubbles/list` vs `treilik/bubblelister` for compose.Dropdown

**Does this make poplar better?** No on both counts. The dropdown's surface is a chromeless inline autocomplete strip: ≤7 rows, value-type sub-model, positionally spliced below the focused header row by `dropdownRows()`. Neither library fits that shape. `treilik/bubblelister` is incompatible at the dependency level and dead upstream. `bubbles/list` brings chrome that the inline splice explicitly cannot accommodate. The hand-rolled `Dropdown` (96 LOC) is already the right size and shape for this surface; there is nothing to replace.

**Feature parity:** Poplar's `Dropdown` provides six operations: `NewDropdown(SuggestFn)`, `WithStyles`, `SetPrefix` (re-runs the seam and resets the cursor), `Empty`, `Selected`, `Clear`, and `Update` (Up/Down wrapping cursor motion). `View()` emits bare `\n`-joined rows — no border, no title, no pagination. `bubbles/list` provides all of that plus filtering, pagination, status bar, help footer, and delegate-pattern row rendering. Every added feature is dead weight here: the dropdown's backing data is already filtered by `SuggestFn`, the cap is 7 rows per ADR-0174 (pagination never triggers), and no chrome can survive the inline splice without corrupting `dropdownRows()`'s pad-to-width contract. `treilik/bubblelister` provides a heavier cursor model (`MoveCursor(int)` on a pointer receiver), configurable `Wrap`, `PrefixGen`/`SuffixGen` interfaces, a sort pipeline, and `EqualsFunc` for cursor-preserve on reset — none of which apply to a transient ≤7-row suggestion list that rebuilds from scratch on every keystroke.

**Customization seams:** `bubbles/list` injects style via `list.Styles` and row rendering via `SetDelegate(ItemDelegate)`. The delegate's `Render` writes to an `io.Writer` — a different shape from `Dropdown.View()`'s string return, requiring an adapter. `bubblelister` styles rows via `termenv.Style` fields (`LineStyle`, `CurrentStyle`) — `termenv.Style`, not `lipgloss.Style`, so the existing `NewStyles(*theme.CompiledTheme)` constructor produces incompatible values. Wrapping contacts.Suggestion into `fmt.Stringer` for bubblelister's `AddItems` variadic also requires a shim type. Poplar's `Dropdown.WithStyles(Styles)` has no analogous ceremony.

**Theming integration:** `bubbles/list` accepts `lipgloss.Style` throughout; compatible with `NewStyles(*theme.CompiledTheme)` in principle, but the style-slot mapping for title/filter/paginator/status has no poplar-side counterparts since none of those sections render. `bubblelister` uses `termenv.Style` for `LineStyle` and `CurrentStyle` — orthogonal to the lipgloss palette. Neither adds value over `Dropdown.styles.DropdownRow` / `DropdownRowSelected` / `DropdownOrg`, which are already wired in `compose/styles.go`.

**Maintenance signal:** `bubbles/list` ships with `charmbracelet/bubbles` (v0.21.x active, v1.0.0 stable). Not a concern. `treilik/bubblelister` v0.2.0 was last pushed 2023-04-29 — three years dormant. It pins `charmbracelet/bubbletea v0.19.3`, a pre-v1 release that is incompatible with poplar's `bubbletea v1.x` major. Importing it would require a `go.mod replace` or a fork to update the dependency; neither is warranted for a 96-LOC replacement target.

**Code delta estimate:** `suggest.go` is 96 LOC covering the full `Dropdown` surface. Replacing it with `bubbles/list` would eliminate ~60 LOC of `SetPrefix`/`Empty`/`Selected`/`Clear`/`Update`/`View`, then add a delegate adapter, chrome-suppression calls, style-slot wiring, and a `dropdownRows` shim to convert the library's `View()` string back into the row-slice shape the compose frame expects. Net: more code, more surface area, no behavior change. `bubblelister` would require a `fmt.Stringer` wrapper type, a `termenv`-to-lipgloss color bridge for two style slots, and a `go.mod replace` for the incompatible bubbletea dependency. Starting cost before a single row renders: higher than the entire current implementation.

**License:** `charmbracelet/bubbles` — MIT. `treilik/bubblelister` — MIT. Neither is a blocker; the licenses are not the issue.

**Verdict:** Keep (hand-rolled Dropdown wins)

`treilik/bubblelister` is eliminated outright: incompatible bubbletea major, dead upstream, `termenv` style model.

**Rationale (one line):** The Dropdown's chromeless inline-splice contract (≤7 rows, bare string output, value-type sub-model) has no match in either library — bubbles/list brings unwanted chrome, bubblelister brings an incompatible dependency tree — and the 96-LOC hand-roll already covers the surface exactly.

**Interacts with:** None. The Dropdown is a leaf value type inside `internal/ui/compose/`; it has no dependency on other evaluated libraries and its verdict does not affect any other candidate.

## bubbles/list × sidebar folder column

**Does this make poplar better?** No. The sidebar's three-group structure (Primary / Disposal / Custom), blank-line separators between groups, and SPUA-aware row rendering are all hand-rolled for principled reasons. `bubbles/list` models a flat, uniform list of items via a delegate; it has no concept of named-group separators as structural elements. The blank lines that divide the three groups are not items — they are emitted conditionally in `View()` when `entry.cf.Group != prevGroup`. There is no delegate hook for inter-item separators, so the group structure would have to move into the item data itself and leak into the delegate. That is not simplification; it is indirection.

**Feature parity:** `bubbles/list` provides scroll, cursor highlight, fuzzy filter, pagination, status bar, and help footer. The sidebar uses scroll (height-capped `lines[:s.height]`) and cursor highlight. Nothing else. Filter is sidebar-external (`SidebarSearch` shelf in `SidebarColumn`); pagination does not apply (folder lists are short); the title, status bar, and help footer are all chrome the sidebar explicitly has no room for. The `bubbles/list` chrome would conflict with `SidebarColumn.View()`'s column-width contract, which emits a fixed-width column consumed row-by-row by `AccountTab` under the SPUA-A-safe join invariant (ADR-0084).

**Customization seams:** `SetDelegate(ItemDelegate)` replaces row rendering. The delegate could reproduce `renderRow` — indicator column, optional icon, SPUA-aware label truncation via `ansix.TruncateEllipsis`, unread badge — but the group separator blank lines are outside the delegate. They would need a sentinel item type with a custom renderer, adding a type switch the delegate has to carry. The unread badge right-alignment (`FillRowToWidth`) is also non-trivial inside a delegate writing to `io.Writer` without knowing the rendered width of what precedes it.

**Theming integration:** Style injection via `list.Styles` accepts `lipgloss.Style` values; compatible with `NewStyles(*theme.CompiledTheme)`. Not a blocker, but not a gain over the existing `Styles` struct in `sidebar/styles.go`.

**Maintenance signal:** Ships with `charmbracelet/bubbles` (v0.21.x / v1.0.0 stable). Not a concern.

**Code delta estimate:** `sidebar/model.go` is 378 LOC. Adopting `bubbles/list` would not reduce that: `buildEntries`, `sortEntries`, `rankOf`, `iconFrom`, and all the accessor methods are domain logic with no library analogue. `renderRow` (61 LOC) is the only candidate for replacement via a delegate — but the delegate shape adds a sentinel-item type and io.Writer-based rendering, gaining nothing over the current string-return form. Net: neutral-to-negative LOC, weaker control over the row-by-row join contract.

**License:** MIT (charmbracelet/bubbles). Clear.

**Verdict:** Keep + harvest

**Rationale (one line):** The three-group separator structure is not expressible via the delegate model without a sentinel-item workaround, the chrome budget is zero, and no meaningful code is removed — the delegate pattern itself is the only harvest, already mirrored in the picker eval (Task 7).

**Interacts with:** Tasks 7 and 8 (same library, different consumers). The picker and dropdown evals reached Keep verdicts for different reasons (chrome conflict with ModalShell; chromeless-splice contract); the sidebar adds a third: structural group separators not addressable via the delegate. None of the three verdicts depends on the others.
