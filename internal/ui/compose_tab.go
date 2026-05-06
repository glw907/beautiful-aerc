// SPDX-License-Identifier: MIT

package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/theme"
)

// ComposeTab is the inline compose surface. Owns header inputs and the
// body editor. Send and discard surface as tea.Msg values that App
// translates into cache ops. Focus model and key routing land in the
// next task.
type ComposeTab struct {
	styles Styles
	icons  IconSet

	from string

	to      textinput.Model
	cc      textinput.Model
	bcc     textinput.Model
	subject textinput.Model
	editor  compose.Editor

	// focus is the active input field: 0=To … 4=Body.
	focus int
	// err is an inline error row rendered above the divider rule. Empty
	// when there is nothing to report.
	err string

	width  int
	height int
}

const (
	composeFocusTo = iota
	composeFocusCc
	composeFocusBcc
	composeFocusSubject
	composeFocusBody
)

// composeLabelWidth is the column reserved for header labels. "Subject:"
// is 8 cells with one space of separation giving 9.
const composeLabelWidth = 9

// NewComposeTab returns a fresh, empty ComposeTab with focus on To.
// The theme parameter is reserved for Catkin theme threading in a
// later pass and is not yet consumed here.
func NewComposeTab(styles Styles, t *theme.CompiledTheme, self string, icons IconSet) *ComposeTab {
	_ = t // reserved: will thread into compose.NewCatkinEditor once the seam lands
	mk := func() textinput.Model {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = ""
		return ti
	}
	c := &ComposeTab{
		styles:  styles,
		icons:   icons,
		from:    self,
		to:      mk(),
		cc:      mk(),
		bcc:     mk(),
		subject: mk(),
		editor:  compose.NewCatkinEditor(),
	}
	c.to.Focus()
	c.focus = composeFocusTo
	return c
}

func (c *ComposeTab) Init() tea.Cmd {
	return c.editor.Init()
}

// SetSize propagates dimensions to all child inputs and the body editor.
func (c *ComposeTab) SetSize(w, h int) {
	c.width = w
	c.height = h

	inputW := w - composeLabelWidth - 1
	if inputW < 1 {
		inputW = 1
	}
	c.to.Width = inputW
	c.cc.Width = inputW
	c.bcc.Width = inputW
	c.subject.Width = inputW

	// 5 header rows + 1 divider row. Error row takes one more when set.
	bodyHeight := h - 5 - 1
	if c.err != "" {
		bodyHeight--
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	c.editor.SetSize(w, bodyHeight)
}

// View renders the compose surface. Every returned line is exactly
// c.width display cells. The width contract is self-enforced so the
// parent can join panes without defensive clipping.
func (c *ComposeTab) View() string {
	if c.width == 0 || c.height == 0 {
		return ""
	}
	var rows []string
	rows = append(rows, c.headerRow("From:", c.from))
	rows = append(rows, c.headerRow("To:", c.to.View()))
	rows = append(rows, c.headerRow("Cc:", c.cc.View()))
	rows = append(rows, c.headerRow("Bcc:", c.bcc.View()))
	rows = append(rows, c.headerRow("Subject:", c.subject.View()))
	if c.err != "" {
		rows = append(rows, c.padRow(c.styles.ErrorBanner.Render(c.err)))
	}
	rows = append(rows, c.padRow(strings.Repeat("─", c.width)))
	for _, line := range strings.Split(c.editor.View(), "\n") {
		rows = append(rows, c.padRow(line))
	}
	for len(rows) < c.height {
		rows = append(rows, c.padRow(""))
	}
	if len(rows) > c.height {
		rows = rows[:c.height]
	}
	return strings.Join(rows, "\n")
}

// headerRow builds one label+value row padded to exactly c.width cells.
func (c *ComposeTab) headerRow(label, value string) string {
	pad := composeLabelWidth - lipgloss.Width(label)
	if pad < 1 {
		pad = 1
	}
	return c.padRow(label + strings.Repeat(" ", pad) + value)
}

// padRow pads or truncates s to exactly c.width display cells.
func (c *ComposeTab) padRow(s string) string {
	w := lipgloss.Width(s)
	if w >= c.width {
		return displayTruncate(s, c.width)
	}
	return s + strings.Repeat(" ", c.width-w)
}

// Update implements the Update half of tea.Model.
func (c *ComposeTab) Update(msg tea.Msg) (*ComposeTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			c.advanceFocus(+1)
			return c, nil
		case tea.KeyShiftTab:
			c.advanceFocus(-1)
			return c, nil
		case tea.KeyEsc:
			if c.focus == composeFocusBody {
				c.setFocus(composeFocusSubject)
			} else {
				c.setFocus(composeFocusBody)
			}
			return c, nil
		}
	}

	var cmd tea.Cmd
	switch c.focus {
	case composeFocusTo:
		c.to, cmd = c.to.Update(msg)
	case composeFocusCc:
		c.cc, cmd = c.cc.Update(msg)
	case composeFocusBcc:
		c.bcc, cmd = c.bcc.Update(msg)
	case composeFocusSubject:
		c.subject, cmd = c.subject.Update(msg)
	case composeFocusBody:
		c.editor, cmd = c.editor.Update(msg)
	}
	return c, cmd
}

func (c *ComposeTab) advanceFocus(delta int) {
	const fields = 5
	c.setFocus(((c.focus + delta) + fields) % fields)
}

func (c *ComposeTab) setFocus(target int) {
	c.to.Blur()
	c.cc.Blur()
	c.bcc.Blur()
	c.subject.Blur()
	c.editor.Blur()
	switch target {
	case composeFocusTo:
		_ = c.to.Focus()
	case composeFocusCc:
		_ = c.cc.Focus()
	case composeFocusBcc:
		_ = c.bcc.Focus()
	case composeFocusSubject:
		_ = c.subject.Focus()
	case composeFocusBody:
		_ = c.editor.Focus()
	}
	c.focus = target
}
