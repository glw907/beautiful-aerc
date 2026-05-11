package movepicker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

func sampleFolders() []mail.FolderEntry {
	return []mail.FolderEntry{
		{Display: "Inbox", Provider: "INBOX", Group: mail.GroupPrimary},
		{Display: "Drafts", Provider: "Drafts", Group: mail.GroupPrimary},
		{Display: "Sent", Provider: "Sent", Group: mail.GroupPrimary},
		{Display: "Archive", Provider: "Archive", Group: mail.GroupDisposal},
		{Display: "Trash", Provider: "Trash", Group: mail.GroupDisposal},
		{Display: "Receipts/2026", Provider: "Receipts/2026", Group: mail.GroupCustom},
		{Display: "Receipts/2025", Provider: "Receipts/2025", Group: mail.GroupCustom},
	}
}

func newTestPicker() Model {
	ct := theme.Themes[theme.DefaultThemeName]
	return New(NewStyles(ct))
}

func TestMovePicker_OpenSetsState(t *testing.T) {
	p := newTestPicker()
	p = p.Open([]mail.UID{"1", "2"}, "INBOX", sampleFolders())
	if !p.IsOpen() {
		t.Fatal("picker should be open after Open")
	}
	if got, want := p.Len(), len(sampleFolders())-1; got != want {
		t.Errorf("Len = %d, want %d", got, want)
	}
	if got := p.MatchCount(); got != p.Len() {
		t.Errorf("MatchCount = %d, want %d (no filter)", got, p.Len())
	}
	if got := p.list.Index(); got != 0 {
		t.Errorf("cursor = %d, want 0", got)
	}
}

func TestMovePicker_FilterNarrows(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	p, _ = p.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	p, _ = p.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if got := p.Filter(); got != "rec" {
		t.Errorf("Filter = %q, want %q", got, "rec")
	}
	if got := p.MatchCount(); got != 2 {
		t.Errorf("MatchCount = %d, want 2 (Receipts/2026, Receipts/2025)", got)
	}
}

func TestMovePicker_FilterCaseInsensitive(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyPressMsg{Code: 'I', Text: "I"})
	if got := p.MatchCount(); got == 0 {
		t.Fatal("expected matches for 'I', got 0")
	}
}

func TestMovePicker_BackspaceWidens(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	p, _ = p.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got := p.Filter(); got != "r" {
		t.Errorf("Filter after backspace = %q, want %q", got, "r")
	}
}

func TestMovePicker_BackspaceEmptyNoOp(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got := p.Filter(); got != "" {
		t.Errorf("Filter = %q, want empty", got)
	}
}

func TestMovePicker_CursorClampsOnFilter(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p = p.SetSize(60, 16)
	for i := 0; i < 5; i++ {
		p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	// After navigating down, filtering should reset cursor to 0.
	p, _ = p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if got := p.list.Index(); got != 0 {
		t.Errorf("cursor = %d, want 0 after filter change", got)
	}
}

func TestMovePicker_NavigationBounds(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p = p.SetSize(60, 16)
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if got := p.list.Index(); got != 0 {
		t.Errorf("up at top: cursor = %d, want 0", got)
	}
	for i := 0; i < 100; i++ {
		p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if got, want := p.list.Index(), p.MatchCount()-1; got != want {
		t.Errorf("down past bottom: cursor = %d, want %d", got, want)
	}
}

func TestMovePicker_EnterEmitsPickedMsg(t *testing.T) {
	// INBOX excluded. p.all = [Drafts, Sent, Archive, Trash, Receipts/2026, Receipts/2025]
	// cursor=0 is Drafts.
	p := newTestPicker().Open([]mail.UID{"42"}, "INBOX", sampleFolders())
	p = p.SetSize(60, 16)
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter returned nil cmd")
	}
	msgs := drainBatch(cmd)
	var picked *PickedMsg
	var sawClosed bool
	for _, m := range msgs {
		switch v := m.(type) {
		case PickedMsg:
			picked = &v
		case ClosedMsg:
			sawClosed = true
		}
	}
	if picked == nil {
		t.Fatal("did not see PickedMsg")
	}
	if !sawClosed {
		t.Error("did not see ClosedMsg")
	}
	if picked.Dest != "Drafts" {
		t.Errorf("Dest = %q, want %q", picked.Dest, "Drafts")
	}
	if picked.Src != "INBOX" {
		t.Errorf("Src = %q, want %q", picked.Src, "INBOX")
	}
	if len(picked.UIDs) != 1 || picked.UIDs[0] != "42" {
		t.Errorf("UIDs = %v, want [42]", picked.UIDs)
	}
}

func TestMovePicker_EnterInertOnEmpty(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p = p.SetSize(60, 16)
	for _, r := range "zzzzz" {
		p, _ = p.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if got := p.MatchCount(); got != 0 {
		t.Fatalf("MatchCount = %d, want 0 (precondition)", got)
	}
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("Enter on empty matches returned non-nil cmd")
	}
}

func TestMovePicker_EscClosesNoOp(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc returned nil cmd")
	}
	msgs := drainBatch(cmd)
	var sawClosed bool
	for _, m := range msgs {
		if _, ok := m.(ClosedMsg); ok {
			sawClosed = true
		}
		if _, ok := m.(PickedMsg); ok {
			t.Error("Esc emitted PickedMsg")
		}
	}
	if !sawClosed {
		t.Error("Esc did not emit ClosedMsg")
	}
}

