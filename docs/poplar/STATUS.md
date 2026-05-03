# Poplar Status

**Current pass:** Pass 8.4-review next — independent review of Cache 0 spec.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.5 | Scaffold → backends → UI → triage → config v1 (see git log; ADRs 0001–0104) | done |
| 8.1 | Gmail preset (ADR-0106/0107/0108) | done |
| 8.2 | Bubbletea cleanup II | done |
| 8.3 | Polish I — msglist, viewer (ADR-0109) | done |
| 8.4 | Cache 0 — design + spec + ADR-0110/0111/0112 | done |
| 8.4-review | Independent multi-angle review of Cache 0 spec → findings doc | next |
| 8.4-revise | Apply review findings; produce revised spec + Pass 8.4a brief | pending |
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

## Next starter prompt (Pass 8.4-review)

> **Goal.** Independent multi-angle review of the Cache 0 spec
> before any implementation begins. Cache 0 becomes a v1.0-frozen
> on-disk format (ADR-0105); the review pass exists to catch
> design errors that would otherwise become migration debt.
>
> **Approach.** Read
> `docs/superpowers/plans/2026-05-02-cache-0-review.md` end-to-end.
> It contains the detailed brief — four parallel review subagents
> (mail-protocol correctness, source-level prior art extension,
> Go-architecture fit, failure-mode adversary), synthesis
> instructions, and pass-end ritual.
>
> **Critical:** Start from a clean session. Do NOT load history
> from Pass 8.4. Read only the spec, the three ADRs (0110/0111/0112),
> the review plan, and the existing poplar codebase. The whole
> point is independent review.
>
> **Outputs.** `docs/superpowers/reviews/2026-05-02-cache-0-review.md`
> + this pass's plan archived. No code change. Standard pass-end
> ritual (skip /simplify and `make install`).
>
> **Hand-off.** STATUS.md → 8.4-review done; current pass becomes
> 8.4-revise. The 8.4-revise plan is already drafted at
> `docs/superpowers/plans/2026-05-02-cache-0-revise.md`.

## Notes

- 8.4-revise plan already drafted at `docs/superpowers/plans/2026-05-02-cache-0-revise.md`.
- `/ultrareview` (user-triggered, billed): recommend on revised-spec branch after 8.4-revise, before 8.4a.
