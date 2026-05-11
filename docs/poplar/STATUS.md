# Poplar Status

**Current pass:** Pass 30 — Audit B.1 (Elm + bubbletea v2
conformance). Pass 29 landed the `App.Update` decomposition
(ADR-0214): per-domain `app_{chrome,outbox,compose,modals,
contacts}.go`, chrome runs first in the dispatcher chain,
`armToast` + `openNewCompose` unify duplicated call sites.
**Beta soak deferred.** Pre-beta rules apply; soak entry gated
on a full audit cycle returning no findings.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 26.1 | Scaffold through Audit A remediation (ADRs 0001–0211) | done |
| 27 | Catkin Elm conformance — all-value path (ADR-0212) | done |
| 28 | Compose Editor wrapper deletion (ADR-0213) | done |
| 29 | `App.Update` decomposition (ADR-0214) | done |
| 30 | **Audit B.1** — Elm + bubbletea v2 conformance | gate |
| 31 | **Audit B.2** — general structural integrity | gate |
| 32 | v2 declarative View fields — ProgressBar + ReportFocus + KeyboardEnhancements | gated |
| 33 | Mouse support (reader + attachments + scroll) | gated |
| 34 | Mouse support (sidebar + cross-pane) — optional split from 33 | gated |
| 35 | Native OAuth for Gmail / Outlook IMAP (#42, BYO client ID) | gated |
| 36 | **Audit C** — feature surface | gate |
| 37 | **Audit Final** — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |
| post-1.0 | Neovim companion (#6), raw RFC822 (#21), beyond | future |

### Next starter prompt (Pass 30)

> **Goal.** Audit B.1 — Elm architecture + bubbletea v2 conformance
> across `internal/ui/`. Find divergences, report findings; fix
> only the no-judgment-call ones inline.
>
> **Scope.** Walk every `internal/ui/**/*.go` Update / View /
> Cmd against the Elm rules in `elm-conventions` and the v2 idioms
> in `docs/poplar/bubbletea-conventions.md` §10. Flag: state
> mutation in View or Cmd closures, blocking I/O outside Cmds,
> `len()` used as width on icon-bearing strings, parent-side
> clipping, callback / parent-pointer back-channels, missing
> `WindowSizeMsg` forwarding, raw `tea.KeyMsg` literals, v1-only
> API leaks, manual chrome setup that should be declarative on
> `tea.View`.
>
> **Settled (do not re-brainstorm):** Audit-only pass. Findings
> classified as P0 (apply inline), P1 (BACKLOG), P2 (note in ADR
> only).
>
> **Still open — brainstorm these:** None. Pure implementation
> pass following `docs/poplar/audit-plan.md`.
>
> **Approach.** Write a findings doc at
> `docs/superpowers/plans/2026-05-12-audit-b1.md`, classify, apply
> P0s, file P1s in BACKLOG. Standard pass-end checklist applies;
> UI work requires reading `docs/poplar/bubbletea-conventions.md`
> and the §10 review checklist at pass-end.
