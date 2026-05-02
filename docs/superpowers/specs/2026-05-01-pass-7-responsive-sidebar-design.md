---
title: Pass 7 — Responsive sidebar for 80×24 polish
status: accepted
date: 2026-05-01
---

# Goal

Make poplar's default-launch experience at 80×24 — the out-of-box
size on macOS Terminal, GNOME Terminal, Konsole, Alacritty, Kitty,
iTerm2, and every other VT100-lineage terminal — look intentional
and polished, not "fits by luck." Wider widths can improve
further; 80×24 is the bar.

The visible defect at 80×24 today: threaded child rows in the
message list lose their date column (`Thu 2026-04-`) and same-day
AM/PM (`3:41` instead of `3:41 PM`). Root cause is panel
under-budget — the sidebar is fixed at 30 cells, leaving the
message-list pane only 48 cells, which is below the natural width
of a threaded row.

# Decision

Sidebar width becomes a function of terminal width:

```
sidebarWidth(termWidth) = clamp(termWidth - 56, 24, 30)
```

- At termWidth ≥ 86 → 30 (current behavior preserved).
- At termWidth = 80 → 24.
- Linear in between (81 → 25, 82 → 26, …, 86 → 30).
- Below 80 → still 24 (the existing `min(sidebarWidth, width/2)`
  half-width clamp in `AccountTab` continues to handle pathologically
  narrow widths).

The 56-cell offset is the message-list natural minimum: flag(2) +
icon(4) + sender(20) + thread-prefix(4) + subject(8) + gap(2) +
date(14) + sep(1) + right-border(1) = 56. At termWidth=80 with
sidebar=24, message-list gets 54 cells — comfortable for threaded
rows, dates, and same-day timestamps.

The cap at 30 preserves long-folder-name readability at wider
widths.

# Closes BACKLOG #15

#15 ("Help popover: responsive layout for narrow terminals") is
closed by this pass with a separate ADR documenting:

- 80×24 is the design polish bar.
- Below 80, the existing `tooNarrow` fallback string covers help
  popover.
- Help popover natural width budgets stay at ~62 (account) and
  ~58 (viewer) — both fit within the message-list pane at 80×24
  once the sidebar narrows.

# Scope

In:
- Replace `const sidebarWidth = 30` in `internal/ui/account_tab.go`
  with a function `sidebarWidthFor(termWidth int) int`.
- Recompute sidebar width on `WindowSizeMsg` and pass into both
  `Sidebar.SetWidth` and `SidebarSearch.SetWidth` (or constructor
  re-call if no SetWidth exists — add one if needed; this is the
  Elm-architecture-correct path).
- Sidebar folder-row rendering: when the available label cell
  budget is less than the natural label width, truncate with `…`.
  The display cells helper already exists (`displayCells`,
  `displayTruncate`); reuse those.
- Status-bar width math (`app.go:349`) — `dividerCol` is currently
  `sidebarWidth` (the const). Switch to the computed value.
- Verify the message-list date drift resolves at 80×24 after the
  sidebar narrows. If the column allocator still misbudgets the
  date column, fix that too (likely: enforce a `dateColMin = 14`
  floor; thread prefix budgets allocated against subject column,
  not date).
- ADR for responsive sidebar (the formula, the 80×24 bar).
- ADR for #15 close (80×24 polish bar; popover budgets).

Out:
- #23 (HTML word fusion) — orthogonal, content/filter package.
- #14 (popover dim) — already handled by `DimANSI`; verify and
  close as separate work, not in this pass.
- #18 (zero-latency tea.Cmds), #17 (key.Matches), #19 (App.View
  trim) — bubbletea-norms cleanups, not 80×24 polish.
- Help popover layout reflow — not needed; popover fits at 80.
- Sidebar group reordering or folder-row redesign — not in scope.

# Verification

Pass-end requires live tmux capture at:

- **80×24**: the polish bar. Account view, viewer open, viewer +
  help popover, account + help popover, search shelf, move
  picker, undo toast, confirm modal. Each must render
  intentionally — date column intact, threaded rows showing full
  subject + date, no border collisions, folder labels truncated
  cleanly with `…`.
- **86×24**: the transition boundary — sidebar should be at 30,
  identical to current behavior.
- **120×40**: regression check — no change from current.
- **60×24**: best-effort floor — sidebar at 24, message-list at
  ~34, content squeezed but no rendering crashes.

Captures saved alongside the pass commit for the ADR record.

# Implementation notes

- `sidebarWidthFor` lives in `internal/ui/account_tab.go` next to
  what is currently the const. Pure function, table-driven test.
- Sidebar's existing `width` field is set via constructor today.
  Add `SetWidth(int)` to `Sidebar` and `SidebarSearch`. Call from
  `AccountTab.SetSize` (or wherever it currently handles size —
  audit during implementation).
- Folder row rendering already uses `displayCells` for width math
  per ADR-0084. Truncation goes through `displayTruncate` to
  preserve SPUA-icon cell counts.
- Counts column (e.g. ` 102`) stays right-aligned. When sidebar
  narrows, label gets the cut, count column does not.
- The half-width fallback `min(sidebarWidth, m.width/2)` in
  `account_tab.go:109,771,806` becomes
  `min(sidebarWidthFor(m.width), m.width/2)`.

# Risks

- **Folder label truncation surprises.** "Membership Committee" →
  "Membership Com…" at sidebar=24 may be unfamiliar. Mitigation:
  the user installs poplar at one terminal width and stays there;
  truncation is consistent within a session. Acceptable.
- **Status bar / chrome divider drift.** `app.go:349`'s
  `dividerCol = sidebarWidth` is now dynamic. Must recompute on
  resize and verify the `┬` and `┴` glyphs land on the divider
  column at all widths. Covered by the verification matrix.
- **SidebarSearch shelf width.** The 3-row search shelf is bound
  to sidebar width. Narrowing it tightens the search input — but
  the shelf at sidebar=24 still has ~20 cells of input, plenty.

# Test plan

- Unit: `sidebarWidthFor` table-driven (60→24, 79→24, 80→24, 81→25,
  85→29, 86→30, 120→30, 200→30).
- Unit: folder label truncation at sidebar=24 with long custom
  folder names.
- Live tmux: the verification matrix above. Saved capture diffs
  in the pass commit.
