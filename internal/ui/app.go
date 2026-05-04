// SPDX-License-Identifier: MIT

package ui

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// pendingEmptyConfirm carries the parameters App needs to emit
// EmptyFolderConfirmedMsg when the user accepts the active confirm
// modal. Zero value (empty folder) means no empty-folder confirm is
// pending.
type pendingEmptyConfirm struct {
	folder string
	source string
}

// App is the root bubbletea model for poplar.
type App struct {
	acct        AccountTab
	icons       IconSet
	styles     Styles
	topLine    TopLine
	statusBar  StatusBar
	footer     Footer
	keys       GlobalKeys
	viewerOpen bool
	helpOpen    bool
	help        HelpPopover
	linkPicker  LinkPicker
	attachPicker AttachPicker
	movePicker  MovePicker
	downloadDir string
	confirm          ConfirmModal
	pendingEmpty     pendingEmptyConfirm
	outbox           OutboxOverlay
	outboxOpen       bool
	conflict         ConflictOverlay
	conflictOpen     bool
	lastOutboxDepth  cache.OutboxDepth
	offlineHinted    bool
	lastErr          ErrorMsg
	toast       pendingAction
	undoSeconds int
	// now returns the wall clock; test seam, defaults to time.Now.
	now func() time.Time
	// opener launches URLs; test seam, defaults to xdgOpenURL.
	opener URLOpener
	width  int
	height int
}

// WithOpener returns a copy of m with the URL opener replaced.
func (m App) WithOpener(opener URLOpener) App {
	m.opener = opener
	return m
}

// NewApp creates the root model with a single AccountTab. Folder loading
// happens in Init's Cmd chain, not in the constructor.
func NewApp(t *theme.CompiledTheme, acct *cache.Account, uiCfg config.UIConfig, icons IconSet) App {
	styles := NewStyles(t)
	sb := NewStatusBar(styles)
	sb = sb.SetConnectionState(Offline)

	return App{
		acct:        NewAccountTab(styles, t, acct, uiCfg, icons),
		icons:       icons,
		styles:      styles,
		topLine:     NewTopLine(styles),
		statusBar:   sb,
		footer:      NewFooter(styles),
		keys:        NewGlobalKeys(),
		linkPicker:   NewLinkPicker(styles),
		attachPicker: NewAttachPicker(styles, icons),
		movePicker:   NewMovePicker(styles),
		downloadDir:  uiCfg.DownloadDir,
		confirm:     NewConfirmModal(styles),
		outbox:      NewOutboxOverlay(styles),
		conflict:    NewConflictOverlay(styles),
		undoSeconds: uiCfg.UndoSeconds,
		now:         time.Now,
		opener:      xdgOpenURL,
	}
}

// Init delegates to the account tab so the initial folder fetch fires,
// and starts the backend update pump.
func (m App) Init() tea.Cmd {
	return tea.Batch(m.acct.Init(), pumpUpdatesCmd(m.acct.Backend()))
}

// deriveChromeFromAcct re-reads AccountTab state and propagates it
// to App-owned chrome (footer, status bar, viewerOpen, linkPicker).
func (m App) deriveChromeFromAcct() App {
	prevViewer := m.viewerOpen
	m.viewerOpen = m.acct.ViewerOpen()
	exists, unseen := m.acct.SelectedFolderCounts()
	m.statusBar = m.statusBar.SetCounts(exists, unseen)
	if m.viewerOpen {
		if !prevViewer {
			m.footer = m.footer.SetContext(ViewerContext)
			m.statusBar = m.statusBar.SetMode(StatusViewer).SetScrollPct(0)
		} else {
			m.statusBar = m.statusBar.SetScrollPct(m.acct.ViewerScrollPct())
		}
	} else if prevViewer {
		m.footer = m.footer.SetContext(AccountContext)
		m.statusBar = m.statusBar.SetMode(StatusAccount)
	}
	m.footer = m.footer.SetCounter(m.acct.WindowCounter())
	return m
}

