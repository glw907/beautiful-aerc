# Poplar Status

**Current pass:** Pass 9d.4 next — live tmux at popover edge sizes.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9d.3 | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split → catkin lint sweep (ADRs 0001–0154) | done |
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

## Next starter prompt (Pass 9d.4)

> **Goal.** Verify the spellcheck popover renders correctly when
> opened over a misspelling near the right or bottom edge of an
> 80×24 terminal. Fix any clipping or off-screen positioning the
> capture reveals.
>
> **Scope.** `internal/catkin/popover.go` placement math and any
> renderer that splices the popover into the editor view. Live
> tmux sessions per `.claude/docs/tmux-testing.md`. Capture both
> 80×24 (polish bar) and 120×40.
>
> **Settled.** Idiomatic-bubbletea size contract (popover honors
> assigned width via wordwrap + hardwrap; no parent-side clipping).
> ADR-0144 / 0149 / 0150 / 0152 stand.
>
> **Still open — brainstorm before coding:** whether right-edge
> clipping should flip the popover to anchor on its right corner
> (mirroring the misspelling) or just shift it left to fit; same
> question for bottom edge (flip vs. shift up).
>
> **Approach.** Capture the current behavior at 80×24 first.
> Brainstorm the flip-vs-shift question, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-popover-edges.md`, then
> implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
