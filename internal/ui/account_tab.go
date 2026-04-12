package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/mail"
)

// sidebarWidth is the fixed width of the sidebar panel.
const sidebarWidth = 30

// AccountTab is the main account view. One pane (like pine): every
// key is always live. J/K/G navigate folders, j/k navigate messages.
type AccountTab struct {
	styles  Styles
	backend mail.Backend
	sidebar Sidebar
	width   int
	height  int
}

// NewAccountTab creates an AccountTab using the given styles and backend.
func NewAccountTab(styles Styles, backend mail.Backend) AccountTab {
	folders, _ := backend.ListFolders()
	sb := NewSidebar(styles, folders, sidebarWidth, 1)

	return AccountTab{
		styles:  styles,
		backend: backend,
		sidebar: sb,
	}
}

// Title returns the current folder name.
func (m AccountTab) Title() string { return m.sidebar.SelectedFolder() }

// Icon returns the folder's Nerd Font icon.
func (m AccountTab) Icon() string { return m.sidebar.SelectedIcon() }

// Closeable returns false — the account tab cannot be closed.
func (m AccountTab) Closeable() bool { return false }

// Init returns no initial command.
func (m AccountTab) Init() tea.Cmd { return nil }

// Update satisfies tea.Model. Delegates to updateTab for typed access.
func (m AccountTab) Update(msg tea.Msg) (AccountTab, tea.Cmd) {
	return m.updateTab(msg)
}

// updateTab handles key events and window size changes, returning the typed model.
func (m AccountTab) updateTab(msg tea.Msg) (AccountTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		sw := min(sidebarWidth, m.width/2)
		m.sidebar.SetSize(sw, m.height-2) // -2 for account name + blank line

	case tea.KeyMsg:
		m.handleKey(msg)
	}
	return m, nil
}

// handleKey dispatches navigation keys by identity. J/K/G move the
// sidebar; j/k will move the message list cursor once it exists.
func (m *AccountTab) handleKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "J", "down":
		m.sidebar.MoveDown()
	case "K", "up":
		m.sidebar.MoveUp()
	case "G":
		m.sidebar.MoveToBottom()
	}
}

// View renders the sidebar + divider + message list placeholder.
func (m AccountTab) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sw := min(sidebarWidth, m.width/2)
	mw := m.width - sw - 1 // -1 for divider

	acctLine := m.styles.SidebarAccount.Width(sw).Render(" " + m.backend.AccountName())
	blank := m.styles.SidebarBg.Width(sw).Render("")

	sidebarFolders := m.sidebar.View()

	var sidebarLines []string
	sidebarLines = append(sidebarLines, acctLine, blank)
	if sidebarFolders != "" {
		sidebarLines = append(sidebarLines, strings.Split(sidebarFolders, "\n")...)
	}

	for len(sidebarLines) < m.height {
		sidebarLines = append(sidebarLines, blank)
	}
	if len(sidebarLines) > m.height {
		sidebarLines = sidebarLines[:m.height]
	}

	sidebarView := strings.Join(sidebarLines, "\n")
	divider := renderDivider(m.height, m.styles)
	msglistView := renderPlaceholder("Message List", mw, m.height, m.styles)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, divider, msglistView)
}

// renderPlaceholder renders a centered label in a panel of the given size.
func renderPlaceholder(label string, width, height int, s Styles) string {
	topPad := max(0, (height-1)/2)
	botPad := max(0, height-1-topPad)
	leftPad := max(0, (width-len(label))/2)

	padStyle := lipgloss.NewStyle().Width(width)
	labelStyle := lipgloss.NewStyle().Width(width).Foreground(s.Dim.GetForeground())

	blankLine := padStyle.Render("")
	var lines []string
	for range topPad {
		lines = append(lines, blankLine)
	}
	lines = append(lines, labelStyle.Render(strings.Repeat(" ", leftPad)+label))
	for range botPad {
		lines = append(lines, blankLine)
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
