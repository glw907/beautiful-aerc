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
| 8.8 | Human-voice audit I — research-grounded style guide, persona, ADR-0141, skill + `/simplify` updates; string-only fixes (C1 comments, C7 errors, C4 prose verbosity) | done |
| 8.9 | Human-voice audit II — structural fixes (C2/C5/C6/C8/C4-structural) + dedupe of resolvePassword | done |
| 8.10 | First-sync header population — JMAP per-folder baseline pull (Email/query + sentinel-id state probe in one roundtrip); FetchHeaders chunked at 500 (ADR-0143) | done |
| 9 | Compose framing — Editor interface, neovim adapter, `go-smtp` | pending |
| 9.5 | Compose enhancements — #5 #12 #24 | pending |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9)

> **Goal.** Land the Compose framing: `Editor` interface, Catkin
> native editor, SMTP backend via `emersion/go-smtp`, draft
> persistence through the cache outbox (`SendArgs`/`AppendArgs`).
>
> **Scope.** `internal/compose/` (new), `internal/mail` SMTP
> shape, `cache.OpKind{KindSend,KindAppend}` wiring, `c` key
> from the message-list and viewer. Inline render — no
> `tea.ExecProcess` takeover. Neovim `--embed` adapter is Pass
> 9.5. References: wireframes.md (compose screen), ADR-0031,
> 0032, 0033, 0076.
>
> **Settled:** Compose is bubbletea-native (Catkin) for v1; the
> Editor interface is the seam neovim plugs into later. SMTP
> auth re-uses backend `password-cmd`. Outbound mail enters the
> cache as a `KindSend` outbox row + a `KindAppend` to the Sent
> folder; the drainer ships them.
>
> **Still open — brainstorm:** Catkin's text model (rune buffer
> vs. line buffer); attachment-add UI inside compose; Reply /
> Reply-all quoting depth; failure surface for SMTP (toast vs.
> conflict overlay).
>
> **Approach.** Brainstorm the open questions, write a plan at
> `docs/superpowers/plans/YYYY-MM-DD-compose.md`, then implement.
> Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
