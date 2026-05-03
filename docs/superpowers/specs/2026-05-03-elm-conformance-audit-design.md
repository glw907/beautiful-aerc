# Pass 8.5b — Elm conformance audit (spec)

**Date:** 2026-05-03
**Status:** approved
**Pass:** 8.5b
**Predecessor:** Pass 8.5 (overengineering audit; ADR-0125/0126/0127)

## Goal

Audit `internal/ui/` for conformance to the elm-conventions skill
and surface substantial refactor opportunities. Apply fixes inline
per the pre-beta posture (CLAUDE.md "Release stance"): clean code
outweighs stability, schema/interface/migration changes are
welcomed, no compat shims, no churn-cost framing. This is one of
the last passes where breakage in service of correctness is
explicitly endorsed before the v0.9.0 → v1.0.0 stabilization.

## Charter

The audit is broader than rule-checklist conformance. Each finding
falls into one of two streams:

1. **Conformance** — violations of the seven elm-conventions rules
   plus the cmd-closure capture rule.
2. **Refactor** — type-design weaknesses, state-shape problems,
   component-boundary problems, Update-method shape problems,
   duplicated idioms, and dead/vestigial code.

Both streams produce findings in the same audit document. Both
streams flow into the same fix sweep. Conformance findings are
fixed without prior approval (the rules are settled). Refactor
proposals are presented for go/no-go before execution; those
declined or deferred are recorded with rationale and either queued
in STATUS or dropped.

## Scope

- **In:** every non-test `.go` file under `internal/ui/`.
- **Out:** `*_test.go` files, every package outside `internal/ui/`.

Test files are excluded because they're not part of the tea loop
and their patterns (helpers, golden capture) follow different
norms. If a fix to production code requires a test update, the
test changes go with the production change.

## Phase 1 — Parallel discovery

Dispatch four `Explore` subagents over disjoint file groups. Each
receives the same charter prompt and returns a structured findings
list.

### File groups

- **Group A — root + chrome:** `app.go`, `account_tab.go`,
  `top_line.go`, `footer.go`, `status_bar.go`, `layout.go`,
  `error_banner.go`, `dim.go`, `overlay.go`
- **Group B — mail surfaces:** `sidebar.go`, `sidebar_search.go`,
  `msglist.go`, `viewer.go`
- **Group C — modals & pickers:** `help_popover.go`,
  `linkpicker.go`, `movepicker.go`, `confirm_modal.go`, `toast.go`
- **Group D — helpers & cross-cutting:** `cmds.go`, `keys.go`,
  `styles.go`, `icons.go`, `iconwidth.go`, `date_format.go`

### Subagent prompt template

Each subagent gets:

- The seven elm-conventions rules verbatim from
  `~/.claude/skills/elm-conventions/SKILL.md`, plus the
  cmd-closure capture rule.
- The expanded refactor charter (type design, state shape,
  component boundary, Update shape, duplicated idioms, dead code).
- Its assigned file list.
- Output format: a YAML or markdown-table block with one row per
  finding: `file`, `line` (or line range), `stream`
  (`conformance` / `refactor`), `rule_or_category`, `severity`
  (`architectural` / `mechanical` / `cosmetic` for conformance;
  `major` / `minor` for refactor), `description`, `proposed_fix`,
  optional `notes`.
- Read-only: subagents do not edit files.

## Phase 2 — Consolidation

The audit document is written to
`docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md`
(this file) under a "Findings" section appended after Phase 1
completes. Structure:

- **Conformance findings table** (severity-ordered).
- **Refactor proposals** (one subsection per proposal: motivation,
  files touched, risk, rough size, recommendation).
- **Cross-cutting themes** — patterns that recur across two or
  more components and want a single shared fix.
- **Negative results** — explicit "no findings of category X"
  statements where they apply, so the audit is provably exhaustive
  rather than ambiguously empty.

Severity definitions:

- **Architectural** (Rules 1-5): package-level mutable state,
  mutation in `View`/`Init`/Cmd-closure, blocking I/O in `Update`,
  callback-based child→parent communication, duplicated ownership
  of shared state. Mandatory fix.
- **Mechanical** (Rules 6-7 + cmd closure): defensive parent-side
  `MaxWidth`/clip, missing wordwrap+hardwrap pairing, `len()` in
  width math on icon-bearing strings, `switch msg.String()` for
  KeyMsg dispatch, Cmd closures capturing `*Model` pointers
  instead of extracted values. Mandatory fix.
