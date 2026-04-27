package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
)

func TestSidebarSearchIdle(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("idle state shows hint row", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		plain := stripANSI(s.View())
		if !strings.Contains(plain, "/ to search") {
			t.Errorf("idle view missing hint: %q", plain)
		}
	})

	t.Run("idle state reports SearchIdle", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		if s.State() != SearchIdle {
			t.Errorf("State() = %v, want SearchIdle", s.State())
		}
	})

	t.Run("idle renders exactly 3 rows", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		lines := strings.Split(s.View(), "\n")
		if len(lines) != 3 {
			t.Errorf("idle view rows = %d, want 3", len(lines))
		}
	})

	t.Run("idle Query is empty", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		if s.Query() != "" {
			t.Errorf("Query() = %q, want empty", s.Query())
		}
	})

	t.Run("idle Mode is SearchModeName", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		if s.Mode() != SearchModeName {
			t.Errorf("Mode() = %v, want SearchModeName", s.Mode())
		}
	})
}

func TestSidebarSearchActivate(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("Activate transitions Idle → Typing and focuses input", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		if s.State() != SearchTyping {
			t.Errorf("State() = %v, want SearchTyping", s.State())
		}
		if !s.input.Focused() {
			t.Error("input should be focused after Activate")
		}
	})

	t.Run("Clear returns to Idle and resets query", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("hello")
		s.Clear()
		if s.State() != SearchIdle {
			t.Errorf("State() = %v, want SearchIdle", s.State())
		}
		if s.Query() != "" {
			t.Errorf("Query() = %q, want empty", s.Query())
		}
		if s.input.Focused() {
			t.Error("input should be blurred after Clear")
		}
	})

	t.Run("Clear also resets mode to SearchModeName", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.mode = SearchModeAll
		s.Clear()
		if s.Mode() != SearchModeName {
			t.Errorf("Mode() after Clear = %v, want SearchModeName", s.Mode())
		}
	})

	t.Run("typing state renders icon + slash + query", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("proj")
		plain := stripANSI(s.View())
		if !strings.Contains(plain, "󰍉") {
			t.Errorf("typing view missing search icon: %q", plain)
		}
		if !strings.Contains(plain, "/proj") {
			t.Errorf("typing view missing '/proj' prompt: %q", plain)
		}
	})

	t.Run("typing state with query renders [name] mode badge", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("x")
		plain := stripANSI(s.View())
		if !strings.Contains(plain, "[name]") {
			t.Errorf("typing view missing [name] badge: %q", plain)
		}
	})
}

func TestSidebarSearchCommit(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("Commit transitions Typing → Active", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("hello")
		s.Commit()
		if s.State() != SearchActive {
			t.Errorf("State() = %v, want SearchActive", s.State())
		}
		if s.Query() != "hello" {
			t.Errorf("Query() preserved = %q, want 'hello'", s.Query())
		}
		if s.input.Focused() {
			t.Error("input should be blurred in Active state")
		}
	})

	t.Run("re-Activate from Active preserves query", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("hello")
		s.Commit()
		s.Activate()
		if s.State() != SearchTyping {
			t.Errorf("State() = %v, want SearchTyping", s.State())
		}
		if s.Query() != "hello" {
			t.Errorf("Query() preserved = %q, want 'hello'", s.Query())
		}
		if !s.input.Focused() {
			t.Error("input should be focused after re-Activate")
		}
	})
}

func TestSidebarSearchUpdate(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("printable rune during typing appends to query", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
		if s.Query() != "pro" {
			t.Errorf("Query() = %q, want 'pro'", s.Query())
		}
	})

	t.Run("Update emits SearchUpdatedMsg on keystroke", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
		if cmd == nil {
			t.Fatal("Update should return a Cmd emitting SearchUpdatedMsg")
		}
		msg := cmd()
		// cmd may be a tea.Batch wrapping multiple cmds; SearchUpdatedMsg
		// might come back wrapped in a BatchMsg. Handle both shapes.
		upd, ok := unwrapSearchUpdated(msg)
		if !ok {
			t.Fatalf("Cmd returned %T, want SearchUpdatedMsg", msg)
		}
		if upd.Query != "p" {
			t.Errorf("SearchUpdatedMsg.Query = %q, want 'p'", upd.Query)
		}
		if upd.Mode != SearchModeName {
			t.Errorf("SearchUpdatedMsg.Mode = %v, want SearchModeName", upd.Mode)
		}
	})

	t.Run("Backspace during typing emits SearchUpdatedMsg with shorter query", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("proj")
		var cmd tea.Cmd
		s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		if s.Query() != "pro" {
			t.Errorf("Query() after backspace = %q, want 'pro'", s.Query())
		}
		msg := cmd()
		upd, ok := unwrapSearchUpdated(msg)
		if !ok || upd.Query != "pro" {
			t.Errorf("expected SearchUpdatedMsg{Query: 'pro'}, got %v", msg)
		}
	})
}

