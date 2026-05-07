package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/messagelist"
	"github.com/glw907/poplar/internal/ui/movepicker"
	"github.com/glw907/poplar/internal/ui/reader"
	"github.com/glw907/poplar/internal/ui/sidebar"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// folderPage tracks lazy-load state for one folder.
type folderPage struct {
	loaded           int
	total            int
	loadMoreInFlight bool
}

// Model is the main account view. One pane (like pine): every
// key is always live. J/K/G navigate folders, j/k navigate messages.
type Model struct {
	styles Styles
	icons  uicore.IconSet
	// acct is the per-account cache handle. UI reads come from
	// acct.QueryFolder. UI writes funnel through acct.QueueOp. The
	// underlying mail.Backend reference lives behind acct.Backend
	// (used for AccountName/AccountEmail accessors and body fetch).
	acct          *cache.Account
	uiCfg         config.UIConfig
	sidebarColumn sidebar.Column
	msglist       messagelist.Model
	viewer        reader.Model
	keys          Keys
	pages         map[string]*folderPage
	swept         map[string]bool
	loading       bool
	spinner       spinner.Model
	layout        uicore.LayoutMode
	width         int
	height        int
	// bodyFetchCancel cancels the in-flight loadBodyCmd goroutine.
	// Set on every openMessage call. Cleared when the result arrives
	// (matched UID) or when the viewer closes.
	bodyFetchCancel context.CancelFunc
	// now returns the wall clock. Test seam, defaults to time.Now.
	now func() time.Time
}

// WithNow returns a copy of m with the clock seam replaced.
func (m Model) WithNow(now func() time.Time) Model {
	m.now = now
	return m
}

// New builds an empty account Model. The initial folder list is
// fetched via Init's returned Cmd, not synchronously.
func New(t *theme.CompiledTheme, acct *cache.Account, uiCfg config.UIConfig, icons uicore.IconSet) Model {
	sidebarStyles := sidebar.NewStyles(t)
	return Model{
		styles: NewStyles(t),
		icons:  icons,
		acct:   acct,
		uiCfg:  uiCfg,
		sidebarColumn: sidebar.NewColumn(sidebarStyles, icons,
			sidebar.New(sidebarStyles, nil, uiCfg, 30, 1, icons),
			sidebar.NewSearch(sidebarStyles, 30, icons),
			acct.AccountEmail(),
		),
		msglist: messagelist.New(messagelist.NewStyles(t), nil, 1, 1, icons),
		viewer:  reader.New(reader.NewStyles(t), t, acct.AccountEmail(), icons),
		keys:    NewKeys(),
		pages:   make(map[string]*folderPage),
		swept:   make(map[string]bool),
		spinner: uicore.NewSpinner(t),
		layout:  uicore.ComputeLayout(80),
		now:     time.Now,
	}
}

func (m Model) Title() string { return m.sidebarColumn.Sidebar().SelectedFolder() }

func (m Model) Backend() mail.Backend { return m.acct.Backend }

func (m Model) Cache() *cache.Account { return m.acct }

func (m Model) AccountEmail() string { return m.acct.AccountEmail() }

func (m Model) Icon() string { return m.sidebarColumn.Sidebar().SelectedIcon() }

// Closeable always returns false. The account tab cannot be closed.
func (m Model) Closeable() bool { return false }

// Init fires the initial folder-list fetch and starts the cache event
// pump.
func (m Model) Init() tea.Cmd {
	return tea.Batch(loadFoldersCmd(m.acct), pumpCacheCmd(m.acct))
}

// Update satisfies tea.Model. Delegates to updateTab for typed access.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m.updateTab(msg)
}

