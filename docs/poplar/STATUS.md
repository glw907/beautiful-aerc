# Poplar Status

**Current pass:** Pass 8.5b next — Elm architecture conformance audit of `internal/ui/`.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset (ADR-0106/0107/0108) | done |
| 8.2 | Bubbletea cleanup II | done |
| 8.3 | Polish I — msglist, viewer (ADR-0109) | done |
| 8.4 | Cache 0 — design + spec + ADR-0110/0111/0112 | done |
| 8.4-review | Independent multi-angle review of Cache 0 spec → findings doc | done |
| 8.4-revise | Apply review findings; revised spec + ADR-0113/0114/0115/0116/0117 | done |
| 8.4a | Cache I foundation — schema, ChangeTracker, syncer, drainer, tests; ADR-0118/0119/0120 | done |
| 8.4a-cutover | UI cutover — `*cache.Account` reads + writes, Backend collapse to `Flag`, App-layer optimistic-state delete; ADR-0121 | done |
| 8.4b | Cache II — body cache + eviction + `poplar cache` CLI | done |
| 8.5 | Overengineering audit — ADR-0125/0126/0127; ~700 LOC net deletion | done |
| 8.5b | Elm architecture conformance audit (`internal/ui/`) | pending |
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

## Next starter prompt (Pass 8.5b)

> **Goal.** Pass 8.5b — Elm architecture conformance audit of
> `internal/ui/`. Verify state-in-models, mutations-only-in-Update,
> I/O-only-in-Cmd, child→parent communication via Msg only, shared
> state hoisted to root.
>
> **Scope.** `internal/ui/` only. Sibling to Pass 8.5 (overengineering
> audit, just shipped); operates on the slimmer UI surface 8.5
> produced.
>
> **Settled (do not re-brainstorm):** elm-conventions skill rules.
> ADR-0023, 0035-0037, 0042, 0044, 0054, 0088 (Elm-architecture ADRs).
>
> **Approach.** Brainstorm → spec at
> `docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md`
> → plan at
> `docs/superpowers/plans/2026-05-03-elm-conformance-audit.md` →
> execute. Standard pass-end checklist applies.

## Queued after 8.5b (Pass 8.4c)

> **Goal.** Land Cache III: visible outbox, offline mode, Q/! per-row
> state overlays, and a connection-state badge in the status bar.
>
> **Scope.** Surface the cache outbox in the UI (queued/executing/failed
> badges per message), implement true offline mode (drainer pauses, reads
> still served from cache), and a status-bar connection indicator.
> Reference: invariants.md "Cache" section, ADR-0117 (typed op events),
> wireframes for status bar.
>
> **Settled (do not re-brainstorm):** ADR-0110–0124 (Cache 0/I/II +
> cutover + policy + CLI). Outbox state machine (ADR-0112/0116). Typed
> op events (ADR-0117).
>
> **Still open — brainstorm these:** Q/! glyph placement vs flag column,
> offline-mode entry/exit triggers (manual toggle vs auto on transient
> backend failures), connection-state cycling vs latching, undo behavior
> while offline.
