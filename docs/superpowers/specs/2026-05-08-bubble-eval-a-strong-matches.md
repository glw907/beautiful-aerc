# Bubble Eval A — Strong Matches

## Core question

Does adopting the community bubble make poplar better? At
minimum it must not make poplar worse. Ideally it makes poplar
better. When in doubt, lean toward bubble (first-class
bubbletea-app is itself a win); when adoption would compromise
mail-client quality, keep the hand-roll.

## Rubric

1. Feature parity — does the bubble cover what poplar does today?
2. Customization seams — can we wire themes, keymaps, and domain
   state without forking?
3. Theming integration — accepts `lipgloss.Style` injection?
4. Maintenance signal — last commit, version cadence.
5. Code delta — LOC removed from poplar vs LOC added.
6. License — MIT/BSD/Apache only.

---

## `rmhubbert/bubbletea-overlay`

**Does this make poplar better?** No. The library's `Composite` function
is the same algorithm as poplar's `PlaceOverlay` — both derive from the
Superfile `overplace.go` origin, both use `charmbracelet/x/ansi` for
cell-width measurement, and the line-by-line compositing logic is
structurally identical. Swapping in `Composite` saves zero net LOC because
the library's `Model` type does not model the cascade: the nine-level
`if IsOpen()` chain in `App.View` stays unchanged, and every call site
still supplies pre-computed `(x, y)` integers. The library adds a
`Position` enum for semantic placement (Top/Center/Bottom/Left/Right),
but poplar already delegates that arithmetic to per-component `Position()`
methods. The result is a new dependency, no behavior change, and no LOC
reduction.

**Feature parity:** `Composite(fg, bg, xPos, yPos, xOff, yOff)` covers
the same cell-compositing operation as `PlaceOverlay(x, y, fg, bg)` with
a slightly different argument order and an added `Position` enum. The
library's `Model` wraps two `Viewable` values and composites them in
`View()`; it does not implement `Update()` routing, so the cascade
ordering and mutual-exclusion logic poplar needs remain entirely in `App`.
No gap in the library's coverage is the problem — the coverage exactly
matches what poplar already has, hand-rolled.