func (m Model) updateTab(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		layout := uicore.ComputeLayout(m.width)
		if layout.Sidebar > m.width/2 {
			layout.Sidebar = m.width / 2
		}
		m.layout = layout
		m.msglist.SetLayout(layout)

		sw := layout.Sidebar
		folderHeight := max(1, m.height-sidebar.HeaderRows-sidebar.ShelfRows)
		sb := m.sidebarColumn.Sidebar()
		sb.SetLayout(layout)
		sb.SetSize(sw, folderHeight)
		ss := m.sidebarColumn.SidebarSearch()
		ss.SetSize(sw)
		// Forward WindowSizeMsg to SidebarSearch so embedded bubbles
		// components (textinput) can reflow.
		var cmds []tea.Cmd
		var c tea.Cmd
		ss, c = ss.Update(msg)
		cmds = append(cmds, c)
		m.sidebarColumn = m.sidebarColumn.
			WithSidebar(sb).
			WithSidebarSearch(ss).
			SetSize(sw, m.height)
		mw := max(1, m.width-sw-1) // -1 for divider
		m.msglist.SetSize(mw, m.height)
		m.viewer = m.viewer.SetSize(mw, m.height)
		// Forward the msg so embedded bubbles components in viewer reflow.
		m.viewer, c = m.viewer.Update(msg)
		cmds = append(cmds, c)
		return m, tea.Batch(cmds...)

	case sidebar.ClearSearchMsg:
		m = m.clearSearchIfActive()
		return m, nil

	case foldersLoadedMsg:
		sb := m.sidebarColumn.Sidebar()
		sb.SetFolders(msg.classified, m.uiCfg)
		m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
		return m.selectionChangedCmds()

	case FolderLoadedMsg:
		m.loading = false
		page := m.pageFor(msg.Name)
		// selectionChangedCmds zeroes the page before firing
		// openFolderCmd, so loaded == 0 reliably means "fresh open"
		// (cursor reset). Any other value means "post-write refresh"
		// (cursor preserved). Snapshot before mutating page.loaded.
		isInitial := page.loaded == 0
		page.loaded = len(msg.Msgs)
		page.total = msg.Total
		fc := m.uiCfg.Folders[m.sidebarColumn.Sidebar().ConfigKey(msg.Name)]
		order := messagelist.SortDateDesc
		if fc.Sort == "date-asc" {
			order = messagelist.SortDateAsc
		}
		threaded := m.uiCfg.Threading
		if fc.ThreadingSet {
			threaded = fc.Threading
		}
		m.msglist.SetSort(order)
		m.msglist.SetThreaded(threaded)
		if isInitial {
			m.msglist.SetMessages(msg.Msgs)
		} else {
			m.msglist.RefreshSource(msg.Msgs)
		}
		if sweep := m.maybeRetentionSweep(msg.Name, msg.Msgs); sweep != nil {
			return m, sweep
		}
		return m, nil

	case folderAppendedMsg:
		page := m.pageFor(msg.name)
		page.loaded += len(msg.msgs)
		page.total = msg.total
		page.loadMoreInFlight = false
		m.msglist.AppendMessages(msg.msgs)
		return m, nil

	case CacheEventMsg:
		// Drainer transition: re-read current folder so any backing
		// state change is reflected. Re-arm the pump.
		cmds := []tea.Cmd{pumpCacheCmd(m.acct)}
		if name := m.currentFolderName(); name != "" {
			cmds = append(cmds, refreshFolderCmd(m.acct, name))
		}
		return m, tea.Batch(cmds...)

	case reader.BodyLoadedMsg:
		if m.viewer.CurrentUID() == msg.UID {
			m.bodyFetchCancel = nil
			m.viewer = m.viewer.SetBody(msg.Blocks)
		}
		return m, nil

	case reader.AttachmentsLoadedMsg:
		if m.viewer.CurrentUID() == msg.UID {
			m.viewer = m.viewer.SetAttachments(msg.Items)
		}
		return m, nil

	case ErrorMsg:
		// App owns the banner. Model ignores it. App.Update runs
		// before delegation, so the App layer captures the message.
		return m, nil

	case movepicker.PickedMsg:
		return m, m.dispatchMoveFromPicker(msg)

	case sweepCompletedMsg:
		// Sweep fired. Cache now has ui_hide=1 on each affected row.
		// Refresh to reflect.
		if name := m.currentFolderName(); name != "" {
			return m, refreshFolderCmd(m.acct, name)
		}
		return m, nil

	case EmptyFolderConfirmedMsg:
		return m, emptyFolderCmd(m.acct, msg.Folder, msg.Source)

	case emptyFolderDoneMsg:
		n := msg.n
		folder := msg.folder
		src := msg.source
		toast := func() tea.Msg {
			return TriageStartedMsg{Op: uicore.TriageEmpty, N: n, Dest: folder}
		}
		return m, tea.Batch(toast, refreshFolderCmd(m.acct, src))

	case sidebar.SearchUpdatedMsg:
		m.msglist.SetFilter(msg.Query, msg.Mode)
		ss := m.sidebarColumn.SidebarSearch()
		ss.SetResultCount(m.msglist.FilterResultCount())
		m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
		return m, nil

	case spinner.TickMsg:
		if m.loading && msg.ID == m.spinner.ID() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		// Tick belongs to the viewer's spinner (or a stale tick from
		// a prior generation). Fall through to the viewer-forward
		// block. The viewer's spinner ID guard rejects stale ticks.

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
// sidebar (and dispatch a folder-load Cmd). j/k move the message-list
// cursor. During an active search, printable keys flow through
// SidebarSearch instead of the account-view handlers.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.viewer.IsOpen() {
		delta := 0
		switch {
		case key.Matches(msg, m.keys.NextMessage):
			delta = 1
		case key.Matches(msg, m.keys.PrevMessage):
			delta = -1
		}
		if delta != 0 {
			if m.viewer.Phase() != reader.PhaseReady {
				return m, nil
			}
			uid, moved := m.msglist.MoveCursor(delta)
			if !moved {
				return m, nil
			}
			info, ok := m.msglist.MessageByUID(uid)
			if !ok {
				return m, nil
			}
			return m.openMessage(info)
		}
		var cmd tea.Cmd
		m.viewer, cmd = m.viewer.Update(msg)
		if !m.viewer.IsOpen() {
			// Viewer just closed. Discard any in-flight body fetch.
			m = m.cancelInflightBodyFetch()
		}
		return m, cmd
	}
	// Route to SidebarSearch when we're in Typing state. It owns
	// the input routing for this modal slice, except for Enter and
	// Esc which transition state.
	if ss := m.sidebarColumn.SidebarSearch(); ss.State() == sidebar.SearchTyping {
		switch {
		case key.Matches(msg, m.keys.SearchCommit):
			ss.Commit()
			m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
			return m, nil
		case key.Matches(msg, m.keys.ClearSearch):
			ss.Clear()
			m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
			m.msglist.ClearFilter()
			return m, nil
		}
		var cmd tea.Cmd
		ss, cmd = ss.Update(msg)
		m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
		return m, cmd
	}

	switch {
	case key.Matches(msg, m.keys.OpenSearch):
		ss := m.sidebarColumn.SidebarSearch()
		if st := ss.State(); st == sidebar.SearchIdle || st == sidebar.SearchActive {
			ss.Activate()
			m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
			return m, nil
		}
	case key.Matches(msg, m.keys.ClearSearch):
		ss := m.sidebarColumn.SidebarSearch()
		if ss.State() == sidebar.SearchActive {
			ss.Clear()
			m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
			m.msglist.ClearFilter()
			return m, nil
		}
	case key.Matches(msg, m.keys.OpenMessage):
		return m.openSelectedMessage()
	case key.Matches(msg, m.keys.SidebarDown):
		m = m.clearSearchIfActive()
		sb := m.sidebarColumn.Sidebar()
		sb.MoveDown()
		m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
		return m.selectionChangedCmds()
	case key.Matches(msg, m.keys.SidebarUp):
		m = m.clearSearchIfActive()
		sb := m.sidebarColumn.Sidebar()
		sb.MoveUp()
		m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
		return m.selectionChangedCmds()
	case key.Matches(msg, m.keys.JumpInbox):
		return m.jumpToFolder("Inbox")
	case key.Matches(msg, m.keys.JumpDrafts):
		return m.jumpToFolder("Drafts")
	case key.Matches(msg, m.keys.JumpSent):
		return m.jumpToFolder("Sent")
	case key.Matches(msg, m.keys.JumpArchive):
		return m.jumpToFolder("Archive")
	case key.Matches(msg, m.keys.JumpSpam):
		return m.jumpToFolder("Spam")
	case key.Matches(msg, m.keys.JumpTrash):
		return m.jumpToFolder("Trash")
	case key.Matches(msg, m.keys.MsgListBottom):
		m.msglist.MoveToBottom()
	case key.Matches(msg, m.keys.MsgListTop):
		m.msglist.MoveToTop()
	case key.Matches(msg, m.keys.MsgListDown):
		m.msglist.MoveDown()
	case key.Matches(msg, m.keys.MsgListUp):
		m.msglist.MoveUp()
	case key.Matches(msg, m.keys.ToggleFold):
		// In visual-select mode, Space toggles the cursor row's mark
		// instead of folding the thread. Outside visual mode, fold
		// behavior is unchanged (and inert during an active search).
		if m.msglist.VisualMode() {
			if cur, ok := m.msglist.SelectedMessage(); ok {
				m.msglist.ToggleMark(cur.UID)
			}
			return m, nil
		}
		if m.sidebarColumn.SidebarSearch().State() == sidebar.SearchActive {
			return m, nil
		}
		m.msglist.ToggleFold()
	case key.Matches(msg, m.keys.ToggleFoldAll):
		if m.sidebarColumn.SidebarSearch().State() == sidebar.SearchActive {
			return m, nil
		}
		m.msglist.ToggleFoldAll()
	case key.Matches(msg, m.keys.Delete):
		return m, m.dispatchTriage(uicore.TriageDelete)
	case key.Matches(msg, m.keys.Archive):
		return m, m.dispatchTriage(uicore.TriageArchive)
	case key.Matches(msg, m.keys.Star):
		return m, m.dispatchTriage(uicore.TriageStar)
	case key.Matches(msg, m.keys.ReadToggle):
		return m, m.dispatchTriage(uicore.TriageRead)
	case key.Matches(msg, m.keys.EnterVisual):
		m.msglist.EnterVisual()
		return m, nil
	case key.Matches(msg, m.keys.Move):
		return m, m.dispatchMove()
	case key.Matches(msg, m.keys.Empty):
		return m, m.dispatchEmpty()
	}
	if cmd := m.maybeLoadMore(); cmd != nil {
		return m, cmd
	}
	return m, nil
}

