// SPDX-License-Identifier: MIT

package contacts

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// smallFixture returns a short, predictable contact list for tests
// that need fine-grained index control independent of the full fixture pool.
func smallFixture() []Contact {
	return []Contact{
		{Kind: KindPerson, Name: "Alice Chen", Given: "Alice", Family: "Chen",
			Org: "ACME", Title: "Senior Engineer",
			Emails: []Email{{Address: "alice@example.com"}},
			Phones: []Phone{{E164: "+15555550100"}},
		},
		{Kind: KindPerson, Name: "Bob Iyer", Given: "Bob", Family: "Iyer",
			Emails: []Email{{Address: "bob@iyer.dev"}},
		},
		{Kind: KindPerson, Name: "Mara Svensson", Given: "Mara", Family: "Svensson",
			Emails: []Email{{Address: "mara@svensson.example"}},
		},
		{Kind: KindOrg, Name: "ACME Support",
			Emails: []Email{{Address: "support@acme.com"}},
		},
	}
}

func TestList_CursorJK(t *testing.T) {
	contacts := smallFixture()
	l := NewList(Styles{}, contacts, SortFirstName)
	l = l.SetSize(80, 20)

	if l.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", l.cursor)
	}

	// j advances
	j := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	l, _ = l.Update(j)
	if l.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", l.cursor)
	}

	// j again
	l, _ = l.Update(j)
	if l.cursor != 2 {
		t.Errorf("after j×2: cursor = %d, want 2", l.cursor)
	}

	// k retreats
	k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	l, _ = l.Update(k)
	if l.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", l.cursor)
	}
}

func TestList_CursorClamped(t *testing.T) {
	contacts := smallFixture()
	l := NewList(Styles{}, contacts, SortFirstName)
	l = l.SetSize(80, 20)

	// k at top is a no-op
	k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	l, _ = l.Update(k)
	if l.cursor != 0 {
		t.Errorf("k at top: cursor = %d, want 0", l.cursor)
	}

	// advance to last row
	j := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	for i := 0; i < len(contacts)-1; i++ {
		l, _ = l.Update(j)
	}
	last := len(contacts) - 1
	if l.cursor != last {
		t.Fatalf("cursor at last row = %d, want %d", l.cursor, last)
	}

	// j past end is a no-op
	l, _ = l.Update(j)
	if l.cursor != last {
		t.Errorf("j past end: cursor = %d, want %d", l.cursor, last)
	}
}

func TestList_NEmitsOpenFormMsgZero(t *testing.T) {
	l := NewList(Styles{}, smallFixture(), SortFirstName)
	l = l.SetSize(80, 20)

	n := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	_, cmd := l.Update(n)
	if cmd == nil {
		t.Fatal("n: want a non-nil tea.Cmd")
	}
	msg := cmd()
	got, ok := msg.(OpenFormMsg)
	if !ok {
		t.Fatalf("n cmd returned %T, want OpenFormMsg", msg)
	}
	if got.FromPopover {
		t.Error("n: FromPopover should be false")
	}
	// Initial must be zero-value Contact
	if got.Initial.Name != "" || got.Initial.Given != "" || len(got.Initial.Emails) != 0 {
		t.Errorf("n: Initial not zero, got %+v", got.Initial)
	}
}

func TestList_EEmitsOpenFormMsgCursor(t *testing.T) {
	contacts := smallFixture()
	l := NewList(Styles{}, contacts, SortFirstName)
	l = l.SetSize(80, 20)

	// advance to index 1
	j := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	l, _ = l.Update(j)

	e := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	_, cmd := l.Update(e)
	if cmd == nil {
		t.Fatal("e: want a non-nil tea.Cmd")
	}
	msg := cmd()
	got, ok := msg.(OpenFormMsg)
	if !ok {
		t.Fatalf("e cmd returned %T, want OpenFormMsg", msg)
	}
	if got.FromPopover {
		t.Error("e: FromPopover should be false")
	}
	cursor := l.Cursor()
	if got.Initial.Name != cursor.Name {
		t.Errorf("e: Initial.Name = %q, want %q", got.Initial.Name, cursor.Name)
	}
}

func TestList_DIsInert(t *testing.T) {
	l := NewList(Styles{}, smallFixture(), SortFirstName)
	l = l.SetSize(80, 20)

	D := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
	l2, cmd := l.Update(D)
	if cmd != nil {
		t.Error("D: expected nil cmd (inert)")
	}
	if l2.cursor != l.cursor {
		t.Error("D: cursor should not move")
	}
}

func TestList_SetSelectionLetterN(t *testing.T) {
	// Use full fixture pool. Under SortFirstName, 'N' maps to Nadia (Given="Nadia").
	all := Fixtures()
	l := NewList(Styles{}, all, SortFirstName)
	l = l.SetSize(80, 20)

	l = l.SetSelectionLetter('N')
	c := l.Cursor()
	sl := firstSortLetterMode(c, SortFirstName)
	if sl != 'N' {
		t.Errorf("SetSelectionLetter('N'): cursor on %q (sort key starts with %c), want N", c.Name, sl)
	}
}

func TestList_SetSelectionLetterLastName(t *testing.T) {
	all := Fixtures()
	l := NewList(Styles{}, all, SortLastName)
	l = l.SetSize(80, 20)

	// Find what letter 'C' resolves to under SortLastName.
	l = l.SetSelectionLetter('C')
	c := l.Cursor()
	sl := firstSortLetterMode(c, SortLastName)
	if sl != 'C' {
		t.Errorf("SetSelectionLetter('C') last-name sort: cursor on %q (key=%c), want C", c.Name, sl)
	}
}

func TestList_RowFormatFirstName(t *testing.T) {
	contacts := []Contact{
		{Kind: KindPerson, Name: "Alice Chen", Given: "Alice", Family: "Chen",
			Org: "ACME", Title: "Senior Engineer",
			Emails: []Email{{Address: "alice@example.com"}},
			Phones: []Phone{{E164: "+15555550100"}},
		},
	}
	l := NewList(Styles{}, contacts, SortFirstName)
	l = l.SetSize(120, 20)
	row := l.formatRow(contacts[0])

	// SortFirstName person: "Given Family" appears before the comma-separated form.
	if !strings.Contains(row, "Alice Chen") {
		t.Errorf("first-name row missing 'Alice Chen': %q", row)
	}
	if strings.Contains(row, "Chen, Alice") {
		t.Errorf("first-name row should not contain 'Chen, Alice': %q", row)
	}
}

func TestList_RowFormatLastName(t *testing.T) {
	contacts := []Contact{
		{Kind: KindPerson, Name: "Alice Chen", Given: "Alice", Family: "Chen",
			Org: "ACME", Title: "Senior Engineer",
			Emails: []Email{{Address: "alice@example.com"}},
			Phones: []Phone{{E164: "+15555550100"}},
		},
	}
	l := NewList(Styles{}, contacts, SortLastName)
	l = l.SetSize(120, 20)
	row := l.formatRow(contacts[0])

	// SortLastName person: "Family, Given" in the name column.
	if !strings.Contains(row, "Chen, Alice") {
		t.Errorf("last-name row missing 'Chen, Alice': %q", row)
	}
}

func TestList_ViewSelfGuardZeroSize(t *testing.T) {
	l := NewList(Styles{}, smallFixture(), SortFirstName)
	// No SetSize call. Width and height are zero.
	if v := l.View(); v != "" {
		t.Errorf("zero-size View() = %q, want empty string", v)
	}
}
