# Poplar Status

**Current pass:** Pass 8.4-revise next — apply Cache 0 review findings.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.5 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset (ADR-0106/0107/0108) | done |
| 8.2 | Bubbletea cleanup II | done |
| 8.3 | Polish I — msglist, viewer (ADR-0109) | done |
| 8.4 | Cache 0 — design + spec + ADR-0110/0111/0112 | done |
| 8.4-review | Independent multi-angle review of Cache 0 spec → findings doc | done |
| 8.4-revise | Apply review findings; produce revised spec + Pass 8.4a brief | next |
| 8.4a | Cache I — schema + headers + `mail.ChangeTracker` impls; unified write path migration | pending |
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

## Next starter prompt (Pass 8.4-revise)

> **Goal.** Apply Pass 8.4-review findings to the Cache 0 spec
> and ADRs 0110/0111/0112; write the Pass 8.4a implementation brief.
>
> **Approach.** Fresh session. Read
> `docs/superpowers/plans/2026-05-02-cache-0-revise.md` end-to-end —
> it has the full step-by-step. Work from the spec, the review at
> `docs/superpowers/reviews/2026-05-02-cache-0-review.md`, and the
> ADRs only. Do NOT load history from Pass 8.4 or 8.4-review.
>
> **Outputs.** Revised spec; new ADRs where decisions reverse; Pass
> 8.4a brief at
> `docs/superpowers/plans/<TODAY>-cache-i-implementation.md`. No
> code. Standard pass-end ritual (skip /simplify and `make install`).
> Recommend `/ultrareview` on the resulting branch in the commit
> message — Cache 0 is the v1.0-frozen on-disk contract.
