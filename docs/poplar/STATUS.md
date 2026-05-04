# Poplar Status

**Current pass:** Pass 8.5d next — content/filter cleanup (HTML word fusing #23 + dead enums #13).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.3 | Scaffold → backends → UI → triage → config v1 → Gmail preset → polish I (see git log; ADRs 0001–0109) | done |
| 8.4 – 8.4b | Cache 0–II — design, foundation, UI cutover, body cache + CLI (ADR-0110–0124) | done |
| 8.5 | Overengineering audit — ADR-0125/0126/0127; ~700 LOC net deletion | done |
| 8.5b | Elm architecture conformance audit (`internal/ui/`) — ADR-0128 | done |
| 8.5c | UI structural cleanup — ModalShell + SidebarColumn + overlay render caching (ADR-0129, 0130) | done |
| 8.4c | Cache III — outbox + offline + `Q`/`!` overlays + status badge | pending |
| 8.5d | Content/filter cleanup — HTML word fusing (#23) + dead enums (#13) | pending |
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

## Next starter prompt (Pass 8.5d)

> **Goal.** Two content/filter cleanups deferred from earlier passes:
> fix HTML→plain-text word fusing (#23) and remove the dead
> `blockKind`/`spanKind` enums (#13).
>
> **Scope.** `internal/filter/html.go` (insert spaces at element
> boundaries before tag stripping, or post-process to re-introduce
> spaces around fused alphanumeric runs) and `internal/content/`
> (drop the unused enum families). Reference:
> `docs/superpowers/specs/2026-05-03-content-filter-cleanup.md`.
>
> **Settled (do not re-brainstorm):** Spec is complete. Both
> findings have concrete fixes called out.
>
> **Still open — brainstorm these:** None — pure implementation
> pass.
>
> **Approach.** Read the spec, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-content-filter-cleanup.md`,
> then implement. Standard pass-end checklist applies.

## Queued

- **Pass 8.4c** — Cache III (visible outbox + offline + `Q`/`!`
  overlays + connection badge).
- **Pass 8.5d** — content/filter cleanup (HTML word fusing #23 +
  dead `blockKind`/`spanKind` enums #13). Spec:
  `docs/superpowers/specs/2026-05-03-content-filter-cleanup.md`.
