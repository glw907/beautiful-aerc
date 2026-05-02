# Pass 7 — Responsive Sidebar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make poplar's 80×24 default-launch experience polished by replacing the fixed 30-cell sidebar with a responsive width that narrows linearly to 24 cells at terminal width 80, freeing the message-list pane from 48 to 54 cells so threaded rows render their full subject + date column.

**Architecture:** `const sidebarWidth = 30` becomes `func sidebarWidthFor(termWidth int) int = clamp(termWidth - 56, 24, 30)`. All five call sites in `account_tab.go` plus two in `app.go` switch to the function. `Sidebar` and `SidebarSearch` gain `SetWidth` methods (Elm-architecture-correct path; SetSize already exists for one but width changes need to flow through). Folder rows truncate labels with `…` when the per-row label budget is exceeded, preserving a 1-cell right margin against the chrome divider.

**Tech Stack:** Go 1.26, bubbletea/bubbles/lipgloss, `internal/ui/iconwidth.go` cell-aware helpers (`displayCells`, `displayTruncate`), tmux for live verification.

---

## Pre-flight

- [ ] **Step 0: Read the spec**

Path: `docs/superpowers/specs/2026-05-01-pass-7-responsive-sidebar-design.md`. Confirm formula, label-truncation rule, 1-cell right-margin invariant, verification matrix.

- [ ] **Step 0b: Confirm working tree clean**

```bash
git status
```

Expected: `nothing to commit, working tree clean` on `master`. (Pre-1.0 poplar passes commit directly to master per project memory; no worktree.)

---

## File Structure

**Create:**
- `docs/poplar/decisions/0096-responsive-sidebar-width.md` — ADR for the formula and clamp.
- `docs/poplar/decisions/0097-eighty-by-twentyfour-polish-bar.md` — ADR closing #15; codifies 80×24 as the design polish bar.
- `internal/ui/sidebar_width_test.go` — table-driven test for `sidebarWidthFor`.

**Modify:**
- `internal/ui/account_tab.go` — replace `const sidebarWidth = 30` with `func sidebarWidthFor`; update 5 call sites.
- `internal/ui/sidebar.go` — add `SetWidth(int)`; update `renderRow` to truncate label with `…`.
- `internal/ui/sidebar_search.go` — add `SetWidth(int)`.
- `internal/ui/iconwidth.go` — add `displayTruncateEllipsis(s string, n int) string`.
- `internal/ui/iconwidth_test.go` — add tests for the new helper.
- `internal/ui/sidebar_test.go` — add tests for label truncation, right-margin invariant.
- `internal/ui/app.go` — update `dividerCol` and `statusBar.View` calls to use `sidebarWidthFor(m.width)`.
- `docs/poplar/invariants.md` — replace fixed-width sidebar fact with responsive formula; update decision-index table.
- `docs/poplar/STATUS.md` — mark Pass 7 done; insert next starter prompt.
- `BACKLOG.md` — close #15 with resolution note.

---

### Task 1: `sidebarWidthFor` function with table-driven test

**Files:**
- Modify: `internal/ui/account_tab.go:20-21`
- Create: `internal/ui/sidebar_width_test.go`

- [ ] **Step 1.1: Write the failing test**

Create `internal/ui/sidebar_width_test.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import "testing"

func TestSidebarWidthFor(t *testing.T) {
	cases := []struct {
		termWidth int
		want      int
	}{
		{60, 24},   // below floor: clamp to 24
		{79, 24},   // just below 80
		{80, 24},   // polish bar
		{81, 25},   // linear
		{82, 26},
		{83, 27},
		{84, 28},
		{85, 29},
		{86, 30},   // full width
		{120, 30},  // wider: capped
		{200, 30},  // far wider: capped
		{0, 24},    // pathological: clamp to 24
	}
	for _, tc := range cases {
		got := sidebarWidthFor(tc.termWidth)
		if got != tc.want {
			t.Errorf("sidebarWidthFor(%d) = %d, want %d",
				tc.termWidth, got, tc.want)
		}
	}
}
```

- [ ] **Step 1.2: Run test to verify it fails**

```bash
go test ./internal/ui/ -run TestSidebarWidthFor -v
```

Expected: compile error — `undefined: sidebarWidthFor`.

- [ ] **Step 1.3: Implement the function**

In `internal/ui/account_tab.go`, replace:

```go
// sidebarWidth is the fixed width of the sidebar panel.
const sidebarWidth = 30
```

with:

```go
// sidebarWidthFor returns the sidebar width in terminal cells given
// the current terminal width. Linear from 24 cells at termWidth=80
// up to 30 cells at termWidth>=86; clamped to [24, 30].
//
// 80x24 is the design polish bar (default launch size on every
// VT100-lineage terminal). The 56-cell offset is the message-list
// natural minimum: flag(2) + icon(4) + sender(20) + thread-prefix(4)
// + subject(8) + gap(2) + date(14) + sep(1) + right-border(1).
//
// See ADR-0096 (responsive sidebar) and ADR-0097 (80x24 polish bar).
func sidebarWidthFor(termWidth int) int {
	const minWidth, maxWidth = 24, 30
	w := termWidth - 56
	if w < minWidth {
		return minWidth
	}
	if w > maxWidth {
		return maxWidth
	}
	return w
}
```