// Update handles global keys and delegates everything else to the
// account tab. Chrome (footer, status bar, link picker) is derived
// by reading AccountTab accessors after each delegation.
func (m App) Update(msg tea.Msg) (App, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		var cmds []tea.Cmd
		var cmd tea.Cmd
		m.linkPicker = m.linkPicker.SetSize(m.width, m.height)
		m.linkPicker, cmd = m.linkPicker.Update(msg)
		cmds = append(cmds, cmd)
		m.attachPicker = m.attachPicker.SetSize(m.width, m.height)
		m.attachPicker, cmd = m.attachPicker.Update(msg)
		cmds = append(cmds, cmd)
		m.movePicker = m.movePicker.SetSize(m.width, m.height)
		m.movePicker, cmd = m.movePicker.Update(msg)
		cmds = append(cmds, cmd)
		m.confirm = m.confirm.SetSize(m.width, m.height)
		m.confirm, cmd = m.confirm.Update(msg)
		cmds = append(cmds, cmd)
		m.outbox = m.outbox.SetSize(m.width, m.height)
		m.conflict = m.conflict.SetSize(m.width, m.height)
		m.help = m.help.SetSize(m.width, m.height)
		contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
		m.acct, cmd = m.acct.Update(contentMsg)
		cmds = append(cmds, cmd)
		// WindowSizeMsg only forwards sizing; chrome derivation is not
		// needed (sizing alone does not change viewer open/close state
		// or folder counts).
		return m, tea.Batch(cmds...)

	case OpenLinkPickerMsg:
		m.linkPicker = m.linkPicker.Open(msg.Links)
		return m, nil

	case LinkPickerClosedMsg:
		m.linkPicker = m.linkPicker.Close()
		return m, nil

	case OpenMovePickerMsg:
		m.movePicker = m.movePicker.Open(msg.UIDs, msg.Src, msg.Folders)
		return m, nil

	case MovePickerClosedMsg:
		m.movePicker = m.movePicker.Close()
		return m, nil

	case MovePickerPickedMsg:
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(msg)
		m = m.deriveChromeFromAcct()
		return m, cmd

	case OpenAttachPickerMsg:
		m.attachPicker = m.attachPicker.Open(msg.UID, msg.Items)
		return m, nil

	case AttachPickerClosedMsg:
		m.attachPicker = m.attachPicker.Close()
		return m, nil

	case OpenAttachmentMsg:
		return m, openAttachmentCmd(m.acct.Cache(), m.opener, msg.UID, msg.Att)

	case SaveAttachmentMsg:
		return m, saveAttachmentCmd(m.acct.Cache(), m.downloadDir, msg.UID, msg.Att)

	case attachmentSavedMsg:
		hadBanner := m.hasBannerRow()
		deadline := m.now().Add(time.Duration(m.undoSeconds) * time.Second)
		m.toast = pendingAction{
			op:       opSaveAttachment,
			dest:     msg.path,
			deadline: deadline,
		}
		cmds := []tea.Cmd{tea.Tick(time.Until(deadline), func(time.Time) tea.Msg {
			return toastExpireMsg{deadline: deadline}
		})}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...)

	case OpenConfirmEmptyMsg:
		body := strconv.Itoa(msg.Total) + " messages will be permanently deleted."
		m.pendingEmpty = pendingEmptyConfirm{folder: msg.Folder, source: msg.Source}
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Empty " + msg.Folder,
			Body:  body,
		})
		return m, nil

	case ConfirmModalYesMsg:
		if m.pendingEmpty.folder != "" {
			folder, source := m.pendingEmpty.folder, m.pendingEmpty.source
			m.pendingEmpty = pendingEmptyConfirm{}
			return m, func() tea.Msg {
				return EmptyFolderConfirmedMsg{Folder: folder, Source: source}
			}
		}
		return m, nil

	case ConfirmModalClosedMsg:
		m.pendingEmpty = pendingEmptyConfirm{}
		m.confirm = m.confirm.Close()
		return m, nil

	case EmptyFolderConfirmedMsg:
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(msg)
		m = m.deriveChromeFromAcct()
		return m, cmd

	case LaunchURLMsg:
		return m, launchURLCmd(m.opener, msg.URL)

	case triageStartedMsg:
		hadBanner := m.hasBannerRow()
		deadline := m.now().Add(time.Duration(m.undoSeconds) * time.Second)
		m.toast = pendingAction{
			op:       msg.op,
			n:        msg.n,
			dest:     msg.dest,
			inverse:  msg.inverse,
			deadline: deadline,
		}
		cmds := []tea.Cmd{tea.Tick(time.Until(deadline), func(time.Time) tea.Msg {
			return toastExpireMsg{deadline: deadline}
		})}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...)

	case toastExpireMsg:
		if m.toast.IsZero() || !msg.deadline.Equal(m.toast.deadline) {
			return m, nil
		}
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		m, rcmd := m.maybeResizeChild(hadBanner)
		return m, rcmd

	case undoRequestedMsg:
		if m.toast.IsZero() {
			return m, nil
		}
		cmd := m.toast.inverse
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		cmds := []tea.Cmd{}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...)

	case ErrorMsg:
		// Errors clear any pending toast; the cache holds the
		// optimistic flip and the user must fire u to revert.
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		m.lastErr = msg
		cmds := make([]tea.Cmd, 0, 2)
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		acct, fcmd := m.acct.Update(msg)
		m.acct = acct
		m = m.deriveChromeFromAcct()
		cmds = append(cmds, fcmd)
		return m, tea.Batch(cmds...)

	case folderLoadedMsg:
		// A fresh folder load (msglist reset by selectionChangedCmds)
		// commits any in-flight toast.
		if !m.toast.IsZero() && m.acct.msglist.Count() == 0 {
			hadBanner := m.hasBannerRow()
			m.toast = pendingAction{}
			var rcmd tea.Cmd
			m, rcmd = m.maybeResizeChild(hadBanner)
			acct, fcmd := m.acct.Update(msg)
			m.acct = acct
			m = m.deriveChromeFromAcct()
			cmds := []tea.Cmd{fcmd}
			if rcmd != nil {
				cmds = append(cmds, rcmd)
			}
			return m, tea.Batch(cmds...)
		}

	case backendUpdateMsg:
		cmds := []tea.Cmd{pumpUpdatesCmd(m.acct.Backend())} // re-arm pump
		if msg.update.Type == mail.UpdateConnState {
			var cs ConnectionState
			switch msg.update.ConnState {
			case mail.ConnConnected:
				cs = Connected
				m.offlineHinted = false
			case mail.ConnReconnecting:
				cs = Reconnecting
			default:
				cs = Offline
				if !m.offlineHinted {
					d := m.lastOutboxDepth
					if d.Pending+d.Executing+d.Failed+d.Conflict > 0 {
						m.lastErr = ErrorMsg{
							Op:  "connection",
							Err: errors.New("offline — queued ops will sync on reconnect"),
						}
						m.offlineHinted = true
					}
				}
			}
			m.statusBar = m.statusBar.SetConnectionState(cs)
		}
		// Other Update types (UpdateNewMail, UpdateFlagsChanged, etc.)
		// delegate to AccountTab in a later pass.
		return m, tea.Batch(cmds...)

	case cacheEventMsg:
		cmds := []tea.Cmd{refreshOutboxDepthCmd(m.acct.Cache())}
		if m.outboxOpen {
			cmds = append(cmds, loadOutboxSummaryCmd(m.acct.Cache()))
		}
		if m.conflictOpen {
			cmds = append(cmds, loadOutboxConflictsCmd(m.acct.Cache()))
		}
		acct, fcmd := m.acct.Update(msg)
		m.acct = acct
		cmds = append(cmds, fcmd)
		return m, tea.Batch(cmds...)

	case OpenConflictsFromOutboxMsg:
		m.outboxOpen = false
		m.conflictOpen = true
		m.conflict = m.conflict.Open(nil)
		return m, loadOutboxConflictsCmd(m.acct.Cache())

	case outboxDepthMsg:
		m.lastOutboxDepth = msg.depth
		inflight := msg.depth.Pending + msg.depth.Executing + msg.depth.Failed
		m.statusBar = m.statusBar.SetOutboxDepth(inflight, msg.depth.Conflict)
		return m, nil

	case outboxSummaryMsg:
		if msg.err != nil {
			m.lastErr = ErrorMsg{Op: "outbox summary", Err: msg.err}
			return m, nil
		}
		if m.outboxOpen {
			m.outbox = m.outbox.SetGroups(msg.groups)
		}
		return m, nil

	case outboxConflictsMsg:
		if msg.err != nil {
			m.lastErr = ErrorMsg{Op: "outbox conflicts", Err: msg.err}
			return m, nil
		}
		if m.conflictOpen {
			m.conflict = m.conflict.SetRows(msg.rows)
			if len(msg.rows) == 0 {
				m.conflict = m.conflict.Close()
				m.conflictOpen = false
			}
		}
		return m, nil

	case RetryConflictMsg:
		return m, retryConflictCmd(m.acct.Cache(), msg.OpID)

	case DiscardConflictMsg:
		return m, discardConflictCmd(m.acct.Cache(), msg.OpID)

	case conflictResolvedMsg:
		if msg.err != nil && !errors.Is(msg.err, cache.ErrNotConflict) {
			m.lastErr = ErrorMsg{Op: "resolve conflict", Err: msg.err}
		}
		return m, tea.Batch(loadOutboxConflictsCmd(m.acct.Cache()), refreshOutboxDepthCmd(m.acct.Cache()))

	case tea.KeyMsg:
		if m.helpOpen {
			if key.Matches(msg, m.keys.CloseHelp) {
				m.helpOpen = false
			}
			return m, nil
		}
		if m.confirm.IsOpen() {
			var cmd tea.Cmd
			m.confirm, cmd = m.confirm.Update(msg)
			return m, cmd
		}
		if m.conflictOpen {
			var cmd tea.Cmd
			m.conflict, cmd = m.conflict.Update(msg)
			if !m.conflict.IsOpen() {
				m.conflictOpen = false
			}
			return m, cmd
		}
		if m.outboxOpen {
			var cmd tea.Cmd
			m.outbox, cmd = m.outbox.Update(msg)
			if !m.outbox.IsOpen() {
				m.outboxOpen = false
			}
			return m, cmd
		}
		if m.linkPicker.IsOpen() {
			var cmd tea.Cmd
			m.linkPicker, cmd = m.linkPicker.Update(msg)
			return m, cmd
		}
		if m.attachPicker.IsOpen() {
			var cmd tea.Cmd
			m.attachPicker, cmd = m.attachPicker.Update(msg)
			return m, cmd
		}
		if m.movePicker.IsOpen() {
			var cmd tea.Cmd
			m.movePicker, cmd = m.movePicker.Update(msg)
			return m, cmd
		}
		switch {
		case key.Matches(msg, m.keys.Undo):
			// Undo is only live while a toast is active; otherwise the
			// 'u' key falls through to AccountTab so other meanings can
			// take over later.
			if !m.toast.IsZero() {
				return m, func() tea.Msg { return undoRequestedMsg{} }
			}
		case key.Matches(msg, m.keys.Quit):
			if m.viewerOpen {
				// Viewer-open: q closes the viewer, not the app.
				// Delegate so AccountTab routes to viewer.handleKey.
				var cmd tea.Cmd
				m.acct, cmd = m.acct.Update(msg)
				m = m.deriveChromeFromAcct()
				return m, cmd
			}
			if m.acct.SearchState() != SearchIdle {
				// Steal q while search is active so it doesn't quit
				// the app mid-search. Send a typed clear msg to
				// AccountTab.
				var cmd tea.Cmd
				m.acct, cmd = m.acct.Update(ClearSidebarSearchMsg{})
				m = m.deriveChromeFromAcct()
				return m, cmd
			}
			return m, tea.Quit
		case key.Matches(msg, m.keys.ForceQuit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.OutboxOverlay):
			m.outboxOpen = true
			m.outbox = m.outbox.Open(nil)
			return m, loadOutboxSummaryCmd(m.acct.Cache())
		case key.Matches(msg, m.keys.ConflictOverlay):
			m.conflictOpen = true
			m.conflict = m.conflict.Open(nil)
			return m, loadOutboxConflictsCmd(m.acct.Cache())
		case key.Matches(msg, m.keys.Help):
			m.helpOpen = true
			ctx := HelpAccount
			if m.viewerOpen {
				ctx = HelpViewer
			}
			m.help = NewHelpPopover(m.styles, ctx).SetSize(m.width, m.height)
			return m, nil
		}
	}

	// Delegate everything else to the account tab.
	var cmd tea.Cmd
	m.acct, cmd = m.acct.Update(msg)
	m = m.deriveChromeFromAcct()
	return m, cmd
}

