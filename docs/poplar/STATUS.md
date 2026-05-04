# Poplar Status

**Current pass:** Pass 9 next — Compose framing.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.3 | Scaffold → backends → UI → triage → config v1 → Gmail preset → polish I (see git log; ADRs 0001–0109) | done |
| 8.4 – 8.4b | Cache 0–II — design, foundation, UI cutover, body cache + CLI (ADR-0110–0124) | done |
| 8.5 | Overengineering audit — ADR-0125/0126/0127; ~700 LOC net deletion | done |
| 8.5b | Elm architecture conformance audit (`internal/ui/`) — ADR-0128 | done |
| 8.5c | UI structural cleanup — ModalShell + SidebarColumn + overlay render caching (ADR-0129, 0130) | done |
| 8.5d | Content/filter cleanup — `Block`/`Span` marker simplification (ADR-0131); #23 already shipped in 9174f85 | done |
| 8.4c | Cache III — outbox + offline + `Q`/`!` overlays + status badge (ADR-0132, 0133, 0134) | done |
| 8.6 | Attachments I — backend (#24) (ADR-0135, 0136, 0137) | done |
| 8.7 | Attachments II — viewer (#24) (ADR-0138, 0139, 0140) | done |
| 9 | Compose framing — Editor interface, neovim adapter, `go-smtp` | pending |
| 9.5 | Compose enhancements — #5 #12 #24 | pending |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 10.5 | Human-voice audit — strip AI fingerprints; ADR-0138, `go-conventions` + `/simplify` updates (spec + plan written 2026-05-04) | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9)

> **Goal.** Compose framing — wire the Editor interface, ship
> Catkin (the v1 native bubbletea editor) for the basic compose
> path, and stand up `go-smtp` for outbound send. Drafts go through
> the cache outbox just like other writes.
>
> **Scope.** New `internal/editor/` package for the Editor
> interface + Catkin impl. New `internal/mailsmtp/` (or extend
> existing wiring) for SMTP submission. `cache.SendArgs` /
> `cache.AppendArgs` come off the reserved list and get drainer
> handlers. UI: a Compose surface launched by `c`/`r`/`R`/`f` —
> inline, no terminal takeover (ADR-0031/0032).
>
> **Settled (do not re-brainstorm):** Editor is pluggable behind
> the interface; v1.1 adds the neovim `--embed` adapter (ADR-0033).
> Compose renders inline, sidebar+chrome stay visible.
> `mail.Backend` does not gain Send/Append — both go through
> `cache.QueueOp` so the outbox is the single forward write path.
>
> **Still open — brainstorm these:** Catkin's textarea contract
> (bubbles textarea vs custom); attachment-attach UX; multi-account
> From: chooser; send feedback shape (toast / banner / both).
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-compose-framing.md`, then
> implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (apply the 8.5c overlay
  cache pattern). Pickup-of-opportunity, not a dedicated pass.
