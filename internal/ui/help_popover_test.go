// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
)

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

// findAccountRow walks accountGroups looking for a row by group title
// and key.
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

func TestHelpPopover_WiredStyling(t *testing.T) {
	styles := NewStyles(theme.Nord)

	// Wired rows use HelpKey (Bold) for the key column; unwired rows
	// use Dim (not Bold) for the entire row. Test via style properties
	// rather than rendered ANSI (lipgloss suppresses ANSI without a TTY).
	if !styles.HelpKey.GetBold() {
		t.Error("HelpKey style must be bold (used for wired key column)")
	}
	if styles.Dim.GetBold() {
		t.Error("Dim style must not be bold (used for unwired rows)")
	}

	// Sanity-check that the data tables use wired=true / wired=false
	// as expected for the rows the render path branches on.
	wiredRow, ok := findAccountRow("Navigate", "j/k")
	if !ok {
		t.Fatal("Navigate j/k row not found in accountGroups")
	}
	if !wiredRow.wired {
		t.Error("Navigate j/k: expected wired=true")
	}

	unwiredRow, ok := findAccountRow("Triage", "d")
	if !ok {
		t.Fatal("Triage d row not found in accountGroups")
	}
	if unwiredRow.wired {
		t.Error("Triage d: expected wired=false")
	}

	// Confirm render path routes correctly: wired row content is
	// present in the account popover, unwired row content is present too.
	view := stripANSI(NewHelpPopover(styles, HelpAccount).View(120, 30))
	for _, want := range []string{"j/k", "up/down", "d", "delete"} {
		if !strings.Contains(view, want) {
			t.Errorf("account popover missing %q", want)
		}
	}
}

func TestHelpPopover_GroupHeadersBoldEvenWhenAllUnwired(t *testing.T) {
	styles := NewStyles(theme.Nord)

	// HelpGroupHeader must be bold — it is used for every group heading
	// including "Reply" which has no wired rows today.
	if !styles.HelpGroupHeader.GetBold() {
		t.Error("HelpGroupHeader style must be bold")
	}

	// Confirm "Reply" heading appears in the rendered account popover.
	view := stripANSI(NewHelpPopover(styles, HelpAccount).View(120, 30))
	if !strings.Contains(view, "Reply") {
		t.Error("account popover: Reply group heading not found")
	}
}

// TestHelpPopover_VerticallyCentered locks in the F3 acceptance: the
// popover's blank-row margins above and below the box are equal (±1).
// Prior regression rendered the box pinned ~1 row from the top.
func TestHelpPopover_VerticallyCentered(t *testing.T) {
	styles := NewStyles(theme.Nord)
	for _, dims := range []struct{ w, h int }{{120, 30}, {120, 40}, {160, 50}} {
		view := stripANSI(NewHelpPopover(styles, HelpAccount).View(dims.w, dims.h))
		lines := strings.Split(view, "\n")
		var first, last int = -1, -1
		for i, ln := range lines {
			if strings.TrimSpace(ln) != "" {
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		if first < 0 {
			t.Fatalf("%dx%d: empty view", dims.w, dims.h)
		}
		topBlank := first
		botBlank := len(lines) - 1 - last
		if diff := topBlank - botBlank; diff < -1 || diff > 1 {
			t.Errorf("%dx%d: top blank %d vs bottom blank %d (diff %d, want |diff|<=1)",
				dims.w, dims.h, topBlank, botBlank, diff)
		}
	}
}