- [ ] **Step 1.4: Run test to verify it passes**

```bash
go test ./internal/ui/ -run TestSidebarWidthFor -v
```

Expected: `--- PASS: TestSidebarWidthFor`.

- [ ] **Step 1.5: Run full UI test suite**

```bash
go test ./internal/ui/...
```

Expected: every existing test still passes. (Removing the const will surface call-site failures — fix them in the next task. If only `sidebarWidthFor` and call sites break, proceed; if anything else breaks, stop and report.)

If compile errors occur at the call sites, that is expected and Task 2 fixes them. Verify the only compile errors are `undefined: sidebarWidth` at the 5 known sites:

```bash
go build ./... 2>&1 | grep "sidebarWidth"
```

Expected output names lines `account_tab.go:73`, `account_tab.go:74`, `account_tab.go:109`, `account_tab.go:771`, `account_tab.go:806`, plus `app.go:349` and `app.go:351`.

- [ ] **Step 1.6: Commit**

```bash
git add internal/ui/account_tab.go internal/ui/sidebar_width_test.go
git commit -m "$(cat <<'EOF'
Pass 7: introduce sidebarWidthFor for responsive sidebar

Replace the fixed const sidebarWidth = 30 with a function that clamps
termWidth - 56 into [24, 30]. Linear narrowing below termWidth=86,
flat at 30 above. Closes the 80x24 polish gap by giving the message
list 54 cells instead of 48.

Pre-ADR; see Pass 7 plan.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

(The build is intentionally broken at this commit — call sites updated next task. Acceptable because this is a single contiguous pass landing on master; commit boundary is for review legibility, not bisectability.)

---

### Task 2: Update `account_tab.go` and `app.go` call sites

**Files:**
- Modify: `internal/ui/account_tab.go:73,74,109,771,806`
- Modify: `internal/ui/app.go:349,351`

- [ ] **Step 2.1: Update NewAccountTab constructor sites**

In `internal/ui/account_tab.go` near line 73-74:

```go
// Initial sidebar width before WindowSizeMsg arrives. Use the
// max width; AccountTab.updateTab recomputes on every resize.
initialSidebar := sidebarWidthFor(96)
return AccountTab{
    styles:        styles,
    icons:         icons,
    backend:       backend,
    uiCfg:         uiCfg,
    sidebar:       NewSidebar(styles, nil, uiCfg, initialSidebar, 1, icons),
    sidebarSearch: NewSidebarSearch(styles, initialSidebar, icons),
    msglist:       NewMessageList(styles, nil, 1, 1, icons),
    viewer:        NewViewer(styles, t, backend.AccountEmail()),
    keys:          NewAccountKeys(),
    pages:         make(map[string]*folderPage),
    swept:         make(map[string]bool),
    spinner:       NewSpinner(t),
}
```

- [ ] **Step 2.2: Update WindowSizeMsg handler**

At `account_tab.go:109`, replace:

```go
sw := min(sidebarWidth, m.width/2)
```

with:

```go
sw := min(sidebarWidthFor(m.width), m.width/2)
```

- [ ] **Step 2.3: Update View()**

At `account_tab.go:771`, replace:

```go
sw := min(sidebarWidth, m.width/2)
```

with:

```go
sw := min(sidebarWidthFor(m.width), m.width/2)
```

At `account_tab.go:806`, replace:

```go
mw := max(1, m.width-min(sidebarWidth, m.width/2)-1)
```

with:

```go
mw := max(1, m.width-min(sidebarWidthFor(m.width), m.width/2)-1)
```

- [ ] **Step 2.4: Update app.go**

At `internal/ui/app.go:349-351`, replace:

```go
dividerCol := sidebarWidth
topLine := m.topLine.View(m.width, dividerCol)
status := m.statusBar.View(m.width, sidebarWidth)
```

with:

```go
dividerCol := sidebarWidthFor(m.width)
topLine := m.topLine.View(m.width, dividerCol)
status := m.statusBar.View(m.width, dividerCol)
```

- [ ] **Step 2.5: Verify build is green**

```bash
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 2.6: Run full test suite**

```bash
go test ./...
```

Expected: all tests pass. If any fail, investigate — likely candidates: tests that hard-coded `30` for sidebar width. Update them to call `sidebarWidthFor(testTermWidth)` rather than baking in 30, since the test's expected sidebar should now be derived.

- [ ] **Step 2.7: Commit**

