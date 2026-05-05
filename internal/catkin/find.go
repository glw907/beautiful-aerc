package catkin

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

type findMode int

const (
	findIdle findMode = iota
	findFind
	findReplace
)

// findState owns the find / replace overlay. The zero value is
// idle. Opening sets mode to findFind or findReplace and the
// overlay reserves footer rows out of Catkin's render height.
type findState struct {
	mode            findMode
	query           string
	replacement     string
	caseInsensitive bool
	matches         []int // rune offsets into the buffer
	cursor          int   // index into matches
	inputFocus      int   // 0 = query, 1 = replacement
}

func (f findState) active() bool { return f.mode != findIdle }

func (f findState) footerRows() int {
	switch f.mode {
	case findReplace:
		return 2
	case findFind:
		return 1
	}
	return 0
}

// recomputeMatches rebuilds matches against src. It preserves the
// cursor's match-anchor when possible by snapping to the nearest
// match at or after caretRune.
func (f *findState) recomputeMatches(src string, caretRune int) {
	f.matches = findAll(src, f.query, f.caseInsensitive)
	f.cursor = 0
	for i, m := range f.matches {
		if m >= caretRune {
			f.cursor = i
			break
		}
	}
}

func findAll(src, query string, caseInsensitive bool) []int {
	if query == "" {
		return nil
	}
	if caseInsensitive {
		src = strings.ToLower(src)
		query = strings.ToLower(query)
	}
	srcRunes := []rune(src)
	qRunes := []rune(query)
	var out []int
	for i := 0; i+len(qRunes) <= len(srcRunes); i++ {
		if string(srcRunes[i:i+len(qRunes)]) == query {
			out = append(out, i)
		}
	}
	return out
}

func (m Model) handleFind(k tea.KeyMsg) (Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.find = findState{}
		return m, nil
	case "enter":
		m = m.findStep(+1)
		return m, nil
	case "shift+enter":
		m = m.findStep(-1)
		return m, nil
	case "tab":
		m.find.caseInsensitive = !m.find.caseInsensitive
		m.find.recomputeMatches(m.buf.Value(), m.buf.RuneOffset())
		m = m.jumpToCurrent()
		return m, nil
	case "ctrl+r":
		m.find.mode = findReplace
		m.find.inputFocus = 1
		return m, nil
	case "ctrl+f":
		m.find.mode = findFind
		m.find.inputFocus = 0
		return m, nil
	}

	if m.find.mode == findReplace {
		switch k.String() {
		case "ctrl+y":
			return m.replaceCurrent(), nil
		case "ctrl+n":
			return m.findStep(+1), nil
		case "ctrl+a":
			return m.replaceAll(), nil
		}
	}

	if k.Type == tea.KeyBackspace {
		if m.find.inputFocus == 1 {
			m.find.replacement = trimLastRune(m.find.replacement)
		} else {
			m.find.query = trimLastRune(m.find.query)
			m.find.recomputeMatches(m.buf.Value(), m.buf.RuneOffset())
			m = m.jumpToCurrent()
		}
		return m, nil
	}

	if k.Type == tea.KeyRunes && len(k.Runes) > 0 {
		s := string(k.Runes)
		if m.find.inputFocus == 1 {
			m.find.replacement += s
		} else {
			m.find.query += s
			m.find.recomputeMatches(m.buf.Value(), m.buf.RuneOffset())
			m = m.jumpToCurrent()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) findStep(dir int) Model {
	if len(m.find.matches) == 0 {
		return m
	}
	n := len(m.find.matches)
	m.find.cursor = (m.find.cursor + dir + n) % n
	return m.jumpToCurrent()
}

func (m Model) jumpToCurrent() Model {
	if len(m.find.matches) == 0 {
		return m
	}
	m.buf.SetRuneOffset(m.find.matches[m.find.cursor])
	return applyScrollOff(m)
}

func (m Model) replaceCurrent() Model {
	if len(m.find.matches) == 0 {
		return m
	}
	at := m.find.matches[m.find.cursor]
	src := m.buf.Value()
	srcRunes := []rune(src)
	qLen := utf8.RuneCountInString(m.find.query)
	rep := m.find.replacement
	out := string(srcRunes[:at]) + rep + string(srcRunes[at+qLen:])
	m.buf.SetValue(out)
	m.recordSnap()
	caret := at + utf8.RuneCountInString(rep)
	m.buf.SetRuneOffset(caret)
	m.find.recomputeMatches(out, caret)
	return applyScrollOff(m)
}

func (m Model) replaceAll() Model {
	if len(m.find.matches) == 0 {
		return m
	}
	srcRunes := []rune(m.buf.Value())
	qLen := utf8.RuneCountInString(m.find.query)
	rep := []rune(m.find.replacement)
	var out []rune
	prev := 0
	for _, at := range m.find.matches {
		out = append(out, srcRunes[prev:at]...)
		out = append(out, rep...)
		prev = at + qLen
	}
	out = append(out, srcRunes[prev:]...)
	final := string(out)
	m.buf.SetValue(final)
	m.recordSnap()
	caret := len(out)
	if last := m.find.matches[len(m.find.matches)-1]; last >= 0 {
		caret = last + len(rep)
	}
	m.buf.SetRuneOffset(caret)
	m.find.recomputeMatches(final, caret)
	return applyScrollOff(m)
}

// renderFindFooter returns the overlay rows. The result has
// f.footerRows() lines, each pre-padded to width cells.
func (f findState) renderFindFooter(width int) string {
	if !f.active() {
		return ""
	}
	flag := "lit"
	if f.caseInsensitive {
		flag = "ci"
	}
	count := fmt.Sprintf("[%d/%d]", f.matchOrdinal(), len(f.matches))
	if len(f.matches) == 0 {
		count = "[0/0]"
	}
	cursor := func(active bool) string {
		if active {
			return "▌"
		}
		return " "
	}
	row1 := fmt.Sprintf("Find: %s%s  %s  %s", f.query, cursor(f.inputFocus == 0), count, flag)
	if f.mode == findReplace {
		row2 := fmt.Sprintf("Replace: %s%s  (^Y accept · ^N skip · ^A all)", f.replacement, cursor(f.inputFocus == 1))
		return padRow(row1, width) + "\n" + padRow(row2, width)
	}
	return padRow(row1, width)
}

func (f findState) matchOrdinal() int {
	if len(f.matches) == 0 {
		return 0
	}
	return f.cursor + 1
}

func padRow(s string, width int) string {
	if width <= 0 {
		return s
	}
	pad := width - utf8.RuneCountInString(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func trimLastRune(s string) string {
	if s == "" {
		return s
	}
	_, n := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-n]
}
