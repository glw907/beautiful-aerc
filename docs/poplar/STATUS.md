# Poplar Status

**Current pass:** Pass 11 — List-Unsubscribe (#36). Pass 10b
landed schedule send + Outbox sidebar; #35 closed.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9z | Scaffold through bubble adoption (ADRs 0001–0182) | done |
| 10a | Outbox delivery controls — undo send (#35 part 1; ADR-0183) | done |
| 10b | Schedule send + sidebar Outbox (#35 part 2; ADR-0184) | done |
| 11 | List-Unsubscribe (#36) | pending |
| 12 | `.ics` viewer (#37) | pending |
| 13 | Search (#38) | pending |
| 14 | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 11)

> **Goal.** List-Unsubscribe (BACKLOG #36) — surface RFC 8058
> one-click unsubscribe in the viewer for messages that carry
> `List-Unsubscribe` and `List-Unsubscribe-Post: List-Unsubscribe=One-Click`.
> Fall back to the mailto: form when no one-click endpoint is
> offered. Plain http: links route through the existing
> `URLOpener` seam.
>
> **Scope.** Parse `List-Unsubscribe`/`List-Unsubscribe-Post`
> from headers in `internal/mail` (or `internal/content`). Wire
> a viewer affordance — likely `U` — that posts the one-click
> request via stdlib `net/http`, or composes a pre-filled
> mailto. Confirmation prompt before firing (POST is
> irreversible). Surface success/failure through the existing
> error banner. No new key chord; modifier-free single key per
> ADR-0076.
>
> **Settled (do not re-brainstorm):** ErrorMsg banner is the
> failure surface. ConfirmModal is the prompt vocabulary. POST
> dispatch is synchronous-with-spinner via tea.Cmd.
>
> **Still open — brainstorm these:** Key choice (`U` likely);
> mailto fallback rendering (open in compose vs xdg-open);
> behavior when both forms are present; whether to remember
> "already unsubscribed" per Message-ID.
>
> **Approach.** Brainstorm the open questions, write a plan at
> `docs/superpowers/plans/YYYY-MM-DD-list-unsubscribe.md`, then
> implement. Standard pass-end checklist.
