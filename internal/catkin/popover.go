package catkin

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// popoverMaxSuggestions caps the suggestion list. 5 matches the
// OS-native context-menu convention (macOS, Firefox, Chrome) and
// stays inside SymSpell's high-quality top-of-list ranking.
const popoverMaxSuggestions = 5

// popoverState is Catkin's misspelling-suggestion overlay. The
// zero value is closed.
type popoverState struct {
	open        bool
	wordRange   Range
	word        string
	suggestions []string
	cursor      int // selected suggestion index
}

var popoverKeys = struct {
	Open, Close       key.Binding
	Next, Prev, Apply key.Binding
	Add, Ignore       key.Binding
}{
	// ";" opens the popover on a misspelling. Falls through to textarea when the
	// cursor is not on a flagged word, so normal typing is unaffected.
	Open:   key.NewBinding(key.WithKeys(";")),
	Close:  key.NewBinding(key.WithKeys("esc")),
	Next:   key.NewBinding(key.WithKeys("down", "ctrl+n")),
	Prev:   key.NewBinding(key.WithKeys("up", "ctrl+p")),
	Apply:  key.NewBinding(key.WithKeys("enter")),
	Add:    key.NewBinding(key.WithKeys("a")),
	Ignore: key.NewBinding(key.WithKeys("i")),
}

func (m Model) findMisspellingAt(byteOff int) *Annotation {
	if m.annotations == nil {
		return nil
	}
	for i := range m.annotations.All {
		a := &m.annotations.All[i]
		if a.Kind != KindMisspelling {
			continue
		}
		if a.Range.Contains(byteOff) {
			return a
		}
	}
	return nil
}

// openPopover seeds popoverState from an annotation and closes any
// active find shelf.
func (m Model) openPopover(a *Annotation) Model {
	mp, ok := a.Payload.(MisspellingPayload)
	if !ok {
		return m
	}
	if m.find.active() {
		m.find = findState{}
	}
	m.popover = popoverState{
		open:        true,
		wordRange:   a.Range,
		word:        mp.Word,
		suggestions: trimTo(mp.Suggestions, popoverMaxSuggestions),
		cursor:      0,
	}
	return m
}

func (m Model) closePopover() Model {
	m.popover = popoverState{}
	return m
}

// handlePopoverKey dispatches a KeyMsg while the popover is open.
// Returns (handled, model). When handled is false, the caller falls
// through to normal Update handling.
func (m Model) handlePopoverKey(k tea.KeyMsg) (bool, Model) {
	switch {
	case key.Matches(k, popoverKeys.Close):
		return true, m.closePopover()
	case key.Matches(k, popoverKeys.Next):
		if len(m.popover.suggestions) > 0 {
			m.popover.cursor = (m.popover.cursor + 1) % len(m.popover.suggestions)
		}
		return true, m
	case key.Matches(k, popoverKeys.Prev):
		if n := len(m.popover.suggestions); n > 0 {
			m.popover.cursor = (m.popover.cursor - 1 + n) % n
		}
		return true, m
	case key.Matches(k, popoverKeys.Apply):
		return true, m.applySelectedSuggestion()
	case key.Matches(k, popoverKeys.Ignore):
		return true, m.ignoreCurrentWord()
	case key.Matches(k, popoverKeys.Add):
		return true, m.addCurrentWordToWordlist()
	}
	// Digit jump-and-apply.
	if len(k.Runes) == 1 && k.Runes[0] >= '1' && k.Runes[0] <= '9' {
		idx := int(k.Runes[0] - '1')
		if idx < len(m.popover.suggestions) {
			m.popover.cursor = idx
			return true, m.applySelectedSuggestion()
		}
		return true, m
	}
	return false, m
}

func (m Model) applySelectedSuggestion() Model {
	if len(m.popover.suggestions) == 0 {
		return m.closePopover()
	}
	repl := m.popover.suggestions[m.popover.cursor]
	src := m.buf.Value()
	r := m.popover.wordRange
	newSrc := src[:r.Start] + repl + src[r.End:]
	m.buf.SetValue(newSrc)
	prefixRunes := utf8.RuneCountInString(src[:r.Start])
	replRunes := utf8.RuneCountInString(repl)
	m.buf.SetRuneOffset(prefixRunes + replRunes)
	m.recordSnap()
	m = m.closePopover()
	if len(m.annotators) > 0 {
		m.srcGen++
	}
	return m
}

func (m Model) ignoreCurrentWord() Model {
	for _, a := range m.annotators {
		if sa, ok := a.(*spellcheckAnnotator); ok {
			sa.IgnoreInSession(m.popover.word)
		}
	}
	m = m.closePopover()
	if len(m.annotators) > 0 {
		m.srcGen++
	}
	return m
}

