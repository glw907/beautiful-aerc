package sidebar

import (
	"strings"

	"github.com/glw907/poplar/internal/ui/uicore"
)

// HeaderRows is the blank/account/blank padding above the folder list.
// AccountTab.View and sidebar sizing both depend on this number.
const HeaderRows = 3

// ShelfRows is the height of the Search shelf pinned to the bottom of Column.
const ShelfRows = 3

// Column is the composite left-hand column: account header rows, folder
// list, spacer, and search shelf. AccountTab holds one by value and
// replaces it via the With* accessors.
type Column struct {
	styles        Styles
	icons         uicore.IconSet
	model         Model
	search        Search
	accountEmail  string
	width, height int
}

func NewColumn(styles Styles, icons uicore.IconSet, model Model, search Search, accountEmail string) Column {
	return Column{
		styles:       styles,
		icons:        icons,
		model:        model,
		search:       search,
		accountEmail: accountEmail,
	}
}

// SetSize records the column's dims for View(). It does not propagate to
// children; AccountTab.WindowSizeMsg sizes them directly.
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

// View renders the sidebar column without the right-edge divider, which
// AccountTab joins on row by row. Every row is exactly c.width cells wide.
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
