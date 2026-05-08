# ROADMAP

> Strategic initiatives. Managed by `/log-project`. Issues tracked in `BACKLOG.md`.

## Active

### SPUA-aware width measurement for bubbletea `spua-width-upstream`
Collapse ADR-0084's architectural cost — `displayCells`, `displayTruncate`,
`padToVisibleWidth`, the `lipgloss.JoinHorizontal` ban on icon-bearing
rows, manual row-by-row joins in `AccountTab.View` / `App.renderFrame`,
the three-mode resolution at startup, the `adrg/sysfont` dep, and the
CPR cell-width probe — without giving up Nerd Font iconography. SPUA-A
glyphs render at 2 cells while `runewidth` reports 1; the fix lives at
the lowest width-measurement layer (`mattn/go-runewidth.DefaultCondition`,
`charmbracelet/x/ansi.StringWidth`, or `lipgloss.Width`). Outcome path:
investigate seams (~half pass); if `runewidth.DefaultCondition.RuneWidth`
is still load-bearing in charm's stack, a one-line startup override
collapses the machinery. If the seam doesn't exist, vendor a thin
`internal/ansix` shim and submit an upstream PR adding a configurable
width method as a community contribution. Success: ADR-0084's
machinery deletes; Eval A reopens for `bubbles/help` and `bubble-table`;
charm-ecosystem alignment improves; Nerd Font users keep their polish.
Related: BACKLOG #20 (closed by ADR-0084 — superseded if this lands)
