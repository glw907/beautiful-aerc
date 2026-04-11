package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

// App is the root bubbletea model for poplar.
// TODO(Task 6): rewire to new chrome layout (TopLine, no tab bar).
type App struct {
	acct      AccountTab
	styles    Styles
	statusBar StatusBar
	footer    Footer
	keys      GlobalKeys
	width     int
	height    int
}

// NewApp creates the root model with a single AccountTab.
func NewApp(t *theme.CompiledTheme, backend mail.Backend) App {
	styles := NewStyles(t)
	acct := NewAccountTab(styles, backend)

	sb := NewStatusBar(styles)
	folders, _ := backend.ListFolders()
	if len(folders) > 0 {
		inbox := folders[0]
		sb.SetCounts(inbox.Exists, inbox.Unseen)
	}
	sb.SetConnected(true)

	return App{
		acct:      acct,
		styles:    styles,
		statusBar: sb,
		footer:    NewFooter(styles),
		keys:      NewGlobalKeys(),
	}
}

// Init returns no initial command.
func (m App) Init() tea.Cmd { return nil }

// Update handles global keys and delegates to the account tab.
func (m App) Update(msg tea.Msg) (App, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(contentMsg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			// Stubbed for 2.5b-5 (help popover)
			return m, nil
		case ":":
			// Stubbed for 2.5b-7 (command mode)
			return m, nil
		}
	}

	// Delegate to account tab
	var cmd tea.Cmd
	m.acct, cmd = m.acct.Update(msg)
	cmds = append(cmds, cmd)
	m.updateFooterContext()

	return m, tea.Batch(cmds...)
}

// View composes the full-screen layout.
func (m App) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Add right border │ to each content line
	rawContent := m.acct.View()
	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(rawContent, "\n")
	for i, line := range contentLines {
		pad := maxInt(0, m.width-1-lipgloss.Width(line))
		contentLines[i] = line + strings.Repeat(" ", pad) + rightBorder
	}
	content := strings.Join(contentLines, "\n")

	status := m.statusBar.View(m.width, sidebarWidth)
	foot := m.footer.View(m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		status,
		foot,
	)
}

// contentHeight returns the height available for the content area.
func (m App) contentHeight() int {
	// status bar (1) + footer (1)
	chrome := 2
	h := m.height - chrome
	if h < 1 {
		return 1
	}
	return h
}

// updateFooterContext switches the footer KeyMap based on the active panel.
func (m *App) updateFooterContext() {
	if m.acct.focused == SidebarPanel {
		m.footer.SetContext(SidebarContext)
	} else {
		m.footer.SetContext(MsgListContext)
	}
}