// jumpToFolder moves the sidebar selection to the canonical folder
// with the given name. No-op (and no Cmd) when no folder matches;
// e.g. an account that doesn't expose a Drafts folder. Behaves like
// J/K otherwise: clears any active search, fires the load Cmd via
// selectionChangedCmds.
func (m Model) jumpToFolder(canonical string) (Model, tea.Cmd) {
	sb := m.sidebarColumn.Sidebar()
	if !sb.SelectByCanonical(canonical) {
		return m, nil
	}
	m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
	m = m.clearSearchIfActive()
	return m.selectionChangedCmds()
}

// cancelInflightBodyFetch cancels any in-flight loadBodyCmd and
// clears the cancel func. No-op when no fetch is in flight.
func (m Model) cancelInflightBodyFetch() Model {
	if m.bodyFetchCancel != nil {
		m.bodyFetchCancel()
		m.bodyFetchCancel = nil
	}
	return m
}

// openMessage opens msg in the viewer, fires the body-fetch Cmd, and
// (for unread messages) queues a FlagSeen=true op via the cache.
// Shared by Enter, n, and N. Cancels any prior in-flight body fetch
// before issuing the new one.
func (m Model) openMessage(msg mail.MessageInfo) (Model, tea.Cmd) {
	m = m.cancelInflightBodyFetch()
	ctx, cancel := context.WithCancel(context.Background())
	m.bodyFetchCancel = cancel
	m.viewer = m.viewer.Open(msg)
	cmds := []tea.Cmd{
		loadBodyCmd(ctx, m.acct, msg.UID),
		loadAttachmentsCmd(m.acct, msg.UID),
		m.viewer.SpinnerTick(),
	}
	if msg.Flags&mail.FlagSeen == 0 {
		cmds = append(cmds, markReadCmd(m.acct, m.currentFolderName(), msg.UID))
	}
	return m, tea.Batch(cmds...)
}

