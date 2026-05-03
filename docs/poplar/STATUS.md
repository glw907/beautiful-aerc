# Poplar Status

**Current pass:** Pass 8.4 next — Cache 0 design.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.5 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset: X-GM-EXT-1, Trash precondition, XOAUTH2 via `password-cmd` (ADR-0106/0107/0108) | done |
| 8.2 | Bubbletea cleanup II — #17/#18/#19 (closed; already resolved in Pass 5) | done |
| 8.3 | Polish I — #23 HTML word-fusion; #26 narrow-terminal msglist; #9 viewer `n/N` filtered (ADR-0109) | done |
| 8.4 | Cache 0 — design + ADR + spec (storage, decorator vs. backend-aware, `ChangeTracker` interface, RFC 4549 + JMAP sync) | next |
| 8.4a | Cache I — envelope/header cache, multi-account namespacing, "stale/syncing" UI indicator (supersedes #4) | pending |
| 8.4b | Cache II — body cache, invalidation on flag/delete/IDLE, eviction (LRU + size + age), `poplar cache size/clear/status` | pending |
| 8.4c | Cache III — offline mode: read-only when offline, queued triage actions, reconciliation, offline chrome indicator (decide beta vs post-1.0) | pending |
| 8.6 | Attachments I — backend (#24): JMAP attachment metadata + blob fetch; IMAP equivalent | pending |
| 8.7 | Attachments II — viewer (#24): per-row indicator, list/preview, save-to-disk picker | pending |
| 9 | Compose framing — `Editor` interface, neovim `--embed` adapter, send via `go-smtp` | pending |
| 9.5 | Compose enhancements — #5 Catkin native editor; #12 `internal/tidy/` collapse; #13 dead enum cleanup; #24 attach files | pending |
| 9.6 | First-run wizard — #27 in-TUI account setup | pending |
| 10 | Polish II — #14 popover dim (decide); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, release notes, tag `v0.9.0` (the public beta; `0.x` conveys pre-stable per Go-CLI norms) | pending |
| **Beta soak** | Bug-fix releases `v0.9.1`, `v0.9.2`… on master; data formats frozen; new features queue on `1.1` branch | pending |
| v1.0.0 | Tag when soak settles (no new bug reports for ~2 weeks) | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta features | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 8.4)

> **Goal.** Design pass — establish the architecture, ADR, and spec
> for the local mail cache. No implementation; the next two passes
> (8.4a envelope/header cache, 8.4b body cache) execute against
> what this pass settles.
>
> **Scope.** Resolve open design questions and produce:
> - A new ADR documenting cache shape and invariants.
> - A spec doc at `docs/superpowers/specs/YYYY-MM-DD-cache-0-design.md`.
> - A plan doc at `docs/superpowers/plans/YYYY-MM-DD-cache-0-design.md`
>   for the design pass itself (since this pass produces docs).
>
> **Settled:** Sync backend interface (ADR-0099); JMAP + IMAP only
> (ADR-0075); pre-beta schema freedom (ADR-0105).
>
> **Still open — brainstorm these:**
> - Storage: SQLite (modernc.org/sqlite cgo-free), BoltDB, or
>   filesystem-of-files? Trade-offs on query, eviction, concurrent
>   IDLE updates, multi-account namespacing.
> - Cache shape: decorator wrapping `mail.Backend`, or
>   backend-aware (each backend reports change tokens)? RFC 4549
>   QRESYNC for IMAP, `Email/changes` for JMAP.
> - `ChangeTracker` interface: shared abstraction for both
>   backends, or per-backend type with a thin adapter?
> - Eviction policy primitives (LRU + size + age) and CLI surface
>   (`poplar cache size/clear/status`).
> - Offline-mode boundary: read-only on master + queued triage,
>   or defer to post-1.0?
>
> **Approach.** Brainstorm, write spec + plan + ADR, archive at
> pass-end. No code in this pass.
