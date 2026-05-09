# Poplar Status

**Current pass:** Pass 9w.1 next — build `internal/ansix/` SPUA-aware
width adapter, migrate `uicore` helpers + 16 callsites. ADR-0180
closed the upstream displaywidth override-hook path; fork prototype
parked at `~/Projects/displaywidth/add-overrides`.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9w | Scaffold through displaywidth investigation (ADRs 0001–0180) | done |
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
> width-math layer over `charmbracelet/x/ansi` — and migrate
> `uicore`'s SPUA helpers + 16 callsites onto it.
>
> **Scope.** New package `internal/ansix/` exporting `Width`,
> `Truncate`, `TruncateEllipsis`, `PadOrTruncate`, `SPUACellWidth`,
> `SetSPUACellWidth`, `SpuaCount`. `uicore.FillRowToWidth` and
> `uicore.ApplyBg` stay in `uicore` (lipgloss-styling concerns) but
> route through `ansix` internally. The JoinHorizontal ban + manual
> row joins in `AccountTab.View` / `App.renderFrame` *stay* — the
> shim doesn't unlock them (an upstream lipgloss change would).
> New ADR codifies the package boundary and links ADR-0180.
>
> **Settled.** Package name `internal/ansix`. API drops to `Width`,
> `Truncate`, etc. SPUA-A range `[0xF0000, 0xFFFFD]`. Cell-width
> default `1`; `SetSPUACellWidth` validates `1 || 2` and panics
> otherwise. Test scaffold copied from `uicore/iconwidth_test.go`.
>
> **Still open.** None. Pure implementation.
>
> **Approach.** Package + tests; callsite sweep in one go;
> `make check` + `/simplify`; standard pass-end ritual.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern).
- **Bubble re-eval (post-9w.1).** Revisit
  `docs/superpowers/archive/specs/2026-05-08-bubble-adoption-design.md`
  and Pass 9v's Eval-A outcomes. ansix flips bubbles whose rejection
  was poplar-side width math; bubbles that call `lipgloss.Width`
  internally (`bubbles/help`, `bubble-table`, `glamour`) stay
  rejected per ADR-0180.