// openSelectedMessage delegates to openMessage with the current
// msglist cursor. No-op when the folder is empty.
func (m Model) openSelectedMessage() (Model, tea.Cmd) {
	msg, ok := m.msglist.SelectedMessage()
	if !ok {
		return m, nil
	}
	return m.openMessage(msg)
}

// clearSearchIfActive clears the shelf and the filter if the shelf
// is in any non-Idle state. No-op when already idle.
func (m Model) clearSearchIfActive() Model {
	ss := m.sidebarColumn.SidebarSearch()
	if ss.State() == sidebar.SearchIdle {
		return m
	}
	ss.Clear()
	m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
	m.msglist.ClearFilter()
	return m
}

// Returns m alongside the Cmd so mutations are visible at the call site.
func (m Model) selectionChangedCmds() (Model, tea.Cmd) {
	folder, ok := m.sidebarColumn.Sidebar().SelectedFolderInfo()
	if !ok {
		return m, nil
	}
	m.loading = true
	// Reset the page so FolderLoadedMsg uses SetMessages (cursor reset)
	// rather than RefreshSource (cursor preserved).
	m.pages[folder.Name] = &folderPage{}
	m.msglist.SetMessages(nil)
	return m, tea.Batch(
		openFolderCmd(m.acct, folder.Name),
		m.spinner.Tick,
	)
}

