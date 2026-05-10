package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

type folderPage struct {
	loaded           int
	total            int
	loadMoreInFlight bool
}

// Model is the account view. Pine-shaped single pane: every key is always
// live, J/K/G navigate folders, j/k navigate messages.
type Model struct {
	styles        Styles
	icons         uicore.IconSet
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
	// Cancels the in-flight loadBodyCmd goroutine. Set on every
	// openMessage. Cleared when the result arrives or the viewer closes.
	bodyFetchCancel context.CancelFunc
	now             func() time.Time // test seam, defaults to time.Now
}

// WithNow returns a copy of m with the clock seam replaced.
func (m Model) WithNow(now func() time.Time) Model {
	m.now = now
	return m
}

// New builds an empty account Model. Init's returned Cmd fetches the
// initial folder list asynchronously.
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

func (m Model) NotifyActivity() {
	m.acct.NotifyActivity()
}

func (m Model) NotifyConnState(online bool) {
	m.acct.NotifyConnState(online)
}

func (m Model) AccountEmail() string { return m.acct.AccountEmail() }

func (m Model) Icon() string { return m.sidebarColumn.Sidebar().SelectedIcon() }

func (m Model) Closeable() bool { return false }

// Init fires the initial folder-list fetch and starts the cache event pump.
func (m Model) Init() tea.Cmd {
	return tea.Batch(loadFoldersCmd(m.acct), pumpCacheCmd(m.acct))
}

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
		// Forward to SidebarSearch so its embedded textinput reflows.
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
		// Forward to the viewer so its embedded sub-models reflow.
		m.viewer, c = m.viewer.Update(msg)
		cmds = append(cmds, c)
		return m, tea.Batch(cmds...)

	case JumpFolderMsg:
		return m.jumpToFolder(msg.Canonical)

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
		// selectionChangedCmds zeroes the page before firing openFolderCmd,
		// so loaded == 0 reliably means "fresh open" (cursor reset). Any
		// other value means "post-write refresh" (cursor preserved).
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
		// Drainer transition: re-read the current folder and re-arm the
		// pump.
		cmds := []tea.Cmd{pumpCacheCmd(m.acct)}
		if name := m.currentFolderName(); name != "" {
			cmds = append(cmds, refreshFolderCmd(m.acct, name))
		}
		return m, tea.Batch(cmds...)

	case reader.BodyLoadedMsg:
		if m.viewer.CurrentUID() == msg.UID {
			m.bodyFetchCancel = nil
			m.viewer = m.viewer.SetBody(msg.Blocks, msg.Unsub, msg.Invite)
		}
		return m, nil

	case reader.AttachmentsLoadedMsg:
		if m.viewer.CurrentUID() == msg.UID {
			m.viewer = m.viewer.SetAttachments(msg.Items)
		}
		return m, nil

	case ErrorMsg:
		// App owns the banner; its Update runs before delegation.
		return m, nil

	case movepicker.PickedMsg:
		return m, m.dispatchMoveFromPicker(msg)

	case sweepCompletedMsg:
		// ui_hide is now set on each affected row; refresh so it shows.
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
		if msg.Scope == uicore.ScopeAll {
			return m, runSearchCmd(m.acct, msg.Query)
		}
		m.msglist.SetFilter(msg.Query)
		ss := m.sidebarColumn.SidebarSearch()
		ss.SetResultCount(m.msglist.FilterResultCount())
		m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
		return m, nil

	case searchResultsMsg:
		msgs := make([]mail.MessageInfo, 0, len(msg.Hits))
		origins := make(map[mail.UID]string, len(msg.Hits))
		for _, h := range msg.Hits {
			msgs = append(msgs, h.MessageInfo)
			origins[h.UID] = h.Folder
		}
		m.msglist.SetSearchResults(msgs, origins)
		ss := m.sidebarColumn.SidebarSearch()
		ss.SetResultCount(len(msg.Hits))
		m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
		return m, nil

	case spinner.TickMsg:
		if m.loading && msg.ID == m.spinner.ID() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		// Tick belongs to the viewer's spinner or is a stale generation;
		// the viewer's ID guard rejects stale ticks. Fall through.

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	// Forward unhandled msgs to the viewer when open so its sub-models
	// keep advancing.
	if m.viewer.IsOpen() {
		var cmd tea.Cmd
		m.viewer, cmd = m.viewer.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleKey dispatches navigation keys by identity. While the viewer is
// open every key routes there first. Otherwise J/K/G move the sidebar (and
// fire a folder-load Cmd), j/k move the message list. During active
// search, printable keys flow through SidebarSearch instead.
func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
			m = m.cancelInflightBodyFetch()
		}
		return m, cmd
	}
	// In Typing state SidebarSearch owns the input routing; Enter and Esc
	// transition out and stay with the account-view handler.
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
		// In visual-select mode Space toggles the cursor row's mark
		// instead of folding the thread; fold behavior is inert during
		// an active search.
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

