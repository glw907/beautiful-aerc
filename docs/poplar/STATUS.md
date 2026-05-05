# Poplar Status

**Current pass:** Pass 9d.2 next — render-path adversarial review.

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

## Next starter prompt (Pass 9d.2)

> **Goal.** Render-path adversarial review of Catkin's annotation
> rendering. Construct inputs the per-task review of 9d would not
> have hit and verify each by hand.
>
> **Scope.** `RenderAnnotated`, `applyAnnotationsToLine`,
> `ansiSpliceAtCol`, `runeToByteOffset`. Adversarial inputs:
> multi-byte runes mid-annotation; annotation spanning the cursor;
> annotation abutting the cursor block on either side; two
> annotations on one row with the cursor between them; annotation
> at the very last column when soft-wrap kicks in; annotation
> across a soft-wrap boundary; empty annotation list.
>
> **Settled.** Pre-beta posture; gofmt + voice gates green.
>
> **Approach.** Write targeted test cases under
> `internal/catkin/render_test.go`. Capture findings as either
> bug fixes (commit) or queued work (BACKLOG.md). Pass-end ritual:
> ADR only if a binding fact emerges; otherwise commit + STATUS
> bump.

## Queued passes (after 9d.2)

- **9d.3** — golangci-lint on `internal/catkin/` with errcheck,
  staticcheck, unused, gocritic, revive, errorlint, unparam,
  nilerr.
- **9d.4** — Live tmux verification at edge sizes. 80×24 with the
  popover open over a misspelling near the right edge and the
  bottom edge.
- **invariants compaction** — file is at the 400-line ceiling.
  Collapse Catkin entries (0144–0151) into two bullets, prune
  stabilized Cache facts, compact the decision-index table.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