// IsSwept reports whether the retention sweep has already fired for
// the named folder this session. Used by tests.
func (m Model) IsSwept(folder string) bool {
	return m.swept[folder]
}

// maybeRetentionSweep checks whether folderName is a Disposal folder
// with a positive retention threshold. If so, and if the sweep has not
// run yet this session, it marks the folder swept, collects expired
// UIDs, and returns a destroyCmd (which may be a no-op if no UIDs
// qualify). Returns nil when retention is disabled or the sweep already ran.
func (m *Model) maybeRetentionSweep(folderName string, loaded []mail.MessageInfo) tea.Cmd {
	folder, ok := m.sidebarColumn.Sidebar().FolderByProviderName(folderName)
	if !ok {
		return nil
	}
	var days int
	switch folder.Role {
	case "trash":
		days = m.uiCfg.TrashRetentionDays
	case "junk", "spam":
		days = m.uiCfg.SpamRetentionDays
	default:
		return nil
	}
	if days <= 0 {
		return nil
	}
	if m.swept[folder.Name] {
		return nil
	}
	m.swept[folder.Name] = true

	cutoff := m.now().Add(-time.Duration(days) * 24 * time.Hour)
	var expired []mail.UID
	for _, msg := range loaded {
		if msg.SentAt.IsZero() {
			continue
		}
		if msg.SentAt.Before(cutoff) {
			expired = append(expired, msg.UID)
		}
	}
	return destroyCmd(m.acct, folder.Name, expired)
}

// dispatchEmpty emits OpenConfirmEmptyMsg when the selected folder is
// Trash or Spam. Returns nil for all other folders. Inert by design.
func (m *Model) dispatchEmpty() tea.Cmd {
	folder, ok := m.sidebarColumn.Sidebar().SelectedFolderInfo()
	if !ok {
		return nil
	}
	var display string
	switch folder.Role {
	case "trash":
		display = "Trash"
	case "junk", "spam":
		display = "Spam"
	default:
		return nil
	}
	total := m.pages[folder.Name].total
	src := folder.Name
	return func() tea.Msg {
		return OpenConfirmEmptyMsg{
			Folder: display,
			Total:  total,
			Source: src,
		}
	}
}

// currentFolderName returns the canonical name of the currently-selected
// sidebar folder, or "" when nothing is selected.
func (m Model) currentFolderName() string {
	folder, ok := m.sidebarColumn.Sidebar().SelectedFolderInfo()
	if !ok {
		return ""
	}
	return folder.Name
}

// WindowCounter returns a "loaded/total" string when the current folder
// has more messages available than are loaded, e.g. "500/2347". Returns
// "" when all messages are loaded, the folder is empty, or no page state
// exists for the current folder.
func (m Model) WindowCounter() string {
	name := m.currentFolderName()
	if name == "" {
		return ""
	}
	page, ok := m.pages[name]
	if !ok || page.total <= 0 || page.loaded >= page.total {
		return ""
	}
	return fmt.Sprintf("%d/%d", page.loaded, page.total)
}

func (m Model) ViewerOpen() bool { return m.viewer.IsOpen() }

// MessageListCount returns the message-list row count. App reads it to
// decide whether a FolderLoadedMsg represents a fresh open (cursor
// reset) so it can commit any in-flight toast.
func (m Model) MessageListCount() int { return m.msglist.Count() }

// SelectedMessage returns the currently selected message in the
// message list, or false when the list is empty.
func (m Model) SelectedMessage() (mail.MessageInfo, bool) {
	return m.msglist.SelectedMessage()
}

// MsgList returns the message-list sub-model. Used by tests that need
// to peek at filter state, action targets, or selection.
func (m Model) MsgList() messagelist.Model { return m.msglist }

