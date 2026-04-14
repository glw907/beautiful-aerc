package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

func TestMessageList(t *testing.T) {
	styles := NewStyles(theme.Nord)
	msgs := mockMessages()

	t.Run("renders all visible messages", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		plain := stripANSI(ml.View())
		for _, msg := range msgs {
			if !strings.Contains(plain, msg.From) {
				t.Errorf("missing sender %q in view", msg.From)
			}
			if !strings.Contains(plain, truncateCells(msg.Subject, 50)) &&
				!strings.Contains(plain, msg.Subject) {
				t.Errorf("missing subject %q in view", msg.Subject)
			}
		}
	})

	t.Run("initial selection is first message", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		if ml.Selected() != 0 {
			t.Errorf("Selected() = %d, want 0", ml.Selected())
		}
		if got, _ := ml.SelectedMessage(); got.UID != msgs[0].UID {
			t.Errorf("SelectedMessage UID = %q, want %q", got.UID, msgs[0].UID)
		}
	})

	t.Run("selected row has cursor character", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		plain := stripANSI(ml.View())
		lines := strings.Split(plain, "\n")
		if len(lines) == 0 || !strings.HasPrefix(lines[0], "▐") {
			t.Errorf("first row should start with ▐ cursor: %q", lines[0])
		}
	})

	t.Run("MoveDown advances selection", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		ml.MoveDown()
		if ml.Selected() != 1 {
			t.Errorf("after MoveDown, Selected() = %d, want 1", ml.Selected())
		}
	})

	t.Run("MoveUp at top stays at 0", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		ml.MoveUp()
		if ml.Selected() != 0 {
			t.Errorf("MoveUp at top: Selected() = %d, want 0", ml.Selected())
		}
	})

	t.Run("MoveDown at bottom stays at last", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		for range len(msgs) + 5 {
			ml.MoveDown()
		}
		if ml.Selected() != len(msgs)-1 {
			t.Errorf("MoveDown past end: Selected() = %d, want %d",
				ml.Selected(), len(msgs)-1)
		}
	})

	t.Run("MoveToBottom jumps to last", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		ml.MoveToBottom()
		if ml.Selected() != len(msgs)-1 {
			t.Errorf("MoveToBottom: Selected() = %d, want %d",
				ml.Selected(), len(msgs)-1)
		}
	})

	t.Run("MoveToTop jumps to first", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		ml.MoveDown()
		ml.MoveDown()
		ml.MoveToTop()
		if ml.Selected() != 0 {
			t.Errorf("MoveToTop: Selected() = %d, want 0", ml.Selected())
		}
	})

	t.Run("HalfPageDown moves by half height", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 10)
		ml.HalfPageDown()
		if ml.Selected() != 5 {
			t.Errorf("HalfPageDown with height 10: Selected() = %d, want 5",
				ml.Selected())
		}
	})

	t.Run("scroll keeps cursor visible", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 4)
		// Step past the visible window.
		for range 6 {
			ml.MoveDown()
		}
		// Cursor at index 6, height 4 → offset should be at least 3.
		view := stripANSI(ml.View())
		lines := strings.Split(view, "\n")
		if len(lines) != 4 {
			t.Fatalf("view lines = %d, want 4", len(lines))
		}
		// The selected row carries the ▐ cursor; it must be visible.
		found := false
		for _, line := range lines {
			if strings.HasPrefix(line, "▐") {
				found = true
				break
			}
		}
		if !found {
			t.Error("cursor row not visible after scrolling past viewport")
		}
	})

	t.Run("all rendered rows have configured width", func(t *testing.T) {
		const w = 90
		ml := NewMessageList(styles, msgs, w, 12)
		for _, line := range strings.Split(ml.View(), "\n") {
			if got := lipgloss.Width(line); got != w {
				t.Errorf("row width = %d, want %d: %q", got, w, stripANSI(line))
			}
		}
	})

	t.Run("unread messages show envelope icon", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		plain := stripANSI(ml.View())
		// First three mock messages are unread.
		if !strings.Contains(plain, "󰇮") {
			t.Error("expected unread envelope icon in view")
		}
	})

	t.Run("flagged messages show flag icon", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "󰈻") {
			t.Error("expected flag icon for flagged message")
		}
	})

	t.Run("answered messages show reply icon", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "󰑚") {
			t.Error("expected reply icon for answered message")
		}
	})

	t.Run("date column is right-aligned", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		plain := stripANSI(ml.View())
		lines := strings.Split(plain, "\n")
		if len(lines) == 0 {
			t.Fatal("empty view")
		}
		// The date column is 12 cells wide; "2026-04-12 10:23" (16 chars)
		// truncates to "2026-04-12 …". Verify the date prefix appears at the
		// tail of the row (right-aligned, not in the middle).
		first := strings.TrimRight(lines[0], " ")
		if !strings.HasSuffix(first, "…") || !strings.Contains(first, "2026-04-12") {
			t.Errorf("expected first row to end with truncated date, got tail: %q", first)
		}
	})

	t.Run("empty list shows placeholder", func(t *testing.T) {
		ml := NewMessageList(styles, nil, 90, 10)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "No messages") {
			t.Errorf("empty list should show placeholder: %q", plain)
		}
	})

	t.Run("SetMessages resets cursor and offset", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 4)
		ml.MoveToBottom()
		ml.SetMessages(msgs[:2])
		if ml.Selected() != 0 {
			t.Errorf("after SetMessages, Selected() = %d, want 0", ml.Selected())
		}
	})

	t.Run("SetSize updates dimensions", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20)
		ml.SetSize(60, 10)
		if ml.width != 60 || ml.height != 10 {
			t.Errorf("size = %dx%d, want 60x10", ml.width, ml.height)
		}
	})

	t.Run("long sender truncated with ellipsis", func(t *testing.T) {
		long := []mail.MessageInfo{
			{UID: "x", From: strings.Repeat("VeryLongName", 5), Subject: "subject", Date: "today"},
		}
		ml := NewMessageList(styles, long, 90, 5)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "…") {
			t.Error("expected ellipsis when sender exceeds column width")
		}
	})
}