// renderFrame builds the full-screen account layout string. It is extracted
// from View so it can be dimmed and composited under the help popover.
func (m App) renderFrame() string {
	rawContent := m.acct.View()
	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(rawContent, "\n")
	// AccountTab.View honors its width contract: every line is exactly
	// m.width-1 display cells. Append the right border directly without
	// per-line measure-and-pad — see TestAccountTabView_HonorsAssignedWidth.
	for i := range contentLines {
		contentLines[i] = contentLines[i] + rightBorder
	}
	content := strings.Join(contentLines, "\n")

	dividerCol := ComputeLayout(m.width).Sidebar
	topLine := m.topLine.View(m.width, dividerCol)
	status := m.statusBar.View(m.width, dividerCol)
	foot := m.footer.View(m.width)

	parts := []string{topLine, content}
	// Precedence: error banner wins; otherwise toast; otherwise the
	// chrome row collapses entirely.
	if bannerRow := m.chromeBannerRow(m.width); bannerRow != "" {
		parts = append(parts, bannerRow)
	}
	parts = append(parts, status, foot)
	// Use strings.Join rather than lipgloss.JoinVertical. JoinVertical pads
	// all rows to the widest row using lipgloss.Width, which undercounts
	// SPUA-A Nerd Font glyphs by 1 cell each. Content rows already have the
	// correct terminal width (guaranteed by AccountTab's width contract);
	// JoinVertical would add spurious 1-cell padding to any row with SPUA-A
	// content, causing those rows to land 1 cell outside the terminal width.
	return strings.Join(parts, "\n")
}