// SidebarColumnValue returns the sidebar column composite. Used by
// tests that inspect sidebar/search state.
func (m Model) SidebarColumnValue() sidebar.Column { return m.sidebarColumn }

// Viewer returns the reader sub-model. Used by tests that drive body
// state directly.
func (m Model) Viewer() reader.Model { return m.viewer }

// CurrentFolderName returns the canonical name of the currently-
// selected sidebar folder, or "" when nothing is selected.
func (m Model) CurrentFolderName() string { return m.currentFolderName() }

// WithViewer returns a copy of m with v as the viewer sub-model. Test
// seam. Production code mutates the viewer through Update / openMessage.
func (m Model) WithViewer(v reader.Model) Model {
	m.viewer = v
	return m
}

// WithMsgList returns a copy of m with l as the message-list sub-model.
// Test seam. Production mutates msglist through Update / RefreshSource.
func (m Model) WithMsgList(l messagelist.Model) Model {
	m.msglist = l
	return m
}

// SelectedFolderCounts returns the (exists, unseen) counts for the
// selected folder, or (0, 0) if no folder is selected. Mirrors the
// payload that FolderChangedMsg used to carry.
func (m Model) SelectedFolderCounts() (int, int) {
	folder, ok := m.sidebarColumn.Sidebar().SelectedFolderInfo()
	if !ok {
		return 0, 0
	}
	return folder.Exists, folder.Unseen
}

// ViewerScrollPct returns the viewer's scroll percentage, or 0 when
// the viewer is closed.
func (m Model) ViewerScrollPct() int {
	if !m.viewer.IsOpen() {
		return 0
	}
	return m.viewer.ScrollPct()
}

func (m Model) SearchState() sidebar.SearchState {
	return m.sidebarColumn.SidebarSearch().State()
}

// dispatchTriage performs an optimistic triage action through the
// cache. The cache QueueOp transactionally writes the optimistic flip
// and the outbox row. The immediate folder refresh re-reads the new
// state. Toast carries the inverse Cmd (a compensating QueueOp).
func (m *Model) dispatchTriage(op uicore.TriageOp) tea.Cmd {
	uids := m.msglist.ActionTargets()
	if len(uids) == 0 {
		return nil
	}
	src := m.currentFolderName()
	m.msglist.ExitVisual()

	switch op {
	case uicore.TriageDelete:
		trash, ok := m.sidebarColumn.Sidebar().FolderNameByCanonical("Trash")
		if !ok {
			return func() tea.Msg {
				return ErrorMsg{Op: string(op), Err: errors.New("no Trash folder configured")}
			}
		}
		return m.queueMove(op, src, trash, uids)

	case uicore.TriageArchive:
		archive, ok := m.sidebarColumn.Sidebar().FolderNameByCanonical("Archive")
		if !ok {
			return func() tea.Msg {
				return ErrorMsg{Op: string(op), Err: errors.New("no Archive folder configured")}
			}
		}
		return m.queueMove(op, src, archive, uids)

	case uicore.TriageStar:
		cursor, ok := m.msglist.SelectedMessage()
		if !ok {
			return nil
		}
		set := cursor.Flags&mail.FlagFlagged == 0
		toastOp := uicore.TriageStar
		if !set {
			toastOp = uicore.TriageUnstar
		}
		return m.queueFlag(toastOp, src, uids, mail.FlagFlagged, set)

	case uicore.TriageRead:
		cursor, ok := m.msglist.SelectedMessage()
		if !ok {
			return nil
		}
		set := cursor.Flags&mail.FlagSeen == 0
		toastOp := uicore.TriageRead
		if !set {
			toastOp = uicore.TriageUnread
		}
		return m.queueFlag(toastOp, src, uids, mail.FlagSeen, set)
	}
	return nil
}

// queueMove queues a move op for each uid from src to dest, then
// emits triageStartedMsg whose inverse undoes the move (queues
// a move from dest back to src for each uid).
func (m *Model) queueMove(op uicore.TriageOp, src, dest string, uids []mail.UID) tea.Cmd {
	label := string(op)
	fwd := queueOpsCmd(m.acct, label, src, uids, func(_ mail.UID) cache.OpArgs {
		return cache.MoveArgs{Dest: dest}
	})
	rev := queueOpsCmd(m.acct, label+" undo", dest, uids, func(_ mail.UID) cache.OpArgs {
		return cache.MoveArgs{Dest: src}
	})
	return startTriageCmd(op, dest, uids, fwd, rev)
}

