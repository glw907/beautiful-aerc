# Chrome family — Pattern Catalog

Patterns harvested from three chrome-family libraries during Pass 9x.2:
`bubbles/help@v1.0.0`, `mistakenelf/teacup` statusbar, and
`daltonsw/bubbleup`. Companion to the Eval B verdicts at
`docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`.
Every Eval B verdict was Keep + harvest; this doc catalogs what to keep
in mind rather than proposing wholesale replacement.

Each entry: pattern → upstream location (file:line) → why it's good →
poplar applicability (applied now / candidate for later / no — with
rationale).

Pass 9z consumes the "Top three" section across 9x.1–9x.3 catalogs to
assemble its adoption roadmap.

---

## `bubbles/help`

Sibling to the `bubbles/list` catalog at
`docs/poplar/research/2026-05-08-bubbles-list-patterns.md`.

---

### 1. `KeyMap` as a two-method interface

**Where.** `help.go:18-28`. `KeyMap` is an interface with two methods:
`ShortHelp() []key.Binding` and `FullHelp() [][]key.Binding`. Both return
`key.Binding` slices; the help bubble consults `kb.Enabled()` per binding
and skips disabled ones automatically.

**Why it's good.** Any struct that implements both methods can drive the
renderer — no inheritance, no registration. The `Enabled()` guard means
bindings self-manage: wiring a binding is a call to `SetEnabled(true)`,
not a separate data-table entry or a render-branch. The interface is small
enough to satisfy in two lines.

**Applicability.** Does not apply to helppopover's named-group grid contract
(settle — see 9x.2 §1 constraint). Helppopover uses `bindingGroup` /
`bindingRow` tables with a `wired bool` flag, not `key.Binding` structs;
the rendering is spatial (three-column named grid + Go To 3×2 tile), not
linear. `key.Binding`'s `ShortHelp()` / `FullHelp()` shape cannot express
positional group layout. See 9x.1 §9 for the related `KeyMap` value-type
(struct) pattern in `bubbles/list`.

---

### 2. `Inline(true)` on separator and key/desc styles

**Where.** `help.go:121` (`separator := m.Styles.ShortSeparator.Inline(true).Render(...)`),
`help.go:136-137` (`m.Styles.ShortKey.Inline(true).Render(...)` and
`m.Styles.ShortDesc.Inline(true).Render(...)`), `help.go:169` (same for
`FullSeparator`). Every style is rendered `Inline(true)` before use.

**Why it's good.** Styles defined at construction time carry block-level
attributes (padding, width) that break mid-string rendering. Calling
`Inline(true)` at the call site strips block attributes and ensures the
rendered fragment composes correctly in a `strings.Builder` stream.
Decoupling construction (palette colors, boldness) from mid-string use
(no block attrs) lets the `Styles` struct be a pure style definition, not
a layout artifact.

**Applicability.** Applied in helppopover already at the render level:
`renderKeyDesc` (`model.go:347-352`) calls `styles.HelpKey.Render(key)` and
`styles.Dim.Render(desc)` on fragments that are then joined by plain string
concatenation. `NewStyles` does not set padding or width on `HelpKey` /
`Dim`, so block-attribute collision is not currently a hazard. If a future
style update adds padding to a fragment-level style, the `Inline(true)` fix
is the right remedy — this pattern names it.

---

### 3. Ellipsis-on-overflow truncation via `shouldAddItem`

**Where.** `help.go:221-231`. `shouldAddItem(totalWidth, width int) (tail string, ok bool)`
computes whether the next item fits. When it does not, it attempts to render
the ellipsis string (`m.Ellipsis`, default `"…"`) styled via
`m.Styles.Ellipsis.Inline(true)` as a tail. If even the ellipsis overflows,
the tail is empty. Both `ShortHelpView` (`help.go:141-146`) and
`FullHelpView` (`help.go:207-211`) call `shouldAddItem` in the same loop
that accumulates items.

