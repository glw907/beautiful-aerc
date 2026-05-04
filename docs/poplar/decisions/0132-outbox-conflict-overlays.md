---
title: Outbox + conflict overlays
status: accepted
date: 2026-05-03
---

## Context

Pass 8.4 — 8.4b shipped the cache + outbox infrastructure but
left it invisible to the user. Drainer state lived only in the
SQLite outbox table; failures surfaced as ErrorMsg banners but
gave the user no recourse beyond restarting the app.

## Decision

Two new modal overlays surface the outbox to the user:

- `Q` opens the outbox overlay — read-only grouped summary, one
  row per `(kind, folder, status)`. No cursor; telemetry only.
- `!` opens the conflict overlay — two-line rows with
  `r retry` / `d discard` per row.

Both are `ModalShell` consumers. Conflict overlay caches its
render via the ADR-0130 `*<T>Cache` pattern (rare events,
view-stable). Outbox overlay re-renders fresh every frame — its
content churns every cache event while a queue drains.

Modal cascade: confirm > conflict > outbox > help > link picker
> move picker.

Grouped summary density (rather than per-op rows) was chosen
because per-op detail (protocol IDs) isn't user-relevant; the
group `(kind, folder, status)` collapses to single rows when
count = 1, so the simple "always group" rule covers the small-
queue case naturally. For Move ops the group folder is the
destination (extracted from args.Dest via `json_extract`); for
other op kinds the folder column is empty since the UI does
not surface a per-row source folder for them.

## Consequences

- Outbox + conflicts are user-discoverable surfaces. New help-
  popover entries (Q, !) document them; r/d live in the conflict
  overlay's own footer since they're overlay-scoped.
- Cache layer gains read queries (`OutboxSummary`,
  `OutboxConflicts`, `OutboxDepth`) — the cache is now the
  source of truth for outbox visibility, mirroring its existing
  source-of-truth status for messages and folders.
- The `r` / `d` keys shadow Triage / Folder-jump bindings while
  the conflict overlay is open. The modal cascade
  short-circuits before delegation, so no functional conflict.
