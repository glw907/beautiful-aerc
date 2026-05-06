# Poplar Status

**Current pass:** Pass 9d.3 next — golangci-lint on `internal/catkin/`.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9d.2a | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split (ADRs 0001–0153) | done |
| 9d.3 | golangci-lint on `internal/catkin/` (errcheck, staticcheck, unused, gocritic, revive, errorlint, unparam, nilerr) | pending |
| 9d.4 | Live tmux at edge sizes — popover near right + bottom edges at 80×24 | pending |
| 9e | `internal/compose/` — Editor interface, CatkinEditor adapter, Draft, AssembleMIME, Seed{Reply,ReplyAll,Forward} | pending |
| 9f | Mail backend Send + Append — JMAP submission, IMAP+SMTP, `[account.smtp]` config | pending |
| 9g | Cache outbox Send/Append dispatch | pending |
| 9h | ComposeTab UI + `c` wiring + tidy seam | pending |
| 9i | Claude Tidy implementation | pending |
| 9.5 | Attachments-richer compose UI (#24) | pending (after 9i) |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9d.3)

> **Goal.** Run golangci-lint over `internal/catkin/` with a
> targeted linter set; fix every flagged issue inline.
>
> **Scope.** `internal/catkin/` only. Linters: `errcheck`,
> `staticcheck`, `unused`, `gocritic`, `revive`, `errorlint`,
> `unparam`, `nilerr`. Add a `lint-catkin` Makefile target if a
> permanent gate is warranted; otherwise one-shot.
>
> **Settled.** Pre-beta posture (apply findings, don't defer).
> Voice and idiom rules from `go-conventions` apply to any
> rewrites. Catkin's binding facts now live in
> `.claude/rules/catkin-invariants.md` (auto-loaded).
>
> **Still open — brainstorm before coding:** whether to wire
> `lint-catkin` into `make check` permanently or run it
> opportunistically; which findings (if any) point at structural
> issues that should grow into their own follow-up pass instead
> of being fixed inline.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-catkin-lint.md`, then
> execute. Standard pass-end checklist applies.

## Queued passes (after 9d.3)

- **9d.4** — Live tmux verification at edge sizes. 80×24 with the
  popover open over a misspelling near the right edge and the
  bottom edge.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