**Why it's good.** The pattern handles three cases in one function: item
fits, item overflows and ellipsis fits, item overflows and ellipsis also
overflows. Callers write a single `if tail, ok := ...; !ok { break }` branch
per loop. The ellipsis is a styled configurable string, not a special-cased
constant, so consumers can match their terminal's font coverage.

**Applicability.** Candidate for later. Helppopover's `Box` (`model.go:224-228`)
falls back to a whole-box `tooNarrow` string when the popover does not fit
the terminal — it does not do inline column truncation. If a future pass
narrows the popover progressively (dropping the rightmost group first),
`shouldAddItem`'s loop shape is the right model. Not a current gap.

---

### 4. `FullHelpView` column-key/desc split with `JoinHorizontal`

**Where.** `help.go:187-203`. For each group, the library splits bindings
into parallel `keys []string` and `descriptions []string` slices, joins
each with `"\n"`, then uses `lipgloss.JoinHorizontal(lipgloss.Top, sep,
keysBlock, " ", descBlock)` to render the column.

**Why it's good.** Key strings and description strings align in a
right-padded key column without per-row width math. `lipgloss.JoinHorizontal`
handles the baseline (lipgloss.Top) so keys and descs start on the same
line even when a group has only one entry.

**Applicability.** No — helppopover avoids `lipgloss.JoinHorizontal` per
ADR-0084 (`joinColumnsRow` with row-by-row `strings.Join` is the SPUA-safe
replacement). The *key/desc split into parallel slices* sub-pattern is the
interesting half: helppopover's `renderRow` (`model.go:336-343`) achieves
the same visual column by padding `r.key` to a fixed `keyWidth = 5` and
joining with two spaces. Fixed-width padding is simpler and correct for
helppopover's fixed vocabulary; the parallel-slice approach adds value when
key widths vary unpredictably across groups. Not a current gap.

---

### 5. `Styles` struct with Short/Full × Key/Desc/Separator naming

**Where.** `help.go:31-43`. Seven fields: `Ellipsis`, `ShortKey`,
`ShortDesc`, `ShortSeparator`, `FullKey`, `FullDesc`, `FullSeparator`.
Names encode context × role: `Short` vs `Full` selects the view mode;
`Key` vs `Desc` vs `Separator` selects the fragment role.

**Why it's good.** The naming scheme is self-documenting at the call site:
`m.Styles.ShortKey.Inline(true).Render(...)` reads as "short-help key
style, inline." No positional confusion. Extension follows the same pattern:
a `FullHeader` slot would fit without ambiguity.

**Applicability.** Partially applied in helppopover. `HelpKey` covers the
key column for all contexts (no Short/Full distinction — helppopover does
not have a short/full toggle). `HelpGroupHeader` covers group titles.
`Dim` covers both desc fragments and unwired rows. The naming is
context-free, which suits a fixed-layout popover. If helppopover ever gains
a compact single-line mode, the context-prefixed naming convention is the
right model to follow.

---

### 6. `shouldRenderColumn` — disabled-binding column skip

**Where.** `help.go:233-240`. Before building a column in `FullHelpView`,
the library calls `shouldRenderColumn(group)`, which returns true only if
at least one binding in the group has `Enabled() == true`. A fully-disabled
column is skipped silently; neither its separator nor its width is charged
against `totalWidth`.

**Why it's good.** Consumers can toggle entire feature groups by disabling
all their bindings. No conditional deletion from the input slice, no `if
len > 0` at the call site. The help view self-prunes.

**Applicability.** Candidate for later. Helppopover hard-codes two context
tables (`accountGroups`, `viewerGroups`). There is no per-group enabled
flag; context selection removes entire tables, not individual groups. If
a future pass adds a third context (compose help, contacts help) built from
a shared group pool with per-group opt-in, a `bindingGroup.enabled bool`
field would replicate this pattern. Not a current gap; the two-context
model is stable.

---

### Diff candidates for Task 6 (helppopover)

No diffs reach the obvious-win bar.