**Customization seams:** The library exposes no style injection points.
`Composite` works on rendered strings, like `PlaceOverlay`. There is no
hook for border styles, background dimming (poplar's `DimANSI` pass), or
themed color injection. The `Model.Foreground` and `Model.Background`
fields accept any `Viewable`, so domain state threads through naturally,
but that is true of `PlaceOverlay` too.

**Theming integration:** None. `Composite` operates on pre-rendered ANSI
strings; theming is the caller's responsibility in both implementations.
The library owns no colors.

**Maintenance signal:** Last commit 2026-04-20 (dependabot bump of
`charmbracelet/x/ansi`). Releases: v0.6.7 (2026-04-20), v0.6.6
(2026-03-18), v0.6.5 (2026-02-09) — roughly one release per six weeks in
the past twelve months. Actively maintained for Bubble Tea / Lipgloss v1;
the README notes v2 users should use built-in compositing.

**Code delta estimate:** `overlay.go` is 76 LOC, `modal_shell.go` is 66
LOC — 142 total. `PlaceOverlay` alone is ~50 LOC of that. Replacing
`PlaceOverlay` with `overlay.Composite` would remove ~50 LOC and add one
`go.mod` entry; `ModalShell` is unrelated and would not move. Net delta is
approximately −50 LOC at the cost of a new dependency whose implementation
is the same algorithm.

**License:** MIT License, Copyright (c) 2024 Hubby.

**Verdict:** **Keep + harvest**

**Rationale (one line):** Library and poplar share the same Superfile-
derived algorithm — adopting it trades a small hand-roll for a dependency
with no behavior gain and no cascade support.

**Interacts with:**
- No other Eval A candidates depend on this swap landing first.
- No other candidates are simplified by adopting or skipping this one.
- Not blocked by any candidate outside Eval A.

---

## `bubbles/help`

**Does this make poplar better?** No. The two `Context` enum branches
map cleanly onto two `KeyMap` implementations — the concept translates
— but the structural layout does not. `FullHelpView` renders a flat
key-column + desc-column per `[]key.Binding` group; poplar renders
three named groups per row with `joinColumnsRow`, a custom `renderGotoGrid`
for the six-cell Go To block, and a bottom hint line outside any group.
That shape cannot be expressed through `[][]key.Binding` without encoding
layout intent in grouping, making the KeyMap impls as bespoke as the
current `accountGroups` slice. Additionally, `FullHelpView` calls
`lipgloss.JoinHorizontal` internally — which poplar bans when
`spuaCellWidth != 1` (ADR-0084). The renderer cannot be patched without
forking. Finally, `bubbles/help` has no `wired bool` concept: disabled
bindings are invisible (`kb.Enabled() == false` skips them), not
dim-but-visible. The planned-binding affordance — show unwired keys
dimmed to advertise future bindings — is a first-class poplar feature and
would be lost.

**Feature parity:** `KeyMap.ShortHelp()` / `FullHelp()` cover the
conceptual shape. Width-aware truncation with ellipsis matches what the
current `Box` does for the `tooNarrow` fallback. What is missing: the
multi-row grouped grid layout, the `wired` dim affordance, the `ModalShell`-
compatible box return (the library renders inline text, not a bordered
box), and SPUA-safe column joining.

**Customization seams:** `help.Styles` has `FullKey`, `FullDesc`,
`FullSeparator`, `ShortKey`, `ShortDesc`, `ShortSeparator`, and `Ellipsis`
— all `lipgloss.Style` fields. Poplar's palette maps onto them cleanly
(`HelpKey` → `FullKey`, `Dim` → `FullDesc`, etc.). The seams exist; they
just cannot recover the layout or `wired` semantics.

**Theming integration:** Clean. All style fields are `lipgloss.Style`;
`NewStyles(*theme.CompiledTheme)` can populate them in one assignment
block. No color hardcoding in the render path; `New()` default colors can
be overridden.

**Maintenance signal:** `bubbles/help` ships inside `charmbracelet/bubbles`
v1.0.0 — pinned in poplar's `go.mod`. Stable API; well-maintained. No
maintenance risk.

**Code delta estimate:** Replacing `helppopover` with `bubbles/help`
would delete ~380 LOC (model.go + styles.go) but require reimplementing
the grid layout either in `KeyMap.FullHelp()` (encoding layout in group
ordering — fragile) or by calling `FullHelpView` as a sub-renderer and
wrapping it in the existing `Box` frame. The `JoinHorizontal` blocker
and `wired` loss mean adoption requires a fork of help.go (~200 LOC) on
top of whatever layout scaffolding stays. Net delta is negative only if
forking is counted as "free"; in practice the total owned code changes by
less than ±50 LOC.

**License:** MIT License, Copyright (c) 2020-2026 Charmbracelet, Inc.

**Verdict:** **Keep + harvest**

**Rationale (one line):** `bubbles/help` cannot render poplar's multi-
column grid, uses `JoinHorizontal` (ADR-0084 banned), and drops the
`wired` dim affordance — the hand-roll is the right shape; the `Styles`
field names are useful terminology to mirror.

**Interacts with:**
- `wired` dim pattern is not affected by any other Eval A candidate.
- `JoinHorizontal` ban (ADR-0084) applies equally to any candidate
  whose render path calls it internally.
- No other Eval A candidate depends on this evaluation's outcome.

---

## `daltonsw/bubbleup`

**Does this make poplar better?** No. `bubbleup` solves the wrong
problem for poplar's toast. The library renders categorized status
alerts (Info/Warn/Error/Debug) with color-fade-in animation via a
`curLerpStep` lerp against a black `BackColor`. Poplar's toast is a
domain-aware triage feedback row: it carries a `triageOp` payload,
a `tea.Cmd` inverse for undo on `u`, and a monotonic `deadline` for
the undo countdown. The two models diverge at every seam. Adopting
`bubbleup` means importing the timing/animation chrome (100 ms tick
loop, `go-colorful` Lab-space lerp, `go.dalton.dog/bubbleup` +
`lucasb-eyer/go-colorful` as new deps) while still owning 100% of
the domain-specific content — `renderToast` is the entire hard part,
and it stays.

**Feature parity:** `bubbleup` covers the auto-dismiss timer and
position logic (six `Position` constants: top/bottom × left/center/
right). Poplar has both already: a `tea.Tick` to `toastExpireMsg`
handles expiry, and the toast renders inside the shared chrome row
above the status bar (fixed position, no float). What `bubbleup`
cannot provide: caller-supplied `View()` — the library owns its
render entirely via `Render(content string)` (string overlay, not
a `Model` the caller styles); no slot for a `triageOp` payload;
no undo inverse Cmd hook; no countdown display. `HasActiveAlert()`
is the only seam back to the caller, and it returns a bool, not
the active payload.

**Customization seams:** None that reach poplar's requirements.
`AlertDefinition` fields — `ForeColor`, `Style`, `Prefix`, `Key` —
cover category-level styling. The toast body is assembled inside
`alert.render()` as `"<prefix> <message>"` with a hardcoded
`baseStyle` (rounded border, padding (0,1)). Callers pass a string
message; they cannot inject a rendered row or read back the active
alert's domain type. The undo-hint slice (`[u undo]`) and the
per-op verb (`Deleted 3 messages`) live in `renderToast` and cannot
be delegated to `bubbleup` without reconstructing them as a plain
string, which means the caller still owns all the formatting work.

**Theming integration:** Partial and conflicting. `ForeColor` is a
hex string passed at registration time; the lerp blends it against
a hardcoded `BackColor = "#000000"`. Poplar's `Styles.Toast` is a
`lipgloss.Style` bound to the compiled theme palette — the two color
systems are orthogonal. Getting poplar's `AccentSuccess` color into
`bubbleup` means hex-encoding the palette slot and accepting that
the library will lerp it against black, not the terminal background.
That is wrong for dark-on-light themes.

**Maintenance signal:** Last commit 2026-05-01 (three commits that
week). 41 stars. Single-author project; no released tags beyond
`v0.x` cadence visible in the API response. Module path is
`go.dalton.dog/bubbleup` (personal vanity domain), not a Go module
proxy canonical path. Active but small-community.

**Code delta estimate:** Adopting `bubbleup` would add two new
dependencies (`go.dalton.dog/bubbleup`, `lucasb-eyer/go-colorful`)
and remove approximately 0 LOC from `toast.go`: the 10-line
`alert.render()` call cannot replace `renderToast` (85 LOC) because
the domain content is the whole function. The only deletable code
would be the `tea.Tick` + `toastExpireMsg` pattern (~10 LOC in
`app.go`) replaced by `bubbleup`'s internal 100 ms tick loop — a
wash that adds a goroutine firing 10× per second while a toast is
active. The animation lerp is net-new behavior poplar does not want:
pine-spirit clients show static status rows, not color-fade alerts.

**License:** MIT License (spdx: MIT, confirmed via GitHub API).

**Verdict:** **Keep + harvest**

**Rationale (one line):** `bubbleup`'s value is the animation/color-fade
chrome — exactly the part poplar does not want; the triage payload,
undo Cmd, and domain verb are hand-rolled content that cannot move
into the library, so adoption adds two deps and a 10 Hz tick for
zero LOC reduction.

**Interacts with:**
- No other Eval A candidate is affected by skipping this library.
- The hand-rolled `tea.Tick` + `toastExpireMsg` pattern in `app.go`
  remains the timer mechanism regardless of this verdict.
- If poplar ever wanted animated notifications (post-1.0), this
  library or the pattern it uses could be revisited then.

---

## `charmbracelet/huh`

**Does this make poplar better?** No, for two structural reasons.

**Q1 — Sub-pane mount.** `huh.Form` implements the full `tea.Model`
interface and accepts `WithWidth` / `WithHeight`, so it can run
embedded inside a larger bubbletea application. However, the form
renders its own chrome — field separators, group titles, a built-in
`bubbles/help` footer — and drives its own viewport scroll
internally. In Contacts mode, poplar mounts the form as a right-pane
column (no modal chrome); the form must render as raw body rows so
the Contacts-mode frame supplies the borders. `huh.Form.View()` has
no "body-only" mode; chrome is always included and its layout math
assumes the form owns its full render height. Stripping the chrome
would require forking `form.go` and `group.go`, which are the core
files. This alone pushes the verdict to Adopt-with-fork at minimum.

**Q2 — Dynamic field set.** `huh.Group` accepts fields at
construction via `NewGroup(fields ...Field)` and stores them in a
selector that is never mutated after construction. There is no public
method to add or remove fields at runtime. Poplar's contacts form is
fundamentally dynamic: users add email rows with `+ add email`, remove
them with `−`, and the focus list rebuilds via `focusList()` on every
mutation. The `(input, cycler, ★, −)` quartet per row is not a value
a `huh.Select` or `huh.Input` can express — the cycler and star are
custom widgets with bespoke key handling (`isCyclerKey`, primary-
rotation logic). Implementing this inside `huh`'s field interface
means writing a custom `Field` implementation that owns all the
dynamic behavior — at which point the huh frame provides no net
value over poplar's current `Form` struct.

The two blockers compound: the library cannot render in right-pane
mode and cannot express dynamic row sets. Even if both were
addressable by forking, the forked code would be larger than the
current `form.go` (480 LOC), because `huh`'s generic field
machinery — `selector`, `skip` callbacks, accessible-mode runners —
is load-bearing infrastructure the fork must carry even when the
dynamic-row fields bypass it.

**Feature parity:** `huh` covers the static subset of poplar's form
well: `Input` for text fields (First, Last, Org, Title), `Select`
for the kind toggle (Person vs Business), `Confirm` for save
destination. What it cannot express: dynamic row sets, the label
cycler widget, the ★-rotates-to-primary affordance, the dual render
mode (`fromPopover` modal vs right-pane column), and the `Dirty` bit
tied to snapshot comparison (`sameContact`). These are all in the
parts of `form.go` that carry actual product design.

**Customization seams:** The `Styles` struct is well-factored
(`lipgloss.Style` fields for `Focused`, `Blurred`, and per-element
sub-structs). Poplar's `Styles.FieldFocus` / `FieldBlur` / `KindOn` /
`KindOff` map naturally onto `huh.Styles`. The seams are present;
they just cannot reach the dynamic-widget surface.

**Theming integration:** Clean in principle. `huh`'s theme functions
construct `lipgloss.Style` values from external color definitions;
`theme.CompiledTheme` slots could populate the style fields in a
single constructor. No color hardcoding in the render path. The
`help.Styles` sub-struct matches the shape noted in the `bubbles/help`
eval above. Integration is blocked by the structural issues above,
not by theming.

**Maintenance signal:** v2.0.3, March 2026. Charmbracelet first-party
library; semver-stable with active release cadence (six releases in
twelve months). Already in poplar's indirect dependency graph via
`bubbles`. No maintenance risk.

**Code delta estimate:** Adopting `huh` for the static fields only
(treating dynamic rows as custom) removes approximately 120 LOC from
`form.go` (the text-field row renderers, the focus-cycle traversal
for text inputs) and adds huh's overhead for those same fields. The
dynamic-row surface — 280+ LOC — remains untouched. Net delta is
near zero for a surface where the hard-to-read code is the dynamic
part, not the text-input wiring. A full replace (including forking
for right-pane mode and writing custom `Field` impls for dynamic
rows) would add a fork of `form.go` + `group.go` (~600 LOC upstream)
on top of the custom `Field` types, for a net LOC increase.

**License:** MIT License, Copyright (c) 2023-2026 Charmbracelet, Inc.

**Verdict:** **Keep + harvest**

**Rationale (one line):** `huh.Group` is static-only at construction
(no runtime add/remove), and `huh.Form.View()` has no body-only mode
— both blockers are structural, so the fork cost exceeds the hand-roll.

**Harvest targets:**

- The `(Focused, Blurred)` style pair naming in `huh.Styles` is a
  cleaner vocabulary than poplar's current `(FieldFocus, FieldBlur)`;
  worth renaming in `contacts/styles.go` for readability parity.
- `huh`'s `WithWidth` propagation pattern (field calculates its own
  input width by subtracting frame size from allocated width) is the
  same pattern poplar's `inputWidth(contentW)` uses — no change
  needed; the idiom is already correct.
