package contacts

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// t9Groups are the eight phone-keypad letter groups in fixed order.
// Non-letter-starting contacts bin to group 0 as a safety valve.
var t9Groups = []string{"ABC", "DEF", "GHI", "JKL", "MNO", "PQRS", "TUV", "WXYZ"}

// groupOfLetter maps an uppercase letter to its t9Groups index. Anything
// outside A–Z bins to 0.
func groupOfLetter(r rune) int {
	switch {
	case r >= 'A' && r <= 'C':
		return 0
	case r >= 'D' && r <= 'F':
		return 1
	case r >= 'G' && r <= 'I':
		return 2
	case r >= 'J' && r <= 'L':
		return 3
	case r >= 'M' && r <= 'O':
		return 4
	case r >= 'P' && r <= 'S':
		return 5
	case r >= 'T' && r <= 'V':
		return 6
	case r >= 'W' && r <= 'Z':
		return 7
	default:
		return 0
	}
}

// Sidebar is the compact T9 letter-group column used in Contacts mode.
// J/K walk groups; a–z jump to per-letter precision with a ┃ cursor.
type Sidebar struct {
	styles        Styles
	contacts      []Contact
	groupCounts   [8]int
	activeGroup   int
	activeLetter  rune // uppercase A–Z. Zero means group-level selection.
	width, height int
}

func NewSidebar(s Styles, all []Contact) Sidebar {
	sb := Sidebar{styles: s, contacts: all}
	sb.recount()
	return sb
}

// SelectionLetter is the active letter (uppercase) or 0.
func (s Sidebar) SelectionLetter() rune { return s.activeLetter }

// SelectionGroup is the t9Groups index of the active group.
func (s Sidebar) SelectionGroup() int { return s.activeGroup }

func (s Sidebar) SetSize(w, h int) Sidebar {
	s.width, s.height = w, h
	return s
}

func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch {
	case key.Matches(k, keys.JUpper):
		if s.activeGroup < len(t9Groups)-1 {
			s.activeGroup++
		}
		s.activeLetter = 0
	case key.Matches(k, keys.KUpper):
		if s.activeGroup > 0 {
			s.activeGroup--
		}
		s.activeLetter = 0
	default:
		if len(k.Text) == 1 {
			r := k.Code
			if r >= 'a' && r <= 'z' {
				up := unicode.ToUpper(r)
				s.activeLetter = up
				s.activeGroup = groupOfLetter(up)
			}
		}
	}
	return s, nil
}

func (s Sidebar) View() string {
	if s.width <= 0 || s.height <= 0 {
		return ""
	}

	var rows []string
	for i, group := range t9Groups {
		row := s.renderGroup(i, group)
		rows = append(rows, row)
		if i < len(t9Groups)-1 {
			rows = append(rows, strings.Repeat(" ", s.width))
		}
	}
	return strings.Join(rows, "\n")
}

func (s Sidebar) renderGroup(idx int, group string) string {
	count := s.groupCounts[idx]
	countStr := s.styles.GroupCount.Render(fmt.Sprintf("%d", count))
	countW := lipgloss.Width(countStr)

	var label string
	if idx == s.activeGroup {
		label = s.renderActiveGroupLabel(group)
	} else {
		label = s.styles.GroupLabel.Render(group)
	}

	labelW := lipgloss.Width(label)
	gap := s.width - labelW - countW
	if gap < 0 {
		gap = 0
	}
	return label + strings.Repeat(" ", gap) + countStr
}

// renderActiveGroupLabel labels the active group. With activeLetter set
// the matching letter gets a ┃ prefix ("A ┃B C"); otherwise the whole
// group renders with CursorRow styling.
func (s Sidebar) renderActiveGroupLabel(group string) string {
	if s.activeLetter == 0 {
		return s.styles.CursorRow.Render(group)
	}

	letters := []rune(group)
	parts := make([]string, len(letters))
	for i, l := range letters {
		if l == s.activeLetter {
			parts[i] = s.styles.LetterTick.Render("┃" + string(l))
		} else {
			parts[i] = s.styles.GroupLabel.Render(string(l))
		}
	}
	return strings.Join(parts, " ")
}

func (s *Sidebar) recount() {
	var counts [8]int
	for _, c := range s.contacts {
		l := firstSortLetterMode(c, SortLastName)
		counts[groupOfLetter(l)]++
	}
	s.groupCounts = counts
}
