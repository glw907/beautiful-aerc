package compose

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/theme"
)

func TestSchedulePicker_PresetCommitsTime(t *testing.T) {
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.Local) // Sat
	p := NewSchedulePicker(theme.OneDark, now, "")
	p.moveDown() // tomorrow afternoon
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("preset Enter: cmd nil")
	}
	got, ok := cmd().(ScheduleAcceptedMsg)
	if !ok {
		t.Fatalf("cmd: %T, want ScheduleAcceptedMsg", cmd())
	}
	want := time.Date(2026, 5, 10, 13, 0, 0, 0, time.Local)
	if !got.When.Equal(want) {
		t.Errorf("got %v, want %v", got.When, want)
	}
}

func TestSchedulePicker_CustomExpandsAndParses(t *testing.T) {
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.Local)
	p := NewSchedulePicker(theme.OneDark, now, "")
	for i := 0; i < 3; i++ {
		p.moveDown()
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // expand
	if !p.customOpen {
		t.Fatal("custom row should be open")
	}
	for _, r := range "tomorrow 3pm" {
		p, _ = p.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("custom Enter: cmd nil")
	}
	got, ok := cmd().(ScheduleAcceptedMsg)
	if !ok {
		t.Fatalf("cmd: %T", cmd())
	}
	want := time.Date(2026, 5, 10, 15, 0, 0, 0, time.Local)
	if !got.When.Equal(want) {
		t.Errorf("got %v, want %v", got.When, want)
	}
}

func TestSchedulePicker_CustomShowsParseError(t *testing.T) {
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.Local)
	p := NewSchedulePicker(theme.OneDark, now, "")
	for i := 0; i < 3; i++ {
		p.moveDown()
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, r := range "garbage" {
		p, _ = p.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	p, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("parse error: cmd should be nil")
	}
	if !strings.Contains(p.View(), "not a recognized date") {
		t.Errorf("view should show parse hint:\n%s", p.View())
	}
}

func TestSchedulePicker_EscCancels(t *testing.T) {
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.Local)
	p := NewSchedulePicker(theme.OneDark, now, "")
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	if _, ok := cmd().(ScheduleCancelledMsg); !ok {
		t.Errorf("got %T, want ScheduleCancelledMsg", cmd())
	}
}
