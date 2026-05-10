package outbox

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/theme"
)

func TestModel_RendersEmptyState(t *testing.T) {
	m := New(theme.OneDark)
	m.SetSize(80, 20)
	if !strings.Contains(m.View(), "Outbox is empty") {
		t.Errorf("empty state missing:\n%s", m.View())
	}
}

func TestModel_RendersRows(t *testing.T) {
	m := New(theme.OneDark)
	m.SetSize(80, 20)
	when := time.Now().Add(2 * time.Hour)
	m.SetRows([]cache.OutboxRow{{
		ID: 1, Kind: cache.KindSend, Subject: "deploy plan",
		To: []string{"a@x"}, ScheduledFor: when, Status: cache.OpPending,
	}})
	v := m.View()
	if !strings.Contains(v, "deploy plan") {
		t.Errorf("subject missing:\n%s", v)
	}
	if !strings.Contains(v, "a@x") {
		t.Errorf("recipient missing:\n%s", v)
	}
}

func TestModel_CancelEmitsMsg(t *testing.T) {
	m := New(theme.OneDark)
	m.SetSize(80, 20)
	m.SetRows([]cache.OutboxRow{
		{ID: 7, Kind: cache.KindSend, Subject: "x", ScheduledFor: time.Now().Add(time.Hour)},
	})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	got, ok := cmd().(CancelMsg)
	if !ok {
		t.Fatalf("got %T, want CancelMsg", cmd())
	}
	if got.OpID != 7 {
		t.Errorf("OpID: got %d, want 7", got.OpID)
	}
}

func TestModel_ReschedulePrefilledFromRow(t *testing.T) {
	m := New(theme.OneDark)
	m.SetSize(80, 20)
	when := time.Date(2026, 6, 1, 9, 0, 0, 0, time.Local)
	m.SetRows([]cache.OutboxRow{{ID: 9, ScheduledFor: when}})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	got, ok := cmd().(RescheduleMsg)
	if !ok {
		t.Fatalf("got %T, want RescheduleMsg", cmd())
	}
	if got.OpID != 9 {
		t.Errorf("OpID: got %d, want 9", got.OpID)
	}
	if got.Initial != "2026-06-01 09:00" {
		t.Errorf("Initial: got %q, want 2026-06-01 09:00", got.Initial)
	}
}

func TestModel_EditAsDraftEmitsMsg(t *testing.T) {
	m := New(theme.OneDark)
	m.SetSize(80, 20)
	d := &cache.DraftRow{DraftID: "d1"}
	m.SetRows([]cache.OutboxRow{{ID: 4, Draft: d}})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	got, ok := cmd().(EditAsDraftMsg)
	if !ok {
		t.Fatalf("got %T, want EditAsDraftMsg", cmd())
	}
	if got.Draft == nil || got.Draft.DraftID != "d1" {
		t.Errorf("draft not threaded: %+v", got.Draft)
	}
}

func TestModel_EditAsDraftWithoutDraftIsInert(t *testing.T) {
	m := New(theme.OneDark)
	m.SetSize(80, 20)
	m.SetRows([]cache.OutboxRow{{ID: 4, Draft: nil}})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd == nil {
		return
	}
	if _, ok := cmd().(EditAsDraftMsg); ok {
		t.Errorf("e should be inert when Draft is nil")
	}
}
