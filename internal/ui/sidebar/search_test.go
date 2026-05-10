package sidebar

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

var ansiReSearch = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSISearch(s string) string {
	return ansiReSearch.ReplaceAllString(s, "")
}

func TestSidebarSearchIdle(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("idle: hint row", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		plain := stripANSISearch(s.View())
		if !strings.Contains(plain, "/ to search") {
			t.Errorf("idle view missing hint: %q", plain)
		}
	})

	t.Run("idle: state", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		if s.State() != SearchIdle {
			t.Errorf("State() = %v, want SearchIdle", s.State())
		}
	})

	t.Run("idle renders exactly 3 rows", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		lines := strings.Split(s.View(), "\n")
		if len(lines) != 3 {
			t.Errorf("idle view rows = %d, want 3", len(lines))
		}
	})

	t.Run("idle Query is empty", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		if s.Query() != "" {
			t.Errorf("Query() = %q, want empty", s.Query())
		}
	})

	t.Run("idle Mode is uicore.ScopeFolder", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		if s.Scope() != uicore.ScopeFolder {
			t.Errorf("Mode() = %v, want uicore.ScopeFolder", s.Scope())
		}
	})
}

func TestSidebarSearchActivate(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("Activate: Idle → Typing", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		if s.State() != SearchTyping {
			t.Errorf("State() = %v, want SearchTyping", s.State())
		}
		if !s.input.Focused() {
			t.Error("input should be focused after Activate")
		}
	})

	t.Run("Clear returns to Idle and resets query", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
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

	t.Run("Clear: mode reset", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s.scope = uicore.ScopeAll
		s.Clear()
		if s.Scope() != uicore.ScopeFolder {
			t.Errorf("Mode() after Clear = %v, want uicore.ScopeFolder", s.Scope())
		}
	})

	t.Run("typing state renders icon + slash + query", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s.input.SetValue("proj")
		plain := stripANSISearch(s.View())
		if !strings.Contains(plain, "󰍉") {
			t.Errorf("typing view missing search icon: %q", plain)
		}
		if !strings.Contains(plain, "/proj") {
			t.Errorf("typing view missing '/proj' prompt: %q", plain)
		}
	})

	t.Run("typing state with query renders [folder] scope badge", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s.input.SetValue("x")
		plain := stripANSISearch(s.View())
		if !strings.Contains(plain, "[folder]") {
			t.Errorf("typing view missing [folder] badge: %q", plain)
		}
	})
}

func TestSidebarSearchCommit(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("Commit transitions Typing → Active", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
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
		s := NewSearch(styles, 30, uicore.FancyIcons)
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
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
		if s.Query() != "pro" {
			t.Errorf("Query() = %q, want 'pro'", s.Query())
		}
	})

	t.Run("Update: SearchUpdatedMsg", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
		if cmd == nil {
			t.Fatal("Update should return a Cmd emitting SearchUpdatedMsg")
		}
		msg := cmd()
		// cmd may be a tea.Batch wrapping multiple cmds. SearchUpdatedMsg
		// might come back wrapped in a BatchMsg. Handle both shapes.
		upd, ok := unwrapSearchUpdated(msg)
		if !ok {
			t.Fatalf("Cmd returned %T, want SearchUpdatedMsg", msg)
		}
		if upd.Query != "p" {
			t.Errorf("SearchUpdatedMsg.Query = %q, want 'p'", upd.Query)
		}
		if upd.Scope != uicore.ScopeFolder {
			t.Errorf("SearchUpdatedMsg.Mode = %v, want uicore.ScopeFolder", upd.Scope)
		}
	})

	t.Run("Backspace during typing emits SearchUpdatedMsg with shorter query", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
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

func TestSidebarSearchScopeToggle(t *testing.T) {
	styles := NewStyles(theme.Nord)

	backslash := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\\'}}

	t.Run("backslash: cycle [folder] → [all folders]", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()

		s, _ = s.Update(backslash)
		if s.Scope() != uicore.ScopeAll {
			t.Errorf("after first \\: Scope = %v, want uicore.ScopeAll", s.Scope())
		}

		s, _ = s.Update(backslash)
		if s.Scope() != uicore.ScopeFolder {
			t.Errorf("after second \\: Scope = %v, want uicore.ScopeFolder", s.Scope())
		}
	})

	t.Run("backslash emits SearchUpdatedMsg with new scope", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s.input.SetValue("proj")

		_, cmd := s.Update(backslash)
		if cmd == nil {
			t.Fatal("\\ should emit a Cmd")
		}
		msg := cmd()
		upd, ok := unwrapSearchUpdated(msg)
		if !ok {
			t.Fatalf("Cmd returned %T, want SearchUpdatedMsg", msg)
		}
		if upd.Scope != uicore.ScopeAll {
			t.Errorf("SearchUpdatedMsg.Scope = %v, want uicore.ScopeAll", upd.Scope)
		}
		if upd.Query != "proj" {
			t.Errorf("SearchUpdatedMsg.Query = %q, want 'proj'", upd.Query)
		}
	})

	t.Run("view shows [all folders] after backslash with non-empty query", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s.input.SetValue("x")
		s, _ = s.Update(backslash)
		plain := stripANSISearch(s.View())
		if !strings.Contains(plain, "[all folders]") {
			t.Errorf("view missing [all folders] badge after \\: %q", plain)
		}
	})
}

func TestSidebarSearchEmptyQuerySuppressesCount(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("info row has no count text when query is empty after Activate", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		lines := strings.Split(stripANSISearch(s.View()), "\n")
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
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s.SetResultCount(5)
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		lines := strings.Split(stripANSISearch(s.View()), "\n")
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
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s.input.SetValue("proj")
		s.SetResultCount(3)
		plain := stripANSISearch(s.View())
		if !strings.Contains(plain, "3 results") {
			t.Errorf("view missing '3 results': %q", plain)
		}
	})

	t.Run("zero results, non-empty query", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s.input.SetValue("asdf")
		s.SetResultCount(0)
		plain := stripANSISearch(s.View())
		if !strings.Contains(plain, "no results") {
			t.Errorf("view missing 'no results': %q", plain)
		}
	})

	t.Run("singular '1 result' for count 1", func(t *testing.T) {
		s := NewSearch(styles, 30, uicore.FancyIcons)
		s.Activate()
		s.input.SetValue("proj")
		s.SetResultCount(1)
		plain := stripANSISearch(s.View())
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
