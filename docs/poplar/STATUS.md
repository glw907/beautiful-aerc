# Poplar Status

**Current pass:** Pass 8.7 next — Attachments II (viewer, #24).

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
| 8.7 | Attachments II — viewer (#24) | pending |
| 9 | Compose framing — Editor interface, neovim adapter, `go-smtp` | pending |
| 9.5 | Compose enhancements — #5 #12 #24 | pending |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 8.7)

> **Goal.** Attachments II — viewer surface. Render attachment
> chips in the message view, support keyboard-driven save/open,
> and handle inline-CID rewriting in HTML body for Pass 8.7's
> renderer.
>
> **Scope.** `internal/ui/viewer.go` (chip row + key dispatch),
> `internal/ui/cmds.go` (save-to-disk Cmd), `internal/content/`
> (CID resolution against `cache.Attachments`).
>
> **Settled (do not re-brainstorm):** Backend + cache shape
> from Pass 8.6 (ADR-0135/0136/0137). `cache.Account.Attachments`
> + `FetchAttachment` are the only entry points; no direct
> backend calls from the UI.
>
> **Still open — brainstorm these:** Chip layout in narrow
> terminals (Spartan tier); save-target prompt vs default
> downloads dir; key dispatch for save vs open; HTML renderer
> CID rewriting strategy.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-attachments-ii.md`, then
> implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (apply the 8.5c overlay
  cache pattern). Pickup-of-opportunity, not a dedicated pass.