func TestSidebarSearchModeCycle(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("Tab cycles mode [name] → [all] → [name]", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()

		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		if s.Mode() != SearchModeAll {
			t.Errorf("after first Tab: Mode = %v, want SearchModeAll", s.Mode())
		}

		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		if s.Mode() != SearchModeName {
			t.Errorf("after second Tab: Mode = %v, want SearchModeName", s.Mode())
		}
	})

	t.Run("Tab cycle emits SearchUpdatedMsg with new mode", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("proj")

		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyTab})
		if cmd == nil {
			t.Fatal("Tab should emit a Cmd")
		}
		msg := cmd()
		upd, ok := unwrapSearchUpdated(msg)
		if !ok {
			t.Fatalf("Cmd returned %T, want SearchUpdatedMsg", msg)
		}
		if upd.Mode != SearchModeAll {
			t.Errorf("SearchUpdatedMsg.Mode = %v, want SearchModeAll", upd.Mode)
		}
		if upd.Query != "proj" {
			t.Errorf("SearchUpdatedMsg.Query = %q, want 'proj'", upd.Query)
		}
	})

	t.Run("view shows [all] after Tab with non-empty query", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("x")
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		plain := stripANSI(s.View())
		if !strings.Contains(plain, "[all]") {
			t.Errorf("view missing [all] badge after Tab: %q", plain)
		}
	})
}

func TestSidebarSearchEmptyQuerySuppressesCount(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("info row has no count text when query is empty after Activate", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		lines := strings.Split(stripANSI(s.View()), "\n")
		if len(lines) != 3 {
			t.Fatalf("view has %d rows, want 3", len(lines))
		}
		infoRow := lines[2]
		if strings.Contains(infoRow, "results") || strings.Contains(infoRow, "result") {
			t.Errorf("info row should have no count with empty query, got %q", infoRow)
		}
		for _, ch := range infoRow {
			if ch >= '0' && ch <= '9' {
				t.Errorf("info row should contain no digit with empty query, got %q", infoRow)
				break
			}
		}
	})

	t.Run("count appears after typing one character", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.SetResultCount(5)
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		lines := strings.Split(stripANSI(s.View()), "\n")
		if len(lines) != 3 {
			t.Fatalf("view has %d rows, want 3", len(lines))
		}
		infoRow := lines[2]
		if !strings.Contains(infoRow, "results") {
			t.Errorf("info row should show count after typing, got %q", infoRow)
		}
	})
}

func TestSidebarSearchResultCount(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("SetResultCount stores the value", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("proj")
		s.SetResultCount(3)
		plain := stripANSI(s.View())
		if !strings.Contains(plain, "3 results") {
			t.Errorf("view missing '3 results': %q", plain)
		}
	})

	t.Run("zero results with non-empty query shows 'no results'", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("asdf")
		s.SetResultCount(0)
		plain := stripANSI(s.View())
		if !strings.Contains(plain, "no results") {
			t.Errorf("view missing 'no results': %q", plain)
		}
	})

	t.Run("singular '1 result' for count 1", func(t *testing.T) {
		s := NewSidebarSearch(styles, 30)
		s.Activate()
		s.input.SetValue("proj")
		s.SetResultCount(1)
		plain := stripANSI(s.View())
		if !strings.Contains(plain, "1 result") {
			t.Errorf("view missing '1 result': %q", plain)
		}
	})
}

// unwrapSearchUpdated walks the result of running a Cmd, looking
// past tea.BatchMsg wrappers, and returns the first SearchUpdatedMsg
// it finds. tea.Batch packs its child Cmds into a BatchMsg of Cmds
// rather than running them eagerly, so we run any embedded Cmds too.
func unwrapSearchUpdated(msg tea.Msg) (SearchUpdatedMsg, bool) {
	if upd, ok := msg.(SearchUpdatedMsg); ok {
		return upd, true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if upd, ok := unwrapSearchUpdated(c()); ok {
				return upd, true
			}
		}
	}
	return SearchUpdatedMsg{}, false
}
