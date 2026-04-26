# Help Popover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the help popover — a modal overlay triggered by `?`
that shows the keybinding map for the current context (account or
viewer). First modal in the codebase. Implements the future-binding
policy decided in the spec (option C1: show all eventual keys, dim
unwired rows, no glyph).

**Architecture:** New `HelpPopover` model in `internal/ui/help_popover.go`,
owned by `App`. Static binding tables (`accountGroups`, `viewerGroups`)
with a `wired bool` per row. `App.Update` gains an early modal branch
that swallows all keys except `?`/`Esc`. `App.View` overlays the
popover via `lipgloss.Place` when `helpOpen` is true (no background dim
in v1). Three new style fields on `Styles` (`HelpTitle`, `HelpGroupHeader`,
`HelpKey`); existing `Dim` and `FrameBorder` cover the rest.

**Tech Stack:** Go 1.26, bubbletea, lipgloss. Conventions from the
`go-conventions` and `elm-conventions` skills (mandatory for any code
in `internal/ui/`).

**Spec:** `docs/superpowers/specs/2026-04-25-help-popover-design.md`

**Existing references in codebase:**
- `internal/ui/app.go:102` — current `?` stub (replace in Task 7)
- `internal/ui/styles.go` — `Styles` struct + `NewStyles` factory
- `internal/ui/styles_test.go` — pattern for `TestNewStyles` table
- `internal/ui/top_line.go` — pattern for custom border edge with
  embedded text (we'll mirror this for the popover title)
- `internal/ui/app_test.go` — `newLoadedApp`, `drainApp`, `stripANSI`
  helpers used in integration tests
- `docs/poplar/styling.md` — palette → surface map (must be updated
  before adding new styles)
- `docs/poplar/wireframes.md` §5 — visual reference

---

## Task 1: Update styling.md with help popover surfaces

**Files:**
- Modify: `docs/poplar/styling.md` (add new subsection under Surface Map)

Per invariants, the styling doc is updated **before** any color or
style change. Help popover introduces three new surfaces; this task
records them.

- [ ] **Step 1: Add a "Help popover" section to styling.md**

Insert this block in `docs/poplar/styling.md` immediately before
the "### Tab bar (unused in current chrome, reserved)" subsection
(so the order roughly tracks visual prominence):

```markdown
### Help popover (modal overlay, `?`)

| Field | fg | bg | Role |
|-------|----|----|------|
| `HelpTitle` | `AccentPrimary` (bold) | — | Popover title embedded in top border ("Message List" / "Message Viewer") |
| `HelpGroupHeader` | `FgBright` (bold) | — | Group headings ("Navigate", "Triage", etc.) |
| `HelpKey` | `FgBright` (bold) | — | Key column for *wired* rows |
| `Dim` (reuse) | `FgDim` | — | Description column (all rows) and entire key+desc for *unwired* (future) rows |
| `FrameBorder` (reuse) | `BgBorder` | — | Rounded box border |

**Wired vs. unwired:** rows whose binding is not yet implemented
render the entire row in `Dim` (no bold). Group headings stay
`HelpGroupHeader` (bright) regardless. The contrast between the
bright-bold key column on wired rows and the flat-dim key column
on unwired rows is the only visual signal — no glyph, no legend.
See ADR for help popover future-binding policy.
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/styling.md
git commit -m "Help popover: document surfaces in styling.md"
```

---

## Task 2: Add help popover styles

**Files:**
- Modify: `internal/ui/styles.go`
- Modify: `internal/ui/styles_test.go`

- [ ] **Step 1: Add the failing test**

In `internal/ui/styles_test.go`, extend the `TestNewStyles` table
(the slice of `{name, style}` pairs near line 13) with three new
entries before the closing `}`:

```go
{"HelpTitle", s.HelpTitle},
{"HelpGroupHeader", s.HelpGroupHeader},
{"HelpKey", s.HelpKey},
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestNewStyles -v`
Expected: compile error — `s.HelpTitle undefined` (and the other two).

- [ ] **Step 3: Add the three fields to the Styles struct**

In `internal/ui/styles.go`, locate the `Styles` struct. Just above
the `// Placeholder text` block (near `Dim lipgloss.Style`), add a
new section:

```go
	// Help popover (modal overlay, `?`)
	HelpTitle       lipgloss.Style
	HelpGroupHeader lipgloss.Style
	HelpKey         lipgloss.Style
```

- [ ] **Step 4: Populate the fields in NewStyles**

In `internal/ui/styles.go`, locate the `NewStyles` factory. Just
above the `Dim: ...` block, add:

```go
		HelpTitle: lipgloss.NewStyle().
			Foreground(t.AccentPrimary).Bold(true),
		HelpGroupHeader: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		HelpKey: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/ui/ -run TestNewStyles -v`
Expected: PASS for all three new entries.

- [ ] **Step 6: Run the full ui test suite**

Run: `go test ./internal/ui/`
Expected: PASS (no regression).

- [ ] **Step 7: Commit**

```bash
git add internal/ui/styles.go internal/ui/styles_test.go
git commit -m "Help popover: add HelpTitle, HelpGroupHeader, HelpKey styles"
```

---

## Task 3: Create help_popover.go skeleton (types + binding tables)

**Files:**
- Create: `internal/ui/help_popover.go`
- Create: `internal/ui/help_popover_test.go`

This task lays down the data structures and binding tables only —
no rendering yet.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/help_popover_test.go`:

```go
package ui

import "testing"

func TestHelpPopover_AccountGroupsCoverage(t *testing.T) {
	wantGroups := []string{
		"Navigate", "Triage", "Reply",
		"Search", "Select", "Threads",
		"Go To",
	}
	if len(accountGroups) != len(wantGroups) {
		t.Fatalf("accountGroups: got %d groups, want %d",
			len(accountGroups), len(wantGroups))
	}
	for i, want := range wantGroups {
		if accountGroups[i].title != want {
			t.Errorf("accountGroups[%d].title = %q, want %q",
				i, accountGroups[i].title, want)
		}
	}
}

func TestHelpPopover_ViewerGroupsCoverage(t *testing.T) {
	wantGroups := []string{"Navigate", "Triage", "Reply"}
	if len(viewerGroups) != len(wantGroups) {
		t.Fatalf("viewerGroups: got %d groups, want %d",
			len(viewerGroups), len(wantGroups))
	}
	for i, want := range wantGroups {
		if viewerGroups[i].title != want {
			t.Errorf("viewerGroups[%d].title = %q, want %q",
				i, viewerGroups[i].title, want)
		}
	}
}

func TestHelpPopover_WiredFlagsAccount(t *testing.T) {
	// Spot-check a handful of rows. Folder jumps and Threads
	// are wired today; Triage and Reply are not.
	cases := []struct {
		group string
		key   string
		want  bool
	}{
		{"Navigate", "j/k", true},
		{"Triage", "d", false},
		{"Reply", "c", false},
		{"Search", "/", true},
		{"Threads", "F", true},
		{"Go To", "I", true},
		{"Go To", "T", true},
	}
	for _, tc := range cases {
		row, ok := findAccountRow(tc.group, tc.key)
		if !ok {
			t.Errorf("group %q key %q: row not found", tc.group, tc.key)
			continue
		}
		if row.wired != tc.want {
			t.Errorf("group %q key %q: wired = %v, want %v",
				tc.group, tc.key, row.wired, tc.want)
		}
	}
}

// findAccountRow is a test helper that walks accountGroups looking
// for a row by group title and key. Returns the row and true if
// found.
func findAccountRow(group, key string) (bindingRow, bool) {
	for _, g := range accountGroups {
		if g.title != group {
			continue
		}
		for _, r := range g.rows {
			if r.key == key {
				return r, true
			}
		}
	}
	return bindingRow{}, false
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run TestHelpPopover -v`
Expected: compile errors — `accountGroups`, `viewerGroups`,
`bindingRow` undefined.

- [ ] **Step 3: Create help_popover.go with types and data**

Create `internal/ui/help_popover.go`:

```go
package ui

// HelpContext selects which binding layout the popover renders.
type HelpContext int

const (
	HelpAccount HelpContext = iota
	HelpViewer
)

// HelpPopover is the modal help overlay. App owns key routing;
// this model only renders.
type HelpPopover struct {
	styles  Styles
	context HelpContext
}

// NewHelpPopover constructs a popover for the given context.
func NewHelpPopover(styles Styles, context HelpContext) HelpPopover {
	return HelpPopover{styles: styles, context: context}
}

// bindingRow is a single key/description entry in the popover.
// wired is false for keys whose action is not yet implemented;
// such rows render dim per the future-binding policy.
type bindingRow struct {
	key   string
	desc  string
	wired bool
}

// bindingGroup is a labeled cluster of bindingRow entries
// (e.g., "Navigate", "Triage").
type bindingGroup struct {
	title string
	rows  []bindingRow
}

// accountGroups is the binding map shown when the popover opens
// from the account view. Order is the visual layout order.
var accountGroups = []bindingGroup{
	{
		title: "Navigate",
		rows: []bindingRow{
			{"j/k", "up/down", true},
			{"g/G", "top/bot", true},
		},
	},
	{
		title: "Triage",
		rows: []bindingRow{
			{"d", "delete", false},
			{"a", "archive", false},
			{"s", "star", false},
			{".", "read/unrd", false},
		},
	},
	{
		title: "Reply",
		rows: []bindingRow{
			{"r", "reply", false},
			{"R", "all", false},
			{"f", "forward", false},
			{"c", "compose", false},
		},
	},
	{
		title: "Search",
		rows: []bindingRow{
			{"/", "search", true},
			{"n", "next", false},
			{"N", "prev", false},
		},
	},
	{
		title: "Select",
		rows: []bindingRow{
			{"v", "select", false},
			{"␣", "toggle", false},
		},
	},
	{
		title: "Threads",
		rows: []bindingRow{
			{"␣", "fold", true},
			{"F", "fold all", true},
		},
	},
	{
		title: "Go To",
		rows: []bindingRow{
			{"I", "inbox", true},
			{"D", "drafts", true},
			{"S", "sent", true},
			{"A", "archive", true},
			{"X", "spam", true},
			{"T", "trash", true},
		},
	},
}

// accountBottomHints is the trailing line under the groups in the
// account context: "Enter open    ?  close".
var accountBottomHints = []bindingRow{
	{"Enter", "open", true},
	{"?", "close", true},
}

// viewerGroups is the binding map shown when the popover opens
// from the message viewer.
var viewerGroups = []bindingGroup{
	{
		title: "Navigate",
		rows: []bindingRow{
			{"j/k", "scroll", true},
			{"g/G", "top/bot", true},
			{"␣/b", "page d/u", true},
			{"1-9", "open link", true},
		},
	},
	{
		title: "Triage",
		rows: []bindingRow{
			{"d", "delete", false},
			{"a", "archive", false},
			{"s", "star", false},
		},
	},
	{
		title: "Reply",
		rows: []bindingRow{
			{"r", "reply", false},
			{"R", "all", false},
			{"f", "forward", false},
			{"c", "compose", false},
		},
	},
}

// viewerBottomHints is the trailing line in the viewer context:
// "Tab link picker    q  close    ?  close".
var viewerBottomHints = []bindingRow{
	{"Tab", "link picker", false},
	{"q", "close", true},
	{"?", "close", true},
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui/ -run TestHelpPopover -v`
Expected: PASS for all three tests.

- [ ] **Step 5: Run the full ui test suite**

Run: `go test ./internal/ui/`
Expected: PASS (no regression).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/help_popover.go internal/ui/help_popover_test.go
git commit -m "Help popover: add HelpPopover skeleton + binding tables"
```

---

## Task 4: Implement HelpPopover.View for account context

**Files:**
- Modify: `internal/ui/help_popover.go`
- Modify: `internal/ui/help_popover_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/help_popover_test.go`:

```go
import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
)

// (Adjust the existing import block at the top of the file to
// include "strings", "testing", and the theme package.)

func TestHelpPopover_AccountViewContent(t *testing.T) {
	styles := NewStyles(theme.Nord)
	h := NewHelpPopover(styles, HelpAccount)

	view := stripANSI(h.View(80, 24))

	// Title in the top border.
	if !strings.Contains(view, "Message List") {
		t.Error("account popover: missing title 'Message List'")
	}

	// Every group heading appears.
	for _, want := range []string{
		"Navigate", "Triage", "Reply",
		"Search", "Select", "Threads", "Go To",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("account popover: missing group heading %q", want)
		}
	}

	// Spot-check binding rows from each group.
	for _, want := range []string{
		"j/k", "up/down",
		"d", "delete",
		"r", "reply",
		"/", "search",
		"v", "select",
		"F", "fold all",
		"I", "inbox", "T", "trash",
		"Enter", "open", "?", "close",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("account popover: missing %q", want)
		}
	}

	// Rounded border corners present.
	for _, want := range []string{"╭", "╮", "╰", "╯"} {
		if !strings.Contains(view, want) {
			t.Errorf("account popover: missing border char %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestHelpPopover_AccountViewContent -v`
Expected: FAIL — `h.View` returns empty string (or compile
error if `View` not yet defined).

- [ ] **Step 3: Implement HelpPopover.View**

Append to `internal/ui/help_popover.go`. Add the missing imports
at the top:

```go
import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)
```

Then add the rendering methods at the bottom of the file:

```go
// View renders the popover centered on a width × height area.
// The caller (App) is expected to pass its full screen dimensions;
// the popover sizes its box from content and lipgloss.Place
// handles centering.
func (h HelpPopover) View(width, height int) string {
	var (
		title         string
		groups        []bindingGroup
		bottomHints   []bindingRow
		layoutBuilder func(styles Styles, groups []bindingGroup) string
	)

	switch h.context {
	case HelpViewer:
		title = "Message Viewer"
		groups = viewerGroups
		bottomHints = viewerBottomHints
		layoutBuilder = renderViewerLayout
	default:
		title = "Message List"
		groups = accountGroups
		bottomHints = accountBottomHints
		layoutBuilder = renderAccountLayout
	}

	body := layoutBuilder(h.styles, groups)
	hintLine := renderHintLine(h.styles, bottomHints)
	inner := body + "\n\n" + hintLine

	// Wrap inner in a rounded box, with top border drawn manually
	// so the title can be embedded.
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(h.styles.FrameBorder.GetForeground()).
		Padding(1, 2).
		Render(inner)

	boxWidth := lipgloss.Width(box)
	topEdge := h.renderTopEdge(title, boxWidth)
	popover := topEdge + "\n" + box

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		popover,
	)
}

// renderTopEdge builds "╭─ <title> ───╮" at the box's natural width.
func (h HelpPopover) renderTopEdge(title string, boxWidth int) string {
	titleSeg := h.styles.HelpTitle.Render(title)
	border := h.styles.FrameBorder
	prefix := border.Render("╭─ ") + titleSeg + border.Render(" ")
	visible := lipgloss.Width(prefix) + 1 // +1 for the closing ╮
	pad := boxWidth - visible
	if pad < 0 {
		pad = 0
	}
	return prefix + border.Render(strings.Repeat("─", pad)+"╮")
}

// renderAccountLayout builds the four-section layout for the
// account context: three rows (Nav/Triage/Reply, then
// Search/Select/Threads, then Go To grid). Bottom hint line is
// added by View.
func renderAccountLayout(styles Styles, groups []bindingGroup) string {
	row1 := lipgloss.JoinHorizontal(lipgloss.Top,
		renderGroup(styles, groups[0]),
		renderGap(),
		renderGroup(styles, groups[1]),
		renderGap(),
		renderGroup(styles, groups[2]),
	)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top,
		renderGroup(styles, groups[3]),
		renderGap(),
		renderGroup(styles, groups[4]),
		renderGap(),
		renderGroup(styles, groups[5]),
	)
	gotoBlock := renderGotoGrid(styles, groups[6])
	return lipgloss.JoinVertical(lipgloss.Left,
		row1, "", row2, "", gotoBlock)
}

// renderViewerLayout builds the single-row layout for the viewer
// context: Nav/Triage/Reply side-by-side.
func renderViewerLayout(styles Styles, groups []bindingGroup) string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		renderGroup(styles, groups[0]),
		renderGap(),
		renderGroup(styles, groups[1]),
		renderGap(),
		renderGroup(styles, groups[2]),
	)
}

// renderGroup builds a single labeled column: heading on top,
// then key/desc rows.
func renderGroup(styles Styles, g bindingGroup) string {
	lines := []string{styles.HelpGroupHeader.Render(g.title)}
	for _, r := range g.rows {
		lines = append(lines, renderRow(styles, r))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderRow builds "<key>  <desc>" with the wired-vs-unwired
// styling. Wired: bright-bold key + dim desc. Unwired: entire
// row dim (no bold).
func renderRow(styles Styles, r bindingRow) string {
	const keyWidth = 5 // padded right-alignment column for keys
	keyPadded := r.key
	for lipgloss.Width(keyPadded) < keyWidth {
		keyPadded += " "
	}
	if r.wired {
		return styles.HelpKey.Render(keyPadded) + "  " +
			styles.Dim.Render(r.desc)
	}
	// Unwired: render key + desc together in Dim, no bold.
	return styles.Dim.Render(keyPadded+"  "+r.desc)
}

// renderGap returns the inter-column spacer used between groups
// on a layout row.
func renderGap() string {
	return "    "
}

// renderGotoGrid builds the Go To group as a 3×2 grid:
// "I inbox    D drafts    S sent" / "A archive  X spam  T trash".
// The group's heading is rendered above.
func renderGotoGrid(styles Styles, g bindingGroup) string {
	heading := styles.HelpGroupHeader.Render(g.title)
	if len(g.rows) != 6 {
		// Defensive: fall back to a flat column if the data shape
		// drifts. Tests cover the 6-row case.
		return renderGroup(styles, g)
	}
	gap := renderGap()
	row1 := renderRow(styles, g.rows[0]) + gap +
		renderRow(styles, g.rows[1]) + gap +
		renderRow(styles, g.rows[2])
	row2 := renderRow(styles, g.rows[3]) + gap +
		renderRow(styles, g.rows[4]) + gap +
		renderRow(styles, g.rows[5])
	return lipgloss.JoinVertical(lipgloss.Left, heading, row1, row2)
}

// renderHintLine builds the bottom hint line: "Enter  open    ?  close".
// Each hint uses the same wired-vs-unwired styling as a row.
func renderHintLine(styles Styles, hints []bindingRow) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		var part string
		if h.wired {
			part = styles.HelpKey.Render(h.key) + "  " +
				styles.Dim.Render(h.desc)
		} else {
			part = styles.Dim.Render(h.key + "  " + h.desc)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "    ")
}
```

A note on `defensive fallback in renderGotoGrid`: this is the
single deliberate exception to the "no defensive code" rule —
the visual grid layout is data-shape-dependent, and a one-line
fallback to flat-column rendering is cheaper than a panic if the
binding tables are edited carelessly. Tests cover the live path.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/ -run TestHelpPopover_AccountViewContent -v`
Expected: PASS.

- [ ] **Step 5: Run the full ui test suite**

Run: `go test ./internal/ui/`
Expected: PASS (no regression).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/help_popover.go internal/ui/help_popover_test.go
git commit -m "Help popover: render account context layout"
```

---

## Task 5: Verify viewer context renders correctly

**Files:**
- Modify: `internal/ui/help_popover_test.go`

The viewer rendering path (`renderViewerLayout`) was already
written in Task 4. This task adds the test coverage.

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/help_popover_test.go`:

```go
func TestHelpPopover_ViewerViewContent(t *testing.T) {
	styles := NewStyles(theme.Nord)
	h := NewHelpPopover(styles, HelpViewer)

	view := stripANSI(h.View(80, 24))

	// Title.
	if !strings.Contains(view, "Message Viewer") {
		t.Error("viewer popover: missing title 'Message Viewer'")
	}

	// Viewer-only rows.
	for _, want := range []string{
		"j/k", "scroll",
		"␣/b", "page d/u",
		"1-9", "open link",
		"Tab", "link picker",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("viewer popover: missing %q", want)
		}
	}

	// Account-only groups must NOT appear.
	for _, missing := range []string{"Search", "Select", "Threads", "Go To"} {
		if strings.Contains(view, missing) {
			t.Errorf("viewer popover: should not contain %q", missing)
		}
	}

	// Border corners.
	for _, want := range []string{"╭", "╮", "╰", "╯"} {
		if !strings.Contains(view, want) {
			t.Errorf("viewer popover: missing border char %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/ui/ -run TestHelpPopover_ViewerViewContent -v`
Expected: PASS (the implementation already covers viewer
context — this test just locks it in).

- [ ] **Step 3: Run the full ui test suite**

Run: `go test ./internal/ui/`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/help_popover_test.go
git commit -m "Help popover: cover viewer context render"
```

---

## Task 6: Verify wired vs. unwired styling at the ANSI level

**Files:**
- Modify: `internal/ui/help_popover_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/help_popover_test.go`:

```go
func TestHelpPopover_WiredStyling(t *testing.T) {
	styles := NewStyles(theme.Nord)
	h := NewHelpPopover(styles, HelpAccount)

	rendered := h.View(120, 30)
	plain := stripANSI(rendered)

	// Locate the line containing "j/k" (a wired row in Navigate)
	// and the line containing "delete" (an unwired row in Triage).
	wiredLine, unwiredLine := "", ""
	for _, line := range strings.Split(rendered, "\n") {
		bare := stripANSI(line)
		if wiredLine == "" && strings.Contains(bare, "j/k") &&
			strings.Contains(bare, "up/down") {
			wiredLine = line
		}
		if unwiredLine == "" && strings.Contains(bare, "delete") {
			unwiredLine = line
		}
	}
	if wiredLine == "" {
		t.Fatalf("did not find wired row in rendered popover:\n%s", plain)
	}
	if unwiredLine == "" {
		t.Fatalf("did not find unwired row in rendered popover:\n%s", plain)
	}

	// Bold escape sequence ("\x1b[" ... "1" ... "m") must appear in
	// the wired line (the key column is bold) and must NOT appear in
	// the unwired line.
	if !strings.Contains(wiredLine, "\x1b[1m") &&
		!strings.Contains(wiredLine, ";1m") {
		t.Errorf("wired line missing bold ANSI: %q", wiredLine)
	}
	if strings.Contains(unwiredLine, "\x1b[1m") ||
		strings.Contains(unwiredLine, ";1m") {
		t.Errorf("unwired line should not contain bold ANSI: %q", unwiredLine)
	}
}

func TestHelpPopover_GroupHeadersBoldEvenWhenAllUnwired(t *testing.T) {
	styles := NewStyles(theme.Nord)
	h := NewHelpPopover(styles, HelpAccount)

	rendered := h.View(120, 30)

	// Reply has no wired rows today; its heading must still be bold.
	for _, line := range strings.Split(rendered, "\n") {
		if !strings.Contains(stripANSI(line), "Reply") {
			continue
		}
		if !strings.Contains(line, "\x1b[1m") &&
			!strings.Contains(line, ";1m") {
			t.Errorf("Reply heading line missing bold ANSI: %q", line)
		}
		return
	}
	t.Fatal("did not find Reply heading line in popover")
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/ui/ -run "TestHelpPopover_(WiredStyling|GroupHeadersBoldEvenWhenAllUnwired)" -v`
Expected: PASS for both. The implementation from Task 4 already
applies the styling correctly; these tests assert it stays that
way.

If FAIL: the most likely cause is `lipgloss` rendering bold via
a different escape sequence on the test environment. Inspect
`wiredLine` to see the actual escape and adjust the assertion to
match (e.g., `\x1b[1;38;5;...m`). The `;1m` clause already
catches the combined-attribute form.

- [ ] **Step 3: Run the full ui test suite**

Run: `go test ./internal/ui/`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/help_popover_test.go
git commit -m "Help popover: assert wired vs unwired ANSI styling"
```

---

## Task 7: Wire help popover into App

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/app_test.go`:

```go
func TestApp_HelpOpenAndCloseWithQuestionMark(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	if app.helpOpen {
		t.Fatal("setup: helpOpen should be false initially")
	}

	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpOpen {
		t.Fatal("after ?: helpOpen should be true")
	}

	view := stripANSI(app.View())
	if !strings.Contains(view, "Message List") {
		t.Errorf("popover view missing 'Message List' title:\n%s", view)
	}

	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if app.helpOpen {
		t.Error("after second ?: helpOpen should be false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestApp_HelpOpenAndCloseWithQuestionMark -v`
Expected: compile error — `app.helpOpen` undefined.

- [ ] **Step 3: Add fields to App and wire `?`**

In `internal/ui/app.go`, modify the `App` struct (near line 14)
to add the help fields:

```go
type App struct {
	acct       AccountTab
	styles     Styles
	topLine    TopLine
	statusBar  StatusBar
	footer     Footer
	keys       GlobalKeys
	viewerOpen bool
	helpOpen   bool
	help       HelpPopover
	width      int
	height     int
}
```

In the `Update` method's `tea.KeyMsg` case (currently starting
near line 81), restructure the handler so the modal branch comes
first:

```go
case tea.KeyMsg:
	if m.helpOpen {
		switch msg.String() {
		case "?", "esc":
			m.helpOpen = false
		}
		return m, nil
	}
	switch msg.String() {
	case "q":
		if m.viewerOpen {
			var cmd tea.Cmd
			m.acct, cmd = m.acct.Update(msg)
			return m, cmd
		}
		if m.acct.sidebarSearch.State() != SearchIdle {
			var cmd tea.Cmd
			m.acct, cmd = m.acct.Update(tea.KeyMsg{Type: tea.KeyEsc})
			return m, cmd
		}
		return m, tea.Quit
	case "ctrl+c":
		return m, tea.Quit
	case "?":
		m.helpOpen = true
		ctx := HelpAccount
		if m.viewerOpen {
			ctx = HelpViewer
		}
		m.help = NewHelpPopover(m.styles, ctx)
		return m, nil
	}
```

In the `View` method (currently starting near line 115), add an
early return when help is open. Insert this block at the very
top of `View`, immediately after the `if m.width == 0 || m.height == 0` guard:

```go
if m.helpOpen {
	return m.help.View(m.width, m.height)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/ -run TestApp_HelpOpenAndCloseWithQuestionMark -v`
Expected: PASS.

- [ ] **Step 5: Run the full ui test suite**

Run: `go test ./internal/ui/`
Expected: PASS (no regression).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Help popover: wire ? toggle and modal view in App"
```

---

## Task 8: App integration tests for modal behavior

**Files:**
- Modify: `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/app_test.go`:

```go
func TestApp_HelpDismissedByEsc(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if app.helpOpen {
		t.Error("Esc should close help")
	}
}

func TestApp_HelpStealsKeys(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	startMsgSelected := app.acct.msglist.Selected()
	startFolderSelected := app.acct.sidebar.Selected()

	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}

	// Send a battery of keys that would normally do something.
	stealKeys := []rune{'j', 'J', 'd', 'r', '/'}
	for _, k := range stealKeys {
		app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
	}

	if app.acct.msglist.Selected() != startMsgSelected {
		t.Errorf("msglist selection moved while help open: %d → %d",
			startMsgSelected, app.acct.msglist.Selected())
	}
	if app.acct.sidebar.Selected() != startFolderSelected {
		t.Errorf("sidebar selection moved while help open: %d → %d",
			startFolderSelected, app.acct.sidebar.Selected())
	}
	if app.acct.sidebarSearch.State() != SearchIdle {
		t.Errorf("search state changed while help open: got %v",
			app.acct.sidebarSearch.State())
	}
	if !app.helpOpen {
		t.Error("help closed unexpectedly during key barrage")
	}
}

func TestApp_HelpQuitSwallowed(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		msg := cmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Error("q during help returned tea.Quit; should be swallowed")
		}
	}
	if !app.helpOpen {
		t.Error("q during help closed the popover; should be swallowed")
	}
}

func TestApp_HelpContextSwitchesWithViewer(t *testing.T) {
	app := newLoadedApp(t, 120, 30)

	// Open help in account context — title is "Message List".
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	view := stripANSI(app.View())
	if !strings.Contains(view, "Message List") {
		t.Errorf("account-context help should title 'Message List':\n%s", view)
	}
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}) // close

	// Open the viewer.
	app, _ = app.Update(ViewerOpenedMsg{})
	if !app.viewerOpen {
		t.Fatal("setup: viewer did not open")
	}

	// Open help — now the title should be "Message Viewer".
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	view = stripANSI(app.View())
	if !strings.Contains(view, "Message Viewer") {
		t.Errorf("viewer-context help should title 'Message Viewer':\n%s", view)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/ui/ -run "TestApp_Help" -v`
Expected: PASS for all four tests. The wiring from Task 7
already implements the behaviors; these tests lock them in.

The accessors used (`MessageList.Selected()`, `Sidebar.Selected()`)
are already defined in `internal/ui/msglist.go:584` and
`internal/ui/sidebar.go:62`. If a future refactor renames them,
update this test to match — do not add a new accessor.

- [ ] **Step 3: Run the full ui test suite**

Run: `go test ./internal/ui/`
Expected: PASS (no regression).

- [ ] **Step 4: Commit**

```bash
git add internal/ui/app_test.go
git commit -m "Help popover: integration tests for modal behavior"
```

---

## Task 9: Live verification via tmux + final check

**Files:** none modified — verification step.

- [ ] **Step 1: Run make check**

Run: `make check`
Expected: PASS (vet + test, the commit gate).

- [ ] **Step 2: Install the binary**

Run: `make install`
Expected: `~/.local/bin/poplar` updated.

- [ ] **Step 3: Live tmux verification**

Per `.claude/docs/tmux-testing.md`, launch poplar in a tmux pane,
capture renders, and verify:

- `?` from the account view opens a popover titled "Message List"
  centered on screen with rounded borders.
- Wired rows (Navigate `j/k`, Threads `F`, Go To `I/D/S/A/X/T`)
  show bright bold keys.
- Unwired rows (Triage `d/a/s/.`, Reply `r/R/f/c`, Search `n/N`,
  Select `v`, viewer `Tab link picker`) render uniformly dim, no
  bold.
- Group headings ("Reply" especially) stay bright even when every
  row in the group is unwired.
- Pressing `?` again closes the popover. `Esc` also closes it.
- Pressing `j`, `J`, `d`, `r`, `/` while the popover is open does
  nothing visible — the underlying state is unchanged when the
  popover is dismissed.
- Open a message with `Enter`. Press `?`. Popover title is now
  "Message Viewer" and the layout is one row of three groups
  (Navigate, Triage, Reply) plus the bottom hint line.

If anything misrenders, fix and re-run `make check`. Commit any
fix as `Help popover: <fix description>`.

- [ ] **Step 4: Verify no uncommitted changes**

Run: `git status`
Expected: `working tree clean` (all earlier commits already
landed; this step has no commit if no fixes were needed).

---

## Pass-end consolidation

Once all nine tasks are complete and `make check` is green, the
pass is **ready to ship** but not yet **shipped**. The pass-end
ritual is invoked separately by the `poplar-pass` skill (trigger
phrases: "ship pass", "finish pass"). It will:

1. Run `/simplify` over the new code.
2. Write new ADRs:
   - **Help popover modal infrastructure** — App owns key
     routing, popover replaces view when open, no background dim
     in v1.
   - **Future-binding policy** — show all eventual keys, dim row
     for unwired, no glyph (option C1).
3. Update `docs/poplar/invariants.md` with the new binding facts.
4. Update `docs/poplar/STATUS.md` (mark 2.5b-5 done, replace
   starter prompt with 2.5b-6).
5. Archive this plan to `docs/superpowers/archive/plans/` and the
   spec to `docs/superpowers/archive/specs/`.
6. `make check`, commit, push, `make install`.
7. Add BACKLOG entries:
   - Background dim for popover overlay.
   - Responsive popover layout for narrow terminals.

The plan implementer does **not** run the consolidation ritual —
it's a separate workflow trigger.
