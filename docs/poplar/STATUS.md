# Poplar Status

**Current pass:** Pass 8.6 next — Attachments I (backend, #24).

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

## Next starter prompt (Pass 8.6)

> **Goal.** Attachments I — backend support for parsing and
> downloading attachments through the cache. UI follows in Pass 8.7.
>
> **Scope.** `internal/cache/` (attachment metadata + download
> path), `internal/mail/` (Attachment type + Backend method), JMAP
> + IMAP backends. No UI changes.
>
> **Settled (do not re-brainstorm):** Cache II body lazy
> population (ADR-0122/0123/0124) is the model — attachments
> follow the same write-through pattern with a separate size
> backstop.
>
> **Still open — brainstorm these:** Inline-vs-attachment
> classification source of truth; per-attachment storage path
> (`bodies` table or new `attachments` table); MIME type
> normalization.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-attachments-i.md`, then
> implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (apply the 8.5c overlay
  cache pattern). Pickup-of-opportunity, not a dedicated pass.
