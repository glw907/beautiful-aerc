package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/mail"
)

// Panel identifies which panel of the AccountTab is focused.
type Panel int

const (
	SidebarPanel Panel = iota
	MsgListPanel
)

// sidebarWidth is the fixed width of the sidebar panel.
const sidebarWidth = 30

// AccountTab is the main account view with sidebar and message list panels.
type AccountTab struct {
	styles      Styles
	backend     mail.Backend
	focused     Panel
	selectedIdx int
	folderCount int
	width       int
	height      int
}

// NewAccountTab creates an AccountTab using the given styles and backend.
func NewAccountTab(styles Styles, backend mail.Backend) AccountTab {
	folders, _ := backend.ListFolders()
	return AccountTab{
		styles:      styles,
		backend:     backend,
		focused:     SidebarPanel,
		folderCount: len(folders),
	}
}

// Init returns no initial command.
func (m AccountTab) Init() tea.Cmd { return nil }

// Update handles key events and window size changes.
func (m AccountTab) Update(msg tea.Msg) (AccountTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyTab:
			if m.focused == SidebarPanel {
				m.focused = MsgListPanel
			} else {
				m.focused = SidebarPanel
			}
		case m.focused == SidebarPanel && msg.String() == "j":
			if m.selectedIdx < m.folderCount-1 {
				m.selectedIdx++
			}
		case m.focused == SidebarPanel && msg.String() == "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
		}
	}
	return m, nil
}

// Folder icons indexed by role or name for sidebar display.
var folderIcons = map[string]string{
	"inbox":         "󰇰",
	"drafts":        "󰏫",
	"sent":          "󰑚",
	"archive":       "󰀼",
	"junk":          "󰍷",
	"trash":         "󰩺",
	"Notifications": "󰂚",
	"Remind":        "󰑴",
}

const defaultFolderIcon = "󰡡"

// folderIcon returns the icon for a folder by role, then name, then default.
func folderIcon(f mail.Folder) string {
	if f.Role != "" {
		if icon, ok := folderIcons[f.Role]; ok {
			return icon
		}
	}
	if icon, ok := folderIcons[f.Name]; ok {
		return icon
	}
	return defaultFolderIcon
}

// Folder groups: primary (inbox..archive), disposal (spam, trash),
// custom (everything else). Groups are separated by blank lines.
func groupFolders(folders []mail.Folder) [][]mail.Folder {
	primary := []string{"inbox", "drafts", "sent", "archive"}
	disposal := []string{"junk", "trash"}
	isPrimary := func(f mail.Folder) bool {
		for _, r := range primary {
			if f.Role == r {
				return true
			}
		}
		return false
	}
	isDisposal := func(f mail.Folder) bool {
		for _, r := range disposal {
			if f.Role == r {
				return true
			}
		}
		return false
	}

	var p, d, c []mail.Folder
	for _, f := range folders {
		switch {
		case isPrimary(f):
			p = append(p, f)
		case isDisposal(f):
			d = append(d, f)
		default:
			c = append(c, f)
		}
	}

	var groups [][]mail.Folder
	if len(p) > 0 {
		groups = append(groups, p)
	}
	if len(d) > 0 {
		groups = append(groups, d)
	}
	if len(c) > 0 {
		groups = append(groups, c)
	}
	return groups
}

// View renders the two-panel layout with sidebar folders and message list.
func (m AccountTab) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sw := minInt(sidebarWidth, m.width/2)
	mw := m.width - sw - 1 // -1 for divider

	sidebar := m.renderSidebar(sw)
	divider := renderDivider(m.height, m.styles)
	msglistContent := renderPlaceholder("Message List", mw, m.height, m.focused == MsgListPanel, m.styles)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, divider, msglistContent)
}

// renderSidebar renders the account name, folder groups, and padding.
func (m AccountTab) renderSidebar(width int) string {
	// Account name line
	acctLine := m.styles.SidebarAccount.Render(
		lipgloss.NewStyle().Width(width).Render(" " + m.backend.AccountName()),
	)

	// Blank separator
	blank := strings.Repeat(" ", width)

	// Build folder lines
	folders, _ := m.backend.ListFolders()
	groups := groupFolders(folders)

	folderIdx := 0
	var folderLines []string
	for gi, group := range groups {
		if gi > 0 {
			folderLines = append(folderLines, blank)
		}
		for _, f := range group {
			icon := folderIcon(f)
			name := f.Name
			selected := m.focused == SidebarPanel && folderIdx == m.selectedIdx

			var countStr string
			if f.Unseen > 0 {
				countStr = fmt.Sprintf("%d", f.Unseen)
			}

			// Build the line: " " + selection + icon + "  " + name + padding + count
			var line string
			if selected {
				line = " ┃ " + icon + "  " + name
			} else {
				line = "   " + icon + "  " + name
			}

			lineWidth := lipgloss.Width(line)
			countWidth := lipgloss.Width(countStr)
			padNeeded := maxInt(0, width-lineWidth-countWidth-1)
			line += strings.Repeat(" ", padNeeded) + countStr

			rendered := lipgloss.NewStyle().Width(width).Render(line)
			if selected {
				rendered = m.styles.SidebarSelected.Width(width).Render(line)
			}
			folderLines = append(folderLines, rendered)
			folderIdx++
		}
	}

	// Assemble: acct name, blank, folders, then pad to full height
	var lines []string
	lines = append(lines, acctLine, blank)
	lines = append(lines, folderLines...)

	// Pad remaining height
	for len(lines) < m.height {
		lines = append(lines, blank)
	}

	return strings.Join(lines[:m.height], "\n")
}

// renderPlaceholder renders a centered label in a panel of the given size.
func renderPlaceholder(label string, width, height int, focused bool, s Styles) string {
	topPad := maxInt(0, (height-1)/2)
	botPad := maxInt(0, height-1-topPad)
	leftPad := maxInt(0, (width-len(label))/2)

	var lines []string
	for i := 0; i < topPad; i++ {
		if focused {
			lines = append(lines, lipgloss.NewStyle().
				Width(width).
				Background(s.Selection.GetBackground()).
				Render(""))
		} else {
			lines = append(lines, strings.Repeat(" ", width))
		}
	}

	centeredLabel := strings.Repeat(" ", leftPad) + label
	if focused {
		lines = append(lines, lipgloss.NewStyle().
			Width(width).
			Foreground(s.Dim.GetForeground()).
			Background(s.Selection.GetBackground()).
			Render(centeredLabel))
	} else {
		lines = append(lines, lipgloss.NewStyle().
			Width(width).
			Foreground(s.Dim.GetForeground()).
			Render(centeredLabel))
	}

	for i := 0; i < botPad; i++ {
		if focused {
			lines = append(lines, lipgloss.NewStyle().
				Width(width).
				Background(s.Selection.GetBackground()).
				Render(""))
		} else {
			lines = append(lines, strings.Repeat(" ", width))
		}
	}

	return strings.Join(lines, "\n")
}

// renderDivider renders a vertical line of │ characters.
func renderDivider(height int, s Styles) string {
	div := s.PanelDivider.Render("│")
	lines := make([]string, height)
	for i := range lines {
		lines[i] = div
	}
	return strings.Join(lines, "\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