The patterns in `bubbles/help` that are not yet applied (§3 ellipsis
truncation, §6 column-skip) address capability gaps that don't exist in
helppopover's current design: the popover uses a whole-box tooNarrow
fallback rather than progressive truncation, and context selection already
removes whole group tables. The `Inline(true)` pattern (§2) is defensively
correct but not a current hazard — `HelpKey` and `Dim` carry no block
attributes. The `FullHelpView` `JoinHorizontal` approach (§4) is actively
rejected by ADR-0084. None of these clear the "obvious improvement without
changing the named-group grid contract" bar.

---

## `mistakenelf/teacup` statusbar

Patterns harvested from `mistakenelf/teacup@main` (`statusbar/statusbar.go`)
during Pass 9x.2. Poplar consumer: `internal/ui/status_bar.go` (`StatusBar`
value type, `View(width, dividerCol int) string`).

---

### 1. `ColorConfig{Foreground, Background lipgloss.AdaptiveColor}`

**Where.** `statusbar.go:14-17`. `ColorConfig` is a two-field struct pairing
`Foreground` and `Background` as `lipgloss.AdaptiveColor`. Per-column color
configuration is then expressed as four `ColorConfig` fields on `Model`:
`FirstColumnColors` through `FourthColumnColors` (lines 21-30).

**Why it's good.** `AdaptiveColor` accepts separate light/dark hex strings
and resolves at render time based on the terminal's background detection.
Grouping foreground and background into one named struct keeps the pair
together at every call site (`New`, `SetColors`, `View`) and makes the
contract explicit: a column's color is a pair, not two independent values.
Compared to four loose `lipgloss.Style` fields or eight separate color
arguments, the pair struct is harder to misorder.

**Applicability.** No — poplar uses pre-compiled `lipgloss.Style` values in
the `Styles` struct rather than runtime `AdaptiveColor` pairs. The
`StatusBar` value type receives a `Styles` at construction
(`NewStatusBar(styles Styles)`); the resolved styles (`StatusConnected`,
`StatusOffline`, `StatusReconnect`, `StatusBar`, `TopLine`) are theme-compiled
values from `internal/theme/`. Switching to `ColorConfig` pairs would
dismantle the compiled-theme contract (ADR — see `docs/poplar/invariants.md`
§Config & theming). The pair-grouping concept is already embodied
structurally: each connection-state branch in `View` (`status_bar.go:125-139`)
selects a pre-compiled `connStyle` value, one per state.

---

### 2. `SetContent` — positional column text injection

**Where.** `statusbar.go:60-67`. `SetContent(first, second, third, fourth string)`
sets all four column strings in one call. No per-column setters exist; the
caller always provides all four strings at once.

**Why it's good.** Atomic batch updates avoid half-configured renders between
individual field assignments. When all four columns change on a state
transition, one call is cleaner than four sequential mutations.

**Applicability.** No direct equivalent. Poplar's `StatusBar` exposes
per-concern setters (`SetCounts`, `SetMode`, `SetScrollPct`,
`SetConnectionState`, `SetOutboxDepth`) that match its semantic segments
rather than positional slots. The value-type copy-on-write pattern (all
setters return `StatusBar`) gives the same atomicity guarantee without
bundling unrelated concerns into one call. The batch-update benefit does
not apply to poplar's update sites: callers change one concern at a time
in response to distinct Msg types.

---

### 3. `muesli/reflow/truncate` ellipsis with a hard character cap

**Where.** `statusbar.go:82-84`. The first column applies
`truncate.StringWithTail(m.FirstColumn, 30, "...")`, hard-capping at 30
characters before applying padding and style. The second column uses the
same function with a dynamic cap derived from the remaining width
(`m.Width - width(first) - width(third) - width(fourth) - 3`, lines 93-97).

**Why it's good.** `muesli/reflow/truncate` is ANSI-aware — it counts
printable cells, not bytes — so the cap is semantically correct for
styled strings. The hard 30-char cap on the first column prevents a long
folder name from collapsing the other columns. The second column's dynamic
cap uses the same function so both slots are ANSI-correct even when the
input already carries escape sequences.

