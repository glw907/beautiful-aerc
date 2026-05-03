# Poplar Status

**Current pass:** Pass 8.4a next — Cache I implementation.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.5 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset (ADR-0106/0107/0108) | done |
| 8.2 | Bubbletea cleanup II | done |
| 8.3 | Polish I — msglist, viewer (ADR-0109) | done |
| 8.4 | Cache 0 — design + spec + ADR-0110/0111/0112 | done |
| 8.4-review | Independent multi-angle review of Cache 0 spec → findings doc | done |
| 8.4-revise | Apply review findings; revised spec + ADR-0113/0114/0115/0116/0117; Pass 8.4a brief | done |
| 8.4a | Cache I — schema + headers + `mail.ChangeTracker` impls; unified write path migration | next |
| 8.4b | Cache II — body cache + eviction + `poplar cache` CLI | pending |
| 8.4c | Cache III — outbox + offline + `Q`/`!` overlays + status badge | pending |
| 8.6 | Attachments I — backend (#24) | pending |
| 8.7 | Attachments II — viewer (#24) | pending |
| 9 | Compose framing — Editor interface, neovim adapter, `go-smtp` | pending |
| 9.5 | Compose enhancements — #5 #12 #13 #24 | pending |
| 9.6 | First-run wizard (#27) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 8.4a)

> **Goal.** Implement Cache I — per-account SQLite (schema v1),
> `mail.ChangeTracker` interface and JMAP/IMAP impls, unified
> write path through `cache.QueueOp`, strangler-fig migration from
> direct-`mail.Backend` triage Cmds to cache-backed reads. Online
> behavior only; outbox table is written to but offline-detection,
> `Q`/`!` overlays, and status badge land in Cache III (8.4c).
>
> **Approach.** Fresh session. Read
> `docs/superpowers/plans/2026-05-02-cache-i-implementation.md`
> end-to-end — it has the full ordered task list. Source of truth
> for design is `docs/superpowers/specs/2026-05-02-cache-0-design.md`
> (status: reviewed) plus ADR-0110/0111/0112/0113/0114/0115/0116/0117.
> The strangler-fig order (cache writes → cache-backed reads →
> delete legacy paths) is mandatory.
>
> **Outputs.** New `internal/cache/` package, `mail.ChangeTracker`
> interface + two backend impls, `mail.Backend` shrunk to `Flag`,
> UI rewired through `*cache.Account`, App-layer optimistic-state
> plumbing deleted, comprehensive tests. Standard pass-end ritual
> (with `/simplify`, idiomatic-bubbletea check, tmux verification
> at 80×24 and 120×40, and `make install`).
