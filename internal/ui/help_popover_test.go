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
