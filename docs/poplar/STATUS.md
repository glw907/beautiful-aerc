# Poplar Status

**Current pass:** Pass 8.1 next — Gmail preset. Pass 8.5 done —
config v1 (ADR-0102/0103/0104) plus integrated roadmap with
release stages (v0.9.0 freeze → soak → v1.0.0; ADR-0105). Every
open backlog item is scheduled to a numbered pass.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.5 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset: X-GM-EXT-1, Trash precondition, label-aware fallbacks, XOAUTH2; re-add `Provider.GmailQuirks` + `capSet.XGM` | next |
| 8.2 | Bubbletea cleanup II — #17 `key.Matches`; #18 intra-model `tea.Cmd` → direct delegation; #19 `App.View` width trust | pending |
| 8.3 | Polish I — #23 HTML word-fusion; #26 narrow-terminal msglist; #9 viewer `n/N` filtered | pending |
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

## Next starter prompt (Pass 8.1)

> **Goal.** Add a `gmail` provider preset adapting the generic IMAP
> backend to Gmail's quirks so Gmail accounts work before beta.
>
> **Scope.** New `gmail` entry in `config.Providers` with
> `GmailQuirks: true`. Re-add `Provider.GmailQuirks` and
> `capSet.XGM` (dropped in Pass 8.5 cleanup as dead fields). Gate
> Gmail-specific behavior in `internal/mailimap/` on the flag:
> assert `X-GM-EXT-1` at Connect; Move-to-Trash must select a
> non-Trash folder before EXPUNGE so Gmail actually deletes;
> X-GM-LABELS as classification fallback if SPECIAL-USE is
> missing. Wire the `internal/mailauth/` XOAUTH2 refresh flow
> into `dialCommand`/`dialIdle`.
>
> **Settled:** Generic IMAP backend (ADR-0099/0100/0101). Provider
> registry (ADR-0098). XOAUTH2 helpers in `internal/mailauth/`.
> Config v1 (ADR-0102/0103/0104). Release model (ADR-0105).
>
> **Still open — brainstorm these:**
> - XOAUTH2 refresh ownership (cache + 401-watch vs pre-refresh).
> - X-GM-LABELS fallback necessity in 2026 Gmail (likely dead-code
>   defense — confirm).
> - Trash-precondition: generic `mail.Backend` contract or Gmail
>   branch on `b.caps.GmailQuirks`?
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-gmail-preset.md`, then
> implement. Standard pass-end checklist applies.