// queueFlag queues a flag set/unset op for each uid, then emits the
// triage toast whose inverse flips the flag back.
func (m *Model) queueFlag(op uicore.TriageOp, src string, uids []mail.UID, flag mail.Flag, set bool) tea.Cmd {
	label := string(op)
	fwd := queueOpsCmd(m.acct, label, src, uids, func(_ mail.UID) cache.OpArgs {
		return cache.FlagArgs{Flag: flag, Set: set}
	})
	rev := queueOpsCmd(m.acct, label+" undo", src, uids, func(_ mail.UID) cache.OpArgs {
		return cache.FlagArgs{Flag: flag, Set: !set}
	})
	return startTriageCmd(op, "", uids, fwd, rev)
}

func (m *Model) dispatchMove() tea.Cmd {
	uids := m.msglist.ActionTargets()
	if len(uids) == 0 {
		return nil
	}
	src := m.currentFolderName()
	folders := m.sidebarColumn.Sidebar().OrderedFolders()
	return func() tea.Msg {
		return movepicker.OpenMsg{UIDs: uids, Src: src, Folders: folders}
	}
}

func (m *Model) dispatchMoveFromPicker(msg movepicker.PickedMsg) tea.Cmd {
	if len(msg.UIDs) == 0 {
		return nil
	}
	m.msglist.ExitVisual()
	return m.queueMove(uicore.TriageMove, msg.Src, msg.Dest, msg.UIDs)
}

// pageFor returns (creating if absent) the folderPage for name.
func (m *Model) pageFor(name string) *folderPage {
	if m.pages[name] == nil {
		m.pages[name] = &folderPage{}
	}
	return m.pages[name]
}

// maybeLoadMore issues a loadMoreCmd when the cursor is near the bottom
// and more messages are available. Returns nil when no action is needed.
func (m *Model) maybeLoadMore() tea.Cmd {
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
	return loadMoreCmd(m.acct, name, page.loaded)
}

// View renders the sidebar column + divider + right pane. SidebarColumn
// produces the left column content. Model owns the row-by-row join
// with the divider and right pane.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var rightLines []string
	switch {
	case m.viewer.IsOpen():
		rightLines = strings.Split(m.viewer.View(), "\n")
	case m.loading && m.msglist.Count() == 0:
		text := m.spinner.View() + " Loading messages…"
		mw := max(1, m.width-m.layout.Sidebar-1)
		rightLines = strings.Split(
			lipgloss.Place(mw, m.height, lipgloss.Center, lipgloss.Center,
				m.styles.Dim.Render(text)),
			"\n")
	default:
		rightLines = strings.Split(m.msglist.View(), "\n")
	}
	return m.assembleColumns(rightLines)
}

// RenderWithRightPane renders the sidebar with an externally-provided right
// pane string in place of the normal msglist/viewer content.
func (m Model) RenderWithRightPane(right string) string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	return m.assembleColumns(strings.Split(right, "\n"))
}

// assembleColumns uses row-by-row concatenation rather than lipgloss.JoinHorizontal;
// see the comment block inside for why that matters with SPUA-A glyphs.
func (m Model) assembleColumns(rightLines []string) string {
	sidebarLines := strings.Split(m.sidebarColumn.View(), "\n")
	divLine := m.styles.PanelDivider.Render("│")

	// Assemble columns row-by-row rather than via lipgloss.JoinHorizontal.
	// JoinHorizontal pads based on lipgloss.Width, which undercounts Nerd
	// Font SPUA-A glyphs by 1 cell each. The sidebar and msglist renderers
	// use displayCells (the correct terminal-cell count) via fillRowToWidth,
	// so each row is already exactly the right terminal width. Direct
	// concatenation preserves that property. JoinHorizontal would add
	// spurious padding to SPUA-A rows and widen them by 1–2 cells.
	n := min(len(sidebarLines), len(rightLines))
	assembled := make([]string, n)
	for i := range n {
		assembled[i] = sidebarLines[i] + divLine + rightLines[i]
	}
	return strings.Join(assembled, "\n")
}
