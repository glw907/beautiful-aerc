package ui

import (
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
	styles  Styles
	backend mail.Backend
	focused Panel
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
		focused: SidebarPanel,
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
		switch {
		case msg.Type == tea.KeyTab:
			if m.focused == SidebarPanel {
				m.focused = MsgListPanel
			} else {
				m.focused = SidebarPanel
			}
			m.sidebar.SetFocused(m.focused == SidebarPanel)

		default:
			if m.focused == SidebarPanel {
				m.handleSidebarKey(msg)
			}
		}
	}
	return m, nil
}

// handleSidebarKey routes key events to sidebar actions.
// J/K (uppercase) navigate folders. j/k are reserved for messages.
func (m *AccountTab) handleSidebarKey(msg tea.KeyMsg) {
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

	// Account name line + blank line + folder rows = sidebar view
	acctLine := m.styles.SidebarAccount.Width(sw).Render(" " + m.backend.AccountName())
	blank := m.styles.SidebarBg.Width(sw).Render("")

	// Sidebar content: account name + blank + folder list
	sidebarFolders := m.sidebar.View()

	var sidebarLines []string
	sidebarLines = append(sidebarLines, acctLine, blank)
	if sidebarFolders != "" {
		sidebarLines = append(sidebarLines, strings.Split(sidebarFolders, "\n")...)
	}

	// Pad to height
	for len(sidebarLines) < m.height {
		sidebarLines = append(sidebarLines, strings.Repeat(" ", sw))
	}
	if len(sidebarLines) > m.height {
		sidebarLines = sidebarLines[:m.height]
	}

	sidebarView := strings.Join(sidebarLines, "\n")
	divider := renderDivider(m.height, m.styles)
	msglistView := renderPlaceholder("Message List", mw, m.height, m.focused == MsgListPanel, m.styles)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, divider, msglistView)
}

// renderPlaceholder renders a centered label in a panel of the given size.
func renderPlaceholder(label string, width, height int, focused bool, s Styles) string {
	topPad := max(0, (height-1)/2)
	botPad := max(0, height-1-topPad)
	leftPad := max(0, (width-len(label))/2)

	padStyle := lipgloss.NewStyle().Width(width)
	labelStyle := lipgloss.NewStyle().Width(width).Foreground(s.Dim.GetForeground())
	if focused {
		bg := s.Selection.GetBackground()
		padStyle = padStyle.Background(bg)
		labelStyle = labelStyle.Background(bg)
	}

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
