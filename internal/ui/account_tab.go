// SPDX-License-Identifier: MIT

package ui

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
)

// sidebarHeaderRows is the blank/account/blank padding reserved at
// the top of the sidebar before the folder list. AccountTab.View
// and the sidebar's own sizing both depend on this number matching.
const sidebarHeaderRows = 3

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
	icons  IconSet
	// acct is the per-account cache handle. UI reads come from
	// acct.QueryFolder; UI writes funnel through acct.QueueOp. The
	// underlying mail.Backend reference lives behind acct.Backend
	// (used for AccountName/AccountEmail accessors and body fetch).
	acct              *cache.Account
	uiCfg             config.UIConfig
	sidebarColumn     SidebarColumn
	msglist           MessageList
	viewer            Viewer
	keys              AccountKeys
	pages             map[string]*folderPage
	swept             map[string]bool
	loading           bool
	spinner           spinner.Model
	layout            LayoutMode
	width             int
	height            int
	// bodyFetchCancel cancels the in-flight loadBodyCmd goroutine.
	// Set on every openMessage call; cleared when the result arrives
	// (matched UID) or when the viewer closes.
	bodyFetchCancel context.CancelFunc
	// now returns the wall clock; test seam, defaults to time.Now.
	now func() time.Time
}

// WithNow returns a copy of m with the clock seam replaced.
func (m AccountTab) WithNow(now func() time.Time) AccountTab {
	m.now = now
	return m
}

// NewAccountTab builds an empty AccountTab. The initial folder list is
// fetched via Init's returned Cmd, not synchronously.
func NewAccountTab(styles Styles, t *theme.CompiledTheme, acct *cache.Account, uiCfg config.UIConfig, icons IconSet) AccountTab {
	return AccountTab{
		styles:        styles,
		icons:         icons,
		acct:          acct,
		uiCfg:         uiCfg,
		sidebarColumn: NewSidebarColumn(styles, icons,
			NewSidebar(styles, nil, uiCfg, 30, 1, icons),
			NewSidebarSearch(styles, 30, icons),
			acct.AccountEmail(),
		),
		msglist: NewMessageList(styles, nil, 1, 1, icons),
		viewer:  NewViewer(styles, t, acct.AccountEmail()),
		keys:    NewAccountKeys(),
		pages:   make(map[string]*folderPage),
		swept:   make(map[string]bool),
		spinner: NewSpinner(t),
		layout:  ComputeLayout(80),
		now:     time.Now,
	}
}

// Title returns the current folder name.
func (m AccountTab) Title() string { return m.sidebarColumn.Sidebar().SelectedFolder() }

// Backend returns the underlying mail.Backend so the App can drive
// the connection-state pump without duplicating the cache reference.
func (m AccountTab) Backend() mail.Backend { return m.acct.Backend }

// Icon returns the folder's Nerd Font icon.
func (m AccountTab) Icon() string { return m.sidebarColumn.Sidebar().SelectedIcon() }

// Closeable returns false — the account tab cannot be closed.
func (m AccountTab) Closeable() bool { return false }