**Applicability.** Candidate for later. Poplar already imports
`github.com/charmbracelet/x/ansi` (via `internal/ansix/`), which exposes
`ansix.TruncateEllipsis` — the same ANSI-aware truncation surface, but with
`…` (U+2026) as the tail. The `muesli/reflow` import would be a
near-duplicate dependency. `buildFill` + pre-measure in `View`
(`status_bar.go:150-156`) makes the fill slot absorb width variance without
truncating any segment, so the hard-cap pattern does not currently apply.
If a future pass adds a variable-length segment to the right side (e.g. an
account-name display), `ansix.TruncateEllipsis` with a dynamic cap is the
right model — no new import needed.

---

### 4. Four-column slot shape — domain mismatch

**Where.** `statusbar.go:21-30` (Model fields) and the `View()` body
(lines 72-101). The library divides the bar into exactly four named
positional slots: First/Second/Third/Fourth. The second slot absorbs all
remaining width; the first, third, and fourth are fixed-or-capped.

**Why it fits teacup.** Teacup is a general-purpose file-manager TUI;
four columns (current-file / spacer / file-info / mode) is a natural
decomposition. The positional contract is fixed and consumers fill slots
by content category.

**Why it does not fit poplar.** Poplar's status bar has six semantic
segments in a specific left-to-right order: fill (variable, absorbs width),
counts/scroll-pct, outbox depth + glyph (conditional), connection icon,
connection text, and `─╯` terminator. The fill is on the left, not in the
middle; the outbox segment is hidden when empty; the connection icon uses a
per-state `lipgloss.Style` rather than a per-column `ColorConfig`. Mapping
this onto four positional slots would require collapsing unrelated concerns
(fill + counts into "second column"), losing the conditional outbox hiding,
and abandoning the drop-rank assembly. The six-segment contract is settled
and is not a refactor target.

**Applicability.** No — domain mismatch, not a quality gap.

---

### 5. `JoinHorizontal` for final assembly — anti-pattern (ADR-0084)

**Where.** `statusbar.go:99-104`. The final render joins all four
rendered-and-styled column strings with:

```go
return lipgloss.JoinHorizontal(lipgloss.Top,
    firstColumn, secondColumn, thirdColumn, fourthColumn)
```

Each column is already padded and sized before the join. `JoinHorizontal`
aligns them along the top baseline.

**Why it is a hazard.** `lipgloss.JoinHorizontal` measures segment widths
using `lipgloss.Width`, which calls `ansi.PrintableRuneWidth`. That function
treats SPUA-PUA codepoints (Nerd Font glyphs, `[U+F0000, U+FFFFD]`) as
zero-width. In terminals where those glyphs actually render as one or two
cells, `JoinHorizontal` miscounts the width of any segment containing a
glyph and misaligns every segment to its right. On a status bar spanning
the full terminal width, one miscounted glyph cascades to a final row that
is one or two cells short or wraps.

**Poplar's counter-approach.** `View` in `status_bar.go:145-156` renders
each segment with its style, measures all rendered widths with
`lipgloss.Width`, computes `fillWidth = max(0, width - rightWidth)`,
renders the fill segment at exactly that width, then concatenates with plain
string concatenation (`fillPart + countsPart + outboxPart + ...`). No
`JoinHorizontal` call. This pre-measure + string-concatenation assembly is
SPUA-safe because the fill absorbs any glyph-width discrepancy without
triggering a second measurement pass. ADR-0084 codifies the ban on
`JoinHorizontal` when `spuaCellWidth != 1` and identifies teacup's
`JoinHorizontal` pattern as the canonical anti-pattern to avoid. This entry
confirms it by naming the exact call site.

**Applicability.** No — teacup's approach is the anti-pattern. Poplar's
pre-measure + fill is the correct alternative and is already in place.

---

### Diff candidates for Task 6 (status_bar)

No diffs reach the bar.