- The `WithInline` field option (renders title + input on one row) is
  the exact shape poplar's `fieldRow` produces manually; no adoption
  needed, just confirmation the approach is idiomatic.

**Interacts with:**
- This verdict does not affect Task 5 (`bubble-table`) or any other
  Eval A candidate.
- Pass 9u (first-run wizard) is the next planned form surface. The
  wizard's field set is static (account name, provider, host, port,
  credentials) — `huh` would fit 9u better than Contacts. This eval
  does not preclude a bounded 9u adoption on a static wizard form;
  that decision is deferred to the 9u plan.
- No other candidate is blocked by or dependent on this verdict.

---

## `evertras/bubble-table`

**Does this make poplar better?** No, for either consumer. The hard
blocker is rendering architecture: `bubble-table`'s `renderRowData`
and `renderHeaders` both call `lipgloss.JoinHorizontal` to assemble
column cells. That is banned under SPUA-A icon mode (ADR-0084); the
SPUA-A compensation that `messagelist.renderRow` performs manually —
`SpuaCount` × `(spuaCellWidth-1)` subtracted from the subject budget,
then `FillRowToWidth` absorbing the slack — cannot be expressed through
`bubble-table`'s column-width API. Every `JoinHorizontal` call inside
the library is a miscount when `spuaCellWidth > 1`, so the blocker
applies in all terminal configurations that trigger SPUA-A mode.

