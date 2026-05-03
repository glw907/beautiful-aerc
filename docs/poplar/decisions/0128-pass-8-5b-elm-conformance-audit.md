---
title: Pass 8.5b Elm conformance audit summary
status: accepted
date: 2026-05-03
---

## Context

Sibling audit to Pass 8.5 (overengineering, ADR-0125–0127), focused
exclusively on `internal/ui/`. Two streams: conformance against the
seven elm-conventions rules + cmd-closure capture rule, and refactor
opportunities (type design, state shape, component boundary, Update
shape, duplicated idioms, dead code). Run as four parallel `Explore`
discovery agents over disjoint file groups followed by a single fix
sweep, per
`docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md`.

## Decision

Apply every architectural and mechanical conformance finding;
selectively apply trivial cosmetic fixes; apply nine of fourteen
refactor proposals inline; queue one as Pass 8.5c; decline four.

Architectural fixes (Rules 1–5):

- **Rule 1**: replace `var openURL func(string) error` package var
  with a `URLOpener` type held on `App` (default `xdgOpenURL`); tests
  inject via `App.WithOpener`. Mirrors the existing `App.now` seam.
- **Rule 2**: remove `Viewer.LinkPickerRequest` accessor that mutated
  `v.pendingLinkPicker` outside `Update`; `Viewer.handleKey` now emits
  a typed `OpenLinkPickerMsg{Links}` Cmd that App handles directly.
  Drops the parallel `AccountTab.pendingLinkPicker` mirror field too.
- **Rule 4**: `ConfirmRequest.OnYes func() tea.Msg` callback removed.
  `ConfirmModal.Update` now emits a typed `ConfirmModalYesMsg{}`; App
  carries `pendingEmptyConfirm{folder, source}` set when the modal
  opens and dispatches `EmptyFolderConfirmedMsg` on `ConfirmModalYesMsg`
  if the pending payload is non-empty. Establishes the
  defunctionalized-callback pattern: child returns a typed Yes msg,
  parent stores a typed payload struct keyed by which `Open` call
  it made.
- **Rule 6**: `lipgloss.JoinHorizontal/JoinVertical` removed from
  `help_popover.go` (per ADR-0084). New helper `joinColumnsRow(gap,
  cols ...string)` splits each pre-rendered column, pads to the
  column's max display-cell width via `lipgloss.Width`, and joins
  row-by-row.

Mechanical fixes:

- Rule 7: declare `key.Binding`s for the four places dispatching on
  `msg.String()`/`msg.Type` (linkpicker digits 1–9, movepicker `q`,
  sidebar-search Tab cycle, account-tab search Enter/Esc).
- Rule 6: forward `WindowSizeMsg` into linkPicker/movePicker/confirm
  Updates (was `SetSize`-only); add `HelpPopover.SetSize` so the
  parent calls match for every overlay.
- Rule 2: hoist picker scroll-offset clamping (was in `Box`) into
  `Update` via a new shared `clampScrollOffset(cursor, visible,
  offset int) int` free function. Convert the
  pointer-receiver-on-value-receiver helpers (`AccountTab.cancel
  InflightBodyFetch`, `clearSearchIfActive`, `selectionChangedCmds`,
  `MovePicker.recompute`) to value-receiver-returning shape.
- Rule 4: replace synthetic `tea.KeyMsg{KeyEsc}` injection used by
  App→AccountTab to clear sidebar search with a typed
  `ClearSidebarSearchMsg{}`.
- Rule 2: move `m.footer.SetCounter` out of `App.View` into
  `deriveChromeFromAcct`.

Applied refactors (Pass 8.5b R-N commits):

- R-1: drop dead `TopLine.SetToast` / `ClearToast` / `toast` field.
- R-2: introduce typed `triageOp` enum (`opDelete`, `opArchive`, …,
  `opEmpty`); `pendingAction.op`, `triageStartedMsg.op`,
  `dispatchTriage`, `queueMove`, `queueFlag`, `startTriageCmd`, and
  `toastVerb`/`renderToast` switches all use the typed constants.
  Compiler now catches typos in the toast set.
- R-3: drop `ui.FolderGroup` parallel enum and `translateGroup`
  function; `FolderEntry.Group` is `mail.Group` directly.
- R-4: move component-state types out of `cmds.go` —
  `SearchMode`/`SearchState`/`SearchUpdatedMsg` to
  `sidebar_search.go`; `OpenMovePickerMsg`/`MovePickerPickedMsg`/
  `MovePickerClosedMsg` to `movepicker.go`. `cmds.go` now holds only
  I/O Cmd builders + cross-component Msg types whose definitions
  don't fit a single component.
- R-5: `HelpPopover` stores width/height + `SetSize`, mirroring the
  other overlays; App's WindowSizeMsg branch sets it.
- R-6: replace package-level `var canonicalDefaultRank map[string]int`
  with a switch in `rankOf`. Constant lookup table no longer mutable
  from anywhere in the package.
- R-7: thread `now func() time.Time` seam onto `AccountTab` (default
  `time.Now`, `WithNow` setter); `maybeRetentionSweep`'s cutoff uses
  `m.now()` instead of calling `time.Now()` directly.
- R-8: drop `Viewer.headerStr` (kept `Viewer.panel`); `layout()`
  derives header text as a local variable.
- S-1 (post-`/simplify`): drop redundant `pendingEmpty.active` field;
  extract `clampScrollOffset` shared free function; replace per-
  iteration `lipgloss.Width` in `joinColumnsRow`'s padding loop with
  a single measure + `strings.Repeat`.

Queued as Pass 8.5c:

- R-modalshell: extract a shared modal-shell helper. Four overlays
  (LinkPicker, MovePicker, ConfirmModal, HelpPopover) each duplicate
  ~30 LOC of `┌─ title ─┐`/`├─┤`/`└─┘` box-drawing, `IsOpen`/`Open`/
  `Close`/`SetSize` lifecycle, and `centerOverlay` delegation. Out
  of audit scope; benefits from focused attention with golden tests
  per overlay.

Declined:

- R-sidebarcol (extract sidebar-column assembly): unlocked by no
  audit finding; current code correct + tested. Re-evaluate at next
  sidebar layout pass.
- R-msglist-update (give MessageList a real Update method): bubbles
  `list` is the wrong analogue for the threading-aware msglist; the
  current direct-method shape is honest about that.
- R-search-msg (Msg-driven sidebar search): documented intentional
  deviation; no reuse benefit (no other parent will host the child).
- R-icons-frozen (freeze icon-table mutability): ADR-0084 already
  names the contract; no caller mutates them.
- R-status-clamp (eliminate status-bar clamp loop): one-shot defensive
  correction; eliminating it would require reworking the layout
  pipeline. Re-evaluate at next status-bar pass.

## Consequences

`internal/ui/` is now mechanically conformant to the seven elm-
conventions rules. The audit doc archived under
`docs/superpowers/archive/specs/` stands as the historical record.

The defunctionalized-callback pattern (typed Yes msg + typed pending
payload on parent) is now the canonical replacement for `func() tea.Msg`
callbacks stored in child models. Future modal/picker patterns
should use this shape, not callback fields.

The cumulative diff is ~720 insertions / ~458 deletions. Most of
the line growth is documentation (the audit findings + dispositions
appended to the spec) and the `R-2` typed-enum boilerplate; the
production-code-only delta is small.

Pass 8.4c (Cache III) is unblocked by this pass: the surfaces it
touches (msglist, status bar, App overlay routing) now have stable
contracts.
