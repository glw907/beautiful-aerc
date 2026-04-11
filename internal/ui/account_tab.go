package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/mail"
)

// Panel identifies which panel of the AccountTab is focused.
type Panel int

const (
	SidebarPanel Panel = iota
	MsgListPanel
)

// sidebarWidth is the fixed width of the sidebar panel.
const sidebarWidth = 30

// AccountTab is the main account view with sidebar and message list panels.
type AccountTab struct {
	styles  Styles
	backend mail.Backend
	focused Panel
	width   int
	height  int
}

// NewAccountTab creates an AccountTab using the given styles and backend.
func NewAccountTab(styles Styles, backend mail.Backend) AccountTab {
	return AccountTab{
		styles:  styles,
		backend: backend,
		focused: SidebarPanel,
	}
}

// Init returns no initial command.
func (m AccountTab) Init() tea.Cmd { return nil }

// Update handles key events and window size changes.
func (m AccountTab) Update(msg tea.Msg) (AccountTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if msg.Type == tea.KeyTab {
			if m.focused == SidebarPanel {
				m.focused = MsgListPanel
			} else {
				m.focused = SidebarPanel
			}
		}
	}
	return m, nil
}

// View renders the two-panel placeholder layout.
func (m AccountTab) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sw := minInt(sidebarWidth, m.width/2)
	mw := m.width - sw - 1 // -1 for divider

	sidebarContent := renderPlaceholder("Sidebar", sw, m.height, m.focused == SidebarPanel, m.styles)
	divider := renderDivider(m.height, m.styles)
	msglistContent := renderPlaceholder("Message List", mw, m.height, m.focused == MsgListPanel, m.styles)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarContent, divider, msglistContent)
}

// renderPlaceholder renders a centered label in a panel of the given size.
func renderPlaceholder(label string, width, height int, focused bool, s Styles) string {
	topPad := maxInt(0, (height-1)/2)
	botPad := maxInt(0, height-1-topPad)
	leftPad := maxInt(0, (width-len(label))/2)

	var lines []string
	for i := 0; i < topPad; i++ {
		if focused {
			lines = append(lines, lipgloss.NewStyle().
				Width(width).
				Background(s.Selection.GetBackground()).
				Render(""))
		} else {
			lines = append(lines, strings.Repeat(" ", width))
		}
	}

	centeredLabel := strings.Repeat(" ", leftPad) + label
	if focused {
		lines = append(lines, lipgloss.NewStyle().
			Width(width).
			Foreground(s.Dim.GetForeground()).
			Background(s.Selection.GetBackground()).
			Render(centeredLabel))
	} else {
		lines = append(lines, lipgloss.NewStyle().
			Width(width).
			Foreground(s.Dim.GetForeground()).
			Render(centeredLabel))
	}

	for i := 0; i < botPad; i++ {
		if focused {
			lines = append(lines, lipgloss.NewStyle().
				Width(width).
				Background(s.Selection.GetBackground()).
				Render(""))
		} else {
			lines = append(lines, strings.Repeat(" ", width))
		}
	}

	return strings.Join(lines, "\n")
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
