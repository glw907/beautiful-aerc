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
		// Strip the trailing right margin space, then verify the date appears
		// at the end of the row (not in the middle).
		first := strings.TrimRight(lines[0], " ")
		if !strings.HasSuffix(first, "10:23 AM") {
			t.Errorf("expected first row to end with date, got tail: %q", first)
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
		{UID: "1", ThreadID: "1", Subject: "Re: Project update for Q2 launch", From: "Alice Johnson", Date: "10:23 AM", Flags: 0},
		{UID: "2", ThreadID: "2", Subject: "Quick question about the API", From: "Bob Smith", Date: "9:45 AM", Flags: 0},
		{UID: "3", ThreadID: "3", Subject: "Lunch tomorrow?", From: "Carol White", Date: "9:12 AM", Flags: 0},
		{UID: "4", ThreadID: "4", Subject: "Meeting notes from yesterday", From: "David Chen", Date: "Yesterday", Flags: mail.FlagSeen},
		{UID: "5", ThreadID: "5", Subject: "Invoice #2847 attached", From: "Billing Dept", Date: "Yesterday", Flags: mail.FlagSeen | mail.FlagFlagged},
		{UID: "6", ThreadID: "6", Subject: "Re: Weekend hiking trip", From: "Emma Wilson", Date: "Yesterday", Flags: mail.FlagSeen | mail.FlagAnswered},
		{UID: "7", ThreadID: "7", Subject: "Your subscription renewal", From: "Acme Cloud", Date: "Apr 8", Flags: mail.FlagSeen},
		{UID: "8", ThreadID: "8", Subject: "Code review: auth refactor PR #42", From: "GitHub", Date: "Apr 8", Flags: mail.FlagSeen},
		{UID: "9", ThreadID: "9", Subject: "New comment on your post", From: "Dev Community", Date: "Apr 7", Flags: mail.FlagSeen},
		{UID: "10", ThreadID: "10", Subject: "Flight confirmation: SFO → SEA", From: "Alaska Airlines", Date: "Apr 7", Flags: mail.FlagSeen | mail.FlagFlagged},
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
}
