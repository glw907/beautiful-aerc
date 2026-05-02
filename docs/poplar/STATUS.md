# Poplar Status

**Current pass:** Pass 8.1 next — Gmail preset on top of the
generic IMAP backend. Pass 8.5 done — config v1 (ADR-0102/0103/0104)
plus integrated v1 roadmap (every open backlog item now scheduled
to a numbered pass; see table).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.5 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset: X-GM-EXT-1, Trash precondition, label-aware fallbacks, XOAUTH2 wiring; re-add `Provider.GmailQuirks` + `capSet.XGM` | next |
| 8.2 | Bubbletea cleanup II — #17 `key.Matches` migration (AccountTab + Viewer); #18 intra-model `tea.Cmd` → direct delegation; #19 `App.View` width trust (after #17) | pending |
| 8.3 | Polish I — #23 HTML→plain word-fusion fix; #26 narrow-terminal msglist polish (date column, sender/subject balance); #9 viewer `n/N` walks filtered set | pending |
| 8.4 | JMAP perf — #4 blob preload (next 2-3 messages on viewer open / msglist scroll) | pending |
| 8.6 | Attachments I — backend (#24): JMAP attachment metadata + blob fetch; IMAP equivalent | pending |
| 8.7 | Attachments II — viewer (#24): per-row indicator, attachment list/preview, save-to-disk picker | pending |
| 9 | Compose framing — `Editor` interface, neovim `--embed` adapter, send via `go-smtp` | pending |
| 9.5 | Compose enhancements — #5 Catkin native bubbletea editor; #12 `internal/tidy/` collapse; #13 dead `blockKind`/`spanKind` enum cleanup; #24 attach files in compose (size/MIME) | pending |
| 9.6 | First-run wizard — #27 in-TUI account setup (uses Pass 9 editor primitives + Pass 8.5 config infra) | pending |
| 10 | Polish II — #14 help popover background dim (decide v1 vs defer); any items surfaced during Pass 9–9.6 | pending |
| 11 | 1.0 prep — docs sweep, README, release notes, tag v1.0, ADR codifying post-1.0 stability stance | pending |
| 1.1 | Neovim companion plugin (#6) | post-v1 |
| 1.2 | View raw RFC822 (#21) | post-v1 |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 8.1)

> **Goal.** Add a `gmail` provider preset adapting the generic IMAP
> backend to Gmail's quirks so Gmail accounts work in v1.
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
> Config v1 (ADR-0102/0103/0104) — `provider = "gmail"` decodes
> through the same path; `password-cmd` is the credential channel
> for the OAuth refresh-token cache.
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