// View composes the full-screen layout. When the help popover is open the
// underlying account frame is rendered, dimmed via DimANSI, and then the
// popover box is composited over it via PlaceOverlay so the underlying
// context remains visible but recedes visually.
func (m App) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	frame := m.renderFrame()

	if m.helpOpen {
		box, tooNarrow := m.help.Box(m.width, m.height)
		dimmed := DimANSI(frame)
		if tooNarrow != "" {
			// Terminal too narrow for the full popover: show the notice
			// centered over the dimmed frame.
			x, y := (m.width-lipgloss.Width(tooNarrow))/2, m.height/2
			if x < 0 {
				x = 0
			}
			return PlaceOverlay(x, y, tooNarrow, dimmed)
		}
		x, y := m.help.Position(box, m.width, m.height)
		return PlaceOverlay(x, y, box, dimmed)
	}

	if m.confirm.IsOpen() {
		box := m.confirm.Box(m.width, m.height)
		x, y := m.confirm.Position(box, m.width, m.height)
		dimmed := DimANSI(frame)
		return PlaceOverlay(x, y, box, dimmed)
	}

	if m.conflictOpen {
		body := m.conflict.View()
		x, y := centerOverlay(body, m.width, m.height)
		dimmed := DimANSI(frame)
		return PlaceOverlay(x, y, body, dimmed)
	}

	if m.outboxOpen {
		body := m.outbox.View()
		x, y := centerOverlay(body, m.width, m.height)
		dimmed := DimANSI(frame)
		return PlaceOverlay(x, y, body, dimmed)
	}

	if m.linkPicker.IsOpen() {
		box := m.linkPicker.Box(m.width, m.height)
		x, y := m.linkPicker.Position(box, m.width, m.height)
		dimmed := DimANSI(frame)
		return PlaceOverlay(x, y, box, dimmed)
	}

	if m.attachPicker.IsOpen() {
		box := m.attachPicker.Box(m.width, m.height)
		x, y := m.attachPicker.Position(box, m.width, m.height)
		dimmed := DimANSI(frame)
		return PlaceOverlay(x, y, box, dimmed)
	}

	if m.movePicker.IsOpen() {
		box := m.movePicker.Box(m.width, m.height)
		x, y := m.movePicker.Position(box, m.width, m.height)
		dimmed := DimANSI(frame)
		return PlaceOverlay(x, y, box, dimmed)
	}

	return frame
}