The domain mismatch between teacup's four fixed slots and poplar's six
semantic segments forecloses every structural harvest. The `ColorConfig`
pair convention is superseded by compiled themes. The `SetContent` batch
call has no poplar analog because callers update one concern at a time. The
`muesli/reflow/truncate` pattern is a near-duplicate of `ansix.TruncateEllipsis`,
which poplar already has, and the fill-absorbs-variance assembly makes a
hard cap on any segment unnecessary. The `JoinHorizontal` final-join is
actively rejected by ADR-0084. Nothing in the teacup statusbar source
reveals an obvious improvement to `status_bar.go` that does not weaken the
SPUA-safe assembly or the compiled-theme contract.

---

## `daltonsw/bubbleup`

Patterns harvested from `daltonsw/bubbleup@main` (`alert.go`, `model.go`,
`position.go`) during Pass 9x.2. Poplar consumer: `internal/ui/toast.go`
(`pendingAction` value type, `renderToast`, `toastExpireMsg` tick, chrome-
banner inline-row contract).

bubbleup has no queue model — one active alert per category, no payload
carries forward. The cascade-precedence question raised in the Pass 9x.2
starter (whether bubbleup's ordering rules inform poplar's App-level overlay
cascade) is App-level, not library-level. bubbleup has no queue and no
cascade. Nothing to harvest for cascade order.

---

### 1. Functional-options pattern for display configuration

**Where.** `model.go:41-70`. Four builder methods on `AlertModel` —
`WithPosition(p Position) AlertModel` (line 41), `WithMinWidth(minWidth int)
AlertModel` (line 47), `WithUnicodePrefix() AlertModel` (line 56),
`WithAllowEscToClose() AlertModel` (line 68) — each return a modified value
copy rather than mutating the receiver.

**Why it's good.** Functional options keep construction declarative and
discoverable: `NewAlertModel().WithPosition(TopRightPosition).WithUnicodePrefix()`
reads as a sentence. Each option is independently composable and testable;
no option order matters. The value-copy return prevents partial mutation
bugs in concurrent setup paths.

**Applicability.** No — `pendingAction` is set in a single assignment from
`triageStartedMsg` data; it has no configuration phase and no optional
display modes. The functional-options shape is suited to reusable components
that are constructed once and reused across many call sites (like
`AlertModel`). `pendingAction` is a short-lived value type owned by one
field on `App`.

---

### 2. `AlertDefinition` — per-category style table

**Where.** `alert.go:146-158`. `AlertDefinition` is a four-field struct:
`Key string`, `ForeColor string` (hex), `Style *lipgloss.Style` (optional),
`Prefix string` (optional). `registerDefaultAlertTypes` (lines 190-236)
inserts four entries (`InfoKey`, `WarnKey`, `ErrorKey`, `DebugKey`) into
`AlertModel.alertTypes map[string]AlertDefinition`. `newAlert` (lines 72-87)
looks up the definition by key at dispatch time and applies the resolved
`ForeColor`, `Prefix`, and `Style` to the new `alert` instance.

**Why it's good.** Separating category metadata from the active alert
instance means adding a new alert type requires one table entry and no
branching. The lookup-at-dispatch pattern keeps the active struct lean —
it carries only the resolved values, not the source definition. Consumers
can extend the type set at runtime via `RegisterNewAlertType` without
forking the render path.

**Applicability.** No — poplar's toast has one category: triage
confirmation. The op enum (`triageOp`) drives verb and undo-hint selection
through a `switch` in `renderToast` (`toast.go:54-72`), which is the
right shape for a fixed, small vocabulary. A `Definition` table would
add a layer of indirection for no gain. If a future pass adds a separate
category (e.g. a non-undoable server-side notification alongside
triage confirmations), the key-indexed definition table is the model to
follow.

---

### 3. Dismiss on Esc affordance

**Where.** `model.go:68-70` (`WithAllowEscToClose`) and `model.go:76-109`
(`Update`). When `allowEscToClose` is set, the `Update` method intercepts
`tea.KeyEsc` and clears the active alert. The guard prevents Esc from
propagating to the rest of the tree when an alert is showing.

**Why it's good.** The affordance is opt-in (off by default), so it does
not break callers that expect Esc to route to their own close logic. The
key-steal is intentional and scoped: only fires when an alert is active.