// jumpToFolder moves the sidebar selection to the named canonical folder.
// Inert (no Cmd) when the account has no such folder. Otherwise behaves
// like J/K: clears any active search and fires the load Cmd.
func (m Model) jumpToFolder(canonical string) (Model, tea.Cmd) {
	sb := m.sidebarColumn.Sidebar()
	if !sb.SelectByCanonical(canonical) {
		return m, nil
	}
	m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
	m = m.clearSearchIfActive()
	return m.selectionChangedCmds()
}

// cancelInflightBodyFetch cancels any in-flight loadBodyCmd.
func (m Model) cancelInflightBodyFetch() Model {
	if m.bodyFetchCancel != nil {
		m.bodyFetchCancel()
		m.bodyFetchCancel = nil
	}
	return m
}

// openMessage opens msg in the viewer, fires the body-fetch Cmd, and
// queues a FlagSeen=true op for unread messages. Shared by Enter, n, and
// N. Cancels any prior in-flight body fetch before issuing the new one.
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

func (m Model) openSelectedMessage() (Model, tea.Cmd) {
	msg, ok := m.msglist.SelectedMessage()
	if !ok {
		return m, nil
	}
	return m.openMessage(msg)
}

// clearSearchIfActive clears the shelf and the filter when the shelf is
// in any non-Idle state.
func (m Model) clearSearchIfActive() Model {
	ss := m.sidebarColumn.SidebarSearch()
	if ss.State() == sidebar.SearchIdle {
		return m
	}
	ss.Clear()
	m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
	if m.msglist.ResultsMode() {
		m.msglist.ClearSearchResults()
	}
	m.msglist.ClearFilter()
	return m
}

// Returns m alongside the Cmd so the page reset is visible at the call site.
func (m Model) selectionChangedCmds() (Model, tea.Cmd) {
	folder, ok := m.sidebarColumn.Sidebar().SelectedFolderInfo()
	if !ok {
		return m, nil
	}
	m.loading = true
	// Reset the page so FolderLoadedMsg picks SetMessages (cursor reset)
	// over RefreshSource (cursor preserved).
	m.pages[folder.Name] = &folderPage{}
	m.msglist.SetMessages(nil)
	return m, tea.Batch(
		openFolderCmd(m.acct, folder.Name),
		m.spinner.Tick,
	)
}

// IsSwept reports whether the retention sweep has already fired for the
// named folder this session.
func (m Model) IsSwept(folder string) bool {
	return m.swept[folder]
}

// maybeRetentionSweep returns a destroyCmd for expired UIDs in folderName
// when retention is configured for that Disposal folder and the sweep has
// not run yet this session. Returns nil otherwise.
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
// Trash or Spam, and is inert otherwise.
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

// currentFolderName returns the canonical name of the selected sidebar
// folder, or "" when nothing is selected.
func (m Model) currentFolderName() string {
	folder, ok := m.sidebarColumn.Sidebar().SelectedFolderInfo()
	if !ok {
		return ""
	}
	return folder.Name
}