// IsLinkPickerOpen reports whether the link picker overlay is visible.
func (m App) IsLinkPickerOpen() bool { return m.linkPicker.IsOpen() }

// IsConfirmOpen reports whether the confirm modal overlay is visible.
func (m App) IsConfirmOpen() bool { return m.confirm.IsOpen() }

// contentHeight returns the height available for the content area.
// The chrome banner row (error banner or toast) takes one extra row
// when either is present; the row collapses when both are absent.
func (m App) contentHeight() int {
	chrome := 3 // top line + status bar + footer
	if m.lastErr.Err != nil || !m.toast.IsZero() {
		chrome++
	}
	h := m.height - chrome
	if h < 1 {
		return 1
	}
	return h
}

// hasBannerRow reports whether the chrome row above the status bar is
// occupied (either by the error banner or by an active toast).
func (m App) hasBannerRow() bool {
	return m.lastErr.Err != nil || !m.toast.IsZero()
}

// maybeResizeChild re-forwards a WindowSizeMsg to the child when the
// chrome banner row's occupancy has changed since hadBanner was
// captured. Returns the (possibly-updated) App and the resize Cmd, or
// the input App and nil when no resize is needed.
func (m App) maybeResizeChild(hadBanner bool) (App, tea.Cmd) {
	if hadBanner == m.hasBannerRow() || m.width <= 0 || m.height <= 0 {
		return m, nil
	}
	contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
	acct, cmd := m.acct.Update(contentMsg)
	m.acct = acct
	return m, cmd
}

// chromeBannerRow renders the single chrome row above the status bar.
// Error banner wins precedence; otherwise the toast renders; otherwise
// the empty string collapses the row.
func (m App) chromeBannerRow(width int) string {
	if banner := renderErrorBanner(m.lastErr, width, m.styles); banner != "" {
		return banner
	}
	if !m.toast.IsZero() {
		return renderToast(m.toast, width, m.styles)
	}
	return ""
}
