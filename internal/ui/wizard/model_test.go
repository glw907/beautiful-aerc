package wizard

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
)

func TestNewModelBuildsStyles(t *testing.T) {
	tc := theme.Themes["one-dark"]
	m := NewModel(tc)
	if m.Theme != tc {
		t.Fatalf("Theme not set")
	}
	if len(m.sections) == 0 {
		t.Fatalf("default sections empty")
	}
}

func TestModelHandlesWindowSize(t *testing.T) {
	m := NewModel(theme.Themes["one-dark"])
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := m2.(Model)
	if got.Width != 80 || got.Height != 24 {
		t.Errorf("size = %dx%d, want 80x24", got.Width, got.Height)
	}
}

func TestWithSectionsFiltersByName(t *testing.T) {
	m := NewModel(theme.Themes["one-dark"]).WithSections([]string{"theme"})
	if len(m.sections) != 1 || m.sections[0].Name() != "theme" {
		t.Fatalf("WithSections filtered to %v", m.sections)
	}
}
