# Poplar Status

**Current pass:** Pass 17c — `bubbles/v2/help` audit + ADRs for
bubbles deviations that survived 15a/17a/17b. Third of the
bubbles-adoption remainder; closes the migration arc before
Polish II.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 16d | Scaffold through slog adoption (ADRs 0001–0197) | done |
| 17a | Sidebar folder hierarchy on a v2 tree component (ADR-0198) | done |
| 17b | `messagelist` on `bubbles/v2/list` with custom item delegate; iter.Seq2 thread walk (ADR-0199) | done |
| **17c** | **`bubbles/v2/help` audit + bubbles-deviation ADRs** | **pending — next** |
| 18 | Polish II — popover dim (#14) + items surfaced during 10–17c | pending |
| 19 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 17c)

> **Goal.** Audit poplar's help-popover wiring against
> `bubbles/v2/help`, adopt where it composes, and ADR every
> deviation that survives. Closes the bubbles-adoption arc
> (15a/17a/17b done; 17c is the wrap).
>
> **Scope.** `internal/ui/helppopover/`, the help-vocabulary
> table that backs it, and the now-duplicate `account.keys.MsgList*`
> bindings (deduplicate against `messagelist.KeyMap` per ADR-0199).
> Audit `bubbles/v2/help.Model` against the wireframe at
> `docs/poplar/wireframes.md` and confirm or write deviation ADRs
> for: (a) "advertise unwired bindings dimmed" (custom rendering
> per ADR-0072), (b) any binding-source composition `bubbles/v2/help`
> doesn't natively support.
>
> **Settled (do not re-brainstorm):** ADR-0072's wired/unwired
> distinction stays. `messagelist.KeyMap` and `sidebar.KeyMap` are
> the canonical binding sources for those panels; `account.keys`
> remains the canonical source for cross-panel actions.
>
> **Still open — brainstorm these:** how `bubbles/v2/help` composes
> (or doesn't) multiple `key.KeyMap` sources; whether the
> account.keys.MsgList* deduplication ships in 17c or as a follow-up;
> whether the help-popover's rounded-border deviation (ModalShell
> non-consumer) gets an ADR or stays implicit.
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-help-bubbles-audit.md`, then
> implement. Standard pass-end checklist applies.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix
in the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