Beyond the `JoinHorizontal` ban, each consumer has structural
incompatibilities of its own.

For `messagelist`: the threading surface is the bulk of the model.
The depth-prefix walk (`├─`, `└─`, `│ `) is computed from a transient
`*threadNode` tree built per bucket in `appendThreadRows`; fold state,
visual-mode marks, `ActionTargets()` expansion, and SPUA-A flag-cell
compensation are all owned by `messagelist.Model`. These cannot be
delegated to `bubble-table` — the library's `Row` type carries a
`RowData map[string]any` per row and has no slot for a thread-prefix
string, a fold state bit, or a `tea.Cmd` inverse. Poplar would supply
the threading prefix as the value for a "prefix" column key, but the
library's `renderRowColumnData` renders it through a `limitStr` cell
with `lipgloss.NewStyle()` inheritance — it loses the `MsgListThreadPrefix`
style that must inherit the row background. The thread-prefix rendering
is the entirety of what makes the messagelist look right; reducing it
to a plain-string column breaks the visual model.

For `contacts/List`: the fit looks better on the surface — three fixed
columns (Name 22, Email 30, Phone 16), sort toggle, `bubbles/viewport`
scroll. But poplar's `contacts.List` is already 137 LOC including
`rebuildViewport`, `formatRow`, `sortContacts`, and the `SetSelectionLetter`
letter-jump the sidebar drives. `bubble-table` adds ~600 LOC of
infrastructure (frozen columns, pagination, filter input, horizontal
scroll, multi-sort, selectable rows) that the contacts list will never
use, and the `JoinHorizontal` ban still applies in the render path.
The contacts list has no filtering requirement (the sidebar's T9
groups drive navigation) and no pagination requirement (viewport
scroll suffices). There is no surface where `bubble-table`'s extras
turn into a reduction in owned code.

