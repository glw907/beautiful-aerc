# Poplar Status

**Current pass:** Pass 8.3 next — narrow-terminal polish I.
Pass 8.2 closed as no-op: #17/#18/#19 were already resolved in
Pass 5 (commits ec0984a/1797927/2b520c9); backlog updated.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.5 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset: X-GM-EXT-1, Trash precondition, XOAUTH2 via `password-cmd` (ADR-0106/0107/0108) | done |
| 8.2 | Bubbletea cleanup II — #17/#18/#19 (closed; already resolved in Pass 5) | done |
| 8.3 | Polish I — #23 HTML word-fusion; #26 narrow-terminal msglist; #9 viewer `n/N` filtered | next |
| 8.4 | Cache 0 — design + ADR + spec (storage, decorator vs. backend-aware, `ChangeTracker` interface, RFC 4549 + JMAP sync) | pending |
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

## Next starter prompt (Pass 8.3)

> **Goal.** Polish I — three behaviour bugs that bite real users at
> the 80×24 polish bar and on plain-text-formatted mail.
>
> **Scope.** BACKLOG items #23, #26, #9:
> - #23: HTML→plain-text fuses words across element boundaries.
>   Fix in `internal/filter/html.go` — insert whitespace at inline
>   element boundaries before tag stripping (or post-process to
>   re-introduce spaces around fused alphanumeric runs).
> - #26: Narrow-terminal msglist polish — date-column adapts to
>   width (`04-30` / `3:41p` at intermediate widths, drop entirely
>   at 80 cols), rebalance subject vs sender allocation, explore
>   sidebar floor below 24 cells.
> - #9: Viewer `n`/`N` walks the filtered row set — couples viewer
>   navigation to msglist's filter state. Bundles with the live
>   backend so prefetch semantics are testable under real latency.
>
> **Settled:** Bubbletea conventions (0077–0084), responsive
> sidebar (0096), 80×24 polish bar (0097).
>
> **Still open — brainstorm these:**
> - #26: which of date format / sender-subject ratio / sidebar
>   floor earn their keep at 80 cols, at which width thresholds?
> - #9: prefetch eagerly while viewer open, or lazily on n/N?
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-polish-i.md`, then implement.
> Standard pass-end checklist applies.
