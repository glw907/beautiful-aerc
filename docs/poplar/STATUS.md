# Poplar Status

**Current pass:** Pass 8.9 active — Human-voice audit II (structural).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.3 | Scaffold → backends → UI → triage → config v1 → Gmail preset → polish I (see git log; ADRs 0001–0109) | done |
| 8.4 – 8.4b | Cache 0–II — design, foundation, UI cutover, body cache + CLI (ADR-0110–0124) | done |
| 8.5 | Overengineering audit — ADR-0125/0126/0127; ~700 LOC net deletion | done |
| 8.5b | Elm architecture conformance audit (`internal/ui/`) — ADR-0128 | done |
| 8.5c | UI structural cleanup — ModalShell + SidebarColumn + overlay render caching (ADR-0129, 0130) | done |
| 8.5d | Content/filter cleanup — `Block`/`Span` marker simplification (ADR-0131); #23 already shipped in 9174f85 | done |
| 8.4c | Cache III — outbox + offline + `Q`/`!` overlays + status badge (ADR-0132, 0133, 0134) | done |
| 8.6 | Attachments I — backend (#24) (ADR-0135, 0136, 0137) | done |
| 8.7 | Attachments II — viewer (#24) (ADR-0138, 0139, 0140) | done |
| 8.8 | Human-voice audit I — research-grounded style guide, persona, ADR-0141, skill + `/simplify` updates; string-only fixes (C1 comments, C7 errors, C4 prose verbosity) | done |
| 8.9 | Human-voice audit II — structural fixes (C2 defensive cruft, C5 renames, C6 test boilerplate, C8 structural symmetry, C4-structural) against 8.8's frozen triage | active |
| 9 | Compose framing — Editor interface, neovim adapter, `go-smtp` | pending |
| 9.5 | Compose enhancements — #5 #12 #24 | pending |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 8.9)

> **Goal.** Apply the structural human-voice findings against
> 8.8's frozen triage list — C2 defensive cruft, C6 test
> boilerplate, C5 renames, C8 structural symmetry, C4-structural
> chorus elimination. (C3 came up empty; no inlines.)
>
> **Scope.** Phase 3b of the plan at
> `docs/superpowers/plans/2026-05-04-human-voice-audit.md`.
> One commit per category; `make check` green between; live tmux
> render at 80×24 + 120×40 after C5 and C3.
>
> **Settled (do not re-brainstorm):** Triage is frozen at end of
> 8.8 Phase 2. Style guide, persona, `go-conventions` skill, and
> `/simplify` voice lens are in production from 8.8. Real-seam
> guard list is in the plan. Coverage-guard rule for C6 case
> removal is in the plan.
>
> **Still open — brainstorm these:** None. Pure implementation
> against the frozen triage.
>
> **Approach.** Read the plan's Phase 3b. Execute task-by-task.
> Standard pass-end checklist applies, plus Phase 6 close-out
> (archive plan + spec + audit findings — 8.8 deferred archival
> to here). When done, write Pass 9 starter prompt (compose
> framing — see git log of this file for prior content).

## Queued

- **#30** — `Sidebar.View` render cache (apply the 8.5c overlay
  cache pattern). Pickup-of-opportunity, not a dedicated pass.