// Init fires the initial folder-list fetch and starts the cache event
// pump.
func (m AccountTab) Init() tea.Cmd {
	return tea.Batch(loadFoldersCmd(m.acct), pumpCacheCmd(m.acct))
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
		layout := ComputeLayout(m.width)
		if layout.Sidebar > m.width/2 {
			layout.Sidebar = m.width / 2
		}
		m.layout = layout
		m.msglist.SetLayout(layout)

		sw := layout.Sidebar
		folderHeight := max(1, m.height-sidebarHeaderRows-searchShelfRows)
		// Re-build the SidebarColumn children with their new sizes, then
		// record the column dims. The verbose-explicit path: each child is
		// updated via its pointer-receiver method on a local copy, then
		// re-wrapped through With*, and SetSize records the column dims.
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

	case ClearSidebarSearchMsg:
		m = m.clearSearchIfActive()
		return m, nil

	case foldersLoadedMsg:
		sb := m.sidebarColumn.Sidebar()
		sb.SetFolders(msg.classified, m.uiCfg)
		m.sidebarColumn = m.sidebarColumn.WithSidebar(sb)
		return m.selectionChangedCmds()

	case folderLoadedMsg:
		m.loading = false
		page := m.pageFor(msg.name)
		// selectionChangedCmds zeroes the page before firing
		// openFolderCmd, so loaded == 0 reliably means "fresh open"
		// (cursor reset); any other value means "post-write refresh"
		// (cursor preserved). Snapshot before mutating page.loaded.
		isInitial := page.loaded == 0
		page.loaded = len(msg.msgs)
		page.total = msg.total
		fc := m.uiCfg.Folders[m.sidebarColumn.Sidebar().ConfigKey(msg.name)]
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
		if isInitial {
			m.msglist.SetMessages(msg.msgs)
		} else {
			m.msglist.RefreshSource(msg.msgs)
		}
		if sweep := m.maybeRetentionSweep(msg.name, msg.msgs); sweep != nil {
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

	case cacheEventMsg:
		// Drainer transition: re-read current folder so any backing
		// state change is reflected. Re-arm the pump.
		cmds := []tea.Cmd{pumpCacheCmd(m.acct)}
		if name := m.currentFolderName(); name != "" {
			cmds = append(cmds, refreshFolderCmd(m.acct, name))
		}
		return m, tea.Batch(cmds...)

	case bodyLoadedMsg:
		if m.viewer.CurrentUID() == msg.uid {
			m.bodyFetchCancel = nil
			m.viewer = m.viewer.SetBody(msg.blocks)
		}
		return m, nil

	case ErrorMsg:
		// App owns the banner; AccountTab ignores. App.Update runs
		// before delegation, so the App layer captures the message.
		return m, nil

	case MovePickerPickedMsg:
		return m, m.dispatchMoveFromPicker(msg)

	case sweepCompletedMsg:
		// Sweep fired; cache now has ui_hide=1 on each affected row.
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
			return triageStartedMsg{op: opEmpty, n: n, dest: folder}
		}
		return m, tea.Batch(toast, refreshFolderCmd(m.acct, src))

	case SearchUpdatedMsg:
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
		// block; the viewer's spinner ID guard rejects stale ticks.

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
		delta := 0
		switch {
		case key.Matches(msg, m.keys.NextMessage):
			delta = 1
		case key.Matches(msg, m.keys.PrevMessage):
			delta = -1
		}
		if delta != 0 {
			if m.viewer.Phase() != viewerReady {
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
			// Viewer just closed; discard any in-flight body fetch.
			m = m.cancelInflightBodyFetch()
		}
		return m, cmd
	}
	// Route to SidebarSearch when we're in Typing state — it owns
	// the input routing for this modal slice, except for Enter and
	// Esc which transition state.
	if m.sidebarColumn.SidebarSearch().State() == SearchTyping {
		ss := m.sidebarColumn.SidebarSearch()
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
		st := m.sidebarColumn.SidebarSearch().State()
		if st == SearchIdle || st == SearchActive {
			ss := m.sidebarColumn.SidebarSearch()
			ss.Activate()
			m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
			return m, nil
		}
	case key.Matches(msg, m.keys.ClearSearch):
		if m.sidebarColumn.SidebarSearch().State() == SearchActive {
			ss := m.sidebarColumn.SidebarSearch()
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
		if m.sidebarColumn.SidebarSearch().State() == SearchActive {
			return m, nil
		}
		m.msglist.ToggleFold()
	case key.Matches(msg, m.keys.ToggleFoldAll):
		if m.sidebarColumn.SidebarSearch().State() == SearchActive {
			return m, nil
		}
		m.msglist.ToggleFoldAll()
	case key.Matches(msg, m.keys.Delete):
		return m, m.dispatchTriage(opDelete)
	case key.Matches(msg, m.keys.Archive):
		return m, m.dispatchTriage(opArchive)
	case key.Matches(msg, m.keys.Star):
		return m, m.dispatchTriage(opStar)
	case key.Matches(msg, m.keys.ReadToggle):
		return m, m.dispatchTriage(opRead)
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
// with the given name. No-op (and no Cmd) when no folder matches —
// e.g. an account that doesn't expose a Drafts folder. Behaves like
// J/K otherwise: clears any active search, fires the load Cmd via
// selectionChangedCmds.
func (m AccountTab) jumpToFolder(canonical string) (AccountTab, tea.Cmd) {
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
func (m AccountTab) cancelInflightBodyFetch() AccountTab {
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
func (m AccountTab) openMessage(msg mail.MessageInfo) (AccountTab, tea.Cmd) {
	m = m.cancelInflightBodyFetch()
	ctx, cancel := context.WithCancel(context.Background())
	m.bodyFetchCancel = cancel
	m.viewer = m.viewer.Open(msg)
	cmds := []tea.Cmd{
		loadBodyCmd(ctx, m.acct, msg.UID),
		m.viewer.SpinnerTick(),
	}
	if msg.Flags&mail.FlagSeen == 0 {
		cmds = append(cmds, markReadCmd(m.acct, m.currentFolderName(), msg.UID))
	}
	return m, tea.Batch(cmds...)
}

// openSelectedMessage delegates to openMessage with the current
// msglist cursor. No-op when the folder is empty.
func (m AccountTab) openSelectedMessage() (AccountTab, tea.Cmd) {
	msg, ok := m.msglist.SelectedMessage()
	if !ok {
		return m, nil
	}
	return m.openMessage(msg)
}

// clearSearchIfActive clears the shelf and the filter if the shelf
// is in any non-Idle state. No-op when already idle.
func (m AccountTab) clearSearchIfActive() AccountTab {
	if m.sidebarColumn.SidebarSearch().State() == SearchIdle {
		return m
	}
	ss := m.sidebarColumn.SidebarSearch()
	ss.Clear()
	m.sidebarColumn = m.sidebarColumn.WithSidebarSearch(ss)
	m.msglist.ClearFilter()
	return m
}

// selectionChangedCmds runs every time the selected folder changes:
// resets the destination page, clears the msglist, sets loading=true,
// and returns the load Cmd batched with the spinner tick. Returns the
// updated AccountTab and the Cmd so mutations are visible at the call
// site. App reads folder counts via SelectedFolderCounts() after
// delegation rather than via a FolderChangedMsg signal.
func (m AccountTab) selectionChangedCmds() (AccountTab, tea.Cmd) {
	folder, ok := m.sidebarColumn.Sidebar().SelectedFolderInfo()
	if !ok {
		return m, nil
	}
	m.loading = true
	// Reset the page so folderLoadedMsg uses SetMessages (cursor reset)
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
func (m AccountTab) IsSwept(folder string) bool {
	return m.swept[folder]
}

// maybeRetentionSweep checks whether folderName is a Disposal folder
// with a positive retention threshold. If so, and if the sweep has not
// run yet this session, it marks the folder swept, collects expired
// UIDs, and returns a destroyCmd (which may be a no-op if no UIDs
// qualify). Returns nil when retention is disabled or the sweep already ran.
func (m *AccountTab) maybeRetentionSweep(folderName string, loaded []mail.MessageInfo) tea.Cmd {
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
// Trash or Spam. Returns nil for all other folders — inert by design.
func (m *AccountTab) dispatchEmpty() tea.Cmd {
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
	page := m.pages[folder.Name]
	total := 0
	if page != nil {
		total = page.total
	}
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
func (m AccountTab) currentFolderName() string {
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
func (m AccountTab) WindowCounter() string {
	name := m.currentFolderName()
	if name == "" {
		return ""
	}
	page, ok := m.pages[name]
	if !ok || page == nil || page.total <= 0 || page.loaded >= page.total {
		return ""
	}
	return fmt.Sprintf("%d/%d", page.loaded, page.total)
}

// ViewerOpen reports whether the viewer is currently open.
func (m AccountTab) ViewerOpen() bool { return m.viewer.IsOpen() }

// SelectedFolderCounts returns the (exists, unseen) counts for the
// selected folder, or (0, 0) if no folder is selected. Mirrors the
// payload that FolderChangedMsg used to carry.
func (m AccountTab) SelectedFolderCounts() (int, int) {
	folder, ok := m.sidebarColumn.Sidebar().SelectedFolderInfo()
	if !ok {
		return 0, 0
	}
	return folder.Exists, folder.Unseen
}

// ViewerScrollPct returns the viewer's scroll percentage, or 0 when
// the viewer is closed.
func (m AccountTab) ViewerScrollPct() int {
	if !m.viewer.IsOpen() {
		return 0
	}
	return m.viewer.ScrollPct()
}

// SearchState exposes the sidebar search state machine.
func (m AccountTab) SearchState() SearchState {
	return m.sidebarColumn.SidebarSearch().State()
}

// dispatchTriage performs an optimistic triage action through the
// cache. The cache QueueOp transactionally writes the optimistic flip
// and the outbox row; the immediate folder refresh re-reads the new
// state. Toast carries the inverse Cmd (a compensating QueueOp).
func (m *AccountTab) dispatchTriage(op triageOp) tea.Cmd {
	uids := m.msglist.ActionTargets()
	if len(uids) == 0 {
		return nil
	}
	src := m.currentFolderName()
	m.msglist.ExitVisual()

	switch op {
	case opDelete:
		trash, ok := m.sidebarColumn.Sidebar().FolderNameByCanonical("Trash")
		if !ok {
			return func() tea.Msg {
				return ErrorMsg{Op: string(op), Err: errors.New("no Trash folder configured")}
			}
		}
		return m.queueMove(op, src, trash, uids)

	case opArchive:
		archive, ok := m.sidebarColumn.Sidebar().FolderNameByCanonical("Archive")
		if !ok {
			return func() tea.Msg {
				return ErrorMsg{Op: string(op), Err: errors.New("no Archive folder configured")}
			}
		}
		return m.queueMove(op, src, archive, uids)

	case opStar:
		cursor, ok := m.msglist.SelectedMessage()
		if !ok {
			return nil
		}
		set := cursor.Flags&mail.FlagFlagged == 0
		toastOp := opStar
		if !set {
			toastOp = opUnstar
		}
		return m.queueFlag(toastOp, src, uids, mail.FlagFlagged, set)

	case opRead:
		cursor, ok := m.msglist.SelectedMessage()
		if !ok {
			return nil
		}
		set := cursor.Flags&mail.FlagSeen == 0
		toastOp := opRead
		if !set {
			toastOp = opUnread
		}
		return m.queueFlag(toastOp, src, uids, mail.FlagSeen, set)
	}
	return nil
}

// queueMove queues a move op for each uid from src to dest, then
// emits triageStartedMsg whose inverse undoes the move (queues
// a move from dest back to src for each uid).
func (m *AccountTab) queueMove(op triageOp, src, dest string, uids []mail.UID) tea.Cmd {
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
func (m *AccountTab) queueFlag(op triageOp, src string, uids []mail.UID, flag mail.Flag, set bool) tea.Cmd {
	label := string(op)
	fwd := queueOpsCmd(m.acct, label, src, uids, func(_ mail.UID) cache.OpArgs {
		return cache.FlagArgs{Flag: flag, Set: set}
	})
	rev := queueOpsCmd(m.acct, label+" undo", src, uids, func(_ mail.UID) cache.OpArgs {
		return cache.FlagArgs{Flag: flag, Set: !set}
	})
	return startTriageCmd(op, "", uids, fwd, rev)
}

func (m *AccountTab) dispatchMove() tea.Cmd {
	uids := m.msglist.ActionTargets()
	if len(uids) == 0 {
		return nil
	}
	src := m.currentFolderName()
	folders := m.sidebarColumn.Sidebar().OrderedFolders()
	return func() tea.Msg {
		return OpenMovePickerMsg{UIDs: uids, Src: src, Folders: folders}
	}
}

func (m *AccountTab) dispatchMoveFromPicker(msg MovePickerPickedMsg) tea.Cmd {
	if len(msg.UIDs) == 0 {
		return nil
	}
	m.msglist.ExitVisual()
	return m.queueMove(opMove, msg.Src, msg.Dest, msg.UIDs)
}

// startTriageCmd batches the forward queueOpsCmd with a triage-toast
// emitter so the chrome row appears in the same Update tick the cache
// flip lands.
func startTriageCmd(op triageOp, dest string, uids []mail.UID, fwd, rev tea.Cmd) tea.Cmd {
	start := func() tea.Msg {
		return triageStartedMsg{op: op, n: len(uids), uids: uids, dest: dest, inverse: rev}
	}
	return tea.Batch(start, fwd)
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
	return loadMoreCmd(m.acct, name, page.loaded)
}

// View renders the sidebar column + divider + right pane. SidebarColumn
// produces the left column content; AccountTab owns the row-by-row join
// with the divider and right pane.
func (m AccountTab) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarLines := strings.Split(m.sidebarColumn.View(), "\n")

	divLine := m.styles.PanelDivider.Render("│")

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

	// Assemble columns row-by-row rather than via lipgloss.JoinHorizontal.
	// JoinHorizontal pads based on lipgloss.Width, which undercounts Nerd
	// Font SPUA-A glyphs by 1 cell each. The sidebar and msglist renderers
	// use displayCells (the correct terminal-cell count) via fillRowToWidth,
	// so each row is already exactly the right terminal width. Direct
	// concatenation preserves that property; JoinHorizontal would add
	// spurious padding to SPUA-A rows and widen them by 1–2 cells.
	n := min(len(sidebarLines), len(rightLines))
	assembled := make([]string, n)
	for i := range n {
		assembled[i] = sidebarLines[i] + divLine + rightLines[i]
	}
	return strings.Join(assembled, "\n")
}