func mockMessages() []mail.MessageInfo {
	return []mail.MessageInfo{
		{UID: "1", ThreadID: "1", Subject: "Re: Project update for Q2 launch", From: "Alice Johnson", Date: "2026-04-12 10:23", Flags: 0},
		{UID: "2", ThreadID: "2", Subject: "Quick question about the API", From: "Bob Smith", Date: "2026-04-12 09:45", Flags: 0},
		{UID: "3", ThreadID: "3", Subject: "Lunch tomorrow?", From: "Carol White", Date: "2026-04-12 09:12", Flags: 0},
		{UID: "4", ThreadID: "4", Subject: "Meeting notes from yesterday", From: "David Chen", Date: "2026-04-11", Flags: mail.FlagSeen},
		{UID: "5", ThreadID: "5", Subject: "Invoice #2847 attached", From: "Billing Dept", Date: "2026-04-10", Flags: mail.FlagSeen | mail.FlagFlagged},
		{UID: "6", ThreadID: "6", Subject: "Re: Weekend hiking trip", From: "Emma Wilson", Date: "2026-04-09", Flags: mail.FlagSeen | mail.FlagAnswered},
		{UID: "7", ThreadID: "7", Subject: "Your subscription renewal", From: "Acme Cloud", Date: "2026-04-08", Flags: mail.FlagSeen},
		{UID: "8", ThreadID: "8", Subject: "Code review: auth refactor PR #42", From: "GitHub", Date: "2026-04-07", Flags: mail.FlagSeen},
		{UID: "9", ThreadID: "9", Subject: "New comment on your post", From: "Dev Community", Date: "2026-04-06", Flags: mail.FlagSeen},
		{UID: "10", ThreadID: "10", Subject: "Flight confirmation: SFO → SEA", From: "Alaska Airlines", Date: "2026-04-05", Flags: mail.FlagSeen | mail.FlagFlagged},
	}
}

