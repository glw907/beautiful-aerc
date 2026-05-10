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
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

// Model is Catkin's tea.Model.
type Model struct {
	buf         Buffer
	width       int
	height      int
	viewportTop int
	styles      Styles
	undo        undoRing
	mode        DisplayMode
	find        findState

	annotators       []Annotator
	annotations      *AnnotationSet
	srcGen           uint64
	tidyA            *tidyAnnotator
	popover          popoverState
	userWordlistPath string
}

func New() Model {
	m := Model{
		buf: NewBuffer(textarea.New()),
	}
	m.undo.seed(snap{"", 0})
	tidy := newTidyAnnotator()
	m.tidyA = tidy
	m.annotators = append(m.annotators, tidy)
	return m
}

// RegisterAnnotator appends an annotator; they run in registration order.
func (m *Model) RegisterAnnotator(a Annotator) {
	m.annotators = append(m.annotators, a)
}

// SetTidyHighlights configures the post-Tidy character-range highlights.
// The annotator returns annotations only while the buffer matches src.
// Any subsequent buffer mutation invalidates the match and clears the
// highlights on the next annotate tick.
func (m *Model) SetTidyHighlights(src string, ranges []Range) {
	if m.tidyA == nil {
		return
	}
	m.tidyA.Set(src, ranges)
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch tm := msg.(type) {
	case annotateRequestMsg:
		if tm.gen != m.srcGen {
			return m, nil
		}
		return m, runAnnotatorsCmd(m.srcGen, m.buf.Value(), m.annotators)
	case annotationsReadyMsg:
		if tm.gen == m.srcGen {
			m.annotations = tm.set
		}
		return m, nil
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if m.popover.open {
			if handled, mm := m.handlePopoverKey(k); handled {
				return mm, m.maybeScheduleAnnotateAfterMutation(mm, nil)
			}
		}
		if !m.popover.open && key.Matches(k, popoverKeys.Open) {
			if a := m.findMisspellingAt(byteOffsetForRune(m.buf.Value(), m.buf.RuneOffset())); a != nil {
				return m.openPopover(a), nil
			}
		}
		if m.find.active() {
			return m.handleFind(k)
		}
		switch k.String() {
		case "ctrl+f":
			m.find = findState{mode: findFind}
			return m, nil
		case "ctrl+r":
			m.find = findState{mode: findReplace, inputFocus: 1}
			return m, nil
		case "ctrl+\\":
			m.mode = m.mode.next()
			return applyScrollOff(m), nil
		case "ctrl+z":
			if s, ok := m.undo.undo(); ok {
				m.buf.SetValue(s.val)
				m.buf.SetRuneOffset(s.cur)
			}
			return applyScrollOff(m), nil
		case "ctrl+y":
			if s, ok := m.undo.redo(); ok {
				m.buf.SetValue(s.val)
				m.buf.SetRuneOffset(s.cur)
			}
			return applyScrollOff(m), nil
		}
		for _, h := range []func(Buffer, tea.KeyPressMsg) (bool, Buffer, tea.Cmd){
			handleCommand, handleWordNav, handleAutoPair,
		} {
			if handled, b, cmd := h(m.buf, k); handled {
				return m.afterEdit(b, cmd)
			}
		}
	}
	b, cmd := m.buf.Update(msg)
	return m.afterEdit(b, cmd)
}

func (m Model) afterEdit(b Buffer, cmd tea.Cmd) (Model, tea.Cmd) {
	prev := m.buf.Value()
	m.buf = b
	m.recordSnap()
	if m.buf.Value() != prev && len(m.annotators) > 0 {
		m.srcGen++
		cmd = tea.Batch(cmd, scheduleAnnotateCmd(m.srcGen))
	}
	m = m.closePopoverIfCursorLeftRange()
	return applyScrollOff(m), cmd
}

func (m Model) closePopoverIfCursorLeftRange() Model {
	if !m.popover.open {
		return m
	}
	off := byteOffsetForRune(m.buf.Value(), m.buf.RuneOffset())
	if !m.popover.wordRange.Contains(off) {
		return m.closePopover()
	}
	return m
}

func (m *Model) recordSnap() {
	m.undo.record(snap{m.buf.Value(), m.buf.RuneOffset()})
}

func (m Model) View() string {
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	body := RenderAnnotated(src, m.width, m.height-m.find.footerRows(), m.viewportTop, cur, m.styles, m.mode, m.annotations)
	if m.popover.open {
		row, col := m.popoverScreenPosition()
		body = overlay(body, m.popover.render(0, m.styles), row, col)
	}
	if !m.find.active() {
		return body
	}
	return body + "\n" + m.find.renderFindFooter(m.width)
}

func (m Model) popoverScreenPosition() (int, int) {
	src := m.buf.Value()
	startRow, startCol := offsetToRowCol(src, utf8.RuneCountInString(src[:m.popover.wordRange.Start]))
	screenRow := startRow - m.viewportTop
	if screenRow < 0 {
		screenRow = 0
	}
	return m.popover.position(screenRow, startCol, m.width, m.height-m.find.footerRows())
}

func (m Model) Mode() DisplayMode { return m.mode }

// SetStyles replaces the render-time style table. The zero value is
// no-op styles; consumers map their theme onto Styles at the boundary.
func (m *Model) SetStyles(s Styles) {
	m.styles = s
	if m.tidyA != nil {
		m.tidyA.SetStyle(s.TidyChange)
	}
}

func (m Model) Value() string { return m.buf.Value() }

// SetValue replaces the buffer and re-seeds the undo ring; programmatic
// loads are not user edits.
func (m *Model) SetValue(s string) {
	m.buf.SetValue(s)
	m.undo.seed(snap{s, m.buf.RuneOffset()})
}

// SetUserWordlistPath sets the file the popover's add-action appends to.
// An empty path makes the add a session-local addition only.
func (m *Model) SetUserWordlistPath(path string) {
	m.userWordlistPath = path
}

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

func (m *Model) Focus() tea.Cmd { return m.buf.Focus() }

func (m *Model) Blur() { m.buf.Blur() }

func (m Model) Focused() bool { return m.buf.Focused() }
