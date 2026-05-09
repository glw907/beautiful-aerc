# Poplar Status

**Current pass:** Pass 9w.1 next — Build `internal/ansix/` SPUA-aware
width adapter and migrate `uicore` width helpers + 16 callsites onto
it. ADR-0180 closed the upstream displaywidth override-hook path
(maintainer's stated wrapper preference; fork prototype parked at
`~/Projects/displaywidth/add-overrides`).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9p | Scaffold through attachments compose UI (ADRs 0001–0179) | done |
| 9v | Bubble Eval A (strong matches) — triage spec; no swaps | done |
| 9w.0 | SPUA seam investigation — path (a) dead; PR-first chosen | done |
| 9w | displaywidth override hook — upstream path closed (ADR-0180) | done |
| 9w.1 | Build `internal/ansix/` shim; migrate uicore + 16 callsites | pending |
| 9q | Outbox delivery controls — undo + schedule send (#35) | pending |
| 9r–9t | List-Unsubscribe (#36), .ics viewer (#37), search (#38) | pending |
| 9u | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 10 | Polish II — popover dim (#14) + items surfaced during 9j–9u | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 9w.1)

> **Goal.** Build `internal/ansix/` — a vendored ANSI/SPUA-aware
> width-math layer over `charmbracelet/x/ansi` — and migrate the
> SPUA helpers out of `uicore` into it. Sweep the 16 poplar
> callsites onto the new package.
>
> **Scope.** New package `internal/ansix/` exporting `Width`,
> `Truncate`, `TruncateEllipsis`, `PadOrTruncate`, `SPUACellWidth`,
> `SetSPUACellWidth`, `SpuaCount`. `uicore.FillRowToWidth` and
> `uicore.ApplyBg` stay in `uicore` (lipgloss-styling concerns) but
> call into `ansix` internally. ADR codifies the package boundary
> and the upstream-blocked rationale (link ADR-0180). The
> JoinHorizontal ban + manual row joins in `AccountTab.View` /
> `App.renderFrame` *stay* — out of scope; the shim doesn't unlock
> them (an upstream lipgloss change would).
>
> **Settled (do not re-brainstorm):** Package name `internal/ansix`.
> Public API mirrors current `uicore.Display*` names dropped to
> `Width`, `Truncate`, etc. SPUA-A range `[0xF0000, 0xFFFFD]`.
> Cell-width state: `1` default, `SetSPUACellWidth` validates
> `1 || 2`, panics otherwise. Test scaffold copied from
> `uicore/iconwidth_test.go` then extended.
>
> **Still open — brainstorm these:** None. Pure implementation.
>
> **Approach.** Write the package + tests, sweep callsites in one
> pass, then `make check` + `/simplify`. Standard pass-end ritual.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
