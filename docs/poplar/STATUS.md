# Poplar Status

**Current pass:** Pass 9d.2a next — invariants compaction via subsystem extraction.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.3 | Scaffold → backends → UI → triage → config v1 → Gmail preset → polish I (ADRs 0001–0109) | done |
| 8.4 – 8.4c | Cache 0–III (ADR-0110–0134) | done |
| 8.5 – 8.5d | Overengineering audit, Elm conformance, UI structural cleanup, content/filter cleanup (ADR-0125–0131) | done |
| 8.6 – 8.7 | Attachments I + II (ADR-0135–0140) | done |
| 8.8 – 8.11 | Human-voice audits + grep gate (ADR-0141, 0142, 0148) | done |
| 8.10 | JMAP per-folder baseline pull (ADR-0143) | done |
| 9 – 9c | Catkin — core, live styling, commands, power-user QoL (ADR-0144–0147) | done |
| 9d | Annotation pipeline + spellcheck consumer (ADR-0149, 0150) | done |
| 9d.1 | AI-tells + dead-code sweep on 9d diff; tree-wide gofmt + fmt-check gate (ADR-0151) | done |
| 9d.2 | Render-path adversarial review — cursor-splice fixes (ADR-0152) | done |
| 9d.2a | Invariants compaction via subsystem extraction — promote stable subsystems to on-demand docs | pending |
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

## Next starter prompt (Pass 9d.2a)

> **Goal.** Stop invariants.md from bumping the 400-line ceiling
> every pass. Promote stable subsystems to on-demand docs and
> leave one-line pointers in invariants.md.
>
> **Scope.** Subsystems whose binding facts have settled and
> rarely change: Cache I/II/III; Attachments I/II; JMAP backend
> + per-folder baseline pull; IMAP backend + idle; Catkin
> (core/styling/commands/QoL/annotation/spellcheck — already
> compacted in 9d.2 but still ~30 lines). Each becomes a doc
> under `docs/poplar/` (e.g. `cache-invariants.md`,
> `attachments-invariants.md`, `mail-backends-invariants.md`,
> `catkin-invariants.md`). Path-scoped auto-load via
> `.claude/rules/` matching the source paths.
>
> **Settled.** Pre-beta posture; the path-scoped pattern is
> already proven by `.claude/rules/ui-invariants.md`. Decision
> index stays in invariants.md (one source of truth for ADR
> mapping).
>
> **Still open — brainstorm before coding:** which subsystems
> are stable enough to promote (mail-backends could still churn
> for Pass 9.6 OAuth refresh); whether the decision-index table
> compacts further once subsystem detail moves out; the rule
> globs for path-scoped auto-load.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/2026-05-05-invariants-compaction.md`,
> then execute. Target: invariants.md ≤ 250 lines after the pass.
> Pass-end ritual: one ADR codifying the split policy.

## Queued passes (after 9d.2a)

- **9d.3** — golangci-lint on `internal/catkin/` with errcheck,
  staticcheck, unused, gocritic, revive, errorlint, unparam,
  nilerr.
- **9d.4** — Live tmux verification at edge sizes. 80×24 with the
  popover open over a misspelling near the right edge and the
  bottom edge.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
