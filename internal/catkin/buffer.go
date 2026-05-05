package catkin

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Buffer wraps a bubbles/textarea.Model. Catkin uses textarea for
// its buffer storage, cursor management, and edit operations; the
// renderer is Catkin's own (see render.go).
type Buffer struct {
	ta textarea.Model
}

// NewBuffer wraps an existing textarea.Model.
func NewBuffer(ta textarea.Model) Buffer { return Buffer{ta: ta} }

func (b Buffer) Update(msg tea.Msg) (Buffer, tea.Cmd) {
	var cmd tea.Cmd
	b.ta, cmd = b.ta.Update(msg)
	return b, cmd
}

func (b Buffer) View() string  { return b.ta.View() }
func (b Buffer) Value() string { return b.ta.Value() }

func (b *Buffer) SetValue(s string) { b.ta.SetValue(s) }
func (b *Buffer) SetWidth(w int)    { b.ta.SetWidth(w) }
func (b *Buffer) SetHeight(h int)   { b.ta.SetHeight(h) }

func (b Buffer) Focus() tea.Cmd { return b.ta.Focus() }
func (b *Buffer) Blur()         { b.ta.Blur() }
func (b Buffer) Focused() bool  { return b.ta.Focused() }
