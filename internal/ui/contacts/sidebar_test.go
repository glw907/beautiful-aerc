package contacts

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
)

func TestSidebar_GroupOrderAndCounts(t *testing.T) {
	s := NewSidebar(NewStyles(theme.OneDark), Fixtures())
	s = s.SetSize(14, 24)
	v := s.View()
	for _, w := range []string{"ABC", "DEF", "GHI", "JKL", "MNO", "PQRS", "TUV", "WXYZ"} {
		if !strings.Contains(v, w) {
			t.Errorf("missing %q\n%s", w, v)
		}
	}
}

func TestSidebar_LetterMicroHighlight(t *testing.T) {
	s := NewSidebar(NewStyles(theme.OneDark), Fixtures())
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if s.activeGroup != 0 {
		t.Errorf("expected ABC group active, got %d", s.activeGroup)
	}
	if s.activeLetter != 'B' {
		t.Errorf("expected letter B, got %c", s.activeLetter)
	}
}

func TestSidebar_WalkGroups(t *testing.T) {
	s := NewSidebar(NewStyles(theme.OneDark), Fixtures())
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if s.activeGroup != 1 {
		t.Errorf("J should advance to DEF group")
	}
}
