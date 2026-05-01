# Poplar Status

**Current pass:** Pass 7 next — Polish I (popover narrow-terminal
+ small render drift cleanup). Pass 6.6 done — `mail.Backend.Destroy`
primitive, opt-in retention sweep on first Disposal-folder visit,
manual empty (`E` + ConfirmModal overlay) with no-undo toast;
ADR-0092/0093/0094.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 4.1, SPUA-policy, 2.5b-4b, 5 | Scaffold through bubbletea cleanup (see git log for breakdown) | done |
| 6 | Triage actions (delete/archive/star/read; toast + undo bar) | done — ADR-0089/0090 |
| 6.5 | Move-to-folder picker (`m` modal; toast + undo) | done — ADR-0091 |
| 6.6 | Trash retention + manual empty (Destroy primitive, sweep, ConfirmModal) | done — ADR-0092/0093/0094 |
| 7 | Polish I — popover narrow-terminal (#15) + small render drift cleanup | next |
| 8 | Gmail IMAP (direct-on-emersion rewrite) | pending |
| 9 | Compose framing: `Editor` interface, neovim `--embed` adapter, send via go-smtp | pending |
| 9.5 | Compose enhancements: Catkin native editor, tidytext (#12), content cleanup (#13) | pending |
| 10 | Config polish | pending |
| 11 | Final polish + 1.0 prep | pending |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |
| 1.1 | Neovim companion plugin (post-v1, #6) | post-v1 |

## Next starter prompt (Pass 7)

> **Goal.** Polish I — fix help-popover overflow on narrow
> terminals (BACKLOG #15) and clean up small render-drift
> findings accumulated since Pass 4.1.
>
> **Scope.** Help popover: at narrow widths the popover currently
> overflows or clips badly; survey behavior in tmux at 60×24,
> 80×24, 100×30, fix the layout. Render drift: walk BACKLOG for
> any "small render bug" entries logged after Pass 4.1, triage,
> fix the cheap ones, log the rest. No new features.
>
> **Settled:** Overlay+dim pattern (ADR-0082/0087/0091/0094).
> Bubbletea size contract (ADR-0083/0084). Help popover
> future-binding policy (ADR-0072).
>
> **Still open — brainstorm:** What's the popover's narrow-mode
> strategy — shrink content, drop columns, fall back to
> single-column scroll, or refuse to render? Pick one before
> coding.
>
> **Approach.** Brainstorm the narrow-mode strategy, plan at
> `docs/superpowers/plans/YYYY-MM-DD-polish-i.md`, implement.
> Standard pass-end checklist applies.

## Audits

- 2026-04-26: [bubbletea conventions](audits/2026-04-26-bubbletea-conventions.md)
- 2026-04-25: [invariants](audits/2026-04-25-invariants-findings.md) · [library packages](audits/2026-04-25-library-packages-findings.md) · [plan shape](audits/2026-04-25-plan-shape-findings.md)
