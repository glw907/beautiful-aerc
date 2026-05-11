---
title: Pass 17c — help popover is not a ModalShell consumer
status: accepted
date: 2026-05-11
---

## Context

`uicore.ModalShell` is the shared frame for overlay chrome:
`ConfirmModal`, `reader.LinkPicker`, `reader.AttachPicker`,
`movepicker.Model`, `OutboxOverlay`, and `ConflictOverlay` all
embed it as `shell uicore.ModalShell` and call
`shell.Box(title, bodyRows, footerRows, contentW)` to render. The
shell owns the squared-corner border, the title row, the body /
footer separator, and width padding.

`helppopover.Model` renders its own border with two deviations
from the shell:

1. **Embedded title.** The top edge is `╭─ Message List ─…─╮` —
   the title sits inside the corner box, not above a separator
   row. `ModalShell` draws the title in a leading row beneath a
   plain `╭───╮` top edge.
2. **Rounded border.** `ModalShell` uses squared corners. The
   popover uses lipgloss's rounded-border style for visual
   distinction (help is a vocabulary surface, not a destructive
   prompt or a picker).

Both deviations are pre-existing; pass 17c is the audit that
makes them binding rather than incidental.

## Decision

`helppopover.Model` is not a `ModalShell` consumer and will not
become one. The rounded-border-with-embedded-title rendering
stays in `internal/ui/helppopover/model.go::Box`, drawn manually
so SPUA-safe row assembly (ADR-0084) composes the title segment
with the border edge atomically.

## Consequences

- The modal-overlay family is six shells + one bespoke popover.
  New overlays default to `ModalShell`; the popover stays the
  one exception.
- The cache structure on `helppopover.Model` (ADR-0130) does not
  share a contract with the shell consumers — the per-frame
  invalidation path stays local.
- A future refactor that wants to add a "rounded variant" to
  `ModalShell` is welcome to fold the popover in, but the
  embedded-title row remains a render concern bubbles/lipgloss
  do not natively model.
- `.claude/rules/ui-invariants.md` already notes the deviation in
  the Overlays section; this ADR is the binding reference.
