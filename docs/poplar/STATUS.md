# Poplar Status

**Current pass:** Pass 9d.1 next — Catkin annotations follow-up audits (split out
from oversized 9d).

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
| 9d.1 | AI-tells sweep on full 9d diff | pending |
| 9d.2 | Render-path adversarial review (RenderAnnotated, applyAnnotationsToLine, ansiSpliceAtCol, runeToByteOffset) | pending |
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

## Next starter prompt (Pass 9d.1)

> **Goal.** Sweep AI-tells across the full Pass 9d diff. Pass 9d
> ran long and Pass 9d.x audits its output rather than trusting
> per-task review.
>
> **Scope.** Run the audit on `git diff 27405df..HEAD --
> internal/catkin/` (the 9d range, pre- to post-pass). A fresh
> subagent enumerates every tell it sees from the 37-tell
> catalogue at `~/.claude/docs/go-comment-voice.md` — listed,
> not severity-classified. Then a second pass applies the fixes.
>
> **Settled.** Pre-beta posture; comments default to none; voice
> palette per `go-conventions` skill.
>
> **Approach.** Dispatch one subagent for enumeration; review the
> list; dispatch a second subagent (or fix inline) for the
> applied edits. Pass-end ritual: ADR if any structural change
> emerges, otherwise a single commit and STATUS bump.

## Queued passes (after 9d.1)

- **9d.2** — Render-path adversarial review. Construct adversarial
  inputs (multi-byte runes mid-annotation, annotation spanning
  the cursor, annotation abutting the cursor block, two
  annotations on one row with the cursor between them) and
  verify each by hand. Targets `RenderAnnotated`,
  `applyAnnotationsToLine`, `ansiSpliceAtCol`, `runeToByteOffset`.
- **9d.3** — golangci-lint on `internal/catkin/` with errcheck,
  staticcheck, unused, gocritic, revive, errorlint, unparam,
  nilerr.
- **9d.4** — Live tmux verification at edge sizes. 80×24 with the
  popover open over a misspelling near the right edge and the
  bottom edge.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
