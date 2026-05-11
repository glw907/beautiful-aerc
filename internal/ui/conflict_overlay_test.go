package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/theme"
)

func newTestConflictOverlay() ConflictOverlay {
	return NewConflictOverlay(NewStyles(theme.Nord))
}

func makeConflictRows(n int) []cache.ConflictRow {
	out := make([]cache.ConflictRow, n)
	for i := range n {
		out[i] = cache.ConflictRow{
			ID:           int64(100 + i),
			Kind:         cache.KindFlag,
			Folder:       "Inbox",
			ProtocolID:   "c0a8b1d4e5",
			ErrorKind:    "auth-failure",
			ErrorMessage: "invalid creds",
			Attempts:     3,
			EnqueuedAt:   time.Unix(900, 0),
		}
	}
	return out
}

func TestConflictOverlay_HiddenUntilOpen(t *testing.T) {
	o := newTestConflictOverlay().SetSize(120, 40)
	if o.IsOpen() {
		t.Errorf("new overlay should be closed")
	}
	if o.View() != "" {
		t.Errorf("closed overlay rendered non-empty")
	}
}

func TestConflictOverlay_EmptyState(t *testing.T) {
	o := newTestConflictOverlay().SetSize(120, 40)
	o = o.Open(nil)
	out := o.View()
	if !strings.Contains(out, "No conflicts.") {
		t.Errorf("missing empty-state row in:\n%s", out)
	}
	if strings.Contains(out, "r retry") {
		t.Errorf("footer should not contain 'r retry' when empty, got:\n%s", out)
	}
}

func TestConflictOverlay_CursorClampedNoWrap(t *testing.T) {
	o := newTestConflictOverlay().SetSize(120, 40)
	o.nowSec = func() int64 { return 1000 }
	o = o.Open(makeConflictRows(2))

	// k at top: cursor stays 0
	o2, _ := o.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if o2.cursor != 0 {
		t.Errorf("k at top: cursor want 0, got %d", o2.cursor)
	}

	// j: cursor advances 0→1
	o3, _ := o2.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if o3.cursor != 1 {
		t.Errorf("j: cursor want 1, got %d", o3.cursor)
	}

	// j at bottom: cursor stays 1
	o4, _ := o3.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if o4.cursor != 1 {
		t.Errorf("j at bottom: cursor want 1, got %d", o4.cursor)
	}
}

func TestConflictOverlay_RetryEmitsMsg(t *testing.T) {
	o := newTestConflictOverlay().SetSize(120, 40)
	o.nowSec = func() int64 { return 1000 }
	o = o.Open(makeConflictRows(2))

	_, cmd := o.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("r did not emit a Cmd")
	}
	msg, ok := cmd().(RetryConflictMsg)
	if !ok {
		t.Fatalf("r emitted %T, want RetryConflictMsg", cmd())
	}
	if msg.OpID != 100 {
		t.Errorf("RetryConflictMsg.OpID want 100, got %d", msg.OpID)
	}
}

func TestConflictOverlay_DiscardEmitsMsg(t *testing.T) {
	o := newTestConflictOverlay().SetSize(120, 40)
	o.nowSec = func() int64 { return 1000 }
	o = o.Open(makeConflictRows(2))

	// advance cursor to row 1
	o, _ = o.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})

	_, cmd := o.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if cmd == nil {
		t.Fatal("d did not emit a Cmd")
	}
	msg, ok := cmd().(DiscardConflictMsg)
	if !ok {
		t.Fatalf("d emitted %T, want DiscardConflictMsg", cmd())
	}
	if msg.OpID != 101 {
		t.Errorf("DiscardConflictMsg.OpID want 101, got %d", msg.OpID)
	}
}

func TestConflictOverlay_BangAndEscClose(t *testing.T) {
	// ! closes
	o := newTestConflictOverlay().SetSize(120, 40).Open(makeConflictRows(1))
	o2, _ := o.Update(tea.KeyPressMsg{Code: '!', Text: "!"})
	if o2.IsOpen() {
		t.Errorf("! did not close the overlay")
	}

	// Esc closes
	o3 := newTestConflictOverlay().SetSize(120, 40).Open(makeConflictRows(1))
	o4, _ := o3.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if o4.IsOpen() {
		t.Errorf("Esc did not close the overlay")
	}
}

func TestConflictOverlay_OverflowShowsMore(t *testing.T) {
	o := newTestConflictOverlay().SetSize(120, 12)
	o.nowSec = func() int64 { return 1000 }
	o = o.Open(makeConflictRows(10))
	out := o.View()
	if !strings.Contains(out, "more") {
		t.Errorf("expected overflow '+N more' line in:\n%s", out)
	}
}

func TestConflictOverlay_Golden_120x40(t *testing.T) {
	o := newTestConflictOverlay().SetSize(120, 40)
	o.nowSec = func() int64 { return 1000 }
	o = o.Open([]cache.ConflictRow{{
		ID:           42,
		Kind:         cache.KindFlag,
		Folder:       "Inbox",
		ProtocolID:   "abc12345xx",
		ErrorKind:    "auth-failure",
		ErrorMessage: "invalid credentials",
		Attempts:     3,
		EnqueuedAt:   time.Unix(900, 0),
	}})
	box := o.View()
	got := compositeOverlay(box, 120, 40)
	checkGolden(t, "conflict_overlay_120x40.txt", got)
}

func TestConflictOverlay_Golden_80x24(t *testing.T) {
	o := newTestConflictOverlay().SetSize(80, 24)
	o.nowSec = func() int64 { return 1000 }
	o = o.Open([]cache.ConflictRow{{
		ID:           42,
		Kind:         cache.KindFlag,
		Folder:       "Inbox",
		ProtocolID:   "abc12345xx",
		ErrorKind:    "auth-failure",
		ErrorMessage: "invalid credentials",
		Attempts:     3,
		EnqueuedAt:   time.Unix(900, 0),
	}})
	box := o.View()
	got := compositeOverlay(box, 80, 24)
	checkGolden(t, "conflict_overlay_80x24.txt", got)
}