**Feature parity:**

*messagelist*: `bubble-table` covers horizontal scrolling,
multi-column sort, per-row and per-cell styling via `RowStyleFuncInput`
and `StyledCell`. What it cannot express: threading depth prefixes
attached to a specific row, fold/unfold toggle, `ActionTargets()`
expansion scope, SPUA-A icon compensation, or `FillRowToWidth`-based
width equalization. The threading surface is not a peripheral feature;
it is the reason the component exists.

*contacts/List*: `bubble-table` covers multi-column layout, sort, and
a cursor-highlight row. What it cannot express: `SetSelectionLetter`
(letter-jump driven by the sidebar), `SortMode` tied to display
content (`nameCol` switches between `Given + Family` and
`Family, Given`), the `metaCol` (title · org) overflow column that
fills the remaining width dynamically, and the `CursorRow` style that
must be the caller's `AccentPrimary` rather than a highlight applied
inside the library's column cell.

**Customization seams:** `bubble-table` exposes
`WithRowStyleFunc(RowStyleFunc)` and `StyledCell{StyleFunc}` as the
two injection points for row- and cell-level styling. The
`RowStyleFuncInput` carries `Index`, `Row`, and `IsHighlighted`. These
seams cover zebra-striping and conditional row color; they do not cover
per-column partial styles (thread prefix dim vs sender bright on the
same row) or multi-style cells (prefix + subject as two adjacent styled
spans within one column). Poplar's `renderRow` assembles eight distinct
styled fragments per row; none of them map to one column in
`bubble-table`'s model.

