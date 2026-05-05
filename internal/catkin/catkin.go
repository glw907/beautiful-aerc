// Package catkin is poplar's markdown-first bubbletea editor.
//
// Catkin wraps bubbles/textarea as its buffer + cursor + edit-op
// primitive, but owns its own renderer so live markdown styling
// and block-aware reflow can drive the display directly from the
// raw source without parsing textarea's ANSI output.
//
// This package depends only on bubbletea, bubbles, lipgloss, and
// muesli/reflow. It has no poplar-specific imports and is
// extractable as github.com/glw907/catkin.
package catkin

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Model is Catkin's tea.Model. Construct with New.
type Model struct {
	buf         Buffer
	width       int
	height      int
	viewportTop int
	styles      Styles
}

// New returns a Model with default settings.
func New() Model {
	return Model{
		buf: NewBuffer(textarea.New()),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if handled, b, cmd := handleWordNav(m.buf, k); handled {
			m.buf = b
			m = applyScrollOff(m)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.buf, cmd = m.buf.Update(msg)
	m = applyScrollOff(m)
	return m, cmd
}

func (m Model) View() string {
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	return Render(src, m.width, m.height, m.viewportTop, cur, m.styles)
}

// SetStyles replaces the render-time style table. The zero value
// is no-op styles; consumers map their theme onto Styles at the
// boundary.
func (m *Model) SetStyles(s Styles) { m.styles = s }

// Value returns the raw markdown source.
func (m Model) Value() string { return m.buf.Value() }

// SetValue replaces the buffer with s.
func (m *Model) SetValue(s string) { m.buf.SetValue(s) }

// SetSize sets the editor's display dimensions.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.buf.SetWidth(w)
	m.buf.SetHeight(h)
}

// SetWidth sets the body wrap width and re-runs reflow.
func (m *Model) SetWidth(w int) {
	if w == m.width {
		return
	}
	m.width = w
	m.buf.SetWidth(w)
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	src, cur = Reflow(src, w, cur)
	m.buf.SetValue(src)
	m.buf.SetRuneOffset(cur)
}

// Focus focuses the editor.
func (m Model) Focus() tea.Cmd { return m.buf.Focus() }

// Blur blurs the editor.
func (m *Model) Blur() { m.buf.Blur() }

// Focused reports whether the editor has focus.
func (m Model) Focused() bool { return m.buf.Focused() }
