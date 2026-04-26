package ui

import (
	"strings"

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
	styles     Styles
	topLine    TopLine
	statusBar  StatusBar
	footer     Footer
	keys       GlobalKeys
	viewerOpen bool
	helpOpen   bool
	help       HelpPopover
	lastErr    ErrorMsg
	width      int
	height     int
}

// NewApp creates the root model with a single AccountTab. Folder loading
// happens in Init's Cmd chain, not in the constructor.
func NewApp(t *theme.CompiledTheme, backend mail.Backend, uiCfg config.UIConfig) App {
	styles := NewStyles(t)
	sb := NewStatusBar(styles)
	sb = sb.SetConnectionState(Offline)

	return App{
		acct:      NewAccountTab(styles, t, backend, uiCfg),
		backend:   backend,
		styles:    styles,
		topLine:   NewTopLine(styles),
		statusBar: sb,
		footer:    NewFooter(styles),
		keys:      NewGlobalKeys(),
	}
}

// Init delegates to the account tab so the initial folder fetch fires,
// and starts the backend update pump.
func (m App) Init() tea.Cmd {
	return tea.Batch(m.acct.Init(), pumpUpdatesCmd(m.backend))
}

// Update handles global keys and delegates everything else to the
// account tab. FolderChangedMsg bubbles up from the child and updates
// the status bar without reaching into child state.
func (m App) Update(msg tea.Msg) (App, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(contentMsg)
		return m, cmd

	case FolderChangedMsg:
		m.statusBar = m.statusBar.SetCounts(msg.Exists, msg.Unseen)
		return m, nil

	case ViewerOpenedMsg:
		m.viewerOpen = true
		m.footer = m.footer.SetContext(ViewerContext)
		m.statusBar = m.statusBar.SetMode(StatusViewer).SetScrollPct(0)
		return m, nil

	case ViewerClosedMsg:
		m.viewerOpen = false
		m.footer = m.footer.SetContext(AccountContext)
		m.statusBar = m.statusBar.SetMode(StatusAccount)
		return m, nil

	case ViewerScrollMsg:
		m.statusBar = m.statusBar.SetScrollPct(msg.Pct)
		return m, nil

	case ErrorMsg:
		// Banner state is App-owned. nil ↔ set transitions toggle the
		// chrome row count, so resize the child when the banner appears
		// or disappears. Last-write-wins between two non-nil errors
		// does not change height, so the resize is skipped.
		hadErr := m.lastErr.Err != nil
		m.lastErr = msg
		cmds := make([]tea.Cmd, 0, 2)
		if hadErr != (m.lastErr.Err != nil) && m.width > 0 && m.height > 0 {
			contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
			acct, rcmd := m.acct.Update(contentMsg)
			m.acct = acct
			cmds = append(cmds, rcmd)
		}
		acct, fcmd := m.acct.Update(msg)
		m.acct = acct
		cmds = append(cmds, fcmd)
		return m, tea.Batch(cmds...)

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
		switch {
		case key.Matches(msg, m.keys.Quit):
			if m.viewerOpen {
				// Viewer-open: q closes the viewer, not the app.
				// Delegate so AccountTab routes to viewer.handleKey.
				var cmd tea.Cmd
				m.acct, cmd = m.acct.Update(msg)
				return m, cmd
			}
			if m.acct.sidebarSearch.State() != SearchIdle {
				// Steal q while search is active so it doesn't quit
				// the app mid-search. Delegate to AccountTab which
				// clears the filter.
				var cmd tea.Cmd
				m.acct, cmd = m.acct.Update(tea.KeyMsg{Type: tea.KeyEsc})
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
	return m, cmd
}

// View composes the full-screen layout.
func (m App) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.helpOpen {
		return m.help.View(m.width, m.height)
	}

	rawContent := m.acct.View()
	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(rawContent, "\n")
	for i, line := range contentLines {
		pad := max(0, m.width-1-lipgloss.Width(line))
		contentLines[i] = line + strings.Repeat(" ", pad) + rightBorder
	}
	content := strings.Join(contentLines, "\n")

	dividerCol := sidebarWidth
	topLine := m.topLine.View(m.width, dividerCol)
	status := m.statusBar.View(m.width, sidebarWidth)
	foot := m.footer.SetCounter(m.acct.WindowCounter()).View(m.width)

	parts := []string{topLine, content}
	if banner := renderErrorBanner(m.lastErr, m.width, m.styles); banner != "" {
		parts = append(parts, banner)
	}
	parts = append(parts, status, foot)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
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

// contentHeight returns the height available for the content area.
// The error banner takes one extra chrome row when present.
func (m App) contentHeight() int {
	chrome := 3 // top line + status bar + footer
	if m.lastErr.Err != nil {
		chrome++
	}
	h := m.height - chrome
	if h < 1 {
		return 1
	}
	return h
}
