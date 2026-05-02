# Poplar Status

**Current pass:** Pass 8.2 next — bubbletea cleanup II. Pass 8.1
done — Gmail preset (ADR-0106/0107/0108): `gmail` provider,
`X-GM-EXT-1` assertion, Destroy via SELECT [Gmail]/Trash, XOAUTH2
access tokens via `password-cmd` (no internal refresh until 9.6).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.5 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset: X-GM-EXT-1, Trash precondition, XOAUTH2 via `password-cmd` (ADR-0106/0107/0108) | done |
| 8.2 | Bubbletea cleanup II — #17 `key.Matches`; #18 intra-model `tea.Cmd` → direct delegation; #19 `App.View` width trust | next |
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

## Next starter prompt (Pass 8.2)

> **Goal.** Bubbletea cleanup II — finish the migration to idiomatic
> bubbletea conventions across the model tree.
>
> **Scope.** BACKLOG items #17, #18, #19:
> - #17: Convert remaining ad-hoc key comparisons to `key.Matches`
>   against `key.Binding` values; ensure new keys are in the help
>   vocabulary (ADR-0072).
> - #18: Replace intra-model `tea.Cmd` round-trips (parent emits Msg
>   to itself) with direct delegation where the call doesn't cross
>   a tree boundary.
> - #19: `App.View` width trust — remove any defensive parent-side
>   `MaxWidth` clipping; children honor their `SetSize` contract.
>
> **Settled:** Bubbletea conventions (ADR-0077/0078/0079/0080/
> 0081/0083/0084). The size contract, wordwrap+hardwrap discipline,
> and JoinHorizontal trust contract from `bubbletea-conventions.md`.
>
> **Still open — brainstorm these:** None expected; this is a pure
> implementation pass. If a deviation surfaces, ADR it.
>
> **Approach.** Read `docs/poplar/bubbletea-conventions.md` and
> `.claude/rules/ui-invariants.md`, walk the diff sites listed in
> BACKLOG, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-bubbletea-cleanup-ii.md`,
> then implement. Standard pass-end checklist applies.
