package sidebar

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func TestUpdate_ExpandCollapseOnParentRow(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Lists/golang"}, DisplayName: "Lists/golang", Group: mail.GroupCustom},
		{Folder: mail.Folder{Name: "Lists/rust"}, DisplayName: "Lists/rust", Group: mail.GroupCustom},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons, ansix.NewMeasurer(1))
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

func TestUpdate_MouseClickSelectsRealFolder(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Inbox"}, Canonical: "Inbox", DisplayName: "Inbox", Group: mail.GroupPrimary},
		{Folder: mail.Folder{Name: "Sent"}, Canonical: "Sent", DisplayName: "Sent", Group: mail.GroupPrimary},
		{Folder: mail.Folder{Name: "Trash"}, Canonical: "Trash", DisplayName: "Trash", Group: mail.GroupDisposal},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons, ansix.NewMeasurer(1))
	// Y=1 maps to the second visible row (Sent).
	m, _ = m.Update(tea.MouseClickMsg{X: 2, Y: 1, Button: tea.MouseLeft})
	if m.Selected() != 1 {
		t.Fatalf("click on row 1 should select index 1, got %d", m.Selected())
	}
	// Y=2 is the blank group separator before Trash. Inert.
	m, _ = m.Update(tea.MouseClickMsg{X: 2, Y: 2, Button: tea.MouseLeft})
	if m.Selected() != 1 {
		t.Fatalf("click on group separator should be inert, got %d", m.Selected())
	}
	// Y=3 is Trash (after separator).
	m, _ = m.Update(tea.MouseClickMsg{X: 2, Y: 3, Button: tea.MouseLeft})
	if m.Selected() != 2 {
		t.Fatalf("click on Trash row should select index 2, got %d", m.Selected())
	}
}

func TestUpdate_MouseClickTogglesSynthesizedExpand(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Lists/golang"}, DisplayName: "Lists/golang", Group: mail.GroupCustom},
		{Folder: mail.Folder{Name: "Lists/rust"}, DisplayName: "Lists/rust", Group: mail.GroupCustom},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons, ansix.NewMeasurer(1))
	prev := m.Selected()
	m, _ = m.Update(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft})
	if !m.IsExpanded("Lists") {
		t.Fatal("click on synthesized Lists should expand")
	}
	if m.Selected() != prev {
		t.Fatal("click on synthesized row should not change selection")
	}
	m, _ = m.Update(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft})
	if m.IsExpanded("Lists") {
		t.Fatal("second click should collapse")
	}
}

func TestUpdate_MouseWheelMovesCursor(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Inbox"}, Canonical: "Inbox", DisplayName: "Inbox", Group: mail.GroupPrimary},
		{Folder: mail.Folder{Name: "Sent"}, Canonical: "Sent", DisplayName: "Sent", Group: mail.GroupPrimary},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons, ansix.NewMeasurer(1))
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.Selected() != 1 {
		t.Fatalf("wheel down should advance cursor to 1, got %d", m.Selected())
	}
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if m.Selected() != 0 {
		t.Fatalf("wheel up should retreat cursor to 0, got %d", m.Selected())
	}
}

func TestUpdate_MovementRoutedThroughKeyMap(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "Inbox"}, Canonical: "Inbox", DisplayName: "Inbox", Group: mail.GroupPrimary},
		{Folder: mail.Folder{Name: "Sent"}, Canonical: "Sent", DisplayName: "Sent", Group: mail.GroupPrimary},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 30, 10, uicore.SimpleIcons, ansix.NewMeasurer(1))
	km := DefaultKeyMap()
	m.SetKeyMap(km)

	// Fire the configured Down key (default "j") and expect cursor advance.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.Selected() != 1 {
		t.Fatalf("Down should advance cursor, got %d", m.Selected())
	}
}