**Theming integration:** `bubble-table` exposes a base `lipgloss.Style`
and per-column `WithStyle`. The cursor highlight is `WithHighlightStyle`.
Poplar's `Styles` struct could populate these fields from a
`*theme.CompiledTheme`, but the `JoinHorizontal` calls in the render
path undo any SPUA-A accounting the caller has done to the column
widths. Theming is technically injectable; the SPUA-A miscount is in
the renderer itself.

**Maintenance signal:** v0.19.2, published 2025-09-06. 569 stars. Last
repo push 2025-09-06; 21 open issues. Single primary maintainer. No
activity in the eight months since v0.19.2; the library predates
`charmbracelet/x/ansi` v1 and Bubble Tea v2, so there is uncertainty
around Bubble Tea v2 compatibility. Not a blocker for current poplar
(uses BT v1), but the stalled cadence is a signal.

**Code delta estimate:** Adopting `bubble-table` for `contacts/List`
would delete ~137 LOC from `list.go` and add a `go.mod` entry plus
~600 LOC of library infrastructure per package tree. The
`SetSelectionLetter`, `metaCol`, and `nameCol(sortMode)` logic would
survive as pre-processing — none is delegatable to the library's
`RowData` map. Net owned LOC reduction: approximately 40 LOC
(cursor handling, viewport sync, scroll math). That is not a
meaningful win. For `messagelist`, adoption would require rewriting the
threading prefix logic as a plain-column render, dropping SPUA-A
compensation, and forking `renderRowData` — a net LOC increase.

**License:** MIT License, Copyright Evertras contributors.

**Verdict:** **Keep + harvest** for both consumers.

**Rationale (one line):** `bubble-table` calls `lipgloss.JoinHorizontal`
in every row and header render (ADR-0084 ban), and neither consumer
can map its domain model — threading depth-prefixes for `messagelist`,
letter-jump + dynamic `metaCol` for `contacts/List` — onto
`bubble-table`'s rectangular `RowData` shape.

**Harvest targets:**

- `bubble-table`'s `WithRowStyleFunc(RowStyleFuncInput)` pattern
  is the idiomatic way to gate highlight style on `IsHighlighted`
  without storing the cursor index in every row. Poplar's
  `contacts/List.formatRow` already does this via an `i == l.cursor`
  check; the naming convention is worth aligning (`CursorRow` →
  `highlightStyle` if contacts styles are ever refactored).
