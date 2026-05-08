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
