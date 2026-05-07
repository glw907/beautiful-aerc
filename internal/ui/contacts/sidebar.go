// SPDX-License-Identifier: MIT

package contacts

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// t9Groups lists the eight phone-keypad letter groups in fixed order.
// The sidebar renders one row per group, with a per-letter cursor when
// a letter jump is active. Non-letter-starting contacts bin to group 0
// by convention. The fixture pool has only letter starts, so this is a
// safety valve, not a display concern.
var t9Groups = []string{"ABC", "DEF", "GHI", "JKL", "MNO", "PQRS", "TUV", "WXYZ"}

// groupOfLetter maps an uppercase letter to its t9Groups index.
// Letters outside A–Z (should not occur after uppercasing) return 0.
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

// firstSortLetter returns the uppercase letter used to bin a contact
// into a T9 group. For KindPerson it uses Family, falling back to
// Given. For KindOrg it uses Name. Non-letter first runes return 'A'
// (bins to group 0).
func firstSortLetter(c Contact) rune {
	var s string
	if c.Kind == KindPerson {
		s = c.Family
		if s == "" {
			s = c.Given
		}
	} else {
		s = c.Name
	}
	for _, r := range s {
		up := unicode.ToUpper(r)
		if up >= 'A' && up <= 'Z' {
			return up
		}
	}
	return 'A'
}

// Sidebar is the compact T9 letter-group column used in Contacts mode.
// It renders one row per group with a right-aligned count, and shows a
// per-letter ┃ cursor when the user has jumped to a specific letter.
//
// Navigation: J/K walk groups; a–z jump to per-letter precision.
// Letter separators within a group use a single space: "A B C".
type Sidebar struct {
	styles        Styles
	contacts      []Contact
	groupCounts   [8]int
	activeGroup   int
	activeLetter  rune // uppercase A–Z. Zero means group-level selection.
	width, height int
}

// NewSidebar constructs a Sidebar with group counts precomputed from all.
func NewSidebar(s Styles, all []Contact) Sidebar {
	sb := Sidebar{styles: s, contacts: all}
	sb.recount()
	return sb
}

// SelectionLetter reports the currently active letter (uppercase), or 0.
func (s Sidebar) SelectionLetter() rune { return s.activeLetter }

// SelectionGroup reports the index into t9Groups of the active group.
func (s Sidebar) SelectionGroup() int { return s.activeGroup }

// SetSize stores width and height. Returns a new Sidebar (pure).
func (s Sidebar) SetSize(w, h int) Sidebar {
	s.width, s.height = w, h
	return s
}

// Update handles J/K group navigation and a–z letter jumps.
func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
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
		// Lowercase a–z: letter jump.
		if len(k.Runes) == 1 {
			r := k.Runes[0]
			if r >= 'a' && r <= 'z' {
				up := unicode.ToUpper(r)
				s.activeLetter = up
				s.activeGroup = groupOfLetter(up)
			}
		}
	}
	return s, nil
}

// View renders the sidebar. Returns "" when width or height is not set.
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

// renderGroup builds one sidebar row for a T9 group.
func (s Sidebar) renderGroup(idx int, group string) string {
	count := s.groupCounts[idx]
	countStr := s.styles.GroupCount.Render(fmt.Sprintf("%d", count))
	countW := lipgloss.Width(countStr)

	// Budget for the label: width minus count and the space before it.
	labelBudget := s.width - countW - 1
	if labelBudget < 0 {
		labelBudget = 0
	}

	var label string
	if idx == s.activeGroup {
		label = s.renderActiveGroupLabel(group, labelBudget)
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

// renderActiveGroupLabel builds the label for the selected group.
// When activeLetter is set, the matching letter gets a ┃ prefix.
// When only the group is selected (J/K navigation), the whole group
// name is rendered with CursorRow styling.
func (s Sidebar) renderActiveGroupLabel(group string, budget int) string {
	if s.activeLetter == 0 {
		return s.styles.CursorRow.Render(group)
	}

	// Per-letter micro-highlight: "A ┃B C" with spaces between letters.
	letters := []rune(group)
	parts := make([]string, len(letters))
	for i, l := range letters {
		if l == s.activeLetter {
			parts[i] = s.styles.LetterTick.Render("┃" + string(l))
		} else {
			parts[i] = s.styles.GroupLabel.Render(string(l))
		}
	}
	_ = budget // display cells stay within 14 for all groups
	return strings.Join(parts, " ")
}

// recount rebuilds groupCounts from s.contacts.
func (s *Sidebar) recount() {
	var counts [8]int
	for _, c := range s.contacts {
		l := firstSortLetter(c)
		counts[groupOfLetter(l)]++
	}
	s.groupCounts = counts
}
