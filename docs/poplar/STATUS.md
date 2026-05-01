# Poplar Status

**Current pass:** Pass 6.7 next — verify Pass 6.6 in tmux + research
the retention/empty pattern against reference apps (mutt, aerc,
himalaya, NeoMutt) to confirm the design before it ossifies.
Pass 6.6 done — `mail.Backend.Destroy` primitive, opt-in retention
sweep, manual empty (`E` + ConfirmModal); ADR-0092/0093/0094.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 5 (incl. SPUA-policy, 2.5b-4b) | Scaffold → backend → UI → bubbletea cleanup (see git log) | done |
| 6 / 6.5 | Triage + undo bar (ADR-0089/0090); move picker (ADR-0091) | done |
| 6.6 | Trash retention + manual empty (Destroy primitive, sweep, ConfirmModal) | done — ADR-0092/0093/0094 |
| 6.7 | Verify retention/empty in tmux + reference-app research on the pattern | next |
| 7 | Polish I — popover narrow-terminal (#15) + small render drift cleanup | pending |
| 8 | Gmail IMAP (direct-on-emersion rewrite) | pending |
| 9 | Compose framing: `Editor` interface, neovim `--embed` adapter, send via go-smtp | pending |
| 9.5 | Compose enhancements: Catkin native editor, tidytext (#12), content cleanup (#13) | pending |
| 10 | Config polish | pending |
| 11 | Final polish + 1.0 prep | pending |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |
| 1.1 | Neovim companion plugin (post-v1, #6) | post-v1 |

## Next starter prompt (Pass 6.7)

> **Goal.** Verify Pass 6.6 (retention sweep + manual empty) end
> to end in a real terminal AND research the pattern against
> reference apps so we either ratify the design or revise it
> before it ossifies.
>
> **Scope.** Two halves, both required:
> 1. Tmux verification per `.claude/docs/tmux-testing.md` and
>    Task 12 of the archived Pass 6.6 plan (`E` confirm flow,
>    no-undo toast, sweep on/off, inert on Inbox). Capture each
>    case to a pane dump.
> 2. Reference-app research: how do mutt, NeoMutt, aerc, himalaya,
>    meli (and a representative GUI client) handle Trash
>    auto-purge, manual empty, permanent-delete bypass, and
>    confirmation copy? Write to
>    `docs/poplar/research/YYYY-MM-DD-trash-retention-norms.md`.
>    Compare to ADR-0092/0093/0094 explicitly.
>
> **Settled:** ADR-0092/0093/0094 are the current design; research
> may recommend revision but does not rewrite invariants without a
> follow-on ADR.
>
> **Still open — answer in the research doc:** retention default 0
> (opt-in) vs 30 (opt-out); sweep trigger (first-visit / every visit
> / timer / on-quit); whether manual empty needs a "type EMPTY"
> affordance for very large folders.
>
> **Approach.** Verify first (may surface bugs that reframe the
> research). Plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-trash-retention-verify.md`
> ties the two halves together. Bugs fix in this pass; design
> recommendations log as a follow-on pass — don't expand scope
> here.

## Audits

- [bubbletea conventions](audits/2026-04-26-bubbletea-conventions.md) · [invariants](audits/2026-04-25-invariants-findings.md) · [library packages](audits/2026-04-25-library-packages-findings.md) · [plan shape](audits/2026-04-25-plan-shape-findings.md)
