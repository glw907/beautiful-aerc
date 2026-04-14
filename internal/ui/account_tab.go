package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// sidebarWidth is the fixed width of the sidebar panel.
const sidebarWidth = 30

// sidebarHeaderRows is the account-name line plus the blank line
// below it, reserved at the top of the sidebar before the folder
// list. AccountTab.View and the sidebar's own sizing both depend on
// this number matching.
const sidebarHeaderRows = 2

// AccountTab is the main account view. One pane (like pine): every
// key is always live. J/K/G navigate folders, j/k navigate messages.
type AccountTab struct {
	styles Styles
	// backend is held as a read-only reference so Update can build
	// tea.Cmd closures that call backend methods. It is never
	// mutated and its results are never cached as owned state —
	// they come back as Msg types through the normal Update flow.
	// This is the elm-conventions Rule 5 exception.
	backend mail.Backend
	uiCfg   config.UIConfig
	sidebar Sidebar
	msglist MessageList
	width   int
	height  int
}

// NewAccountTab builds an empty AccountTab. The initial folder list is
// fetched via Init's returned Cmd, not synchronously.
func NewAccountTab(styles Styles, backend mail.Backend, uiCfg config.UIConfig) AccountTab {
	return AccountTab{
		styles:  styles,
		backend: backend,
		uiCfg:   uiCfg,
		sidebar: NewSidebar(styles, nil, uiCfg, sidebarWidth, 1),
		msglist: NewMessageList(styles, nil, 1, 1),
	}
}

// Title returns the current folder name.
func (m AccountTab) Title() string { return m.sidebar.SelectedFolder() }

// Icon returns the folder's Nerd Font icon.
func (m AccountTab) Icon() string { return m.sidebar.SelectedIcon() }

// Closeable returns false — the account tab cannot be closed.
func (m AccountTab) Closeable() bool { return false }

// Init fires the initial folder-list fetch.
func (m AccountTab) Init() tea.Cmd {
	return loadFoldersCmd(m.backend)
}

// Update satisfies tea.Model. Delegates to updateTab for typed access.
func (m AccountTab) Update(msg tea.Msg) (AccountTab, tea.Cmd) {
	return m.updateTab(msg)
}

// updateTab handles the message cases and returns a typed AccountTab.
func (m AccountTab) updateTab(msg tea.Msg) (AccountTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		sw := min(sidebarWidth, m.width/2)
		m.sidebar.SetSize(sw, m.height-sidebarHeaderRows)
		mw := max(1, m.width-sw-1) // -1 for divider
		m.msglist.SetSize(mw, m.height)
		return m, nil

	case foldersLoadedMsg:
		m.sidebar.SetFolders(mail.Classify(msg.folders), m.uiCfg)
		return m, m.selectionChangedCmds()

	case folderLoadedMsg:
		order := SortDateDesc
		if fc, ok := m.uiCfg.Folders[msg.name]; ok && fc.Sort == "date-asc" {
			order = SortDateAsc
		}
		m.msglist.SetSort(order)
		m.msglist.SetMessages(msg.msgs)
		return m, nil

	case backendErrMsg:
		// TODO(pass-2.5b-6): surface via status/toast.
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey dispatches navigation keys by identity. J/K/G move the
// sidebar (and dispatch a folder-load Cmd); j/k/Ctrl-d/Ctrl-u move the
// message list cursor.
func (m AccountTab) handleKey(msg tea.KeyMsg) (AccountTab, tea.Cmd) {
	switch msg.String() {
	case "J":
		m.sidebar.MoveDown()
		return m, m.selectionChangedCmds()
	case "K":
		m.sidebar.MoveUp()
		return m, m.selectionChangedCmds()
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
	return m, nil
}

// selectionChangedCmds returns the batch of Cmds that run every time
// the selected folder changes: a FolderChangedMsg emission so App's
// status bar updates, plus a load Cmd that will populate the message
// list when it resolves.
func (m AccountTab) selectionChangedCmds() tea.Cmd {
	folder, ok := m.sidebar.SelectedFolderInfo()
	if !ok {
		return nil
	}
	return tea.Batch(
		folderChangedCmd(folder),
		loadFolderCmd(m.backend, folder.Name),
	)
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
