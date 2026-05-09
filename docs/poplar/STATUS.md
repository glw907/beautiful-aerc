# Poplar Status

**Current pass:** Pass 10b — schedule send + outbox sidebar (#35
completion). Pass 10a delivered the undo-send headline.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9z | Scaffold through bubble adoption (ADRs 0001–0182) | done |
| 10a | Outbox delivery controls — undo send (#35 part 1; ADR-0183) | done |
| 10b | Schedule send + sidebar Outbox (#35 part 2) | pending |
| 11 | List-Unsubscribe (#36) | pending |
| 12 | `.ics` viewer (#37) | pending |
| 13 | Search (#38) | pending |
| 14 | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 10b)

> **Goal.** Schedule send + outbox sidebar — completes BACKLOG #35.
> Builds on the `scheduled_for` foundation landed in 10a.
>
> **Scope.** Add `cache.RescheduleOp` and `cache.OutboxScheduled`
> reads. Add a synthetic Outbox virtual folder above Trash in the
> sidebar (visible only when the queue is non-empty). New
> `internal/ui/outbox` subpackage with a list view and
> cancel/reschedule/edit-as-draft keys. Compose schedule picker
> modal (presets + custom dateparse). Edit-as-draft is a join
> query against `drafts` — no MIME parser.
>
> **Settled (do not re-brainstorm):** Schema v10's two-column
> design (10a). Reuse `pendingAction` + chrome banner only for
> short-window undo; long schedules render in the sidebar Outbox
> count. Picker is presets + custom (Gmail/Fastmail-shape).
>
> **Still open — brainstorm these:** Compose key for schedule
> (`s` is taken; pick from free letters); preset list calibration;
> dateparse vendoring; sidebar virtual-folder seam in
> `sidebar.Model`.
>
> **Approach.** Brainstorm the open questions, write a plan at
> `docs/superpowers/plans/YYYY-MM-DD-schedule-send.md`, then
> implement. Standard pass-end checklist.