// WindowCounter returns "loaded/total" when the current folder has more
// messages available than loaded (e.g. "500/2347"), or "" when fully
// loaded, empty, or unselected.
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
// decide whether a FolderLoadedMsg represents a fresh open so it can
// commit any in-flight toast.
func (m Model) MessageListCount() int { return m.msglist.Count() }

func (m Model) SelectedMessage() (mail.MessageInfo, bool) {
	return m.msglist.SelectedMessage()
}

func (m Model) MsgList() messagelist.Model         { return m.msglist }
func (m Model) SidebarColumnValue() sidebar.Column { return m.sidebarColumn }
func (m Model) Viewer() reader.Model               { return m.viewer }
func (m Model) CurrentFolderName() string          { return m.currentFolderName() }

// WithViewer returns a copy of m with v as the viewer sub-model.
// Test seam; production mutates the viewer through Update / openMessage.
func (m Model) WithViewer(v reader.Model) Model {
	m.viewer = v
	return m
}

// WithMsgList returns a copy of m with l as the message-list sub-model.
// Test seam; production mutates msglist through Update / RefreshSource.
func (m Model) WithMsgList(l messagelist.Model) Model {
	m.msglist = l
	return m
}

// SelectedFolderCounts returns (exists, unseen) for the selected folder,
// or (0, 0) when nothing is selected.
func (m Model) SelectedFolderCounts() (int, int) {
	folder, ok := m.sidebarColumn.Sidebar().SelectedFolderInfo()
	if !ok {
		return 0, 0
	}
	return folder.Exists, folder.Unseen
}

// ViewerScrollPct returns the viewer's scroll percentage (0 when closed).
func (m Model) ViewerScrollPct() int {
	if !m.viewer.IsOpen() {
		return 0
	}
	return m.viewer.ScrollPct()
}

func (m Model) SearchState() sidebar.SearchState {
	return m.sidebarColumn.SidebarSearch().State()
}

// SetOutboxCount propagates n into the sidebar's synthetic Outbox entry.
func (m *Model) SetOutboxCount(n int) {
	sb := m.sidebarColumn.Sidebar()
	sb.SetOutboxCount(n)
	m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
}

// dispatchTriage runs an optimistic triage action through the cache.
// QueueOp writes the optimistic flip and the outbox row in one
// transaction; the immediate folder refresh re-reads the new state. The
// toast carries the inverse Cmd (a compensating QueueOp).
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

// queueMove queues one move op per uid from src to dest. The triage
// toast's inverse queues a move back from dest to src.
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

// queueFlag queues one flag set/unset op per uid. The triage toast's
// inverse flips the flag back.
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

// pageFor returns the folderPage for name, creating it on first access.
func (m *Model) pageFor(name string) *folderPage {
	if m.pages[name] == nil {
		m.pages[name] = &folderPage{}
	}
	return m.pages[name]
}

// maybeLoadMore returns a loadMoreCmd when the cursor is near the bottom
// and more messages are available, otherwise nil.
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

// View renders sidebar column + divider + right pane. SidebarColumn
// produces the left column; Model owns the row-by-row join.
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

// RenderWithRightPane renders the sidebar with an externally-provided
// right-pane string in place of msglist/viewer content. ComposeTab uses
// this so its body replaces the right pane while the chrome stays drawn.
func (m Model) RenderWithRightPane(right string) string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	return m.assembleColumns(strings.Split(right, "\n"))
}

// assembleColumns joins columns row-by-row instead of via
// lipgloss.JoinHorizontal. JoinHorizontal pads on lipgloss.Width, which
// undercounts SPUA-A glyphs by one cell each; sidebar and msglist already
// pad rows to exact terminal width via FillRowToWidth, so direct
// concatenation preserves correctness. ADR-0084.
func (m Model) assembleColumns(rightLines []string) string {
	sidebarLines := strings.Split(m.sidebarColumn.View(), "\n")
	divLine := m.styles.PanelDivider.Render("│")

	n := min(len(sidebarLines), len(rightLines))
	assembled := make([]string, n)
	for i := range n {
		assembled[i] = sidebarLines[i] + divLine + rightLines[i]
	}
	return strings.Join(assembled, "\n")
}
