package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// App is the root bubbletea model for poplar.
type App struct {
	acct      AccountTab
	styles    Styles
	topLine   TopLine
	statusBar StatusBar
	footer    Footer
	keys      GlobalKeys
	width     int
	height    int
}

// NewApp creates the root model with a single AccountTab. Folder loading
// happens in Init's Cmd chain, not in the constructor.
func NewApp(t *theme.CompiledTheme, backend mail.Backend, uiCfg config.UIConfig) App {
	styles := NewStyles(t)
	sb := NewStatusBar(styles)
	sb = sb.SetConnectionState(Connected)

	return App{
		acct:      NewAccountTab(styles, backend, uiCfg),
		styles:    styles,
		topLine:   NewTopLine(styles),
		statusBar: sb,
		footer:    NewFooter(styles),
		keys:      NewGlobalKeys(),
	}
}

// Init delegates to the account tab so the initial folder fetch fires.
func (m App) Init() tea.Cmd {
	return m.acct.Init()
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

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if m.acct.sidebarSearch.State() != SearchIdle {
				// Steal q while search is active so it doesn't quit
				// the app mid-search. Delegate to AccountTab which
				// clears the filter.
				var cmd tea.Cmd
				m.acct, cmd = m.acct.Update(tea.KeyMsg{Type: tea.KeyEsc})
				return m, cmd
			}
			return m, tea.Quit
		case "ctrl+c":
			return m, tea.Quit
		case "?":
			// Stubbed for 2.5b-5 (help popover)
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
	foot := m.footer.View(m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		topLine,
		content,
		status,
		foot,
	)
}

// contentHeight returns the height available for the content area.
func (m App) contentHeight() int {
	chrome := 3 // top line + status bar + footer
	h := m.height - chrome
	if h < 1 {
		return 1
	}
	return h
}