func TestMessageListThreading(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("groups by ThreadID with explicit root", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "1", ThreadID: "1", From: "A", Date: "Apr 1", Flags: mail.FlagSeen},
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "Apr 5", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "Apr 6", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		if got, want := len(ml.rows), 3; got != want {
			t.Fatalf("len(rows) = %d, want %d", got, want)
		}
		var rootUIDs []mail.UID
		var childUIDs []mail.UID
		for _, r := range ml.rows {
			if r.isThreadRoot {
				rootUIDs = append(rootUIDs, r.msg.UID)
			} else {
				childUIDs = append(childUIDs, r.msg.UID)
			}
		}
		if len(rootUIDs) != 2 {
			t.Errorf("rootUIDs = %v, want exactly 2", rootUIDs)
		}
		if len(childUIDs) != 1 || childUIDs[0] != "11" {
			t.Errorf("childUIDs = %v, want [11]", childUIDs)
		}
		for _, r := range ml.rows {
			if r.isThreadRoot && r.msg.UID == "10" && r.threadSize != 2 {
				t.Errorf("T1 root threadSize = %d, want 2", r.threadSize)
			}
			if r.isThreadRoot && r.msg.UID == "1" && r.threadSize != 1 {
				t.Errorf("standalone threadSize = %d, want 1", r.threadSize)
			}
		}
	})

	t.Run("children sort chronologically ascending within a thread", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "Apr 1", Flags: mail.FlagSeen},
			{UID: "12", ThreadID: "T1", InReplyTo: "10", From: "Late", Date: "Apr 3", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Early", Date: "Apr 2", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		if got, want := len(ml.rows), 3; got != want {
			t.Fatalf("len(rows) = %d, want %d", got, want)
		}
		wantOrder := []mail.UID{"10", "11", "12"}
		for i, want := range wantOrder {
			if got := ml.rows[i].msg.UID; got != want {
				t.Errorf("rows[%d].UID = %q, want %q", i, got, want)
			}
		}
	})

	t.Run("thread latest-activity computed correctly", func(t *testing.T) {
		bucket := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", Date: "Apr 1"},
			{UID: "11", ThreadID: "T1", Date: "Apr 5"},
			{UID: "12", ThreadID: "T1", Date: "Apr 3"},
		}
		if got, want := latestActivity(bucket), "Apr 5"; got != want {
			t.Errorf("latestActivity = %q, want %q", got, want)
		}
	})

	t.Run("threads sorted by latest activity descending by default", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			// Older thread first in input.
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Old", Date: "Apr 1", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "OldReply", Date: "Apr 2", Flags: mail.FlagSeen},
			// Newer thread second in input.
			{UID: "20", ThreadID: "T2", InReplyTo: "", From: "New", Date: "Apr 5", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		if ml.rows[0].msg.UID != "20" {
			t.Errorf("first row UID = %q, want 20 (T2 root)", ml.rows[0].msg.UID)
		}
		if ml.rows[1].msg.UID != "10" {
			t.Errorf("second row UID = %q, want 10 (T1 root)", ml.rows[1].msg.UID)
		}
		if ml.rows[2].msg.UID != "11" {
			t.Errorf("third row UID = %q, want 11 (T1 child)", ml.rows[2].msg.UID)
		}
	})

	t.Run("threads sorted ascending when SortDateAsc", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "20", ThreadID: "T2", InReplyTo: "", From: "New", Date: "Apr 5", Flags: mail.FlagSeen},
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Old", Date: "Apr 1", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		ml.SetSort(SortDateAsc)
		if ml.rows[0].msg.UID != "10" {
			t.Errorf("first row UID = %q, want 10 (T1)", ml.rows[0].msg.UID)
		}
	})

	t.Run("synthetic root when no message has empty InReplyTo", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "999", From: "First", Date: "Apr 5", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "999", From: "Second", Date: "Apr 6", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		if got, want := len(ml.rows), 2; got != want {
			t.Fatalf("len(rows) = %d, want %d", got, want)
		}
		var rootUID mail.UID
		for _, r := range ml.rows {
			if r.isThreadRoot {
				rootUID = r.msg.UID
				break
			}
		}
		if rootUID != "10" {
			t.Errorf("synthetic root UID = %q, want 10", rootUID)
		}
	})

	t.Run("ToggleFold collapses thread under cursor", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		if got, want := visibleRowCount(ml), 2; got != want {
			t.Fatalf("initial visible rows = %d, want %d", got, want)
		}
		ml.ToggleFold()
		if got, want := visibleRowCount(ml), 1; got != want {
			t.Errorf("after fold visible rows = %d, want %d", got, want)
		}
		if got, want := ml.rows[0].prefix, "[2] "; got != want {
			t.Errorf("collapsed root prefix = %q, want %q", got, want)
		}
	})

	t.Run("ToggleFold from child row folds the thread root", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		ml.MoveDown() // cursor on UID 11 (child)
		ml.ToggleFold()
		if got, want := visibleRowCount(ml), 1; got != want {
			t.Errorf("after fold from child, visible rows = %d, want %d", got, want)
		}
		if got := ml.Selected(); got != 0 {
			t.Errorf("cursor index after fold = %d, want 0", got)
		}
	})

	t.Run("FoldAll and UnfoldAll", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "RootA", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "ReplyA", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
			{UID: "20", ThreadID: "T2", InReplyTo: "", From: "RootB", Date: "2026-04-06 10:00", Flags: mail.FlagSeen},
			{UID: "21", ThreadID: "T2", InReplyTo: "20", From: "ReplyB", Date: "2026-04-06 11:00", Flags: mail.FlagSeen},
			{UID: "30", ThreadID: "T3", InReplyTo: "", From: "Solo", Date: "2026-04-07 10:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		if got, want := visibleRowCount(ml), 5; got != want {
			t.Fatalf("initial visible = %d, want %d", got, want)
		}
		ml.FoldAll()
		if got, want := visibleRowCount(ml), 3; got != want {
			t.Errorf("after FoldAll visible = %d, want %d", got, want)
		}
		ml.UnfoldAll()
		if got, want := visibleRowCount(ml), 5; got != want {
			t.Errorf("after UnfoldAll visible = %d, want %d", got, want)
		}
	})

	t.Run("SetMessages resets fold state", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		ml.ToggleFold()
		ml.SetMessages(msgs) // same data
		if got, want := visibleRowCount(ml), 2; got != want {
			t.Errorf("after SetMessages reload, visible = %d, want %d", got, want)
		}
	})

	t.Run("box-drawing prefixes for branching thread", func(t *testing.T) {
		// Tree shape:
		//   Root (UID 10)
		//   ├─ Reply A (UID 11)
		//   │  └─ Deep (UID 12)
		//   └─ Reply B (UID 13)
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "ReplyA", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
			{UID: "12", ThreadID: "T1", InReplyTo: "11", From: "Deep", Date: "2026-04-05 12:00", Flags: mail.FlagSeen},
			{UID: "13", ThreadID: "T1", InReplyTo: "10", From: "ReplyB", Date: "2026-04-05 13:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		if got, want := len(ml.rows), 4; got != want {
			t.Fatalf("len(rows) = %d, want %d", got, want)
		}
		want := []struct {
			uid    mail.UID
			prefix string
			depth  uint8
		}{
			{"10", "", 0},
			{"11", "├─ ", 1},
			{"12", "│  └─ ", 2},
			{"13", "└─ ", 1},
		}
		for i, w := range want {
			if got := ml.rows[i].msg.UID; got != w.uid {
				t.Errorf("rows[%d].UID = %q, want %q", i, got, w.uid)
			}
			if got := ml.rows[i].prefix; got != w.prefix {
				t.Errorf("rows[%d].prefix = %q, want %q", i, got, w.prefix)
			}
			if got := ml.rows[i].depth; got != w.depth {
				t.Errorf("rows[%d].depth = %d, want %d", i, got, w.depth)
			}
		}
	})
}

// visibleRowCount counts the displayRows that aren't hidden by fold
// state. Used by tests to check fold behavior.
func visibleRowCount(ml MessageList) int {
	n := 0
	for _, r := range ml.rows {
		if !r.hidden {
			n++
		}
	}
	return n
}
