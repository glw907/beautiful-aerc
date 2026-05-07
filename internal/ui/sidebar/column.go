package sidebar

import (
	"strings"

	"github.com/glw907/poplar/internal/ui/uicore"
)

// HeaderRows is the blank/account/blank padding reserved at the top of the
// sidebar before the folder list. AccountTab.View and sidebar sizing both
// depend on this number matching.
const HeaderRows = 3

// ShelfRows is the height of the Search shelf pinned to the bottom of Column.
const ShelfRows = 3

// Column is the composite left-hand column component: account header rows,
// folder list (Model), spacer, and search shelf (Search). It owns the column
// geometry and delegates SetSize/SetLayout to its children.
//
// Column is immutable-style: all mutating helpers return a new value.
// AccountTab holds one by value and replaces it via the With* accessors,
// preserving the Elm contract. Column.SetSize stores its own dims for
// View() but does not propagate to children. AccountTab.WindowSizeMsg
// calls SetLayout/SetSize on each child directly, then wraps via SetSize.
// Column never needs to know about LayoutMode.
type Column struct {
	styles        Styles
	icons         uicore.IconSet
	model         Model
	search        Search
	accountEmail  string
	width, height int
}

// NewColumn assembles a Column from its constituent parts.
func NewColumn(styles Styles, icons uicore.IconSet, model Model, search Search, accountEmail string) Column {
	return Column{
		styles:       styles,
		icons:        icons,
		model:        model,
		search:       search,
		accountEmail: accountEmail,
	}
}

// SetSize records the column's width and height for View(). It does NOT
// propagate SetSize to child components. The AccountTab WindowSizeMsg branch
// handles child sizing explicitly and then calls SetSize to record the final dims.
func (c Column) SetSize(w, h int) Column {
	c.width = w
	c.height = h
	return c
}

func (c Column) Sidebar() Model        { return c.model }
func (c Column) SidebarSearch() Search { return c.search }

func (c Column) WithSidebar(m Model) Column {
	c.model = m
	return c
}

func (c Column) WithSidebarSearch(s Search) Column {
	c.search = s
	return c
}

// View renders the sidebar column: blank / account / blank header rows,
// folder region, spacer, and search shelf, but not the divider. The
// AccountTab owns the row-by-row join of sidebar + divider + right pane.
// Every row is exactly c.width display cells wide.
func (c Column) View() string {
	if c.width == 0 || c.height == 0 {
		return ""
	}
	sw := c.width

	acctName := uicore.DisplayTruncateEllipsis(c.accountEmail, sw-1)
	acctLine := c.styles.SidebarAccount.Width(sw).Render(" " + acctName)
	blank := c.styles.SidebarBg.Width(sw).Render("")

	sidebarFolders := c.model.View()
	shelfView := c.search.View()

	var lines []string
	lines = append(lines, blank, acctLine, blank)
	if sidebarFolders != "" {
		lines = append(lines, strings.Split(sidebarFolders, "\n")...)
	}
	// Pad the folder region so the search shelf lands at the bottom.
	targetFolderEnd := c.height - ShelfRows
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
