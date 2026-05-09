# Poplar Status

**Current pass:** Pass 9q next — outbox delivery controls (undo
+ schedule send, #35). Pass 9w.1 shipped `internal/ansix/`; the
JoinHorizontal ban from ADR-0084 stands.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9w.1 | Scaffold through ansix shim (ADRs 0001–0181) | done |
| 9q | Outbox delivery controls — undo + schedule send (#35) | pending |
| 9r–9t | List-Unsubscribe (#36), .ics viewer (#37), search (#38) | pending |
| 9u | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 10 | Polish II — popover dim (#14) + items surfaced during 9j–9u | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 9q)

> **Goal.** Land outbox delivery controls — undo-send window and
> schedule-send — per BACKLOG #35.
>
> **Scope.** Compose `Ctrl+X` enqueues with a configurable hold
> window (default 5s, off via `[ui] undo_send_seconds = 0`); a
> status-bar segment exposes the active hold; `u` while held
> cancels and reopens the draft. Schedule-send adds a picker
> (presets + absolute) emitting an `outbox.deliver_at` row that
> the drainer respects. Reuse the existing optimistic outbox
> (cache schema v6) — no new tables.
>
> **Settled.** Send path stays through `AssembleMIME` +
> `cache.Account.QueueOutbound` (ADR-0160). Undo cancels the
> outbox row (`DiscardOp`); schedule-send extends `outbox.args`
> with `deliver_at`.
>
> **Still open — brainstorm these:** UX for the schedule picker
> (presets list, custom absolute input, time-zone display);
> status-bar segment shape under the existing `⇅N`/`⚠N` rules;
> what happens when an undo fires after the message has already
> dispatched (race vs. user expectation).
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-outbox-delivery.md`, then
> implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern).
- **Bubble re-eval (post-9w.1).** Revisit
  `archive/specs/2026-05-08-bubble-adoption-design.md`. Two lists:
  (a) bubbles `ansix` alone unlocks (swap), (b) bubbles gated on
  `lipgloss.Width` internal calls (`bubbles/help`, `bubble-table`,
  `glamour`). For (b) the binary is **fork** x/ansi or lipgloss
  via `replace` (unlocks fully; permanent rebase; contradicts
  ADR-0002/0075) vs. **accept** the gating. Decide once (b) has
  names.
