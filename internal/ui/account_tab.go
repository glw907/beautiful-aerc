package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// sidebarWidth is the fixed width of the sidebar panel.
const sidebarWidth = 30

// sidebarHeaderRows is the account-name line plus the blank line
// below it, reserved at the top of the sidebar before the folder
// list. AccountTab.View and the sidebar's own sizing both depend on
// this number matching.
const sidebarHeaderRows = 2

// searchShelfRows is the height of the SidebarSearch shelf pinned
// to the bottom of the sidebar column.
const searchShelfRows = 3

// folderPage tracks lazy-load state for one folder.
type folderPage struct {
	loaded           int
	total            int
	loadMoreInFlight bool
}

// AccountTab is the main account view. One pane (like pine): every
// key is always live. J/K/G navigate folders, j/k navigate messages.
type AccountTab struct {
	styles Styles
	// backend is held as a read-only reference so Update can build
	// tea.Cmd closures that call backend methods. It is never
	// mutated and its results are never cached as owned state —
	// they come back as Msg types through the normal Update flow.
	// This is the elm-conventions Rule 5 exception.
	backend       mail.Backend
	uiCfg         config.UIConfig
	sidebar       Sidebar
	sidebarSearch SidebarSearch
	msglist       MessageList
	viewer        Viewer
	pages         map[string]*folderPage
	width         int
	height        int
}

