# Poplar Status

**Current pass:** Pass 31 — Audit B.2 (general structural
integrity). Pass 30 closed Audit B.1 (ADR-0215) with three
inline fixes — wizard `writeConfig` as a Cmd, catkin
`SetVirtualCursor(false)`, `WindowSizeMsg` forwards in
messagelist + reader — and one ROADMAP project (`ui-all-value`,
the eight-subpackage receiver-discipline remediation).
**Beta soak deferred.** Pre-beta rules apply; soak entry gated
on a full audit cycle returning no findings.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 26.1 | Scaffold through Audit A remediation (ADRs 0001–0211) | done |
| 27 | Catkin Elm conformance — all-value path (ADR-0212) | done |
| 28 | Compose Editor wrapper deletion (ADR-0213) | done |
| 29 | `App.Update` decomposition (ADR-0214) | done |
| 30 | Audit B.1 — Elm + bubbletea v2 conformance (ADR-0215) | done |
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

### Next starter prompt (Pass 31)

> **Goal.** Audit B.2 — general structural integrity across
> `internal/ui/` after the `app.go` decomposition + Editor-
> wrapper deletion. Find divergences on non-Elm dimensions
> (file size, interface design, package layering); fix only
> the no-judgment-call ones inline.
>
> **Scope.** Walk against the focus list in
> `docs/poplar/audit-plan.md` §"Phase B.2": `app.go`
> decomposition regression (no `update<Screen>` >150 lines,
> no back-channel coupling, dispatch exhaustive), file-size
> budget (no `internal/ui/` file > ~600 lines without a named
> reason), interface count after `Editor` deletion (any new
> single-impl interfaces named with their seam), package-
> boundary leaks (no subpackage imports `internal/ui`).
>
> **Settled (do not re-brainstorm):** Audit-only pass. Findings
> classified as P0 (apply inline), P1 (BACKLOG/ROADMAP), P2
> (note in ADR only). The `ui-all-value` ROADMAP project is
> already booked; do not refile it as a B.2 finding.
>
> **Still open — brainstorm these:** None.
>
> **Approach.** Write a findings doc at
> `docs/superpowers/plans/2026-05-13-audit-b2.md`, classify,
> apply P0s, file P1s in BACKLOG / ROADMAP. Standard pass-end
> checklist applies.
