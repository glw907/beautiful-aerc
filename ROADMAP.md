# ROADMAP

> Strategic initiatives. Managed by `/log-project`. Issues tracked in `BACKLOG.md`.

## Active

### SPUA-aware width measurement for bubbletea `spua-width-upstream`
Collapse ADR-0084's architectural cost — `displayCells`, `displayTruncate`,
`padToVisibleWidth`, the `lipgloss.JoinHorizontal` ban on icon-bearing
rows, manual row-by-row joins in `AccountTab.View` / `App.renderFrame`,
the three-mode resolution at startup, and the CPR cell-width probe —
without giving up Nerd Font iconography.

**Status (Pass 9w, 2026-05-08):** upstream PR path closed; vendored
`internal/ansix/` shim queued as Pass 9w.1. The investigation found
that `charmbracelet/x/ansi` v0.11+ routes through
`clipperhouse/displaywidth`, which has no per-rune override. A PR
adding `Options.Overrides []OverrideRange` was implemented and
benchmarked on a local fork (parked at
`~/Projects/displaywidth/add-overrides`), but a maintainer comment
on PR #23 (2026-05-02) signaled a preference for wrapping the
library rather than accepting an in-library hook, and the repo had
zero external-contributor merges in the prior seven months. ADR-0180
codifies the decision; archived specs and PR draft live in
`docs/superpowers/archive/specs/`.

**Re-engagement trigger (future).** The fork prototype is preserved.
File the upstream PR — or, more likely, an issue linking the
maintainer's comment — once **two** conditions hold: (a) poplar
(or the eventual ansix package, if spun out) has demonstrable
adoption beyond a single project, *and* (b) the load-bearing
argument shifts from "we want a hook" to "the lipgloss-internal
callsites (`JoinHorizontal`, components like `bubbles/help`,
`bubble-table`) cannot be wrapped at the application layer because
they call `lipgloss.Width` inside their own render code, and the
library's 'just wrap it' answer therefore doesn't compose." That
argument is sharper than the original use-case framing and is what
would land this without burning review goodwill on a second cold
attempt. Until both hold, ansix is sufficient.

Related: BACKLOG #20 (closed by ADR-0084 — superseded if upstream
ever lands).