- **Cosmetic**: nit-level (e.g., `len()` on guaranteed-ASCII where
  correctness isn't at risk). Fix only if trivial.
- **Major refactor**: changes a type signature, splits a
  component, or moves state across the model tree.
- **Minor refactor**: localized rename, helper extract/inline,
  field re-typing.

## Phase 3 — Fix sweep

Two sub-phases, in order:

1. **Conformance fixes** — apply all architectural and mechanical
   findings. Cosmetic fixes only when trivial. Commit-per-theme so
   diffs stay reviewable. `make check` green between commits.
2. **Refactor execution** — present the proposal list for
   go/no-go, then apply approved proposals. Each proposal is its
   own commit.

If a proposal is large enough to warrant its own pass, it gets
queued in STATUS (e.g., "Pass 8.5c — viewer link mode extraction")
rather than stuffed into 8.5b.

Per pre-beta posture: no compat shims, no `// removed`
breadcrumbs, no preserved-for-future fields. Breaking renames are
fine; the commit message and (if behavior changed) an ADR carry
the rationale.

## Phase 4 — Pass-end ritual

Standard `poplar-pass` end-of-pass checklist:

- `simplify` skill on the cumulative diff.
- ADR(s) only when a *new* binding decision emerges. Conformance
  fixes do not need ADRs — the existing Elm-architecture ADRs
  already cover them. A refactor that codifies a previously-
  unspecified pattern earns one.
- `invariants.md` edit only if a fact changed.
- `STATUS.md`: flip 8.5b → done; queue 8.4c (already drafted) as
  next.
- Archive plan + spec via `git mv` to
  `docs/superpowers/archive/{plans,specs}/`.
- `make check` → commit → push → `make install`.

## Success criteria

- Every file in scope has been audited (positively or negatively).
- All architectural and mechanical conformance findings are fixed
  on master.
- Every refactor proposal has a disposition: applied, deferred
  (queued in STATUS with a starter prompt), or declined (with
  one-line rationale in the audit doc).
- `make check` green.
- The audit doc, archived under `docs/superpowers/archive/specs/`,
  stands as the historical record of the pass's findings.

## Non-goals

- Adding regression-prevention infrastructure (grep-based hooks,
  `make elm-check` target). The pass-end checklist (step 1b in
  `poplar-pass`) plus reviewer discipline already cover this.
- Auditing test files.
- Auditing packages outside `internal/ui/`.
- Touching bubbletea-conventions §10 review checklist items
  beyond what overlaps with Rules 6-7 (size contract, key
  bindings). Those have their own checklist baked into every UI
  pass-end ritual.

## Risks

- **Subagent finding overlap or contradiction.** Mitigated by
  disjoint file groups and a single consolidation step where
  conflicts are resolved by re-reading the file.
- **Refactor scope creep.** Mitigated by the go/no-go gate before
  Phase 3.2 and the "queue in STATUS as own pass" escape hatch.
- **Test fragility from architectural refactors.** Acceptable per
  pre-beta posture. Tests that break get rewritten in the same
  commit as the production change.

---

## Findings

Discovery output from four parallel `Explore` subagents (Groups
A–D), consolidated. Each row was sanity-checked by re-reading the
flagged source. Duplicates across groups merged; one finding
(`maybeRetentionSweep` calling `time.Now()` directly) reclassified
from R3 → state-shape minor — `time.Now()` is non-blocking and the
real concern is bypassing the existing `App.now` seam.

### Conformance findings

| file | line | rule | severity | description | proposed fix |
|------|------|------|----------|-------------|--------------|
| cmds.go | 333 | R1 | architectural | `var openURL = func(url string) error { … }` is a package-level mutable function pointer (test swaps it). | Replace with a value injected at construction (e.g. `App.openURL func(string) error`); test passes a stub via constructor option. |
| confirm_modal.go | 18 | R4 | architectural | `ConfirmRequest.OnYes func() tea.Msg` is a callback stored in the child model (set in `app.go:139` for empty-folder confirm). Child executes parent-supplied logic at Update time instead of returning a sentinel Msg. | Drop `OnYes`; have `ConfirmModal.Update` return a typed `ConfirmModalYesMsg{}` on `y`. App keeps a single `pendingConfirm` discriminator (`confirmKind` enum) it set when it called `Open`, and on `ConfirmModalYesMsg` dispatches the right downstream Msg (e.g. `EmptyFolderConfirmedMsg`). |
| help_popover.go | 232,239,247,254,270,317 | R6 | architectural | Uses `lipgloss.JoinHorizontal` / `JoinVertical` for layout. Per ADR-0084 / invariants.md, these are forbidden in this codebase (kept out under both icon modes); replace with row-by-row `strings.Join` over pre-padded children. | Rewrite each call: pre-pad each column to a fixed cell width via `displayCells`, then `strings.Join(rows, "\n")`. |
| viewer.go | 141–147 | R2 | architectural | `LinkPickerRequest()` is a `*Viewer` accessor that mutates `v.pendingLinkPicker` outside `Update`. Doc-comment even names it: "Pointer receiver because the accessor mutates the one-shot pending field." Mutations outside `Update` are exactly what R2 forbids. | Remove `pendingLinkPicker` field. On Tab, `Viewer.Update` returns `ViewerOpenLinkPickerMsg{Links: …}`; `App.Update` handles the msg and opens the picker directly. |
| app.go | 102–113 | R6 | mechanical | `WindowSizeMsg` handler calls `SetSize` on overlays but does not forward the msg into their `Update` methods. Rule 6 mandates SetSize **and** forward. | After each `SetSize`, also call `child.Update(msg)` and batch the cmd. (Plus `HelpPopover` needs a `SetSize` method — see refactor R-help-sized.) |
| app.go | 313 | R4 | mechanical | App synthesizes a fake `tea.KeyMsg{Type: tea.KeyEsc}` and pushes it into `AccountTab.Update` to clear sidebar search on `q`. Undocumented wire protocol between parent and child. | Define a typed `ClearSidebarSearchMsg{}`; AccountTab handles it explicitly in `updateTab`. |
| app.go | 355 | R2 | mechanical | `View()` calls `m.footer.SetCounter(m.acct.WindowCounter()).View(m.width)`. Returned Footer copy is discarded — currently a no-op since SetCounter is a pure setter, but the shape is wrong (View deriving transient state). | Move the SetCounter call into `deriveChromeFromAcct` so View only renders. |
| account_tab.go | 301–306 | R7 | mechanical | Search-shelf Enter/Esc transitions dispatch on raw `msg.Type == tea.KeyEnter` / `tea.KeyEsc` instead of `key.Matches`. | Add `SearchCommit` and `SearchCancel` `key.Binding`s to `AccountKeys`; dispatch via `key.Matches`. |
| account_tab.go | 464–478, 451–457, 413–418 | R2 | mechanical | `selectionChangedCmds`, `clearSearchIfActive`, `cancelInflightBodyFetch` are pointer-receiver helpers called from value-receiver Update paths. They mutate `m.pages` / `m.loading` / `m.shelf` through the pointer, making mutation invisible at the call site. | Convert each to value-receiver returning `(AccountTab, …)`; assign at call site so mutation is explicit. |
| sidebar_search.go | 110 | R7 | mechanical | Tab key dispatched via raw `key.Type == tea.KeyTab` rather than `key.Matches` against a declared `key.Binding`. No KeyMap entry exists for the mode-cycle action — invisible to help renderer, can't be disabled. | Add `CycleMode key.Binding` (`key.WithKeys("tab")`) to a sidebar-search KeyMap; dispatch with `key.Matches`. |
| msglist.go | 256 | R6 | mechanical | `[all]` search mode searches `msg.Date` (the legacy RFC2822 wire string) instead of `row.dateText` (the rendered relative-date string the user actually sees). Doc-comment says "+date text" but the implementation searches the wire field. | Filter at the row layer (after `dateText` is computed): pass `dateText` into `matchMessage` or move filter into `rebuild`. |
| linkpicker.go | 115–116 | R7 | mechanical | Digit keys `1`–`9` dispatched via `keyMsg.String()` byte comparison. | Add nine `key.Binding`s (or one binding listing keys `"1".."9"`) to `linkPickerKeys`; dispatch with `key.Matches`. |
| linkpicker.go | 170–177 | R2 | mechanical | `LinkPicker.Box` (called from `View`) writes `p.offset` on the value-receiver copy. Mutation is silently discarded (value semantics). State that should live across renders is being recomputed in the render path. | Move scroll-offset clamping into `Update` after cursor moves; `Box` reads `p.offset` only. |
| linkpicker.go | 180 | R6 | mechanical | `len(title)` for width math on `" Links "`. Convention is `lipgloss.Width`. | Replace with `lipgloss.Width(title)`. |
| movepicker.go | 144 | R7 | mechanical | `q`-swallow uses `keyMsg.String() == "q"`. | Add `Swallow` (or `Quit`) `key.Binding` to `movePickerKeys`; dispatch with `key.Matches`. |
| movepicker.go | 192–205 | R2 | mechanical | `MovePicker.Box` writes `p.offset` in the render path. Same defect as LinkPicker. | Move offset clamping into `Update`. |
| movepicker.go | 88 | R2 | mechanical | `recompute` is a `*MovePicker` receiver called from value-receiver methods (`Open`, `Update`). Auto-addressing makes it work, but the mixed receiver shape contradicts the value-everywhere model used elsewhere on the type. | Convert to value receiver returning `MovePicker`. |
| sidebar_search.go | 215 | R6 | cosmetic | `runewidth.StringWidth` on plain ASCII labels (`modeLabel`, `countText`). Result is correct; convention is `lipgloss.Width`. | Replace. |
| msglist.go | 877 | R6 | cosmetic | `runewidth.StringWidth(row.prefix)` on box-drawing characters (all 1-cell). Convention is `lipgloss.Width`. | Replace. |
| confirm_modal.go | 129 | R6 | cosmetic | Magic constant `3` for box-border width math. | Extract `const borderOverhead = 3`. |

### Refactor proposals

#### R-toplinetoast: Remove dead `TopLine.SetToast` / `ClearToast`

- **Motivation:** `TopLine.toast`, `SetToast`, `ClearToast` are unreferenced in production. Toast rendering lives in `App.chromeBannerRow` / `toast.go`, not the top line. Vestigial since toast rework.
- **Files touched:** `top_line.go`, `top_line_test.go`.
- **Risk:** Trivial. Verified by `grep`: no production caller.
- **Size:** ~20 LOC removed.
- **Recommendation:** apply this pass.

#### R-triageop: Type-design — `triageOp` typed constants

- **Motivation:** Triage op identity is a string discriminator (`"delete"`, `"archive"`, `"star"`, `"read"`, `"empty"`) used in three switch tables: `account_tab.dispatchTriage`, `toast.toastVerb`, `pendingAction.op`. A typo or new variant silently falls through `default`.
- **Files touched:** `account_tab.go`, `toast.go`, `app.go`, tests.
- **Risk:** Low. Mechanical rename + compiler-checked exhaustiveness on the resulting switches.
- **Size:** ~50 LOC.
- **Recommendation:** apply this pass.

#### R-foldergroup: Drop `ui.FolderGroup`, use `mail.Group` directly

- **Motivation:** `ui.FolderGroup` is a parallel enum to `mail.Group` with identical semantics. `translateGroup` exists solely to convert. UI layer can use `mail.Group` directly.
- **Files touched:** `sidebar.go`, `movepicker.go`, tests.
- **Risk:** Low. Pure rename + delete.
- **Size:** ~30 LOC net deletion.
- **Recommendation:** apply this pass.

#### R-modalshell: Extract a shared modal-shell helper

- **Motivation:** Four overlays (LinkPicker, MovePicker, ConfirmModal, HelpPopover) repeat the same `┌─ title ─┐ … └─┘` box-drawing frame, `IsOpen`/`Open`/`Close` lifecycle, and `centerOverlay` delegation. Each new overlay duplicates ~30 LOC of frame boilerplate.
- **Files touched:** new `modal_shell.go`; `linkpicker.go`, `movepicker.go`, `confirm_modal.go`, `help_popover.go`.
- **Risk:** Medium — touches every overlay, easy to regress border math.
- **Size:** ~150 LOC added, ~200 removed (net win).
- **Recommendation:** **queue as own pass** (Pass 8.5c — "modal shell extraction"). Outside the audit-then-fix scope of 8.5b; benefits from focused attention with golden tests per overlay before/after.

#### R-cmds-msgmove: Move component-state types out of `cmds.go`

- **Motivation:** `cmds.go` mixes I/O cmd builders with domain-state type definitions: `SearchMode`/`SearchState`/`SearchUpdatedMsg` (sidebar-search-only) and `OpenMovePickerMsg`/`MovePickerPickedMsg`/`MovePickerClosedMsg` (move-picker-only). Layer mixing.
- **Files touched:** `cmds.go`, `sidebar_search.go`, `movepicker.go`.
- **Risk:** Trivial — pure move.
- **Size:** ~60 LOC moved.
- **Recommendation:** apply this pass.

#### R-help-sized: `HelpPopover` stores width/height like other overlays

- **Motivation:** Three other overlays store dims and expose `SetSize`; `HelpPopover` requires `(width, height)` threaded through every `Box`/`View` call. Inconsistent and means App's `WindowSizeMsg` handler can't `SetSize` it. (Pairs with the R6 forwarding fix for overlays.)
- **Files touched:** `help_popover.go`, `app.go`.
- **Risk:** Low.
- **Size:** ~30 LOC.
- **Recommendation:** apply this pass.

#### R-pickerstate: Hoist picker scroll state out of `Box`

- **Motivation:** Subsumed by the R2 mechanical fixes on linkpicker/movepicker `Box`. Tracked here as a refactor only because the architectural fix doubles as a structural cleanup.
- **Recommendation:** rolled into the M-N conformance commits, not a separate refactor.

#### R-sidebarcol: Extract sidebar-column assembly

- **Motivation:** `AccountTab.View` (lines 770–833) manually assembles the sidebar column: blank rows, account line, folder padding, shelf clamping, row-by-row join with the divider. Belongs inside a `SidebarColumn` component or helper.
- **Files touched:** `account_tab.go`, possibly new `sidebar_column.go`.
- **Risk:** Medium — view assembly with subtle padding behavior; needs golden tests.
- **Size:** ~80 LOC moved.
- **Recommendation:** **decline** for 8.5b — the current assembly is correct and tested, and the refactor doesn't unlock anything Pass 8.5b's charter promises. Worth doing eventually but not driven by an audit finding. Re-evaluate if a future pass changes sidebar layout.

#### R-msglist-update: Give `MessageList` a real `Update` method

- **Motivation:** `MessageList` has no `Update`; AccountTab type-switches and calls many specific MessageList methods. As surface grows (visual mode, fold, filter, nav), AccountTab is the fat dispatch for both.
- **Files touched:** `msglist.go`, `account_tab.go`, tests.
- **Risk:** High — MessageList is the most-tested UI component; changing its shape ripples.
- **Size:** ~150 LOC.
- **Recommendation:** **decline.** Bubbles `list` is not the right analogue for the threading-aware msglist (which owns fold/group state); the current direct-method shape is honest about that. Keep AccountTab as the dispatch site.

#### R-viewer-receivers: Pick one receiver style for `Viewer`

- **Motivation:** Mixed pointer/value receivers across Viewer methods. `Open`/`Close`/`SetBody`/`SetSize` are value (caller assigns); `layout()` is pointer (called on local copy inside SetBody/SetSize); `LinkPickerRequest` is pointer (mutates outside Update — see R2 architectural fix).
- **Files touched:** `viewer.go`.
- **Risk:** Low once R2 architectural fix lands (which removes `LinkPickerRequest`).
- **Size:** ~15 LOC.
- **Recommendation:** rolled into the architectural Viewer fix; not a separate refactor.

#### R-search-msg: Convert sidebar-search to Msg-driven control flow

- **Motivation:** `SidebarSearch.Activate`/`Commit`/`Clear` are direct method calls from `AccountTab.Update`, documented as intentional. Per R4, conformant pattern is child returning `SearchActivatedMsg` / `SearchCommittedMsg` / `SearchClearedMsg`.
- **Recommendation:** **decline.** Existing pattern is documented and the search shelf tightly co-evolves with AccountTab; turning it Msg-driven would multiply state transitions without any reuse benefit (no other parent will host this child). Acknowledged deviation.

#### R-canonicaldefaultrank: Convert `canonicalDefaultRank` map var to a switch

- **Motivation:** Package-level `var` lookup map mutable by anything in the package. Pure constant table.
- **Files touched:** `sidebar.go`.
- **Risk:** Trivial.
- **Size:** ~15 LOC.
- **Recommendation:** apply this pass.

#### R-now-seam: Use `App.now` seam in retention sweep

- **Motivation:** `account_tab.maybeRetentionSweep` calls `time.Now()` directly while the App-level seam `m.now func() time.Time` exists for testability. Threaded `now` makes the sweep deterministic in tests.
- **Files touched:** `account_tab.go`, tests.
- **Risk:** Low.
- **Size:** ~10 LOC.
- **Recommendation:** apply this pass.

#### R-icons-frozen: Document/freeze icon-table mutability

- **Motivation:** `SimpleIcons` and `FancyIcons` are package-level `var IconSet` values. They are intended to be init-only but nothing in the type prevents mutation.
- **Recommendation:** **decline.** ADR-0084 already names the icon-table contract; no caller mutates them, and freezing via accessor function adds boilerplate without enabling anything. Convention is sufficient.

#### R-viewerheader-cache: Drop `Viewer.headerStr` or `Viewer.panel`

- **Motivation:** `headerStr` and `panel` duplicate state — `panel` is always `styles.ViewerHeader.Width(v.width).Render(headerStr)`, recomputed in `layout()`. Only `panel` is read by `View`.
- **Files touched:** `viewer.go`.
- **Risk:** Low.
- **Size:** ~10 LOC.
- **Recommendation:** apply this pass.

#### R-status-clamp: Eliminate status-bar clamp loop

- **Motivation:** `status_bar.View` has a corrective clamp loop (lines 143–151) for width-math drift between styled-segment widths and the plain-text template. Suggests measurement is being done with the wrong primitive.
- **Files touched:** `status_bar.go`.
- **Risk:** Medium — rendering correctness needs golden tests.
- **Size:** ~30 LOC.
- **Recommendation:** **queue as own pass** if it survives `/simplify`; otherwise decline. Defer the call to disposition step.

### Cross-cutting themes

- **String-typed discriminators where a typed sum belongs.** Triage op (`"delete"`/`"archive"`/`"star"`/`"read"`/`"empty"`) appears in three switch tables. ConfirmModal's `OnYes` callback hides what would otherwise be a `confirmKind` enum on App. Single shared fix: introduce typed enums and let the compiler check exhaustiveness. Captured as R-triageop and R-OnYes-removal (the latter is the architectural R4 fix).
- **R7 misses on locally-declared keys.** Three places dispatch on `keyMsg.String()` / `keyMsg.Type`: linkpicker digits, movepicker `q`, sidebar-search Tab, account-tab search Enter/Esc. Pattern: a key is "obviously" a single literal so the developer skipped declaring a `key.Binding`. Single shared fix in M-N theme: declare bindings, dispatch via `key.Matches`.
- **Render-path mutation in pickers.** Both `LinkPicker.Box` and `MovePicker.Box` write `p.offset` while rendering. Single shared fix: hoist offset clamping into Update.
- **Children mutated through pointer-receiver helpers from value-receiver Update.** `account_tab.go` has three: `selectionChangedCmds`, `clearSearchIfActive`, `cancelInflightBodyFetch`. Single shared fix: convert all to value-receiver-returning shape.
- **Modal/picker boilerplate.** Four overlays. Captured as R-modalshell, queued as Pass 8.5c rather than landed inline.

### Negative results

Files audited with zero findings:

- `layout.go`
- `error_banner.go`
- `dim.go`
- `overlay.go`
- `iconwidth.go` (the `spuaCellWidth` mutable var is ADR-0084-exempt)

Categories with no findings of the listed severity:

- **No R3 violations.** No blocking I/O in `Update` or `View` was identified anywhere in `internal/ui/`. (Discovery initially flagged `time.Now()` calls in Update; reclassified — `time.Now()` is non-blocking and idiomatic. The real concern is bypassing `App.now`, recorded as R-now-seam.)
- **No R5 violations.** No duplicated ownership of shared state was identified. `cache.Account` and `mail.Backend` references are held only at the layers documented in invariants.md.
- **No R1 violations beyond `cmds.go:openURL`.** `iconwidth.go:spuaCellWidth` is exempt; `icons.go:SimpleIcons`/`FancyIcons` are init-only by convention (declined as R-icons-frozen); `sidebar.go:canonicalDefaultRank` is captured as a minor refactor.
- **No callback-style child→parent communication beyond `ConfirmRequest.OnYes`.** Sidebar-search uses direct method calls (R-search-msg, declined as documented deviation), but no other child holds a parent-supplied callback.

