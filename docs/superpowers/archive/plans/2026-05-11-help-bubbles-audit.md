# Pass 17c — `bubbles/v2/help` audit + bubbles-deviation ADRs

## Goal

Audit poplar's help-popover wiring against `bubbles/v2/help`,
adopt where it composes, and ADR every deviation that survives.
Closes the bubbles-adoption arc (15a/17a/17b done).

## Audit summary

`bubbles/v2/help` (v2.1.0) offers:

- `KeyMap` interface: `ShortHelp() []key.Binding` + `FullHelp() [][]key.Binding`.
- `ShortHelpView` — single-line `key · desc · key · desc …` with
  width truncation + ellipsis.
- `FullHelpView` — `[][]key.Binding` rendered as columns of
  `keys\n…` next to `descs\n…` joined via `lipgloss.JoinHorizontal`.
- One width, one set of styles, no group headers, no row prefixes,
  no per-binding wired/unwired flag (disabled bindings drop, not dim).

`helppopover.Model` needs:

1. **Wired vs unwired dimming** — ADR-0072 binding. `bubbles/v2/help`
   has no slot for this; `key.Binding.SetEnabled(false)` drops the
   row rather than dimming it.
2. **Group headers** — "Navigate", "Triage", "Reply", "Search",
   "Select", "Threads", "Go To" are rendered as styled headings
   above each column. `bubbles/v2/help` has no concept.
3. **3×2 Go To grid** — six folder jumps in a balanced 3×2 layout,
   not a column.
4. **Embedded title in border** — `╭─ Message List ──────╮` is one
   atomic row drawn manually.
5. **Cached render** — heap-allocated `*cache` keyed on
   `(context, w, h)` per ADR-0130 (view-stable overlay escape hatch).
6. **`lipgloss.JoinHorizontal` is forbidden** when `spuaCellWidth
   != 1` (ADR-0084). `bubbles/v2/help.FullHelpView` uses it
   internally — adopting it would regress SPUA-A safety.

Adoption verdict: **none of `bubbles/v2/help` lifts cleanly**. The
popover's custom rendering pipeline is justified at every step.
Pass 17c ADRs the deviation and ships the adjacent cleanup.

## Tasks

T1. **ADR-0200: help popover does not adopt `bubbles/v2/help`.**
    Records the six concrete reasons above. Status: accepted. Links
    ADR-0072 (wired/unwired) and ADR-0084 (JoinHorizontal trust)
    as the binding constraints.

T2. **ADR-0201: help popover is not a `uicore.ModalShell` consumer.**
    Documents the rounded-border-with-embedded-title rendering as a
    surviving deviation from the modal-shell family (ConfirmModal,
    LinkPicker, AttachPicker, MovePicker, OutboxOverlay,
    ConflictOverlay). Reason: the title-in-border treatment plus
    the manual top-edge composition needed for SPUA-safe assembly
    do not fit `ModalShell.Box(title, bodyRows, footerRows,
    contentW)`. Already noted implicitly in
    `.claude/rules/ui-invariants.md` (Overlays section); ADR makes
    the rationale binding.

T3. **Remove `account.keys.MsgListTop/Bottom/Down/Up`.** Dead since
    ADR-0199 routed nav through `messagelist.KeyMap`. The
    forwarding case at `internal/ui/account/model.go:407` becomes
    `case key.Matches(msg, mlKeys.Down), key.Matches(msg, mlKeys.Up),
    key.Matches(msg, mlKeys.Top), key.Matches(msg, mlKeys.Bottom):`
    where `mlKeys := m.msglist.KeyMap()`.

T4. **Update `docs/poplar/decisions/INDEX.md`** with rows for
    ADR-0200 and ADR-0201.

T5. **Invariants.md edit-in-place.** The Architecture/Overlays
    discussion in `.claude/rules/ui-invariants.md` already notes
    "helppopover.Model uses lipgloss.Style with a rounded border and
    is not a ModalShell consumer." Add the ADR reference; no other
    new binding facts.

T6. **`make check`** must be green.

T7. **Pass-end ritual** — STATUS.md mark 17c done, next starter
    prompt (Pass 18 — Polish II), `git mv` plan/spec to archive,
    commit / push / install.

## Out of scope

- Refactoring `helppopover.Model`'s render pipeline — there is no
  cleaner home for it pre-beta. The ADRs codify the status quo.
- Adopting `bubbles/v2/help` for the **command footer** at the
  bottom of the screen. The footer is rank-prioritized hint
  dropping (see ui-invariants.md "Visual language"), not generic
  short help. Separate question, separate pass if ever.
- Touching `helppopover.Styles` or the binding tables. The
  vocabulary is correct as of 17b.
