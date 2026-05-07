package ui

import (
	"database/sql"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/theme"
)

func newTestOutboxOverlay() OutboxOverlay {
	return NewOutboxOverlay(NewStyles(theme.Nord))
}

func TestOutboxOverlay_HiddenUntilOpen(t *testing.T) {
	o := newTestOutboxOverlay().SetSize(120, 40)
	if o.IsOpen() {
		t.Errorf("new overlay should be closed")
	}
	if o.View() != "" {
		t.Errorf("closed overlay rendered non-empty")
	}
}

func TestOutboxOverlay_EmptyState(t *testing.T) {
	o := newTestOutboxOverlay().SetSize(120, 40)
	o = o.Open(nil)
	if !strings.Contains(o.View(), "Outbox is empty.") {
		t.Errorf("missing empty-state row in:\n%s", o.View())
	}
}

func TestOutboxOverlay_RendersGroups(t *testing.T) {
	o := newTestOutboxOverlay().SetSize(120, 40)
	o.nowSec = func() int64 { return 1000 }
	groups := []cache.OutboxGroup{
		{Kind: cache.KindFlag, Folder: "Inbox", Status: cache.OpExecuting, Count: 1},
		{Kind: cache.KindMove, Folder: "Archive", Status: cache.OpPending, Count: 23},
		{Kind: cache.KindDestroy, Folder: "Trash", Status: cache.OpFailed, Count: 1,
			NextAt: sql.NullInt64{Valid: true, Int64: 1012 * int64(1e9)}},
	}
	o = o.Open(groups)
	out := o.View()
	for _, want := range []string{
		"Flag · 1 executing",
		"Move → Archive · 23 pending",
		"Delete · 1 failed, retrying in 12s",
		"! conflicts",
		"q close",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestOutboxOverlay_QSwallowed(t *testing.T) {
	o := newTestOutboxOverlay().SetSize(120, 40).Open(nil)
	o2, cmd := o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !o2.IsOpen() {
		t.Errorf("q should be swallowed (overlay stays open)")
	}
	if cmd != nil {
		t.Errorf("q produced a Cmd: %v", cmd)
	}
}

func TestOutboxOverlay_EscClosesAndCapitalQ(t *testing.T) {
	o := newTestOutboxOverlay().SetSize(120, 40).Open(nil)
	o2, _ := o.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if o2.IsOpen() {
		t.Errorf("esc did not close")
	}
	o3 := newTestOutboxOverlay().SetSize(120, 40).Open(nil)
	o3, _ = o3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	if o3.IsOpen() {
		t.Errorf("Q did not close")
	}
}

func TestOutboxOverlay_BangEmitsTransition(t *testing.T) {
	o := newTestOutboxOverlay().SetSize(120, 40).Open(nil)
	o2, cmd := o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})
	if o2.IsOpen() {
		t.Errorf("! should close outbox overlay")
	}
	if cmd == nil {
		t.Fatal("! did not emit a Cmd")
	}
	if _, ok := cmd().(OpenConflictsFromOutboxMsg); !ok {
		t.Errorf("! emitted %T, want OpenConflictsFromOutboxMsg", cmd())
	}
}

func TestOutboxOverlay_Golden_120x40(t *testing.T) {
	o := newTestOutboxOverlay().SetSize(120, 40)
	o.nowSec = func() int64 { return 1000 }
	o = o.Open([]cache.OutboxGroup{
		{Kind: cache.KindFlag, Folder: "Inbox", Status: cache.OpPending, Count: 2},
		{Kind: cache.KindMove, Folder: "Archive", Status: cache.OpPending, Count: 7},
	})
	box := o.View()
	got := compositeOverlay(box, 120, 40)
	checkGolden(t, "outbox_overlay_120x40.txt", got)
}

func TestOutboxOverlay_Golden_80x24(t *testing.T) {
	o := newTestOutboxOverlay().SetSize(80, 24)
	o.nowSec = func() int64 { return 1000 }
	o = o.Open([]cache.OutboxGroup{
		{Kind: cache.KindFlag, Folder: "Inbox", Status: cache.OpPending, Count: 2},
	})
	box := o.View()
	got := compositeOverlay(box, 80, 24)
	checkGolden(t, "outbox_overlay_80x24.txt", got)
}