**Applicability.** No — poplar's toast is a triage-confirmation banner,
not a dismissable notification. The `[u undo]` affordance is the
interaction surface: `u` fires the inverse Cmd and implicitly commits or
reverses the action. Esc is not a toast interaction; it routes to the
viewer or modal layers per the overlay cascade. Grafting Esc-dismiss onto
the toast would confuse Esc's role as the universal overlay-close key.

---

### 4. Color-lerp tick — domain mismatch

**Where.** `alert.go:33` (`DefaultLerpIncrement = 0.18`), `alert.go:102-139`
(`render` method). Each 100 ms tick advances `curLerpStep` by
`DefaultLerpIncrement`. `render` calls `go-colorful.BlendLab(foreColor,
baseColor, curLerpStep)` to produce the current foreground color, creating
a fade animation over approximately 6 ticks (~600 ms). The background color
is fixed (`BackColor = "#000000"`).

**Why it's unsuitable.** The lerp tick is a *category-styled* animation
tied to alert identity — each alert type has its own foreground hue that
fades to a dark base. Poplar's toast is an informational triage confirmation,
not a categorized notification. Poplar uses no per-type hue; the toast
renders in a single `styles.Toast` compiled style from the theme. Importing
color animation would graft bubbleup's visual vocabulary (hue = severity)
onto a surface that uses brightness (FgDim, FgBright) as its primary visual
register. The domain mismatch is structural, not a calibration question.

**Applicability.** No — domain mismatch. Catalog only.

---

### 5. Chrome baked into the rendering primitive — anti-pattern

**Where.** `model.go:121-145` (`Render` method) and `position.go:1-45`
(`Position` type + six constants). `Render` takes the full terminal output
as a string, splits it into lines, and overlays the alert at the position
specified by the `Position` enum. The border style is applied inside
`alert.render` (`alert.go:102-139`) using `baseStyle` (a `lipgloss.Style`
with rounded border, defined at package scope, line 53). Position handling
and border chrome are both baked into the rendering primitive — the caller
has no way to separate "produce the alert string" from "overlay it at this
corner with this border."

**Why this is a problem.** Fusing overlay geometry and border styling into
the render primitive makes the component impossible to embed in a layered
compositor. Poplar's overlay system (`PlaceOverlay`, `DimANSI`) is a
post-render compositor: each component produces its own string, and App
composites them. A component that manages its own screen-position and
border chrome cannot participate in that pipeline without redundant
geometry negotiation. It also means the border cannot be themed through
the caller's style system — `baseStyle` is hardcoded at package scope.

**Poplar's counter-approach.** `renderToast` (`toast.go:49-94`) produces a
styled string via `styles.Toast.Render(...)` and returns it. Placement is
the caller's concern: `App.chromeBannerRow` inserts the toast string into
the chrome row between the error banner and the status bar. Chrome (border,
rounding) comes from the compiled theme, not from the render function.
This separation lets the toast participate in the App layout without any
geometry coupling.

**Applicability.** No — bubbleup's approach is the anti-pattern. Poplar's
string-return + caller-placement model is deliberate and correct. This
entry names what the project avoids.

---

### Diff candidates for Task 6 (toast)

No diffs reach the bar.

The domain gap between bubbleup and `toast.go` is wider than the teacup
gap: bubbleup is a category-styled animated notification overlay;
`toast.go` is a triage-confirmation banner with an undo payload. The five
patterns above are either already embodied in poplar's design (string-return
primitive, compiled styles), actively avoided (color lerp, chrome-baked
overlay), or suitable only for a multi-type extension that has no current
consumer (AlertDefinition table, functional options). The Esc-dismiss
affordance conflicts with poplar's overlay-close routing. No bubbleup
technique improves `toast.go` without importing a category-alert model that
the triage-confirmation surface does not need.

---

## Top three ways their code beats ours

Only one pattern across all three libraries reaches the "candidate for later"
bar with a clear poplar consumer. The other two "candidate for later" entries
(bubbles/help §3, teacup §3) address gaps that don't exist in the current
design or have no current trigger. The count is honest: one, not three.

