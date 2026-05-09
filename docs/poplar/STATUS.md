# Poplar Status

**Current pass:** Pass 10 — outbox delivery controls (undo + schedule
send, #35). Bubble-eval roadmap fully closed.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9x.3 | Scaffold through table/form harvest (ADRs 0001–0181) | done |
| 9y | Bubble consolidation verdict — nine libraries Keep; ADR-0182 closes the eval roadmap | done |
| 9z | Bubble adoption — both carry-forward items deferred under named flip conditions; no-op outcome | done |
| 10 | Outbox delivery controls — undo + schedule send (#35) | pending |
| 11 | List-Unsubscribe (#36) | pending |
| 12 | `.ics` viewer (#37) | pending |
| 13 | Search (#38) | pending |
| 14 | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 10)

> **Goal.** Outbox delivery controls — undo-send window and
> schedule-send (BACKLOG.md #35). First feature pass after the
> harvest cycle.
>
> **Scope.** Wire user-controllable delay between `QueueSend` and
> dispatch (the undo window) and absolute-time scheduling for
> "send later." Cache outbox already carries the rows; this is a
> drainer + UI surface pass. Out of scope: server-side scheduling
> (JMAP `EmailSubmission/set { onSend }` and IMAP equivalents are
> a v1.1 question).
>
> **Settled (do not re-brainstorm):** Outbox state machine and
> schema v6 payload-bearing rows (invariants — Send + Append).
> Toast + undo affordance vocabulary (ADRs 0089–0091). The drainer
> is the dispatch authority; UI signals via `tea.Cmd`.
>
> **Still open — brainstorm these:** Default undo window length
> and configurability; UX for an in-flight schedule (status bar
> indicator? sidebar Outbox folder count?); cancel-scheduled vs
> reschedule semantics; whether undo and schedule share a single
> "delay until" field on the row or split into two states.
>
> **Approach.** Brainstorm the open questions, survey the major
> clients (TB, Apple Mail, Gmail) for prior art, write a plan at
> `docs/superpowers/plans/YYYY-MM-DD-outbox-delivery-controls.md`,
> then implement. Pass-size budget applies — split if scope
> exceeds 12 tasks. Standard pass-end checklist.