- The `StyledCell` value type (wrapping data + `StyleFunc` in one
  `any` slot) is a readable pattern for flagging icon cells in
  `messagelist`. Poplar uses a flat `renderFlagCell` function
  instead — no structural change needed, but the conceptual shape
  confirms the cell-level approach is idiomatic.
- `bubble-table`'s `SortColumn{ColumnKey, Direction}` slice model
  for multi-column sort is cleaner than a bespoke enum when more
  than one sort axis exists. `contacts/List` has one axis today
  (`SortFirstName`/`SortLastName`); if a secondary axis is added
  post-1.0, this is the shape to follow.

**Interacts with:**

- The `JoinHorizontal` ban (ADR-0084) is the same blocker named in
  the `bubbles/help` eval. No fork of `bubble-table` resolves the
  ban without rewriting the core render path — that rewrite is
  effectively a different library.
- No other Eval A candidate depends on this verdict.
- Pass 9v's scope does not include a `contacts/List` or
  `messagelist` rewrite; this verdict blocks neither pass from
  proceeding.

---

## Outcome and harvest targets

### Verdict tally

All five Eval A candidates landed **Keep + harvest**. No adoption is
authorized by this triage. The original roadmap assumption (9v.1
`bubbletea-overlay` swap, 9v.2 `huh` swap) is rescinded.

### Why every candidate failed

Two blockers recur across the set. ADR-0084 bans `lipgloss.JoinHorizontal`
under SPUA-A icon mode; `bubbles/help` and `bubble-table` both call it
in their core render path with no seam to override. The second blocker
is poplar's domain shape: wired/unwired help bindings, payload-bearing
triage toast with an undo inverse, dynamic contact-form rows, and the
threading walk plus `ActionTargets` expansion are all runtime-mutable
constructs that the candidates model at construction time. Adoption
would require forking or accepting behavior loss at the product-defining
seams — in each case the fork cost meets or exceeds the hand-roll.

### Harvest targets

- `rmhubbert/bubbletea-overlay`: The `Position` enum vocabulary
  (Top/Center/Bottom/Left/Right) is useful if a future overlay
  grows semantic placement beyond the current centered default.
  Nothing to lift now; the shared Superfile algorithm is already
  in `overlay.go`.
- `bubbles/help`: The `Styles` field names — `FullKey`, `FullDesc`,
  `FullSeparator`, `ShortKey`, `ShortDesc`, `ShortSeparator`,
  `Ellipsis` — are cleaner terminology than the current
  `helppopover` names. Worth mirroring in `helppopover/styles.go`
  on the next pass that touches that file.
- `daltonsw/bubbleup`: Nothing to lift. The animation/color-fade
  chrome is the library's value proposition; poplar does not want
  it. The hand-rolled `tea.Tick` + `toastExpireMsg` pattern stays.
- `charmbracelet/huh`: The `(Focused, Blurred)` style-pair naming
  in `huh.Styles` is cleaner than poplar's current `(FieldFocus,
  FieldBlur)` — rename in `contacts/styles.go` for readability
  parity. Note: the 9u first-run wizard has a fully static field
  set (account name, provider, host, port, credentials); `huh`
  may still fit that surface. This eval does not foreclose a
  bounded 9u adoption — that decision belongs to the 9u plan.
- `evertras/bubble-table`: Nothing actionable. The `JoinHorizontal`
  blocker (ADR-0084) applies to any future table candidate whose
  core render path calls it; flag this in Eval B if table
  sub-candidates appear there.

### Implications for upcoming passes

- **9q (outbox delivery controls):** unaffected — the schedule-send
  modal builds on the existing `ModalShell` + `PlaceOverlay` chrome.
- **9u (first-run wizard):** the 9u plan may still evaluate `huh`
  against a static field set; this eval does not authorize or
  foreclose that.
- **Eval B (medium matches):** proceeds as planned — pickers,
  viewport, and table sub-candidates not covered here.
- **9y consolidation:** no swaps queued from Eval A; consolidation
  focuses on Eval B outcomes.