func (m Model) addCurrentWordToWordlist() Model {
	word := m.popover.word
	if m.userWordlistPath != "" {
		appendUserWord(m.userWordlistPath, word)
	}
	for _, a := range m.annotators {
		if sa, ok := a.(*spellcheckAnnotator); ok && sa.speller != nil {
			sa.speller.known[strings.ToLower(word)] = 1
		}
	}
	m = m.closePopover()
	if len(m.annotators) > 0 {
		m.srcGen++
	}
	return m
}

// appendUserWord appends word to path, creating it 0o600 if missing.
// Errors are swallowed: the in-memory speller still picks up the word
// for this session, and the popover key handler has no surface for
// reporting an I/O failure.
func appendUserWord(path, word string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", word)
}

// width returns the popover's outer width including borders. Sized to
// the longest content row, capped at 32 cols so the popover stays out
// of the way of long lines.
func (p popoverState) width() int {
	const maxWidth = 32
	w := lipgloss.Width(`"` + p.word + `"`)
	for _, s := range p.suggestions {
		w = max(w, lipgloss.Width("  "+s))
	}
	w = max(w, lipgloss.Width("a add  i ignore  esc x"))
	if w > maxWidth {
		w = maxWidth
	}
	return w + 2
}

func (p popoverState) height() int {
	if len(p.suggestions) == 0 {
		return 4
	}
	return 4 + len(p.suggestions)
}

// render returns the framed popover content. The width arg is unused;
// the natural width() drives sizing.
func (p popoverState) render(_ int, st Styles) string {
	innerW := p.width() - 2
	header := lipgloss.NewStyle().Width(innerW).Render(`"` + p.word + `"`)
	var rows []string
	rows = append(rows, header)
	for i, s := range p.suggestions {
		marker := "  "
		if i == p.cursor {
			marker = "> "
		}
		row := lipgloss.NewStyle().Width(innerW).Render(marker + s)
		if i == p.cursor && st.PopoverSelected.GetForeground() != nil {
			row = st.PopoverSelected.Width(innerW).Render(marker + s)
		}
		rows = append(rows, row)
	}
	if len(p.suggestions) > 0 {
		rows = append(rows, lipgloss.NewStyle().Width(innerW).Render(""))
	}
	rows = append(rows, lipgloss.NewStyle().Width(innerW).Render("a add  i ignore  esc x"))
	body := strings.Join(rows, "\n")
	frame := st.Popover
	if frame.GetBorderStyle() == (lipgloss.Border{}) {
		frame = frame.Border(lipgloss.RoundedBorder())
	}
	return frame.Render(body)
}

// position returns the (row, col) where the popover's top-left corner
// should sit, given the cursor position in screen coordinates and the
// viewport dimensions.
func (p popoverState) position(cursorRow, cursorCol, viewportW, viewportH int) (int, int) {
	row := cursorRow + 1
	if row+p.height() > viewportH {
		row = cursorRow - p.height()
		if row < 0 {
			row = 0
		}
	}
	col := cursorCol
	if col+p.width() > viewportW {
		col = viewportW - p.width()
		if col < 0 {
			col = 0
		}
	}
	return row, col
}

// overlay composites a popover onto rendered body, returning a new string
// with the popover's rows replacing the corresponding horizontal slices.
// ANSI-safe via ansiSpliceAtCol.
func overlay(body, pop string, row, col int) string {
	bodyLines := strings.Split(body, "\n")
	popLines := strings.Split(pop, "\n")
	for i, pl := range popLines {
		idx := row + i
		if idx < 0 || idx >= len(bodyLines) {
			continue
		}
		bodyLines[idx] = ansiSpliceAtCol(bodyLines[idx], col, lipgloss.Width(pl), pl)
	}
	return strings.Join(bodyLines, "\n")
}

func trimTo(xs []string, n int) []string {
	if len(xs) <= n {
		out := make([]string, len(xs))
		copy(out, xs)
		return out
	}
	out := make([]string, n)
	copy(out, xs[:n])
	return out
}

func byteOffsetForRune(src string, runeOff int) int {
	if runeOff <= 0 {
		return 0
	}
	count := 0
	for i := range src {
		if count == runeOff {
			return i
		}
		count++
	}
	return len(src)
}

// maybeScheduleAnnotateAfterMutation issues an annotate-tick when the
// model's srcGen advanced (apply/ignore both bump it). Returns the merged Cmd.
func (m Model) maybeScheduleAnnotateAfterMutation(mm Model, cmd tea.Cmd) tea.Cmd {
	if mm.srcGen != m.srcGen && len(mm.annotators) > 0 {
		return tea.Batch(cmd, scheduleAnnotateCmd(mm.srcGen))
	}
	return cmd
}