```bash
git add internal/ui/account_tab.go internal/ui/app.go
git commit -m "$(cat <<'EOF'
Pass 7: thread sidebarWidthFor through every call site

Five sites in account_tab.go (constructor, WindowSizeMsg, View, the
loading branch) and two in app.go (dividerCol, statusBar.View) now
compute sidebar width from the current terminal width.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: `displayTruncateEllipsis` helper

**Files:**
- Modify: `internal/ui/iconwidth.go`
- Modify: `internal/ui/iconwidth_test.go`

- [ ] **Step 3.1: Write the failing test**

Append to `internal/ui/iconwidth_test.go`:

```go
func TestDisplayTruncateEllipsis(t *testing.T) {
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"fits exactly", "Inbox", 5, "Inbox"},
		{"fits with room", "Inbox", 10, "Inbox"},
		{"truncates with ellipsis", "Membership Committee", 14, "Membership C…"},
		{"truncates short label", "Buccaneer 18", 8, "Buccane…"},
		{"empty input", "", 5, ""},
		{"zero budget", "anything", 0, ""},
		{"one-cell budget", "anything", 1, "…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := displayTruncateEllipsis(tc.in, tc.n)
			if got != tc.want {
				t.Errorf("displayTruncateEllipsis(%q, %d) = %q, want %q",
					tc.in, tc.n, got, tc.want)
			}
			if displayCells(got) > tc.n {
				t.Errorf("result %q exceeds budget %d (%d cells)",
					got, tc.n, displayCells(got))
			}
		})
	}
}
```

- [ ] **Step 3.2: Run test to verify it fails**

```bash
go test ./internal/ui/ -run TestDisplayTruncateEllipsis -v
```

Expected: compile error — `undefined: displayTruncateEllipsis`.

- [ ] **Step 3.3: Implement the helper**

Append to `internal/ui/iconwidth.go`:

```go
// displayTruncateEllipsis truncates s to at most n terminal display
// cells, appending '…' (1 cell) when truncation occurs. Returns ""
// when n <= 0. Returns "…" when n == 1 and s is non-empty and longer
// than 1 cell.
func displayTruncateEllipsis(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if displayCells(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return displayTruncate(s, n-1) + "…"
}
```

- [ ] **Step 3.4: Run test to verify it passes**

```bash
go test ./internal/ui/ -run TestDisplayTruncateEllipsis -v
```

Expected: `--- PASS`. All sub-cases green.

- [ ] **Step 3.5: Commit**

```bash
git add internal/ui/iconwidth.go internal/ui/iconwidth_test.go
git commit -m "$(cat <<'EOF'
Pass 7: add displayTruncateEllipsis cell-aware helper

Wraps displayTruncate with a trailing '…' when truncation occurs.
Used by sidebar folder-row rendering to truncate long folder labels
at narrow widths while preserving the 1-cell right margin.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: `Sidebar.SetWidth` method

**Files:**
- Modify: `internal/ui/sidebar.go`

- [ ] **Step 4.1: Inspect existing SetSize**

```bash
grep -n "func (s \*\?Sidebar) Set" internal/ui/sidebar.go
```

Note whether `Sidebar` is already a pointer-receiver or value-receiver type. Match the existing style.

- [ ] **Step 4.2: Write the failing test**

Append to `internal/ui/sidebar_test.go`:

```go
func TestSidebarSetWidth(t *testing.T) {
	s := NewSidebar(testStyles(), nil, config.UIConfig{}, 30, 10, SimpleIcons)
	s.SetWidth(24)
	if s.width != 24 {
		t.Errorf("after SetWidth(24), width = %d, want 24", s.width)
	}
	s.SetWidth(30)
	if s.width != 30 {
		t.Errorf("after SetWidth(30), width = %d, want 30", s.width)
	}
}
```

(If `testStyles()` does not exist, use the existing pattern from other tests in `sidebar_test.go` for constructing a `Sidebar`. Read the file first to match.)

- [ ] **Step 4.3: Run test to verify it fails**

```bash
go test ./internal/ui/ -run TestSidebarSetWidth -v
```

Expected: compile error — `undefined: (*Sidebar).SetWidth` or similar.

- [ ] **Step 4.4: Add the method**

In `internal/ui/sidebar.go`, near the existing `SetSize`/`SetSelected`/etc. methods, add:

```go
// SetWidth updates the rendered column width. Height is unchanged.
// Folder labels truncate to fit when the new width is too narrow for
// their natural width (see renderRow).
func (s *Sidebar) SetWidth(w int) {
	s.width = w
}
```

(Match the actual receiver style — adjust to value receiver if `Sidebar` uses value receivers consistently. If sidebar already has a `SetSize(w, h int)` method, this is a thin sibling that updates width only — `SetSize` already does both, so an alternative is to just call `s.SetSize(w, currentHeight)` from the parent. Pick whichever is more idiomatic for the existing code; if `SetSize` is the only mutator pattern, **delete this task** and use `SetSize` from the parent instead, passing the current height.)

- [ ] **Step 4.5: Run test to verify it passes**

```bash
go test ./internal/ui/ -run TestSidebarSetWidth -v
```

Expected: `--- PASS`.

- [ ] **Step 4.6: Commit**

Defer commit until Task 5 (couples with the parent re-wiring).

---

### Task 5: `SidebarSearch.SetWidth` method

**Files:**
- Modify: `internal/ui/sidebar_search.go`

- [ ] **Step 5.1: Inspect existing setters**

```bash
grep -n "func (s \*\?SidebarSearch) Set" internal/ui/sidebar_search.go
```

- [ ] **Step 5.2: If `SetSize(w int)` already exists, skip this task**

`SidebarSearch.SetSize(sw)` is called from `account_tab.go:112` with a single arg — search shelf has no height parameter. If `SetSize` already updates width, no `SetWidth` is needed; the `account_tab.go` resize handler already calls it.

- [ ] **Step 5.3: Otherwise add SetWidth**

Mirror Task 4's pattern.

- [ ] **Step 5.4: Run tests**

```bash
go test ./internal/ui/...
```

Expected: green.

---

### Task 6: Truncate folder labels in `renderRow`

**Files:**
- Modify: `internal/ui/sidebar.go:252-291` (renderRow)
- Modify: `internal/ui/sidebar_test.go`

- [ ] **Step 6.1: Write the failing tests**

Append to `internal/ui/sidebar_test.go`:

```go
func TestSidebarRenderRow_TruncatesLongLabel(t *testing.T) {
	// Construct a sidebar at width 24 with a folder whose label
	// is longer than the per-row label budget.
	s := newTestSidebarWithFolder(t, 24, "Membership Committee", 0)
	out := s.View()
	// First non-blank row should contain the truncated label,
	// ending with the '…' glyph somewhere in the row.
	if !strings.Contains(out, "Membership C") {
		t.Errorf("output missing truncated label start: %q", out)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("output missing ellipsis glyph: %q", out)
	}
	// No row may contain the full untruncated label at width 24.
	if strings.Contains(out, "Membership Committee") {
		t.Errorf("output contains untruncated label at width 24: %q", out)
	}
}

func TestSidebarRenderRow_PreservesRightMargin(t *testing.T) {
	// At every sidebar width in [24, 30], every rendered row's
	// last display cell must be whitespace (the 1-cell right margin
	// before the chrome divider).
	for w := 24; w <= 30; w++ {
		s := newTestSidebarWithFolder(t, w, "Membership Committee", 5)
		out := s.View()
		for i, line := range strings.Split(out, "\n") {
			if line == "" {
				continue
			}
			// Strip ANSI to inspect cells.
			plain := ansi.Strip(line)
			if displayCells(plain) != w {
				t.Errorf("width=%d row %d: cells=%d, want %d (%q)",
					w, i, displayCells(plain), w, plain)
			}
			runes := []rune(plain)
			if len(runes) == 0 {
				continue
			}
			last := runes[len(runes)-1]
			if last != ' ' {
				t.Errorf("width=%d row %d: last rune %q, want space",
					w, i, last)
			}
		}
	}
}
```

Add helper `newTestSidebarWithFolder` in the same test file. Sketch:

```go
// newTestSidebarWithFolder builds a Sidebar with a single classified
// folder for narrow-width layout tests. Mirrors the fixture style
// already used in sidebar_test.go (read existing test setup before
// extending — match its construction pattern verbatim, including
// IconSet selection and Styles construction).
func newTestSidebarWithFolder(t *testing.T, w int, label string, unread int) *Sidebar {
    t.Helper()
    folders := []mail.ClassifiedFolder{
        {
            Folder:      mail.Folder{Name: label, Unseen: unread},
            DisplayName: label,
            Group:       mail.GroupCustom,
        },
    }
    s := NewSidebar(testStyles(), folders, config.UIConfig{}, w, 5, SimpleIcons)
    return &s
}
```

If `testStyles()` is named differently in the existing test file, substitute. If the existing tests construct `IconSet` via a helper, use it. The point: a single short folder list at the requested width.

- [ ] **Step 6.2: Run tests to verify they fail**

```bash
go test ./internal/ui/ -run "TestSidebarRenderRow" -v
```

Expected: failures — long label is not truncated, or row width is wrong, or last cell isn't whitespace.

- [ ] **Step 6.3: Update `renderRow` to truncate the label**

In `internal/ui/sidebar.go` `renderRow`, restructure as follows. The existing local `rightMargin := 1` becomes a single-source value used by both budget computation and gap math.

Replace the block from `icon := applyBg(...)` through `gap := max(1, ...)` with:

```go
icon := applyBg(textStyle, bgStyle).Render(entry.icon)

// countStr/countWidth must be known before truncating the label,
// because the label budget depends on countWidth.
var countStr string
var countWidth int
if hasUnread {
    countStr = applyBg(textStyle, bgStyle).Render(strconv.Itoa(entry.cf.Folder.Unseen))
    countWidth = lipgloss.Width(countStr)
}

// Per-row layout (cells):
//   indicator(1) + sp(1) + icon(2 or 4) + sp×2
//   + name(labelBudget) + gap(>=1) + countStr + rightMargin(1)
// Solve for labelBudget and truncate the display name to fit.
const rightMargin = 1
leadCells := displayCells(indicator) + 1 + displayCells(icon) + 2
countGap := 0
if hasUnread {
    countGap = 1 // ensure at least 1 cell separates label from count
}
labelBudget := s.width - leadCells - countWidth - countGap - rightMargin
if labelBudget < 1 {
    labelBudget = 1
}
displayName := displayTruncateEllipsis(entry.cf.DisplayName, labelBudget)
name := applyBg(textStyle, bgStyle).Render(displayName)

leftContent := indicator + bgStyle.Render(" ") + icon + bgStyle.Render("  ") + name
leftWidth := displayCells(leftContent)

gap := max(1, s.width-leftWidth-countWidth-rightMargin)
```

The existing `row := leftContent + ... + countStr + ... + rightMargin spaces` line and the `fillRowToWidth` call below are unchanged; they consume the variables defined above.

Note: the existing `rightMargin := 1` line inside the function (further down) must be removed — there is now a single `const rightMargin = 1` near the top of the function. Likewise the existing `var countStr string ... countWidth = lipgloss.Width(countStr)` block (which was below the `name :=` line) must be deleted because it has been moved up.

- [ ] **Step 6.4: Run tests to verify they pass**

```bash
go test ./internal/ui/ -run "TestSidebarRenderRow" -v
```

Expected: `--- PASS` for both `TruncatesLongLabel` and `PreservesRightMargin`.

- [ ] **Step 6.5: Run full UI test suite**

```bash
go test ./internal/ui/...
```

Expected: green. Existing sidebar tests should still pass — short labels (`Inbox`, `Sent`) fit under any budget ≥ their natural width, so truncation is a no-op.

- [ ] **Step 6.6: Commit**

```bash
git add internal/ui/sidebar.go internal/ui/sidebar_search.go internal/ui/sidebar_test.go
git commit -m "$(cat <<'EOF'
Pass 7: truncate folder labels with '…' at narrow sidebar widths

renderRow now computes the per-row label budget and truncates the
folder display name via displayTruncateEllipsis when it exceeds the
budget. The 1-cell right margin before the chrome divider is
preserved at every sidebar width in [24, 30]; verified by a unit
test that asserts every rendered row's last cell is whitespace.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Live tmux verification at 80×24

**Files:**
- None modified; verification only.

This is the polish-bar gate. If anything looks off, file a follow-up task before moving on.

- [ ] **Step 7.1: Install the binary**

```bash
make install
```

Expected: `GOBIN=... go install ./cmd/poplar`, no errors.

- [ ] **Step 7.2: Capture base account view at 80×24**

```bash
tmux kill-session -t p7-base 2>/dev/null
tmux new-session -d -s p7-base -x 80 -y 24 'poplar'
sleep 2.5
tmux capture-pane -t p7-base -p > /tmp/p7-80x24-base.txt
cat /tmp/p7-80x24-base.txt
```

Expected:
- Sidebar width is 24 cells (left border `─` count = 24 in top chrome line).
- Long folder labels (e.g. "Membership Committee", "Notifications") are truncated with `…`.
- Last cell of every sidebar row before the `│` divider is whitespace.
- Message-list pane shows full sender + thread-prefix + subject + date including 4-digit year.
- No `╭` border collisions, no clipped subjects, no truncated dates.

- [ ] **Step 7.3: Capture viewer + help popover**

```bash
tmux kill-session -t p7-viewer-help 2>/dev/null
tmux new-session -d -s p7-viewer-help -x 80 -y 24 'poplar'
sleep 2.5
tmux send-keys -t p7-viewer-help 'Enter'
sleep 1.0
tmux send-keys -t p7-viewer-help '?'
sleep 0.4
tmux capture-pane -t p7-viewer-help -p > /tmp/p7-80x24-viewer-help.txt
cat /tmp/p7-80x24-viewer-help.txt
```

Expected: viewer-help popover (~58 cells natural) renders cleanly within the message-list pane; no chrome border collisions.

- [ ] **Step 7.4: Capture account help popover**

```bash
tmux kill-session -t p7-account-help 2>/dev/null
tmux new-session -d -s p7-account-help -x 80 -y 24 'poplar'
sleep 2.5
tmux send-keys -t p7-account-help '?'
sleep 0.4
tmux capture-pane -t p7-account-help -p > /tmp/p7-80x24-account-help.txt
cat /tmp/p7-80x24-account-help.txt
```

Expected: account-help popover (~62 cells natural) fits within message-list pane (54 cells)... 

**WAIT — this is a problem.** The account-help popover natural width is ~62 cells. With a 24-cell sidebar at 80×24, the message-list pane is 54 cells. The popover overlay still centers on the full terminal (80 wide), so it should still fit horizontally — but it will overhang the sidebar/divider area instead of sitting inside the message-list pane. That's the same overlay behavior as today; only the dimmed-underlay layout shifts.

Verify visually: popover renders with `╭...╮` top edge fully visible, contents legible, no border bleed past the terminal right edge. If the popover is now bleeding into the chrome's top/bottom borders due to centering, the `tooNarrow` fallback should fire — confirm by inspecting whether `boxWidth > 80`. Account popover at 62 cells fits in 80 with 18-cell margin; nothing has changed for the popover at 80×24.

- [ ] **Step 7.5: Capture confirm modal**

```bash
tmux kill-session -t p7-confirm 2>/dev/null
tmux new-session -d -s p7-confirm -x 80 -y 24 'poplar'
sleep 2.5
tmux send-keys -t p7-confirm 'T'
sleep 1.0
tmux send-keys -t p7-confirm 'E'
sleep 0.4
tmux capture-pane -t p7-confirm -p > /tmp/p7-80x24-confirm.txt
cat /tmp/p7-80x24-confirm.txt
```

Expected: Empty Trash modal centered, square borders intact, no collision.

- [ ] **Step 7.6: Capture move picker**

```bash
tmux kill-session -t p7-move 2>/dev/null
tmux new-session -d -s p7-move -x 80 -y 24 'poplar'
sleep 2.5
tmux send-keys -t p7-move 'm'
sleep 0.4
tmux capture-pane -t p7-move -p > /tmp/p7-80x24-move.txt
cat /tmp/p7-80x24-move.txt
```

Expected: move picker fits, folder list legible (folder labels in the picker are NOT subject to sidebar narrowing; they may render full-length).

- [ ] **Step 7.7: Capture undo toast**

```bash
tmux kill-session -t p7-toast 2>/dev/null
tmux new-session -d -s p7-toast -x 80 -y 24 'poplar'
sleep 2.5
tmux send-keys -t p7-toast 'd'
sleep 0.3
tmux capture-pane -t p7-toast -p > /tmp/p7-80x24-toast.txt
cat /tmp/p7-80x24-toast.txt
```

Expected: `✓ Deleted 1 message   [u undo]` row renders cleanly above the status bar.

- [ ] **Step 7.8: Capture search shelf**

```bash
tmux kill-session -t p7-search 2>/dev/null
tmux new-session -d -s p7-search -x 80 -y 24 'poplar'
sleep 2.5
tmux send-keys -t p7-search '/' 'a' 's' 'c'
sleep 0.4
tmux capture-pane -t p7-search -p > /tmp/p7-80x24-search.txt
cat /tmp/p7-80x24-search.txt
```

Expected: search shelf at sidebar width 24; results render with full subject + date in message-list pane.

- [ ] **Step 7.9: Verify message-list date drift is RESOLVED**

Inspect `/tmp/p7-80x24-base.txt` and `/tmp/p7-80x24-search.txt`. Look for threaded child rows (those with `├─` or `└─` prefix). For each:
- Subject must be present (not empty).
- Date column must show full `Day YYYY-MM-DD` or `H:MM AM/PM` — never `Thu 2026-04-` or `3:41` (clipped).

If date drift persists, file a sub-task: column allocator floor adjustment. Likely fix in `internal/ui/msglist_render.go` (or wherever the column budget is computed) — enforce `dateColMin = 14` and let subject column take the variable cut.

- [ ] **Step 7.10: Capture transition boundary 86×24**

```bash
tmux kill-session -t p7-86 2>/dev/null
tmux new-session -d -s p7-86 -x 86 -y 24 'poplar'
sleep 2.5
tmux capture-pane -t p7-86 -p > /tmp/p7-86x24-base.txt
cat /tmp/p7-86x24-base.txt
```

Expected: sidebar = 30 cells. Folder labels like "Membership Committee" render full (no `…`). Message-list pane = 55 cells.

- [ ] **Step 7.11: Capture regression at 120×40**

```bash
tmux kill-session -t p7-120 2>/dev/null
tmux new-session -d -s p7-120 -x 120 -y 40 'poplar'
sleep 2.5
tmux capture-pane -t p7-120 -p > /tmp/p7-120x40-base.txt
cat /tmp/p7-120x40-base.txt
```

Expected: identical to pre-Pass-7 behavior at 120×40 — sidebar = 30, full folder labels.

- [ ] **Step 7.12: Capture floor at 60×24**

```bash
tmux kill-session -t p7-60 2>/dev/null
tmux new-session -d -s p7-60 -x 60 -y 24 'poplar'
sleep 2.5
tmux capture-pane -t p7-60 -p > /tmp/p7-60x24-base.txt
cat /tmp/p7-60x24-base.txt
```

Expected: sidebar = 24 (clamped at floor). Best-effort rendering; no crashes.

- [ ] **Step 7.13: Save the captures**

```bash
mkdir -p docs/poplar/captures/2026-05-01-pass-7
cp /tmp/p7-80x24-*.txt /tmp/p7-86x24-*.txt /tmp/p7-120x40-*.txt /tmp/p7-60x24-*.txt \
   docs/poplar/captures/2026-05-01-pass-7/
```

These accompany the ADRs in step 8.

- [ ] **Step 7.14: Commit captures**

```bash
git add docs/poplar/captures/2026-05-01-pass-7/
git commit -m "$(cat <<'EOF'
Pass 7: tmux capture matrix (80x24, 86x24, 120x40, 60x24)

Verification record for the responsive-sidebar pass. Polish bar at
80x24 is met across base view, viewer + help, account help, confirm
modal, move picker, undo toast, search shelf. Boundary 86x24 returns
to 30-cell sidebar. 120x40 unchanged. 60x24 clamps to 24-cell floor.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Write ADR-0096 and ADR-0097

**Files:**
- Create: `docs/poplar/decisions/0096-responsive-sidebar-width.md`
- Create: `docs/poplar/decisions/0097-eighty-by-twentyfour-polish-bar.md`

- [ ] **Step 8.1: Write ADR-0096**

Create `docs/poplar/decisions/0096-responsive-sidebar-width.md`:

```markdown
---
title: Responsive sidebar width
status: accepted
date: 2026-05-01
---

## Context

Pre-Pass-7, sidebar width was a fixed `const sidebarWidth = 30`. At
the design polish bar of 80×24 (ADR-0097), this left the message-list
pane only 48 cells, below the natural minimum for a threaded row.
Visible drift: threaded child rows lost the date column ("Thu
2026-04-") and same-day timestamps lost AM/PM.

## Decision

Sidebar width is `sidebarWidthFor(termWidth) = clamp(termWidth - 56,
24, 30)`. Linear from 24 at termWidth=80 up to 30 at termWidth=86,
flat at 30 above. The 56-cell offset is the message-list natural
minimum: flag(2) + icon(4) + sender(20) + thread-prefix(4) +
subject(8) + gap(2) + date(14) + sep(1) + right-border(1).

Folder labels in the sidebar truncate with `…` via
`displayTruncateEllipsis` when their natural width exceeds the
per-row label budget. Every rendered folder row preserves a 1-cell
right margin before the chrome divider, regardless of width.

## Consequences

The 80×24 polish bar is met. Long custom folder labels truncate at
narrower terminals (e.g., "Membership Committee" → "Membership C…"
at sidebar=24). Truncation is consistent within a session because
terminal width does not change without a resize. The half-width
fallback `min(sidebarWidthFor(width), width/2)` continues to handle
pathologically narrow widths.
```

- [ ] **Step 8.2: Write ADR-0097**

Create `docs/poplar/decisions/0097-eighty-by-twentyfour-polish-bar.md`:

```markdown
---
title: 80×24 is the design polish bar
status: accepted
date: 2026-05-01
---

## Context

80×24 is the default first-launch terminal size on every VT100-lineage
terminal — macOS Terminal.app, GNOME Terminal (Ubuntu/Linux Mint),
Konsole, Alacritty, Kitty, iTerm2, xterm. Only Windows Terminal
defaults wider (120×30). For poplar, 80×24 is therefore the
default-launch user experience for ~95% of the target audience.

BACKLOG #15 ("Help popover responsive layout for narrow terminals")
imagined progressive reflow strategies for sub-80 widths (single-
column stacking, column dropping). After the Pass 7 audit, sub-80
terminals are deemed an uncommon use case for an email client; the
existing `tooNarrow` fallback string in `HelpPopover.Box` covers them
adequately.

## Decision

80×24 is the design polish bar. Every overlay, panel, and rendering
path must look intentional at 80×24 — date columns intact, threaded
rows complete, folder labels truncated cleanly, no border collisions,
no clipped subjects.

Below 80×24, rendering is best-effort. The help popover's `tooNarrow`
fallback fires when its natural box width exceeds the terminal width
(`Terminal too narrow for help popover`). No further reflow strategy
is implemented.

Help popover natural-width budget: account context ≤62 cells, viewer
context ≤58 cells. Both fit at 80 cols.

Closes BACKLOG #15.

## Consequences

Pass 7's responsive sidebar (ADR-0096) is the load-bearing change for
this bar. Future passes that touch overlays must verify at 80×24 as
part of the pass-end checklist. The pass-end consolidation ritual in
the `poplar-pass` skill already enforces a 120×40 capture; this ADR
adds an 80×24 capture as a peer requirement when UI is touched.

#15 is closed without further code change.
```

- [ ] **Step 8.3: Commit ADRs**

```bash
git add docs/poplar/decisions/0096-responsive-sidebar-width.md \
        docs/poplar/decisions/0097-eighty-by-twentyfour-polish-bar.md
git commit -m "$(cat <<'EOF'
Pass 7: ADR-0096 responsive sidebar; ADR-0097 80x24 polish bar

0096: sidebarWidthFor(termWidth) = clamp(termWidth - 56, 24, 30) with
folder-label ellipsis truncation and 1-cell right margin invariant.

0097: 80x24 is the polish bar for every UI surface. Closes BACKLOG
#15: sub-80 widths handled by HelpPopover.Box's existing tooNarrow
fallback.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Update invariants.md, BACKLOG.md, STATUS.md

**Files:**
- Modify: `docs/poplar/invariants.md`
- Modify: `BACKLOG.md`
- Modify: `docs/poplar/STATUS.md`

- [ ] **Step 9.1: Update invariants.md**

Find the existing sidebar fact (likely under "Components" → "Sidebar" or in the path-scoped UI rules). Update or add:

> Sidebar width is responsive: `sidebarWidthFor(termWidth) = clamp(termWidth - 56, 24, 30)`. Linear narrowing from 30 at termWidth≥86 down to 24 at termWidth=80; clamped to 24 below. Folder labels truncate with `…` when the per-row label budget is exceeded; every rendered folder row preserves a 1-cell right margin before the chrome divider. 80×24 is the design polish bar for every UI surface.

Update the decision-index table at the bottom to add ADR-0096 and ADR-0097 to the relevant rows. Add a row for the polish-bar invariant:

| Invariant theme | ADRs |
|---|---|
| Responsive sidebar; 80×24 polish bar | 0096, 0097 |

Keep `invariants.md` ≤300 lines (enforced by `.claude/hooks/claude-md-size.sh`). Trim or rewrite obsolete sidebar facts to compensate.

- [ ] **Step 9.2: Close BACKLOG #15**

In `BACKLOG.md`, mark the #15 entry resolved:

```markdown
- [x] **#15** ~~Help popover: responsive layout for narrow terminals~~ `#improvement` `#poplar` *(2026-04-25)* (closed 2026-05-01)
  Resolved 2026-05-01 by Pass 7 / ADR-0097. 80×24 is the design polish bar; sub-80 widths handled by `HelpPopover.Box`'s existing `tooNarrow` fallback. The popover's natural width (≤62 account, ≤58 viewer) fits at 80 cols; no progressive reflow needed. Pass 7 also resolved the underlying drift (threaded-row date clipping) by introducing a responsive sidebar (ADR-0096).
```

- [ ] **Step 9.3: Update STATUS.md**

Mark Pass 7 done in the table; replace the "Next starter prompt" with the next pass.

In the pass table:

```markdown
| 7 | Polish I — responsive sidebar + 80×24 polish bar (#15 closed) | done — ADR-0096/0097 |
| 8 | Gmail IMAP (direct-on-emersion rewrite) | next |
```

Replace the starter prompt with the Pass 8 starter (preserve format from the `poplar-pass` skill):

```markdown
## Next starter prompt (Pass 8)

> **Goal.** Add Gmail IMAP backend on `emersion/go-imap` v1, paralleling
> the JMAP backend in `internal/mailjmap/` so Gmail accounts are usable
> in v1.
>
> **Scope.** New package `internal/mailimap/` implementing the
> `mail.Backend` interface against `emersion/go-imap` v1 with vendored
> XOAUTH2 + Gmail X-GM-EXT helpers from `internal/mailauth/`. Folder
> classification reuses the existing `mail.Classify`. Keep changes
> within `internal/mailimap/`, `internal/config/` (account decode),
> and `cmd/poplar/` (account dispatch).
>
> **Settled:** mail.Backend stays synchronous (ADR-0075). Direct-on-
> emersion (no aerc fork). XOAUTH2 + X-GM-EXT vendored helpers
> already in `internal/mailauth/`.
>
> **Still open — brainstorm before coding:**
> - Token refresh ownership: in `mailauth/` or `mailimap/`?
> - Idle/keepalive strategy for the IMAP connection.
> - How `Destroy` (ADR-0092) maps to IMAP UID EXPUNGE.
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-gmail-imap.md`, then implement.
> Standard pass-end checklist applies.
```

Keep `STATUS.md` ≤60 lines.

- [ ] **Step 9.4: Commit**

```bash
git add docs/poplar/invariants.md BACKLOG.md docs/poplar/STATUS.md
git commit -m "$(cat <<'EOF'
Pass 7 consolidation: invariants, BACKLOG #15 close, STATUS

invariants.md: replace fixed-30 sidebar fact with responsive formula
+ 1-cell right margin invariant + 80x24 polish bar; decision index
extended for ADR-0096/0097.

BACKLOG #15 closed with reference to Pass 7 + ADR-0097.

STATUS: Pass 7 marked done; Pass 8 (Gmail IMAP) starter prompt set.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Archive plan + spec

**Files:**
- Move: `docs/superpowers/plans/2026-05-01-pass-7-responsive-sidebar.md` → `docs/superpowers/archive/plans/`
- Move: `docs/superpowers/specs/2026-05-01-pass-7-responsive-sidebar-design.md` → `docs/superpowers/archive/specs/`

- [ ] **Step 10.1: git mv plan and spec**

```bash
git mv docs/superpowers/plans/2026-05-01-pass-7-responsive-sidebar.md \
       docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-01-pass-7-responsive-sidebar-design.md \
       docs/superpowers/archive/specs/
```

- [ ] **Step 10.2: Commit**

```bash
git commit -m "$(cat <<'EOF'
Pass 7: archive plan + spec

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: /simplify pass

**Files:**
- Whatever the simplify skill flags.

- [ ] **Step 11.1: Invoke simplify**

Invoke the `simplify` skill (per workstation CLAUDE.md, run before every commit; here we run it before the final ship commit since the prior commits are tightly scoped pass-internal work).

- [ ] **Step 11.2: Apply genuine wins**

Aggregate the three reviewer agents' findings; apply only the changes you'd defend in code review (reuse, quality, efficiency). Reject churn.

- [ ] **Step 11.3: Commit if anything changed**

```bash
git add <files>
git commit -m "$(cat <<'EOF'
Pass 7: simplify pass — <one-line summary of wins, or "no changes">

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: Final ship

- [ ] **Step 12.1: make check**

```bash
make check
```

Expected: green (vet + tests).

- [ ] **Step 12.2: Push**

```bash
git push
```

- [ ] **Step 12.3: Install**

```bash
make install
```

Pass 7 ships.
