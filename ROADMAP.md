# ROADMAP

> Strategic initiatives. Managed by `/log-project`. Issues tracked in `BACKLOG.md`.

## Active

### SPUA-aware width measurement for bubbletea `spua-width-upstream`
Collapse ADR-0084's architectural cost — `displayCells`, `displayTruncate`,
`padToVisibleWidth`, the `lipgloss.JoinHorizontal` ban on icon-bearing
rows, manual row-by-row joins in `AccountTab.View` / `App.renderFrame`,
the three-mode resolution at startup, the `adrg/sysfont` dep, and the
CPR cell-width probe — without giving up Nerd Font iconography.
Investigation (2026-05-08) found that `runewidth.DefaultCondition` is
no longer load-bearing: `charmbracelet/x/ansi` v0.11+ migrated to
`clipperhouse/displaywidth`, which has no per-rune override. The
"one-line override" outcome path is dead. Path forward: file an
upstream PR at `clipperhouse/displaywidth` proposing
`Options.Overrides []OverrideRange` (declarative range-table,
mirroring `EastAsianWidth` / `ControlSequences` precedents). PR-first,
not issue-first — pattern research confirmed the repo's culture. On
merge, file a plumbing PR at `charmbracelet/x/ansi` to expose the
field through `dwOptions`; on release, queue a poplar-side cleanup
pass to collapse the ADR-0084 machinery. Findings and PR draft live
in `docs/superpowers/specs/2026-05-08-spua-width-upstream-investigation.md`
and `docs/superpowers/specs/2026-05-08-displaywidth-issue-draft.md`.
Related: BACKLOG #20 (closed by ADR-0084 — superseded if this lands).
