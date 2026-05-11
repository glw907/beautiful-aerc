package catkin

import (
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

// Buffer wraps a bubbles/textarea.Model. Catkin uses textarea for
// its buffer storage, cursor management, and edit operations. The
// renderer is Catkin's own (see render.go).
//
// Buffer is a value type. Mutators return a new Buffer; callers
// reassign. The wrapped textarea.Model is sealed — it never leaves
// the package.
type Buffer struct {
	ta textarea.Model
}

func NewBuffer(ta textarea.Model) Buffer { return Buffer{ta: ta} }

func (b Buffer) Update(msg tea.Msg) (Buffer, tea.Cmd) {
	var cmd tea.Cmd
	b.ta, cmd = b.ta.Update(msg)
	return b, cmd
}

func (b Buffer) Value() string { return b.ta.Value() }
func (b Buffer) Focused() bool { return b.ta.Focused() }

func (b Buffer) WithValue(s string) Buffer {
	b.ta.SetValue(s)
	return b
}

func (b Buffer) WithWidth(w int) Buffer {
	b.ta.SetWidth(w)
	return b
}

func (b Buffer) WithHeight(h int) Buffer {
	b.ta.SetHeight(h)
	return b
}

// WithFocus focuses the buffer and returns the cursor-blink command from textarea.
func (b Buffer) WithFocus() (Buffer, tea.Cmd) {
	cmd := b.ta.Focus()
	return b, cmd
}

func (b Buffer) WithBlur() Buffer {
	b.ta.Blur()
	return b
}

// RuneOffset returns the cursor's rune offset from the start of the value.
func (b Buffer) RuneOffset() int {
	row := b.ta.Line()
	value := b.ta.Value()
	lines := strings.Split(value, "\n")
	off := 0
	for i := range min(row, len(lines)) {
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

// WithRuneOffset positions the cursor at rune offset off.
func (b Buffer) WithRuneOffset(off int) Buffer {
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
	// Navigate to the row first: SetCursorColumn clamps col to the
	// current row's length, so the column must be set last.
	for b.ta.Line() < row {
		b.ta.CursorDown()
	}
	for b.ta.Line() > row {
		b.ta.CursorUp()
	}
	b.ta.SetCursorColumn(col)
	return b
}
