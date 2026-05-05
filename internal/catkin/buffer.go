package catkin

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Buffer wraps a bubbles/textarea.Model. Catkin uses textarea for
// its buffer storage, cursor management, and edit operations. The
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

func (b Buffer) Value() string { return b.ta.Value() }

func (b *Buffer) SetValue(s string) { b.ta.SetValue(s) }
func (b *Buffer) SetWidth(w int)    { b.ta.SetWidth(w) }
func (b *Buffer) SetHeight(h int)   { b.ta.SetHeight(h) }

func (b *Buffer) Focus() tea.Cmd { return b.ta.Focus() }
func (b *Buffer) Blur()          { b.ta.Blur() }
func (b Buffer) Focused() bool   { return b.ta.Focused() }

// RuneOffset returns the cursor's rune offset from the start of the value.
func (b Buffer) RuneOffset() int {
	row := b.ta.Line()
	value := b.ta.Value()
	lines := strings.Split(value, "\n")
	off := 0
	for i := 0; i < row && i < len(lines); i++ {
		off += utf8.RuneCountInString(lines[i]) + 1
	}
	li := b.ta.LineInfo()
	off += li.StartColumn + li.ColumnOffset
	total := utf8.RuneCountInString(value)
	if off > total {
		off = total
	}
	return off
}

// SetRuneOffset positions the cursor at rune offset off.
func (b *Buffer) SetRuneOffset(off int) {
	value := b.ta.Value()
	total := utf8.RuneCountInString(value)
	if off > total {
		off = total
	}
	lines := strings.Split(value, "\n")
	row, col := 0, off
	for i, l := range lines {
		ln := utf8.RuneCountInString(l)
		if col <= ln {
			break
		}
		col -= ln + 1
		row = i + 1
	}
	if row >= len(lines) {
		row = len(lines) - 1
		col = utf8.RuneCountInString(lines[row])
	}
	// Navigate to the target row first. SetCursor operates on the current row,
	// so col must be set after the row is in place or it will be clamped to the
	// wrong line's length.
	for b.ta.Line() < row {
		b.ta.CursorDown()
	}
	for b.ta.Line() > row {
		b.ta.CursorUp()
	}
	b.ta.SetCursor(col)
}
