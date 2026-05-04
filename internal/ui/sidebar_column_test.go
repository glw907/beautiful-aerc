// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// sidebarColumnFolders returns a representative classified folder set
// for SidebarColumn render tests: Inbox (unread), Drafts, Sent, Archive,
// Spam (unread), Trash. Matches the mock backend layout used by other tests.
func sidebarColumnFolders() []mail.ClassifiedFolder {
	return mail.Classify([]mail.Folder{
		{Name: "Inbox", Role: "inbox", Unseen: 3, Exists: 10},
		{Name: "Drafts", Role: "drafts", Exists: 2},
		{Name: "Sent", Role: "sent", Exists: 45},
		{Name: "Archive", Role: "archive", Exists: 200},
		{Name: "Spam", Role: "junk", Unseen: 1, Exists: 5},
		{Name: "Trash", Role: "trash", Exists: 7},
	})
}

// newSidebarColumnAt builds a SidebarColumn at a given width with
// representative content.  height is the total column height so the
// search shelf lands correctly.
func newSidebarColumnAt(t *testing.T, width, height int) SidebarColumn {
	t.Helper()
	styles := NewStyles(theme.Nord)
	layout := ComputeLayout(width)
	uiCfg := config.DefaultUIConfig()

	sb := NewSidebar(styles, sidebarColumnFolders(), uiCfg, width, max(1, height-sidebarHeaderRows-searchShelfRows), SimpleIcons)
	sb.SetLayout(layout)
	ss := NewSidebarSearch(styles, width, SimpleIcons)
	ss.SetSize(width)
	return NewSidebarColumn(styles, SimpleIcons, sb, ss, "user@example.com").
		SetSize(width, height)
}

// TestSidebarColumn_SetSizeAndView verifies that SetSize wires width and
// height into the column and that View() returns content at the correct
// height with every row exactly width display cells wide.
func TestSidebarColumn_SetSizeAndView(t *testing.T) {
	cases := []struct {
		name  string
		w, h  int
	}{
		{"spartan-14x20", 14, 20},
		{"intermediate-22x30", 22, 30},
		{"full-30x40", 30, 40},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			col := newSidebarColumnAt(t, tc.w, tc.h)
			got := col.View()
			if got == "" {
				t.Fatal("View() returned empty string")
			}
			lines := strings.Split(got, "\n")
			if len(lines) != tc.h {
				t.Errorf("View() returned %d lines, want %d", len(lines), tc.h)
			}
			for i, line := range lines {
				if w := displayCells(line); w != tc.w {
					t.Errorf("line %d: width %d, want %d (line=%q)", i, w, tc.w, line)
				}
			}
		})
	}
}

// TestSidebarColumn_EmptyBeforeSize verifies that View() is a no-op when
// SetSize has not been called (width or height zero).
func TestSidebarColumn_EmptyBeforeSize(t *testing.T) {
	styles := NewStyles(theme.Nord)
	uiCfg := config.DefaultUIConfig()
	sb := NewSidebar(styles, sidebarColumnFolders(), uiCfg, 22, 20, SimpleIcons)
	ss := NewSidebarSearch(styles, 22, SimpleIcons)
	col := NewSidebarColumn(styles, SimpleIcons, sb, ss, "user@example.com")
	// No SetSize call — width and height are zero.
	if got := col.View(); got != "" {
		t.Errorf("View() with zero size returned %q, want empty", got)
	}
}

// TestSidebarColumn_Accessors verifies that Sidebar/SidebarSearch
// accessors round-trip through With*.
func TestSidebarColumn_Accessors(t *testing.T) {
	col := newSidebarColumnAt(t, 22, 20)

	sb := col.Sidebar()
	sb2 := col.WithSidebar(sb).Sidebar()
	if sb.selected != sb2.selected || sb.width != sb2.width {
		t.Errorf("WithSidebar round-trip changed sidebar: got selected=%d w=%d, want %d %d",
			sb2.selected, sb2.width, sb.selected, sb.width)
	}

	ss := col.SidebarSearch()
	ss2 := col.WithSidebarSearch(ss).SidebarSearch()
	if ss.state != ss2.state {
		t.Errorf("WithSidebarSearch round-trip changed state: got %v, want %v", ss2.state, ss.state)
	}
}

// TestSidebarColumn_AccountEmailRendered verifies that the account email
// appears in the second row of View() (the header row after the leading blank).
func TestSidebarColumn_AccountEmailRendered(t *testing.T) {
	col := newSidebarColumnAt(t, 22, 20)
	lines := strings.Split(stripANSI(col.View()), "\n")
	// Row 0 = blank, row 1 = account line.
	if len(lines) < 2 {
		t.Fatal("not enough lines")
	}
	if !strings.Contains(lines[1], "user@example") {
		t.Errorf("account line = %q, expected to contain account email", lines[1])
	}
}

// TestSidebarColumn_SearchHintAtBottom verifies that the search hint row
// lands in the last searchShelfRows of the output.
func TestSidebarColumn_SearchHintAtBottom(t *testing.T) {
	col := newSidebarColumnAt(t, 22, 20)
	lines := strings.Split(stripANSI(col.View()), "\n")
	hintRow := -1
	for i, line := range lines {
		if strings.Contains(line, "/ to search") {
			hintRow = i
			break
		}
	}
	if hintRow < 0 {
		t.Fatal("'/ to search' hint not found in SidebarColumn output")
	}
	if hintRow < len(lines)-searchShelfRows {
		t.Errorf("hint row %d is above the bottom shelf (total rows=%d, shelf=%d)",
			hintRow, len(lines), searchShelfRows)
	}
}

// TestSidebarColumn_GoldenWidths captures golden output for the three ADR-0109
// sidebar layout tiers and a representative terminal height.
func TestSidebarColumn_GoldenWidths(t *testing.T) {
	cases := []struct {
		name  string
		w, h  int
	}{
		// Three layout tiers from ADR-0109: Spartan (14), Intermediate (22), Full (30).
		{"sidebarcolumn_w14_h24.txt", 14, 24},
		{"sidebarcolumn_w22_h24.txt", 22, 24},
		{"sidebarcolumn_w30_h24.txt", 30, 24},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			col := newSidebarColumnAt(t, tc.w, tc.h)
			got := col.View()
			checkGolden(t, tc.name, got)
		})
	}
}
