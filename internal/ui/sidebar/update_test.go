package sidebar

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func TestUpdate_ExpandCollapseOnParentRow(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Lists/golang"}, DisplayName: "Lists/golang", Group: mail.GroupCustom},
		{Folder: mail.Folder{Name: "Lists/rust"}, DisplayName: "Lists/rust", Group: mail.GroupCustom},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons)
	// Cursor on the synthesized "Lists" parent (only visible Custom row).
	if got := m.SelectedFolder(); got != "Lists" {
		t.Fatalf("want cursor on Lists (synthesized), got %q", got)
	}

	right := tea.KeyPressMsg{Code: tea.KeyRight}
	m, _ = m.Update(right)
	if !m.IsExpanded("Lists") {
		t.Fatal("right arrow on parent should expand")
	}

	left := tea.KeyPressMsg{Code: tea.KeyLeft}
	m, _ = m.Update(left)
	if m.IsExpanded("Lists") {
		t.Fatal("left arrow should collapse")
	}
}

func TestUpdate_MovementRoutedThroughKeyMap(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Inbox"}, Canonical: "Inbox", DisplayName: "Inbox", Group: mail.GroupPrimary},
		{Folder: mail.Folder{Name: "Sent"}, Canonical: "Sent", DisplayName: "Sent", Group: mail.GroupPrimary},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons)
	km := DefaultKeyMap()
	m.SetKeyMap(km)

	// Fire the configured Down key (default "j") and expect cursor advance.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.Selected() != 1 {
		t.Fatalf("Down should advance cursor, got %d", m.Selected())
	}
}
