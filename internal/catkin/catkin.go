// Package catkin is poplar's markdown-first bubbletea editor.
//
// Catkin wraps bubbles/textarea as its buffer + cursor + edit-op
// primitive, but owns its own renderer so live markdown styling
// and block-aware reflow can drive the display directly from the
// raw source without parsing textarea's ANSI output.
//
// This package depends only on bubbletea, bubbles, and lipgloss. It
// has no poplar-specific imports and is extractable as
// github.com/glw907/catkin.
package catkin

import (
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

func pasteInto(buf Buffer, runes []rune, cur int, payload string) (Buffer, int) {
	payloadRunes := []rune(payload)
	newVal := string(runes[:cur]) + payload + string(runes[cur:])
	newCur := cur + len(payloadRunes)
	return buf.WithValue(newVal).WithRuneOffset(newCur), newCur
}

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

// WithAnnotator returns a Model with a appended to the annotator
// list. Mount-time configuration: call before the tea loop sees the
// model. Annotators run in registration order.
func (m Model) WithAnnotator(a Annotator) Model {
	m.annotators = append(m.annotators, a)
	return m
}

// WithTidyHighlights configures the post-Tidy character-range highlights.
// The annotator returns annotations only while the buffer matches src.
// Any subsequent buffer mutation invalidates the match and clears the
// highlights on the next annotate tick.
func (m Model) WithTidyHighlights(src string, ranges []Range) Model {
	if m.tidyA != nil {
		m.tidyA.Set(src, ranges)
	}
	return m
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
	if tm, ok := msg.(tea.PasteMsg); ok {
		payload := tm.Content
		if payload == "" {
			return m, nil
		}
		runes := []rune(m.buf.Value())
		cur := m.buf.RuneOffset()
		var b Buffer
		if start, end, ok := urlWrapTarget(runes, cur, payload); ok {
			word := string(runes[start:end])
			replacement := "[" + word + "](" + payload + ")"
			newVal := string(runes[:start]) + replacement + string(runes[end:])
			b = m.buf.WithValue(newVal).WithRuneOffset(start + utf8.RuneCountInString(replacement))
		} else {
			b, _ = pasteInto(m.buf, runes, cur, payload)
		}
		m.buf = b
		m.undo.push(snap{b.Value(), b.RuneOffset()})
		var cmd tea.Cmd
		if len(m.annotators) > 0 {
			m.srcGen++
			cmd = scheduleAnnotateCmd(m.srcGen)
		}
		m = m.closePopoverIfCursorLeftRange()
		return applyScrollOff(m), cmd
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
				m.buf = m.buf.WithValue(s.val).WithRuneOffset(s.cur)
			}
			return applyScrollOff(m), nil
		case "ctrl+y":
			if s, ok := m.undo.redo(); ok {
				m.buf = m.buf.WithValue(s.val).WithRuneOffset(s.cur)
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
	m.undo.record(snap{m.buf.Value(), m.buf.RuneOffset()})
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

// WithStyles replaces the render-time style table. The zero value is
// no-op styles; consumers map their theme onto Styles at the boundary.
func (m Model) WithStyles(s Styles) Model {
	m.styles = s
	if m.tidyA != nil {
		m.tidyA.SetStyle(s.TidyChange)
	}
	return m
}

func (m Model) Value() string { return m.buf.Value() }

// WithValue replaces the buffer and re-seeds the undo ring; programmatic
// loads are not user edits.
func (m Model) WithValue(s string) Model {
	m.buf = m.buf.WithValue(s)
	m.undo.seed(snap{s, m.buf.RuneOffset()})
	return m
}

// Reflowed re-wraps the current buffer at the configured width. Callers
// use this after WithValue when the seeded text predates the editor's
// width (drafts loaded from cache, replies whose quoted body was wrapped
// at a wider canonical column than the live terminal).
func (m Model) Reflowed() Model {
	if m.width <= 0 {
		return m
	}
	src, cur := Reflow(m.buf.Value(), m.width, m.buf.RuneOffset())
	m.buf = m.buf.WithValue(src).WithRuneOffset(cur)
	return m
}

// WithUserWordlistPath sets the file the popover's add-action appends to.
// An empty path makes the add a session-local addition only.
func (m Model) WithUserWordlistPath(path string) Model {
	m.userWordlistPath = path
	return m
}

// WithSize sets the viewport dimensions.
func (m Model) WithSize(w, h int) Model {
	m.width, m.height = w, h
	m.buf = m.buf.WithWidth(w).WithHeight(h)
	return m
}

// WithWidth sets the body wrap width. Source is not mutated; wrap is
// resolved at render time.
func (m Model) WithWidth(w int) Model {
	if w == m.width {
		return m
	}
	m.width = w
	m.buf = m.buf.WithWidth(w)
	return m
}

func (m Model) WithFocus() (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.buf, cmd = m.buf.WithFocus()
	return m, cmd
}

func (m Model) WithBlur() Model {
	m.buf = m.buf.WithBlur()
	return m
}

func (m Model) Focused() bool { return m.buf.Focused() }
