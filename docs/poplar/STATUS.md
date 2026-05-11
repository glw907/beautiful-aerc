# Poplar Status

**Current pass:** Pass 17b — `messagelist` on `bubbles/v2/list` with
custom item renderer. Second of the bubbles-adoption remainder after
the 16-series modernization locked modern-Go conventions.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 16d | Scaffold through slog adoption (ADRs 0001–0197) | done |
| 17a | Sidebar folder hierarchy on a v2 tree component (ADR-0198) | done |
| **17b** | **`messagelist` on `bubbles/v2/list` (custom item renderer; absorbs BACKLOG #46 `iter.Seq2`)** | **pending — next** |
| 17c | `bubbles/v2/help` audit + ADRs for bubbles deviations that survive 15a/17a/17b | pending |
| 18 | Polish II — popover dim (#14) + items surfaced during 10–17c | pending |
| 19 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 17b)

> **Goal.** Migrate `messagelist` to `bubbles/v2/list` with a
> custom item renderer for the thread-prefix walk. Second of the
> bubbles-adoption remainder (15a, 15a.5 done; 17a done; **17b** /
> 17c queued) before Polish II and the v0.9.0 freeze. The 16
> series locked modern-Go conventions before this pass, so the
> `iter.Seq2` walk lands as native shape, not a follow-up.
>
> **Scope.** `internal/ui/messagelist/` — `Model`, `displayRow`,
> `appendThreadRows`. Replace the imperative `MoveUp`/`MoveDown`/
> `MoveCursor` with `Update` + exported `KeyMap` (#45 item 2).
> Replace `appendThreadRows`'s manual prefix buffer with an
> `iter.Seq2`-style `(Node, Depth, IsLast)` walk (#46). Thread
> fold state, visual mode, and `ActionTargets` semantics all
> stay.
>
> **Settled (do not re-brainstorm):** `bubbles/v2/list` is the
> target dep (same family as 15a). The thread-row data model
> (group → sort → flatten → display) stays. Visual mode
> (`v` / `Space` / `Esc`) routing stays.
>
> **Still open — brainstorm these:** how to interleave date /
> sender / subject columns with `bubbles/v2/list`'s item
> delegate (custom delegate vs. pre-rendered string per row);
> whether `list.Filter` replaces the search shelf or stays
> sidebar-owned; whether fold state lives on the list's items
> or stays on `Model`.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-messagelist-list.md`,
> then implement. Standard pass-end checklist applies.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix
in the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
