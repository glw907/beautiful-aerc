# Poplar Status

**Current pass:** Pass 8.4c next — Cache III (visible outbox + offline + `Q`/`!` overlays + connection badge).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.3 | Scaffold → backends → UI → triage → config v1 → Gmail preset → polish I (see git log; ADRs 0001–0109) | done |
| 8.4 – 8.4b | Cache 0–II — design, foundation, UI cutover, body cache + CLI (ADR-0110–0124) | done |
| 8.5 | Overengineering audit — ADR-0125/0126/0127; ~700 LOC net deletion | done |
| 8.5b | Elm architecture conformance audit (`internal/ui/`) — ADR-0128 | done |
| 8.5c | UI structural cleanup — ModalShell + SidebarColumn + overlay render caching (ADR-0129, 0130) | done |
| 8.5d | Content/filter cleanup — `Block`/`Span` marker simplification (ADR-0131); #23 already shipped in 9174f85 | done |
| 8.4c | Cache III — outbox + offline + `Q`/`!` overlays + status badge | pending |
| 8.6 | Attachments I — backend (#24) | pending |
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

## Next starter prompt (Pass 8.4c)

> **Goal.** Cache III — make the outbox visible to the user: a
> connection-state badge in the chrome, `Q` overlay listing
> pending/executing/failed ops, and `!` overlay for conflicts that
> need user intervention. Plus offline-mode handling so triage
> queues cleanly when the network is down.
>
> **Scope.** `internal/cache/` (drainer status surfacing),
> `internal/ui/` (badge + two new overlays + key dispatch). No
> changes to backends or to schema (already in place).
>
> **Settled (do not re-brainstorm):** Outbox state machine,
> drainer events, terminal classification — all locked in
> ADR-0110/0114/0115/0117. Connection-state visual language
> (`●` / `◐` / `○`) locked by ui-invariants. Modal overlays use
> `ModalShell` (ADR-0129).
>
> **Still open — brainstorm these:** `Q` density (per-op vs
> grouped); `!` action surface (retry / discard); badge placement
> in chrome; offline-mode failure threshold.
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-cache-iii.md`, then implement.
> Standard pass-end checklist applies.

## Queued

- **Pass 8.4c** — Cache III (visible outbox + offline + `Q`/`!`
  overlays + connection badge).
- **#30** — `Sidebar.View` render cache (apply the 8.5c overlay
  cache pattern). Pickup-of-opportunity, not a dedicated pass.