func TestMovePicker_QSwallowed(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	beforeFilter := p.Filter()
	p2, cmd := p.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		t.Errorf("q produced cmd, want nil (swallowed)")
	}
	if p2.Filter() != beforeFilter {
		t.Errorf("q modified filter to %q, want unchanged %q", p2.Filter(), beforeFilter)
	}
}

func TestMovePicker_BoxFitsWidth(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 24)
	box := p.Box(80, 24)
	for i, line := range strings.Split(box, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line %d width = %d, want <= 80: %q", i, w, line)
		}
	}
}

func TestMovePicker_BoxHeightBounded(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 24)
	box := p.Box(80, 24)
	if h := strings.Count(box, "\n") + 1; h > 24 {
		t.Errorf("box height = %d, want <= 24", h)
	}
}

func TestMovePicker_RendersAllFolders(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 30)
	box := p.Box(80, 30)
	for _, want := range []string{"Inbox", "Trash", "Receipts/2026"} {
		if !strings.Contains(box, want) {
			t.Errorf("box missing %q", want)
		}
	}
}

func TestMovePicker_HelpRowAlwaysShown(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 24)
	box := p.Box(80, 24)
	if !strings.Contains(box, "select") || !strings.Contains(box, "pick") || !strings.Contains(box, "cancel") {
		t.Errorf("box missing help row, got:\n%s", box)
	}
}

func TestMovePicker_ViewClosedEmpty(t *testing.T) {
	p := newTestPicker()
	if p.View() != "" {
		t.Errorf("closed picker View = %q, want empty", p.View())
	}
}

func TestMovepicker_listModel_cursorAndFilter(t *testing.T) {
	folders := []mail.FolderEntry{
		{Provider: "INBOX", Display: "Inbox"},
		{Provider: "Archive", Display: "Archive"},
		{Provider: "Sent", Display: "Sent"},
		{Provider: "Junk", Display: "Junk"},
	}
	p := newTestPicker().Open([]mail.UID{mail.UID("42")}, "INBOX", folders)
	p = p.SetSize(60, 16)

	if got := p.Len(); got != 3 {
		t.Fatalf("Len after Open = %d, want 3 (INBOX excluded)", got)
	}

	for _, r := range "ar" {
		p, _ = p.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if got := p.Filter(); got != "ar" {
		t.Fatalf("Filter = %q, want %q", got, "ar")
	}
	if got := p.MatchCount(); got != 1 {
		t.Fatalf("MatchCount after filter = %d, want 1", got)
	}

	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got := p.Filter(); got != "a" {
		t.Fatalf("Filter after backspace = %q, want %q", got, "a")
	}
}

func TestMovepicker_listModel_enterEmitsPicked(t *testing.T) {
	folders := []mail.FolderEntry{
		{Provider: "INBOX", Display: "Inbox"},
		{Provider: "Archive", Display: "Archive"},
	}
	p := newTestPicker().Open([]mail.UID{mail.UID("42")}, "INBOX", folders)
	p = p.SetSize(60, 16)

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter returned nil cmd")
	}
	msg := drainBatch(cmd)
	var picked *PickedMsg
	for _, m := range msg {
		if v, ok := m.(PickedMsg); ok {
			picked = &v
		}
	}
	if picked == nil {
		t.Fatalf("Enter cmd did not return PickedMsg")
	}
	if picked.Dest != "Archive" || picked.Src != "INBOX" || len(picked.UIDs) != 1 {
		t.Fatalf("PickedMsg = %+v, want UIDs=[42] Src=INBOX Dest=Archive", picked)
	}
}

// BenchmarkMovePickerView measures Box() cost on a realistic 7-folder list.
func BenchmarkMovePickerView(b *testing.B) {
	p := newTestPicker()
	p = p.Open(nil, "", sampleFolders())
	p = p.SetSize(80, 24)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Box(80, 24)
	}
}

func drainBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drainBatch(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}
