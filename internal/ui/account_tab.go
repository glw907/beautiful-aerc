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
	msglist MessageList
	width   int
	height  int
}

// NewAccountTab creates an AccountTab using the given styles and backend.
func NewAccountTab(styles Styles, backend mail.Backend) AccountTab {
	folders, _ := backend.ListFolders()
	sb := NewSidebar(styles, folders, sidebarWidth, 1)

	tab := AccountTab{
		styles:  styles,
		backend: backend,
		sidebar: sb,
		msglist: NewMessageList(styles, nil, 1, 1),
	}
	tab.loadSelectedFolder()
	return tab
}

// loadSelectedFolder fetches messages for the currently selected
// sidebar folder and seeds the message list. Mock-backed for now;
// Pass 3 will plumb this through real JMAP/IMAP fetches.
func (m *AccountTab) loadSelectedFolder() {
	name := m.sidebar.SelectedFolder()
	if name == "" {
		m.msglist.SetMessages(nil)
		return
	}
	if err := m.backend.OpenFolder(name); err != nil {
		// TODO(pass3): surface OpenFolder error via toast/status.
		m.msglist.SetMessages(nil)
		return
	}
	msgs, err := m.backend.FetchHeaders(nil)
	if err != nil {
		// TODO(pass3): surface FetchHeaders error via toast/status.
		m.msglist.SetMessages(nil)
		return
	}
	m.msglist.SetMessages(msgs)
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
		mw := max(1, m.width-sw-1)        // -1 for divider
		m.msglist.SetSize(mw, m.height)

	case tea.KeyMsg:
		m.handleKey(msg)
	}
	return m, nil
}

// handleKey dispatches navigation keys by identity. J/K/G move the
// sidebar (and refresh the message list); j/k/Ctrl-d/Ctrl-u move the
// message list cursor.
func (m *AccountTab) handleKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "J":
		m.sidebar.MoveDown()
		m.loadSelectedFolder()
	case "K":
		m.sidebar.MoveUp()
		m.loadSelectedFolder()
	case "G":
		m.msglist.MoveToBottom()
	case "g":
		m.msglist.MoveToTop()
	case "j", "down":
		m.msglist.MoveDown()
	case "k", "up":
		m.msglist.MoveUp()
	case "ctrl+d":
		m.msglist.HalfPageDown()
	case "ctrl+u":
		m.msglist.HalfPageUp()
	case "ctrl+f", "pgdown":
		m.msglist.PageDown()
	case "ctrl+b", "pgup":
		m.msglist.PageUp()
	}
}

// View renders the sidebar + divider + message list.
func (m AccountTab) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sw := min(sidebarWidth, m.width/2)

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
	msglistView := m.msglist.View()

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, divider, msglistView)
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
