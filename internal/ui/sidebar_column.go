// SPDX-License-Identifier: MIT

package ui

import "strings"

// SidebarColumn is the composite left-hand column component: account
// header rows, folder list (Sidebar), spacer, and search shelf
// (SidebarSearch). It owns the column geometry and delegates
// SetSize/SetLayout to its children.
//
// SidebarColumn is immutable-style: all mutating helpers return a new
// value. AccountTab holds one by value and replaces it via the With*
// accessors, preserving the Elm contract.
//
// Layout/SetSize threading choice (verbose-explicit path, per plan):
// SidebarColumn.SetSize stores its own dims for View(). The
// WindowSizeMsg branch in AccountTab still calls Sidebar.SetLayout and
// Sidebar.SetSize explicitly, then re-wraps via WithSidebar +
// WithSidebarSearch + SetSize. Verbose but correct. SidebarColumn never
// needs to know about LayoutMode, keeping its API narrow.
type SidebarColumn struct {
	styles        Styles
	icons         IconSet
	sidebar       Sidebar
	sidebarSearch SidebarSearch
	accountEmail  string
	width, height int
}

// NewSidebarColumn assembles a SidebarColumn from its constituent parts.
func NewSidebarColumn(styles Styles, icons IconSet, sidebar Sidebar, sidebarSearch SidebarSearch, accountEmail string) SidebarColumn {
	return SidebarColumn{
		styles:        styles,
		icons:         icons,
		sidebar:       sidebar,
		sidebarSearch: sidebarSearch,
		accountEmail:  accountEmail,
	}
}

// SetSize records the column's width and height for View(). It does NOT
// propagate SetSize to child components. The AccountTab WindowSizeMsg
// branch handles child sizing explicitly and then calls SetSize to record
// the final dims.
func (c SidebarColumn) SetSize(w, h int) SidebarColumn {
	c.width = w
	c.height = h
	return c
}

func (c SidebarColumn) Sidebar() Sidebar { return c.sidebar }

func (c SidebarColumn) SidebarSearch() SidebarSearch { return c.sidebarSearch }

func (c SidebarColumn) WithSidebar(s Sidebar) SidebarColumn {
	c.sidebar = s
	return c
}

func (c SidebarColumn) WithSidebarSearch(s SidebarSearch) SidebarColumn {
	c.sidebarSearch = s
	return c
}

// View renders the sidebar column: blank / account / blank header rows,
// folder region, spacer, and search shelf, but not the divider. The
// AccountTab owns the row-by-row join of sidebar + divider + right pane.
// Every row is exactly c.width display cells wide (enforced by the child
// renderers and the blank row style).
func (c SidebarColumn) View() string {
	if c.width == 0 || c.height == 0 {
		return ""
	}
	sw := c.width

	acctName := displayTruncateEllipsis(c.accountEmail, sw-1)
	acctLine := c.styles.SidebarAccount.Width(sw).Render(" " + acctName)
	blank := c.styles.SidebarBg.Width(sw).Render("")

	sidebarFolders := c.sidebar.View()
	shelfView := c.sidebarSearch.View()

	var lines []string
	lines = append(lines, blank, acctLine, blank)
	if sidebarFolders != "" {
		lines = append(lines, strings.Split(sidebarFolders, "\n")...)
	}
	// Pad the folder region so the search shelf lands at the bottom.
	targetFolderEnd := c.height - searchShelfRows
	for len(lines) < targetFolderEnd {
		lines = append(lines, blank)
	}
	if len(lines) > targetFolderEnd {
		lines = lines[:targetFolderEnd]
	}
	lines = append(lines, strings.Split(shelfView, "\n")...)
	if len(lines) > c.height {
		lines = lines[:c.height]
	}
	return strings.Join(lines, "\n")
}
