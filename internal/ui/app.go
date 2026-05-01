// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// App is the root bubbletea model for poplar.
type App struct {
	acct       AccountTab
	backend    mail.Backend
	icons      IconSet
	styles     Styles
	topLine    TopLine
	statusBar  StatusBar
	footer     Footer
	keys       GlobalKeys
	viewerOpen bool
	helpOpen   bool
	help        HelpPopover
	linkPicker  LinkPicker
	movePicker  MovePicker
	lastErr     ErrorMsg
	toast       pendingAction
	undoSeconds int
	// now returns the wall clock; test seam, defaults to time.Now.
	now    func() time.Time
	width  int
	height int
}

// NewApp creates the root model with a single AccountTab. Folder loading
// happens in Init's Cmd chain, not in the constructor.
func NewApp(t *theme.CompiledTheme, backend mail.Backend, uiCfg config.UIConfig, icons IconSet) App {
	styles := NewStyles(t)
	sb := NewStatusBar(styles)
	sb = sb.SetConnectionState(Offline)

	return App{
		acct:        NewAccountTab(styles, t, backend, uiCfg, icons),
		backend:     backend,
		icons:       icons,
		styles:      styles,
		topLine:     NewTopLine(styles),
		statusBar:   sb,
		footer:      NewFooter(styles),
		keys:        NewGlobalKeys(),
		linkPicker:  NewLinkPicker(styles),
		movePicker:  NewMovePicker(styles),
		undoSeconds: uiCfg.UndoSeconds,
		now:         time.Now,
	}
}

// Init delegates to the account tab so the initial folder fetch fires,
// and starts the backend update pump.
func (m App) Init() tea.Cmd {
	return tea.Batch(m.acct.Init(), pumpUpdatesCmd(m.backend))
}

// deriveChromeFromAcct re-reads AccountTab state and propagates it
// to App-owned chrome (footer, status bar, viewerOpen, linkPicker).
// Called after every delegation that may have changed child state.
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
	if links, ok := (&m.acct).LinkPickerRequest(); ok {
		m.linkPicker = m.linkPicker.Open(links)
	}
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
		m.linkPicker = m.linkPicker.SetSize(m.width, m.height)
		m.movePicker = m.movePicker.SetSize(m.width, m.height)
		contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(contentMsg)
		// WindowSizeMsg only forwards sizing; chrome derivation is not
		// needed (sizing alone does not change viewer open/close state
		// or folder counts).
		return m, cmd

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

	case LaunchURLMsg:
		return m, launchURLCmd(msg.URL)

	case triageStartedMsg:
		hadBanner := m.hasBannerRow()
		deadline := m.now().Add(time.Duration(m.undoSeconds) * time.Second)
		m.toast = pendingAction{
			op:       msg.op,
			n:        msg.n,
			dest:     msg.dest,
			inverse:  msg.inverse,
			onUndo:   msg.onUndo,
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
		if m.toast.onUndo != nil {
			m.toast.onUndo()
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
		// Banner state is App-owned. The chrome banner row (error or
		// toast) takes one row; transitions in or out of having a row
		// resize the child. An ErrorMsg also rolls back any in-flight
		// toast: the local flip is reversed via onUndo, the toast
		// clears, and the error replaces it.
		hadBanner := m.hasBannerRow()
		if !m.toast.IsZero() && m.toast.onUndo != nil {
			m.toast.onUndo()
		}
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

	case folderQueryDoneMsg:
		// A folder change commits any in-flight toast: the optimistic
		// flip stands, no inverse fires. The pending state simply
		// clears so the chrome row collapses. The msg still flows
		// through to AccountTab below for normal load handling.
		if msg.reset && !m.toast.IsZero() {
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
		cmds := []tea.Cmd{pumpUpdatesCmd(m.backend)} // re-arm pump
		if msg.update.Type == mail.UpdateConnState {
			m.statusBar = m.statusBar.SetConnectionState(translateConnState(msg.update.ConnState))
		}
		// Other Update types (UpdateNewMail, UpdateFlagsChanged, etc.)
		// delegate to AccountTab in a later pass.
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if m.helpOpen {
			if key.Matches(msg, m.keys.CloseHelp) {
				m.helpOpen = false
			}
			return m, nil
		}
		if m.linkPicker.IsOpen() {
			var cmd tea.Cmd
			m.linkPicker, cmd = m.linkPicker.Update(msg)
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
				// the app mid-search. Delegate to AccountTab which
				// clears the filter.
				var cmd tea.Cmd
				m.acct, cmd = m.acct.Update(tea.KeyMsg{Type: tea.KeyEsc, Runes: []rune{}})
				m = m.deriveChromeFromAcct()
				return m, cmd
			}
			return m, tea.Quit
		case key.Matches(msg, m.keys.ForceQuit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.helpOpen = true
			ctx := HelpAccount
			if m.viewerOpen {
				ctx = HelpViewer
			}
			m.help = NewHelpPopover(m.styles, ctx)
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

	dividerCol := sidebarWidth
	topLine := m.topLine.View(m.width, dividerCol)
	status := m.statusBar.View(m.width, sidebarWidth)
	foot := m.footer.SetCounter(m.acct.WindowCounter()).View(m.width)

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

	if m.linkPicker.IsOpen() {
		box := m.linkPicker.Box(m.width, m.height)
		x, y := m.linkPicker.Position(box, m.width, m.height)
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

// translateConnState maps mail.ConnState to the UI ConnectionState type.
func translateConnState(s mail.ConnState) ConnectionState {
	switch s {
	case mail.ConnConnected:
		return Connected
	case mail.ConnReconnecting:
		return Reconnecting
	default:
		return Offline
	}
}

// IsLinkPickerOpen reports whether the link picker overlay is visible.
func (m App) IsLinkPickerOpen() bool { return m.linkPicker.IsOpen() }

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
