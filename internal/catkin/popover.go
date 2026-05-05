package catkin

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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

// findMisspellingAt returns the annotation whose range covers byteOff,
// or nil if none does.
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

// closePopover resets the overlay to its zero state.
func (m Model) closePopover() Model {
	m.popover = popoverState{}
	return m
}

// handlePopoverKey dispatches a KeyMsg while the popover is open.
// Returns (handled, model, cmd). When handled is false, the caller
// falls through to normal Update handling.
func (m Model) handlePopoverKey(k tea.KeyMsg) (bool, Model, tea.Cmd) {
	switch {
	case key.Matches(k, popoverKeys.Close):
		return true, m.closePopover(), nil
	case key.Matches(k, popoverKeys.Next):
		if len(m.popover.suggestions) > 0 {
			m.popover.cursor = (m.popover.cursor + 1) % len(m.popover.suggestions)
		}
		return true, m, nil
	case key.Matches(k, popoverKeys.Prev):
		if n := len(m.popover.suggestions); n > 0 {
			m.popover.cursor = (m.popover.cursor - 1 + n) % n
		}
		return true, m, nil
	case key.Matches(k, popoverKeys.Apply):
		return true, m.applySelectedSuggestion(), nil
	case key.Matches(k, popoverKeys.Ignore):
		return true, m.ignoreCurrentWord(), nil
	case key.Matches(k, popoverKeys.Add):
		return true, m.addCurrentWordToWordlist(), nil
	}
	// Digit jump-and-apply.
	if len(k.Runes) == 1 && k.Runes[0] >= '1' && k.Runes[0] <= '9' {
		idx := int(k.Runes[0] - '1')
		if idx < len(m.popover.suggestions) {
			m.popover.cursor = idx
			return true, m.applySelectedSuggestion(), nil
		}
		return true, m, nil
	}
	return false, m, nil
}

func (m Model) applySelectedSuggestion() Model {
	if !m.popover.open || len(m.popover.suggestions) == 0 {
		return m.closePopover()
	}
	repl := m.popover.suggestions[m.popover.cursor]
	src := m.buf.Value()
	r := m.popover.wordRange
	if r.Start < 0 || r.End > len(src) || r.Start >= r.End {
		return m.closePopover()
	}
	newSrc := src[:r.Start] + repl + src[r.End:]
	m.buf.SetValue(newSrc)
	// Position cursor at end of the replacement (rune-counted).
	prefixRunes := len([]rune(src[:r.Start]))
	replRunes := len([]rune(repl))
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

// appendUserWord opens path in append mode (creating it 0o600 if missing)
// and writes word + "\n". Errors are swallowed: a persistence failure is
// recoverable. The in-memory speller picks up the word for this session, and
// surfacing the error from a key handler would require an error channel this
// pass deliberately does not introduce.
func appendUserWord(path, word string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", word)
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

// byteOffsetForRune translates a rune offset to a byte offset against src.
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