1. **`shouldAddItem` loop shape for progressive truncation** (`bubbles/help`
   §3, `help.go:221-231`). The three-case handler — item fits / item overflows
   and ellipsis fits / both overflow — is the right model if helppopover ever
   needs to drop rightmost groups progressively rather than falling back to a
   whole-box `tooNarrow` string. There is no current trigger, but the pattern
   names the solution so it does not need re-derivation.

Two further "candidate for later" entries exist in the catalog but do not
reach the top-three bar:

- **`shouldRenderColumn` column-skip** (`bubbles/help` §6): useful only if
  helppopover gains a third context built from a shared group pool with
  per-group opt-in. The two-context model is stable; this is speculative.
- **`ansix.TruncateEllipsis` with a dynamic cap** (teacup §3): right if a
  variable-length segment is added to the status bar right side. No current
  trigger; the fill-absorbs-variance assembly makes it unnecessary today.

Pass 9z should receive the one honest recommendation and defer the two
speculative ones.

---

## What did NOT survive the harvest

- **`KeyMap` two-method interface** (`bubbles/help` §1): helppopover's
  named-group grid layout cannot be expressed as `ShortHelp() []key.Binding` /
  `FullHelp() [][]key.Binding` — the interface is linear; the layout is
  spatial.
- **`FullHelpView` `JoinHorizontal` column assembly** (`bubbles/help` §4):
  actively rejected by ADR-0084. The key/desc parallel-slice sub-pattern is
  interesting but unnecessary — helppopover's fixed `keyWidth = 5` does the
  same job with less mechanism.
- **`Ellipsis` style slot** (`bubbles/help` §3 partial): the styled
  configurable ellipsis tail is the right shape for a linear help bar;
  helppopover's whole-box `tooNarrow` fallback is the right shape for a
  modal grid. Different problems.
- **`ColorConfig{Foreground, Background}` pair struct** (teacup §1):
  superseded by compiled themes. Switching to runtime `AdaptiveColor` pairs
  would dismantle ADR compliance.
- **Four-slot positional statusbar shape** (teacup §4): domain mismatch.
  Poplar's six semantic segments with a left-side fill cannot map onto four
  fixed positional slots without collapsing unrelated concerns.
- **`JoinHorizontal` final-join for status bar assembly** (teacup §5):
  canonical ADR-0084 anti-pattern. Named here as the concrete call site the
  ADR was written against.
- **`SetContent` batch column setter** (teacup §2): no analog; poplar's
  callers update one semantic concern at a time in response to distinct Msg
  types.
- **Functional-options pattern** (bubbleup §1): suited to reusable components
  constructed once and shared across many call sites. `pendingAction` is a
  short-lived value type with no configuration phase.
- **`AlertDefinition` key-indexed type table** (bubbleup §2): one-category
  toast has no current need for a dispatch table; a `switch` on `triageOp`
  is the right shape for a fixed small vocabulary.
- **Esc-dismiss affordance** (bubbleup §3): conflicts with Esc's role as the
  universal overlay-close key in poplar's overlay cascade.
- **Color-lerp tick** (bubbleup §4): domain mismatch. Hue-as-severity visual
  vocabulary grafted onto a brightness-register triage banner.
- **Chrome-baked overlay** (bubbleup §5): anti-pattern. Fusing border and
  screen-position into the render primitive prevents participation in
  poplar's post-render compositor (`PlaceOverlay`, `DimANSI`).

---

## Cross-cutting code candidates

Nothing crosses the threshold. The patterns in this catalog are either
surface-specific (helppopover's named-group grid, toast's triage vocabulary,
status bar's fill-absorbs-variance assembly) or already embodied as distinct
implementations that serve different contracts. No helper spans two or more
of the three surveyed chrome surfaces at a call-site density that would
justify extraction.

The `uicore.ListBodyRows` extraction candidate from 9x.1 remains the only
pending cross-cutting candidate; that surface is `internal/ui/`, not the
chrome family. Re-evaluate at the 9x.3 (table/form) catalog if a fourth
modal list appears.
