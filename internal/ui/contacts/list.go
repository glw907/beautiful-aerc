// SPDX-License-Identifier: MIT

package contacts

import (
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// SortMode controls the display and sort order of the contact list.
type SortMode int

const (
	SortFirstName SortMode = iota // Given Family for persons, Name for orgs
	SortLastName                  // Family, Given for persons, Name for orgs
)

// listKeys are the bindings for the contact list.
var listKeys = struct {
	J key.Binding
	K key.Binding
	N key.Binding
	E key.Binding
	D key.Binding
}{
	J: key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "down")),
	K: key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "up")),
	N: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new contact")),
	E: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit contact")),
	D: key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete contact")),
}

const (
	colName  = 22
	colEmail = 30
	colPhone = 16
)

// List is the scrollable middle column in Contacts mode. It renders one
// row per contact with name, primary email, primary phone, and role/org.
// Navigation is j/k; n opens a blank form; e opens the cursor contact.
type List struct {
	styles        Styles
	all           []Contact // sorted at construction, never mutated after that
	sortMode      SortMode
	cursor        int
	vp            viewport.Model
	width, height int
}

// NewList builds a List with contacts sorted according to sortMode.
// The caller-supplied slice is copied. The original is not retained.
func NewList(s Styles, all []Contact, sortMode SortMode) List {
	sorted := make([]Contact, len(all))
	copy(sorted, all)
	sortContacts(sorted, sortMode)

	l := List{
		styles:   s,
		all:      sorted,
		sortMode: sortMode,
		vp:       viewport.New(0, 0),
	}
	return l
}

// Cursor returns the contact at the current cursor position.
func (l List) Cursor() Contact {
	if len(l.all) == 0 {
		return Contact{}
	}
	return l.all[l.cursor]
}

// SetSelectionLetter scrolls the cursor to the first contact whose sort
// key starts with letter (case-folded). If no match exists, cursor is
// unchanged.
func (l List) SetSelectionLetter(letter rune) List {
	target := unicode.ToUpper(letter)
	for i, c := range l.all {
		if firstSortLetterMode(c, l.sortMode) == target {
			l.cursor = i
			l.syncViewport()
			return l
		}
	}
	return l
}

// SetSize updates the display area and rebuilds the viewport content.
func (l List) SetSize(w, h int) List {
	l.width, l.height = w, h
	l.vp.Width = w
	l.vp.Height = h
	l.rebuildViewport()
	return l
}

func (l List) Update(msg tea.Msg) (List, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return l, nil
	}

	switch {
	case key.Matches(k, listKeys.J):
		if l.cursor < len(l.all)-1 {
			l.cursor++
			l.syncViewport()
			l.rebuildViewport()
		}
	case key.Matches(k, listKeys.K):
		if l.cursor > 0 {
			l.cursor--
			l.syncViewport()
			l.rebuildViewport()
		}
	case key.Matches(k, listKeys.N):
		return l, func() tea.Msg { return OpenFormMsg{Initial: Contact{}, FromPopover: false} }
	case key.Matches(k, listKeys.E):
		c := l.Cursor()
		return l, func() tea.Msg { return OpenFormMsg{Initial: c, FromPopover: false} }
	case key.Matches(k, listKeys.D):
		// D is intercepted (no-op until 9.3 deletes the contact).
		return l, nil
	}

	return l, nil
}

// View renders the contact list. Returns "" when width or height is zero.
func (l List) View() string {
	if l.width <= 0 || l.height <= 0 {
		return ""
	}
	return l.vp.View()
}

// formatRow renders one contact as a padded multi-column row string.
// The name column is 22 cells, email 30, phone 16, and the remainder
// holds title · org (or just org for KindOrg).
func (l List) formatRow(c Contact) string {
	rest := l.width - colName - colEmail - colPhone - 3 // 3 separating spaces
	if rest < 0 {
		rest = 0
	}

	name := l.nameCol(c)
	email := primaryEmail(c)
	phone := primaryPhone(c)
	meta := metaCol(c)

	namePad := uicore.PadOrTruncate(name, colName)
	emailPad := uicore.PadOrTruncate(email, colEmail)
	phonePad := uicore.PadOrTruncate(phone, colPhone)
	metaTrunc := uicore.TruncateToWidth(meta, rest)

	return namePad + " " + emailPad + " " + phonePad + " " + metaTrunc
}

func (l List) nameCol(c Contact) string {
	if c.Kind == KindOrg {
		return c.Name
	}
	if l.sortMode == SortLastName && c.Family != "" {
		if c.Given != "" {
			return c.Family + ", " + c.Given
		}
		return c.Family
	}
	return c.Name
}

func (l *List) rebuildViewport() {
	rows := make([]string, len(l.all))
	for i, c := range l.all {
		row := l.formatRow(c)
		if i == l.cursor {
			row = l.styles.CursorRow.Render(row)
		}
		rows[i] = row
	}
	l.vp.SetContent(strings.Join(rows, "\n"))
}

func (l *List) syncViewport() {
	if l.height <= 0 {
		return
	}
	top := l.vp.YOffset
	bottom := top + l.height - 1
	if l.cursor < top {
		l.vp.SetYOffset(l.cursor)
	} else if l.cursor > bottom {
		l.vp.SetYOffset(l.cursor - l.height + 1)
	}
}

func firstSortLetterMode(c Contact, mode SortMode) rune {
	var s string
	if c.Kind == KindOrg {
		s = c.Name
	} else if mode == SortLastName && c.Family != "" {
		s = c.Family
	} else {
		s = c.Given
		if s == "" {
			s = c.Name
		}
	}
	for _, r := range s {
		up := unicode.ToUpper(r)
		if up >= 'A' && up <= 'Z' {
			return up
		}
	}
	return 'A'
}

func sortContacts(cs []Contact, mode SortMode) {
	sort.SliceStable(cs, func(i, j int) bool {
		return sortKey(cs[i], mode) < sortKey(cs[j], mode)
	})
}

func sortKey(c Contact, mode SortMode) string {
	if c.Kind == KindOrg {
		return strings.ToLower(c.Name)
	}
	if mode == SortLastName {
		return strings.ToLower(c.Family + "\x00" + c.Given)
	}
	return strings.ToLower(c.Given + "\x00" + c.Family)
}

func primaryEmail(c Contact) string {
	if len(c.Emails) == 0 {
		return ""
	}
	return c.Emails[0].Address
}

func primaryPhone(c Contact) string {
	if len(c.Phones) == 0 {
		return ""
	}
	return c.Phones[0].E164
}

// metaCol returns the title · org string, or just org/name-note for KindOrg.
func metaCol(c Contact) string {
	if c.Kind == KindOrg {
		// Use first line of Note as supplementary text when Name is the only identity.
		if c.Note != "" {
			note := strings.SplitN(c.Note, "\n", 2)[0]
			return note
		}
		return ""
	}
	switch {
	case c.Title != "" && c.Org != "":
		return c.Title + " · " + c.Org
	case c.Title != "":
		return c.Title
	case c.Org != "":
		return c.Org
	default:
		return ""
	}
}
