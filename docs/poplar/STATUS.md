# Poplar Status

**Current pass:** Pass 28 — Compose Editor wrapper deletion.
Pass 27 landed the Catkin all-value path (ADR-0212): Model and
Buffer are value types with With* setters; textarea sealed inside
Buffer; CatkinEditor shim keeps compose untouched until Pass 28.
**Beta soak deferred.** Pre-beta rules apply; soak entry gated
on a full audit cycle returning no findings (`docs/poplar/audit-plan.md`).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 26.1 | Scaffold through Audit A remediation (ADRs 0001–0211) | done |
| 27 | Catkin Elm conformance — all-value path (ADR-0212) | done |
| 28 | **Compose Editor wrapper deletion** | done |
| 29 | `app.go` decomposition — split 874-line `App.Update` | gated |
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

### Next starter prompt (Pass 29)

> **Goal.** Decompose `internal/ui/app.go` so no single file
> exceeds ~600 lines and `App.Update` is split into per-screen
> controllers.
>
> **Scope.** Split the 874-line `App.Update` into
> `updateAccount`, `updateContacts`, `updateCompose`,
> `updateModals`, `updateOutbox`, `updateWizard`. Peel the file
> into `app_update.go` / `app_view.go` / `app_chrome.go`. No
> behavior change. Touches `internal/ui/`.
>
> **Settled (do not re-brainstorm):** Per-screen controllers is
> the direction. File-size cap is the "no single file > ~600
> lines" budget per `STATUS.md`'s pass-size note.
>
> **Still open — brainstorm these:** Exact controller boundaries
> for cross-screen messages (modals over compose, popovers over
> account); how `App.Update` dispatches to controllers when the
> active screen owns the keystroke vs. a modal intercepts.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-app-decomposition.md`,
> then implement. Standard pass-end checklist applies; UI work
> requires reading `docs/poplar/bubbletea-conventions.md` and
> the §10 review checklist at pass-end.