// NewAccountTab builds an empty AccountTab. The initial folder list is
// fetched via Init's returned Cmd, not synchronously.
func NewAccountTab(styles Styles, t *theme.CompiledTheme, backend mail.Backend, uiCfg config.UIConfig) AccountTab {
	return AccountTab{
		styles:        styles,
		backend:       backend,
		uiCfg:         uiCfg,
		sidebar:       NewSidebar(styles, nil, uiCfg, sidebarWidth, 1),
		sidebarSearch: NewSidebarSearch(styles, sidebarWidth),
		msglist:       NewMessageList(styles, nil, 1, 1),
		viewer:        NewViewer(styles, t, backend.AccountName()),
		pages:         make(map[string]*folderPage),
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
		folderHeight := max(1, m.height-sidebarHeaderRows-searchShelfRows)
		m.sidebar.SetSize(sw, folderHeight)
		m.sidebarSearch.SetSize(sw)
		mw := max(1, m.width-sw-1) // -1 for divider
		m.msglist.SetSize(mw, m.height)
		m.viewer = m.viewer.SetSize(mw, m.height)
		return m, nil

	case foldersLoadedMsg:
		m.sidebar.SetFolders(mail.Classify(msg.folders), m.uiCfg)
		return m, m.selectionChangedCmds()

	case folderQueryDoneMsg:
		page := m.pageFor(msg.name)
		page.total = msg.total
		if !msg.reset {
			page.loadMoreInFlight = true
		}
		return m, fetchHeadersCmd(m.backend, msg.name, msg.uids, msg.reset)

	case headersAppliedMsg:
		page := m.pageFor(msg.name)
		page.loaded = len(msg.msgs)
		fc := m.uiCfg.Folders[m.sidebar.ConfigKey(msg.name)]
		order := SortDateDesc
		if fc.Sort == "date-asc" {
			order = SortDateAsc
		}
		threaded := m.uiCfg.Threading
		if fc.ThreadingSet {
			threaded = fc.Threading
		}
		m.msglist.SetSort(order)
		m.msglist.SetThreaded(threaded)
		m.msglist.SetMessages(msg.msgs)
		return m, nil

	case headersAppendedMsg:
		page := m.pageFor(msg.name)
		page.loaded += len(msg.msgs)
		page.loadMoreInFlight = false
		m.msglist.AppendMessages(msg.msgs)
		return m, nil

	case bodyLoadedMsg:
		if m.viewer.CurrentUID() == msg.uid {
			m.viewer = m.viewer.SetBody(msg.blocks)
		}
		return m, nil

	case ErrorMsg:
		// App owns the banner; AccountTab ignores. App.Update runs
		// before delegation, so the App layer captures the message.
		return m, nil

	case SearchUpdatedMsg:
		m.msglist.SetFilter(msg.Query, msg.Mode)
		m.sidebarSearch.SetResultCount(m.msglist.FilterResultCount())
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward any other Msg (spinner ticks, etc.) to the viewer when
	// it's open so its embedded sub-models keep advancing.
	if m.viewer.IsOpen() {
		var cmd tea.Cmd
		m.viewer, cmd = m.viewer.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleKey dispatches navigation keys by identity. When the viewer
// is open, every key routes there first. Otherwise: J/K/G move the
// sidebar (and dispatch a folder-load Cmd); j/k move the message-list
// cursor. During an active search, printable keys flow through
// SidebarSearch instead of the account-view handlers.
func (m AccountTab) handleKey(msg tea.KeyMsg) (AccountTab, tea.Cmd) {
	if m.viewer.IsOpen() {
		var cmd tea.Cmd
		m.viewer, cmd = m.viewer.Update(msg)
		return m, cmd
	}
	// Route to SidebarSearch when we're in Typing state — it owns
	// the input routing for this modal slice, except for Enter and
	// Esc which transition state.
	if m.sidebarSearch.State() == SearchTyping {
		switch msg.Type {
		case tea.KeyEnter:
			m.sidebarSearch.Commit()
			return m, nil
		case tea.KeyEsc:
			m.sidebarSearch.Clear()
			m.msglist.ClearFilter()
			return m, nil
		}
		var cmd tea.Cmd
		m.sidebarSearch, cmd = m.sidebarSearch.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "/":
		if m.sidebarSearch.State() == SearchIdle || m.sidebarSearch.State() == SearchActive {
			m.sidebarSearch.Activate()
			return m, nil
		}
	case "esc":
		if m.sidebarSearch.State() == SearchActive {
			m.sidebarSearch.Clear()
			m.msglist.ClearFilter()
			return m, nil
		}
	case "enter":
		return m.openSelectedMessage()
	case "J":
		m.clearSearchIfActive()
		m.sidebar.MoveDown()
		return m, m.selectionChangedCmds()
	case "K":
		m.clearSearchIfActive()
		m.sidebar.MoveUp()
		return m, m.selectionChangedCmds()
	case "I", "D", "S", "A", "X", "T":
		return m.jumpToFolder(folderJumpTargets[msg.String()])
	case "G":
		m.msglist.MoveToBottom()
	case "g":
		m.msglist.MoveToTop()
	case "j", "down":
		m.msglist.MoveDown()
	case "k", "up":
		m.msglist.MoveUp()
	case " ":
		if m.sidebarSearch.State() == SearchActive {
			return m, nil
		}
		m.msglist.ToggleFold()
	case "F":
		if m.sidebarSearch.State() == SearchActive {
			return m, nil
		}
		m.msglist.ToggleFoldAll()
	}
	if cmd := m.maybeLoadMore(); cmd != nil {
		return m, cmd
	}
	return m, nil
}

// folderJumpTargets maps the uppercase folder-jump key to its
// canonical folder name.
var folderJumpTargets = map[string]string{
	"I": "Inbox",
	"D": "Drafts",
	"S": "Sent",
	"A": "Archive",
	"X": "Spam",
	"T": "Trash",
}

// jumpToFolder moves the sidebar selection to the canonical folder
// with the given name. No-op (and no Cmd) when no folder matches —
// e.g. an account that doesn't expose a Drafts folder. Behaves like
// J/K otherwise: clears any active search, fires FolderChangedMsg +
// loadFolderCmd via selectionChangedCmds.
func (m AccountTab) jumpToFolder(canonical string) (AccountTab, tea.Cmd) {
	if !m.sidebar.SelectByCanonical(canonical) {
		return m, nil
	}
	m.clearSearchIfActive()
	return m, m.selectionChangedCmds()
}

// openSelectedMessage opens the current msglist selection in the
// viewer, fires the body-fetch Cmd, and (for unread messages) flips
// the seen flag locally + fires a backend MarkRead.
func (m AccountTab) openSelectedMessage() (AccountTab, tea.Cmd) {
	msg, ok := m.msglist.SelectedMessage()
	if !ok {
		return m, nil
	}
	m.viewer = m.viewer.Open(msg)
	cmds := []tea.Cmd{
		loadBodyCmd(m.backend, msg.UID),
		viewerOpenedCmd(),
		m.viewer.SpinnerTick(),
	}
	if msg.Flags&mail.FlagSeen == 0 {
		m.msglist.MarkSeen(msg.UID)
		cmds = append(cmds, markReadCmd(m.backend, msg.UID))
	}
	return m, tea.Batch(cmds...)
}

// clearSearchIfActive clears the shelf and the filter if the shelf
// is in any non-Idle state. No-op when already idle.
func (m *AccountTab) clearSearchIfActive() {
	if m.sidebarSearch.State() == SearchIdle {
		return
	}
	m.sidebarSearch.Clear()
	m.msglist.ClearFilter()
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
		openFolderCmd(m.backend, folder.Name),
	)
}

// currentFolderName returns the provider name of the currently-selected
// sidebar folder, or "" when nothing is selected.
func (m AccountTab) currentFolderName() string {
	folder, ok := m.sidebar.SelectedFolderInfo()
	if !ok {
		return ""
	}
	return folder.Name
}

// pageFor returns (creating if absent) the folderPage for name.
func (m *AccountTab) pageFor(name string) *folderPage {
	if m.pages[name] == nil {
		m.pages[name] = &folderPage{}
	}
	return m.pages[name]
}

// loadMoreTrigger is how many rows from the bottom trigger a load-more.
const loadMoreTrigger = 20

// maybeLoadMore issues a loadMoreCmd when the cursor is near the bottom
// and more messages are available. Returns nil when no action is needed.
func (m *AccountTab) maybeLoadMore() tea.Cmd {
	name := m.currentFolderName()
	if name == "" {
		return nil
	}
	page := m.pageFor(name)
	if page.loadMoreInFlight || page.loaded >= page.total {
		return nil
	}
	if !m.msglist.IsNearBottom(loadMoreTrigger) {
		return nil
	}
	page.loadMoreInFlight = true
	return loadMoreCmd(m.backend, name, page.loaded)
}

// View renders the sidebar + divider + message list. The sidebar
// column is composed top-to-bottom as: account header (2 rows),
// folder region (flex), search shelf (3 rows pinned to bottom).
func (m AccountTab) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sw := min(sidebarWidth, m.width/2)

	acctLine := m.styles.SidebarAccount.Width(sw).Render(" " + m.backend.AccountName())
	blank := m.styles.SidebarBg.Width(sw).Render("")

	sidebarFolders := m.sidebar.View()
	shelfView := m.sidebarSearch.View()

	var sidebarLines []string
	sidebarLines = append(sidebarLines, acctLine, blank)
	if sidebarFolders != "" {
		sidebarLines = append(sidebarLines, strings.Split(sidebarFolders, "\n")...)
	}
	// Pad the folder region with blank rows so the shelf lands at
	// the bottom of the column regardless of how many folders exist.
	targetFolderEnd := m.height - searchShelfRows
	for len(sidebarLines) < targetFolderEnd {
		sidebarLines = append(sidebarLines, blank)
	}
	if len(sidebarLines) > targetFolderEnd {
		sidebarLines = sidebarLines[:targetFolderEnd]
	}
	sidebarLines = append(sidebarLines, strings.Split(shelfView, "\n")...)
	if len(sidebarLines) > m.height {
		sidebarLines = sidebarLines[:m.height]
	}

	sidebarView := strings.Join(sidebarLines, "\n")
	divider := renderDivider(m.height, m.styles)
	right := m.msglist.View()
	if m.viewer.IsOpen() {
		right = m.viewer.View()
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, divider, right)
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
