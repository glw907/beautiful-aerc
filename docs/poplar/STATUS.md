# Poplar Status

**Current pass:** Pass 8.4a-cutover next — UI through cache + Backend collapse.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.5 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset (ADR-0106/0107/0108) | done |
| 8.2 | Bubbletea cleanup II | done |
| 8.3 | Polish I — msglist, viewer (ADR-0109) | done |
| 8.4 | Cache 0 — design + spec + ADR-0110/0111/0112 | done |
| 8.4-review | Independent multi-angle review of Cache 0 spec → findings doc | done |
| 8.4-revise | Apply review findings; revised spec + ADR-0113/0114/0115/0116/0117 | done |
| 8.4a | Cache I foundation — schema, ChangeTracker, syncer, drainer, tests; ADR-0118/0119/0120 | done |
| 8.4a-cutover | UI cutover — `*cache.Account` reads + writes, Backend collapse to `Flag`, App-layer optimistic-state delete | next |
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

## Next starter prompt (Pass 8.4a-cutover)

> **Goal.** Wire the UI through `cache.Account` and delete the
> legacy direct-Backend code paths. Cache foundation is shipped
> (Pass 8.4a, ADR-0118).
>
> **Scope.** Phases 3 + 6 of
> `docs/superpowers/plans/2026-05-02-cache-i-implementation.md`:
> shrink `mail.Backend` (drop MarkRead/MarkUnread/MarkAnswered/
> Delete — Flag is canonical); thread `*cache.Account` into
> `App.NewApp` + `AccountTab`; switch reads through
> `cache.Account.QueryFolder`, writes through `cache.QueueOp`;
> replace `triageStartedMsg.onUndo` with a compensating-`QueueOp`
> inverse; delete `MessageList.Apply*` and the App-layer
> optimistic-state plumbing (ADR-0089); add `pumpCacheCmd` for
> `(*Account).Events()`. Strangler-fig order (write switch →
> read switch → legacy delete) is mandatory.
>
> **Settled (do not re-brainstorm):** ADR-0110/0111/0112/0113/
> 0114/0115/0116/0117/0118/0119/0120.
>
> **Approach.** Pre-beta refactor freedom — migrate UI tests
> to the new shape rather than carrying shims. Standard pass-end
> ritual; archive the cache-i plan when this pass closes.
