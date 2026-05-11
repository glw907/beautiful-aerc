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
| 28 | **Compose Editor wrapper deletion** | next |
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

### Next starter prompt (Pass 28)

> **Goal.** Delete `mailcompose.Editor` + `CatkinEditor`; compose
> embeds `catkin.Model` directly. Mechanical follow-up to Pass 27
> (ADR-0212).
>
> **Scope.** Remove `internal/mailcompose/editor.go`. Change
> `compose.Model.editor` field type to `catkin.Model`. Rewire the
> constructor and every call site in
> `internal/ui/compose/model.go` and `internal/ui/compose/tidy.go`
> to use `With*` setters. Fold the tidy result handler's paired
> mutation into one statement (value + highlights in a single
> `WithValue(...).WithTidyHighlights(...)` chain). Convert
> `internal/ui/compose/*_test.go` to the value-setter form.
> Update ADR-0033 Consequences to mark the interface-based
> adapter strategy superseded; the goal survives. Write
> ADR-0213.
>
> **Settled (do not re-brainstorm):** Direction is set by
> ADR-0212. The Editor interface is a single-impl seam; it
> deletes. Compose mutates the embedded catkin from its own
> Update via `c.editor = c.editor.WithX(...)`, mirroring the
> wizard.
>
> **Still open — brainstorm these:** None. Pure mechanical
> follow-up.
>
> **Approach.** Execute the Pass 28 section of
> `docs/superpowers/plans/2026-05-11-catkin-elm.md` (Tasks 8–10).
> Standard pass-end checklist applies; UI work requires the
> idiomatic-bubbletea §10 checklist at pass-end.
