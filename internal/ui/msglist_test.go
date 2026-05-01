// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

func TestMessageList(t *testing.T) {
	styles := NewStyles(theme.Nord)
	msgs := mockMessages()

	t.Run("renders all visible messages", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
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
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		if ml.Selected() != 0 {
			t.Errorf("Selected() = %d, want 0", ml.Selected())
		}
		if got, _ := ml.SelectedMessage(); got.UID != msgs[0].UID {
			t.Errorf("SelectedMessage UID = %q, want %q", got.UID, msgs[0].UID)
		}
	})

	t.Run("selected row has cursor character", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		plain := stripANSI(ml.View())
		lines := strings.Split(plain, "\n")
		if len(lines) == 0 || !strings.HasPrefix(lines[0], "▐") {
			t.Errorf("first row should start with ▐ cursor: %q", lines[0])
		}
	})

	t.Run("MoveDown advances selection", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveDown()
		if ml.Selected() != 1 {
			t.Errorf("after MoveDown, Selected() = %d, want 1", ml.Selected())
		}
	})

	t.Run("MoveUp at top stays at 0", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveUp()
		if ml.Selected() != 0 {
			t.Errorf("MoveUp at top: Selected() = %d, want 0", ml.Selected())
		}
	})

	t.Run("MoveDown at bottom stays at last", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		for range len(msgs) + 5 {
			ml.MoveDown()
		}
		if ml.Selected() != len(msgs)-1 {
			t.Errorf("MoveDown past end: Selected() = %d, want %d",
				ml.Selected(), len(msgs)-1)
		}
	})

	t.Run("MoveToBottom jumps to last", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveToBottom()
		if ml.Selected() != len(msgs)-1 {
			t.Errorf("MoveToBottom: Selected() = %d, want %d",
				ml.Selected(), len(msgs)-1)
		}
	})

	t.Run("MoveToTop jumps to first", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveDown()
		ml.MoveDown()
		ml.MoveToTop()
		if ml.Selected() != 0 {
			t.Errorf("MoveToTop: Selected() = %d, want 0", ml.Selected())
		}
	})

	t.Run("HalfPageDown moves by half height", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 10, FancyIcons)
		ml.HalfPageDown()
		if ml.Selected() != 5 {
			t.Errorf("HalfPageDown with height 10: Selected() = %d, want 5",
				ml.Selected())
		}
	})

	t.Run("scroll keeps cursor visible", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 4, FancyIcons)
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
		ml := NewMessageList(styles, msgs, w, 12, FancyIcons)
		for _, line := range strings.Split(ml.View(), "\n") {
			if got := displayCells(line); got != w {
				t.Errorf("row width = %d, want %d: %q", got, w, stripANSI(line))
			}
		}
	})

	t.Run("read and unread rows have identical displayCells width at multiple widths", func(t *testing.T) {
		// SPUA-A flag glyphs (unread/flagged/answered) render as 2 terminal
		// cells; displayCells is the authoritative counter. Both read and unread
		// rows must reach exactly w display cells so the right border lands at
		// a fixed column regardless of flag state.
		//
		// Parameterized across three icon/width modes to lock in the invariant
		// that displayCells(row) == terminal width regardless of SetSPUACellWidth.
		for _, mode := range []struct {
			name    string
			width   int
			iconSet IconSet
		}{
			{"simple_w1", 1, SimpleIcons},
			{"fancy_w1", 1, FancyIcons},
			{"fancy_w2", 2, FancyIcons},
		} {
			t.Run(mode.name, func(t *testing.T) {
				SetSPUACellWidth(mode.width)
				defer SetSPUACellWidth(2) // restore the package init() default

				for _, w := range []int{80, 100, 120, 160} {
					readMsg := mail.MessageInfo{
						UID: "r", ThreadID: "r", From: "Alice", Subject: "Hello",
						Date: "Mon 2026-04-26", Flags: mail.FlagSeen,
					}
					unreadMsg := mail.MessageInfo{
						UID: "u", ThreadID: "u", From: "Bob", Subject: "World",
						Date: "Mon 2026-04-26", Flags: 0,
					}
					ml := NewMessageList(styles, []mail.MessageInfo{readMsg, unreadMsg}, w, 5, mode.iconSet)
					lines := strings.Split(ml.View(), "\n")
					if len(lines) < 2 {
						t.Fatalf("mode=%s w=%d: expected at least 2 rows, got %d", mode.name, w, len(lines))
					}
					readW := displayCells(lines[0])
					unreadW := displayCells(lines[1])
					if readW != w {
						t.Errorf("mode=%s w=%d: read row displayCells=%d, want %d", mode.name, w, readW, w)
					}
					if unreadW != w {
						t.Errorf("mode=%s w=%d: unread row displayCells=%d, want %d", mode.name, w, unreadW, w)
					}
				}
			})
		}
	})

	t.Run("unread messages show envelope icon", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		plain := stripANSI(ml.View())
		// First three mock messages are unread.
		if !strings.Contains(plain, "󰇮") {
			t.Error("expected unread envelope icon in view")
		}
	})

	t.Run("flagged messages show flag icon", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "󰈻") {
			t.Error("expected flag icon for flagged message")
		}
	})

	t.Run("answered messages show reply icon", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "󰑚") {
			t.Error("expected reply icon for answered message")
		}
	})

	t.Run("date column is right-aligned", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		plain := stripANSI(ml.View())
		lines := strings.Split(plain, "\n")
		if len(lines) == 0 {
			t.Fatal("empty view")
		}
		// The date column is 18 cells wide; the fixture's 23-char date
		// "2026-04-12 10:23:47 UTC" truncates. Verify the date prefix appears
		// at the tail of the row (right-aligned, not in the middle).
		first := strings.TrimRight(lines[0], " ")
		if !strings.HasSuffix(first, "…") || !strings.Contains(first, "2026-04-12") {
			t.Errorf("expected first row to end with truncated date, got tail: %q", first)
		}
	})

	t.Run("empty list shows placeholder", func(t *testing.T) {
		ml := NewMessageList(styles, nil, 90, 10, FancyIcons)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "No messages") {
			t.Errorf("empty list should show placeholder: %q", plain)
		}
	})

	t.Run("SetMessages resets cursor and offset", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 4, FancyIcons)
		ml.MoveToBottom()
		ml.SetMessages(msgs[:2])
		if ml.Selected() != 0 {
			t.Errorf("after SetMessages, Selected() = %d, want 0", ml.Selected())
		}
	})

	t.Run("SetSize updates dimensions", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetSize(60, 10)
		if ml.width != 60 || ml.height != 10 {
			t.Errorf("size = %dx%d, want 60x10", ml.width, ml.height)
		}
	})

	t.Run("long sender truncated with ellipsis", func(t *testing.T) {
		long := []mail.MessageInfo{
			{UID: "x", From: strings.Repeat("VeryLongName", 5), Subject: "subject", Date: "today"},
		}
		ml := NewMessageList(styles, long, 90, 5, FancyIcons)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "…") {
			t.Error("expected ellipsis when sender exceeds column width")
		}
	})
}

func TestRenderFlagCell(t *testing.T) {
	// renderFlagCell must always return a string that is exactly
	// mlFlagWidth display cells wide, regardless of icon mode.
	styles := NewStyles(theme.Nord)

	tests := []struct {
		name      string
		flags     mail.Flag
		isUnread  bool
		iconSet   IconSet
		spuaWidth int
	}{
		// Fancy mode (spuaCellWidth=2): SPUA glyph counts as 2 cells already.
		{"fancy flagged unread", mail.FlagFlagged, true, FancyIcons, 2},
		{"fancy answered read", mail.FlagAnswered, false, FancyIcons, 2},
		{"fancy unread", 0, true, FancyIcons, 2},
		{"fancy none", mail.FlagSeen, false, FancyIcons, 2},
		// Simple mode (spuaCellWidth=1): narrow glyph needs padding.
		{"simple flagged unread", mail.FlagFlagged, true, SimpleIcons, 1},
		{"simple answered read", mail.FlagAnswered, false, SimpleIcons, 1},
		{"simple unread", 0, true, SimpleIcons, 1},
		{"simple none", mail.FlagSeen, false, SimpleIcons, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetSPUACellWidth(tt.spuaWidth)
			defer SetSPUACellWidth(2)

			ml := NewMessageList(styles, nil, 90, 10, tt.iconSet)
			msg := mail.MessageInfo{Flags: tt.flags}
			bg := styles.MsgListBg
			rendered := ml.renderFlagCell(msg, tt.isUnread, bg)
			got := displayCells(rendered)
			if got != mlFlagWidth {
				t.Errorf("displayCells(renderFlagCell(%s)) = %d, want %d (rendered=%q)",
					tt.name, got, mlFlagWidth, stripANSI(rendered))
			}
		})
	}
}

func mockMessages() []mail.MessageInfo {
	return []mail.MessageInfo{
		{UID: "1", ThreadID: "1", Subject: "Re: Project update for Q2 launch", From: "Alice Johnson", Date: "2026-04-12 10:23:47 UTC", Flags: 0},
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
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
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
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
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
		if got, want := latestActivity(bucket).Date, "Apr 5"; got != want {
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
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
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
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetSort(SortDateAsc)
		if ml.rows[0].msg.UID != "10" {
			t.Errorf("first row UID = %q, want 10 (T1)", ml.rows[0].msg.UID)
		}
	})

	t.Run("SetThreaded(false) flattens thread tree", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
			{UID: "20", ThreadID: "T2", InReplyTo: "", From: "Other", Date: "2026-04-05 12:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetThreaded(false)
		if got, want := len(ml.rows), 3; got != want {
			t.Fatalf("len(rows) = %d, want %d", got, want)
		}
		for i, r := range ml.rows {
			if !r.isThreadRoot {
				t.Errorf("rows[%d] isThreadRoot = false, want true (every row is its own thread when flat)", i)
			}
			if r.threadSize != 1 {
				t.Errorf("rows[%d] threadSize = %d, want 1", i, r.threadSize)
			}
			if r.prefix != "" {
				t.Errorf("rows[%d] prefix = %q, want empty (no thread tree)", i, r.prefix)
			}
		}
	})

	t.Run("synthetic root when no message has empty InReplyTo", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "999", From: "First", Date: "Apr 5", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "999", From: "Second", Date: "Apr 6", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
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
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
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
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveDown() // cursor on UID 11 (child)
		ml.ToggleFold()
		if got, want := visibleRowCount(ml), 1; got != want {
			t.Errorf("after fold from child, visible rows = %d, want %d", got, want)
		}
		if got := ml.Selected(); got != 0 {
			t.Errorf("cursor index after fold = %d, want 0", got)
		}
	})

	t.Run("ToggleFoldAll flips between folded and unfolded", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "RootA", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "ReplyA", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
			{UID: "20", ThreadID: "T2", InReplyTo: "", From: "RootB", Date: "2026-04-06 10:00", Flags: mail.FlagSeen},
			{UID: "21", ThreadID: "T2", InReplyTo: "20", From: "ReplyB", Date: "2026-04-06 11:00", Flags: mail.FlagSeen},
			{UID: "30", ThreadID: "T3", InReplyTo: "", From: "Solo", Date: "2026-04-07 10:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		if got, want := visibleRowCount(ml), 5; got != want {
			t.Fatalf("initial visible = %d, want %d", got, want)
		}
		ml.ToggleFoldAll() // any unfolded → fold all
		if got, want := visibleRowCount(ml), 3; got != want {
			t.Errorf("after first toggle visible = %d, want %d", got, want)
		}
		ml.ToggleFoldAll() // all folded → unfold all
		if got, want := visibleRowCount(ml), 5; got != want {
			t.Errorf("after second toggle visible = %d, want %d", got, want)
		}
	})

	t.Run("ToggleFoldAll from mixed state folds everything first", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "RootA", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "ReplyA", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
			{UID: "20", ThreadID: "T2", InReplyTo: "", From: "RootB", Date: "2026-04-06 10:00", Flags: mail.FlagSeen},
			{UID: "21", ThreadID: "T2", InReplyTo: "20", From: "ReplyB", Date: "2026-04-06 11:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		// Fold only T1 so the list is in a mixed state.
		ml.ToggleFold()
		if got, want := visibleRowCount(ml), 3; got != want {
			t.Fatalf("after folding T1 visible = %d, want %d", got, want)
		}
		// Mixed state → toggle collapses everything.
		ml.ToggleFoldAll()
		if got, want := visibleRowCount(ml), 2; got != want {
			t.Errorf("mixed → ToggleFoldAll visible = %d, want %d", got, want)
		}
	})

	t.Run("SetMessages resets fold state", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
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
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
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

	t.Run("renders box-drawing prefix in subject column", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", Subject: "Root subject", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", Subject: "Re: Root subject", From: "ReplyA", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
			{UID: "12", ThreadID: "T1", InReplyTo: "10", Subject: "Re: Root subject", From: "ReplyB", Date: "2026-04-05 12:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 100, 20, FancyIcons)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "├─ Re: Root subject") {
			t.Error("expected ├─ prefix on first reply")
		}
		if !strings.Contains(plain, "└─ Re: Root subject") {
			t.Error("expected └─ prefix on last reply")
		}
	})

	t.Run("renders [N] badge on collapsed thread root", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", Subject: "Root", From: "R", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", Subject: "Re: Root", From: "A", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
			{UID: "12", ThreadID: "T1", InReplyTo: "10", Subject: "Re: Root", From: "B", Date: "2026-04-05 12:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 100, 20, FancyIcons)
		ml.ToggleFold()
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "[3] Root") {
			t.Errorf("expected [3] Root in collapsed view, got: %q", plain)
		}
	})

	t.Run("MoveDown skips hidden rows", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "1", ThreadID: "1", From: "Above", Subject: "above", Date: "2026-04-10", Flags: mail.FlagSeen},
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Subject: "thread", Date: "2026-04-09", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Subject: "thread", Date: "2026-04-09 11:00", Flags: mail.FlagSeen},
			{UID: "2", ThreadID: "2", From: "Below", Subject: "below", Date: "2026-04-08", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		// Default sort puts these in date-desc order: Above, Root, Reply, Below.
		ml.MoveDown() // cursor on Root (index 1)
		ml.ToggleFold()
		// Now visible rows: Above (0), Root (1, folded), Below (3 — index 2 hidden).
		// MoveDown from Root should land on Below (index 3), skipping hidden index 2.
		ml.MoveDown()
		if got, want := ml.Selected(), 3; got != want {
			t.Errorf("after MoveDown across hidden row, Selected() = %d, want %d", got, want)
		}
	})

	t.Run("MoveUp skips hidden rows", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "1", ThreadID: "1", From: "Above", Subject: "above", Date: "2026-04-10", Flags: mail.FlagSeen},
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Subject: "thread", Date: "2026-04-09", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Subject: "thread", Date: "2026-04-09 11:00", Flags: mail.FlagSeen},
			{UID: "2", ThreadID: "2", From: "Below", Subject: "below", Date: "2026-04-08", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveDown() // cursor on Root
		ml.ToggleFold()
		ml.MoveDown() // → index 3 (Below)
		ml.MoveUp()
		if got, want := ml.Selected(), 1; got != want {
			t.Errorf("after MoveUp across hidden row, Selected() = %d, want %d", got, want)
		}
	})

	t.Run("MoveToBottom lands on last visible row", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-09", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-09 11:00", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.ToggleFold() // fold T1, child at index 1 hidden
		ml.MoveToBottom()
		if got, want := ml.Selected(), 0; got != want {
			t.Errorf("MoveToBottom with only root visible: Selected() = %d, want %d", got, want)
		}
	})
}

func TestMessageListWithMockBackend(t *testing.T) {
	styles := NewStyles(theme.Nord)
	b := mail.NewMockBackend()
	msgs, err := b.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}

	ml := NewMessageList(styles, msgs, 120, 30, FancyIcons)

	t.Run("14 source messages produce 14 displayRows expanded", func(t *testing.T) {
		if got, want := len(ml.rows), 14; got != want {
			t.Errorf("len(rows) = %d, want %d", got, want)
		}
	})

	t.Run("threaded conversation has correct prefix vocabulary", func(t *testing.T) {
		var t1Prefixes []string
		for _, r := range ml.rows {
			if r.msg.ThreadID == "T1" {
				t1Prefixes = append(t1Prefixes, r.prefix)
			}
		}
		if len(t1Prefixes) != 4 {
			t.Fatalf("T1 row count = %d, want 4", len(t1Prefixes))
		}
		// Frank Lee root, then Grace (├─), then Frank-deep (│  └─), then Henry (└─).
		// Children sorted chronologically asc; the actual mock dates are all
		// "Apr 5" so order falls back to insertion order via SliceStable.
		want := []string{"", "├─ ", "│  └─ ", "└─ "}
		for i, w := range want {
			if t1Prefixes[i] != w {
				t.Errorf("T1 prefix[%d] = %q, want %q", i, t1Prefixes[i], w)
			}
		}
	})

	t.Run("ToggleFoldAll collapses the threaded conversation", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 120, 30, FancyIcons)
		ml.ToggleFoldAll()
		visible := visibleRowCount(ml)
		// 10 single-message threads (unaffected) + 1 visible folded root = 11.
		if visible != 11 {
			t.Errorf("visible after ToggleFoldAll = %d, want 11", visible)
		}
		var foundBadge bool
		for _, r := range ml.rows {
			if r.isThreadRoot && r.msg.ThreadID == "T1" {
				if r.prefix != "[4] " {
					t.Errorf("collapsed T1 root prefix = %q, want %q", r.prefix, "[4] ")
				}
				foundBadge = true
			}
		}
		if !foundBadge {
			t.Error("never found T1 thread root after ToggleFoldAll")
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

func TestMessageListFilter(t *testing.T) {
	styles := NewStyles(theme.Nord)

	msgs := []mail.MessageInfo{
		{UID: "1", ThreadID: "1", Subject: "Project update", From: "Alice", Date: "Apr 10"},
		{UID: "2", ThreadID: "2", Subject: "Weekend plans", From: "Bob", Date: "Apr 09"},
		{UID: "3", ThreadID: "3", Subject: "Invoice #2847", From: "Billing", Date: "Apr 08"},
	}

	t.Run("empty query keeps all rows", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("", SearchModeName)
		if got := len(ml.rows); got != 3 {
			t.Errorf("len(rows) after empty filter = %d, want 3", got)
		}
	})

	t.Run("substring match on subject", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("project", SearchModeName)
		if got := len(ml.rows); got != 1 {
			t.Errorf("len(rows) = %d, want 1", got)
		}
		if ml.rows[0].msg.UID != "1" {
			t.Errorf("matched row = %q, want 1", ml.rows[0].msg.UID)
		}
	})

	t.Run("substring match on sender", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("bob", SearchModeName)
		if got := len(ml.rows); got != 1 {
			t.Errorf("len(rows) = %d, want 1", got)
		}
		if ml.rows[0].msg.UID != "2" {
			t.Errorf("matched row = %q, want 2", ml.rows[0].msg.UID)
		}
	})

	t.Run("case-insensitive", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("ALICE", SearchModeName)
		if got := len(ml.rows); got != 1 {
			t.Errorf("len(rows) = %d, want 1", got)
		}
	})

	t.Run("no matches returns empty rows", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("zzz-nothing", SearchModeName)
		if got := len(ml.rows); got != 0 {
			t.Errorf("len(rows) = %d, want 0", got)
		}
	})

	t.Run("ClearFilter restores all rows", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("project", SearchModeName)
		ml.ClearFilter()
		if got := len(ml.rows); got != 3 {
			t.Errorf("len(rows) after clear = %d, want 3", got)
		}
	})

	t.Run("[name] mode does not match date", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("Apr 10", SearchModeName)
		if got := len(ml.rows); got != 0 {
			t.Errorf("len(rows) for Apr 10 under [name] = %d, want 0", got)
		}
	})

	t.Run("[all] mode matches date", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("Apr 10", SearchModeAll)
		if got := len(ml.rows); got != 1 {
			t.Errorf("len(rows) for Apr 10 under [all] = %d, want 1", got)
		}
		if ml.rows[0].msg.UID != "1" {
			t.Errorf("matched row = %q, want 1", ml.rows[0].msg.UID)
		}
	})

	t.Run("[all] mode also matches subject and sender", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("project", SearchModeAll)
		if got := len(ml.rows); got != 1 {
			t.Errorf("len(rows) for project under [all] = %d, want 1", got)
		}
	})

	t.Run("[all] and [name] differ on date-only queries", func(t *testing.T) {
		mlName := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		mlAll := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		mlName.SetFilter("Apr 09", SearchModeName)
		mlAll.SetFilter("Apr 09", SearchModeAll)
		if len(mlName.rows) != 0 {
			t.Errorf("[name] matched date: len(rows) = %d, want 0", len(mlName.rows))
		}
		if len(mlAll.rows) != 1 {
			t.Errorf("[all] missed date: len(rows) = %d, want 1", len(mlAll.rows))
		}
	})

	t.Run("cursor saved on first filter application", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveDown()
		ml.MoveDown()
		if ml.selected != 2 {
			t.Fatalf("setup: selected = %d, want 2", ml.selected)
		}
		ml.SetFilter("project", SearchModeName)
		if ml.preSearchCursor != 2 {
			t.Errorf("preSearchCursor = %d, want 2", ml.preSearchCursor)
		}
	})

	t.Run("subsequent keystrokes don't overwrite saved cursor", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveDown()
		ml.MoveDown()
		ml.SetFilter("p", SearchModeName)
		ml.SetFilter("pr", SearchModeName)
		ml.SetFilter("pro", SearchModeName)
		if ml.preSearchCursor != 2 {
			t.Errorf("preSearchCursor after more typing = %d, want 2", ml.preSearchCursor)
		}
	})

	t.Run("clear restores pre-search cursor", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveDown()
		ml.MoveDown()
		ml.SetFilter("project", SearchModeName)
		ml.ClearFilter()
		if ml.selected != 2 {
			t.Errorf("selected after clear = %d, want 2", ml.selected)
		}
	})

	t.Run("clear with invalid saved cursor clamps to 0", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.MoveDown()
		ml.MoveDown()
		ml.SetFilter("project", SearchModeName)
		ml.SetMessages(msgs[:1])
		ml.ClearFilter()
		if ml.selected != 0 {
			t.Errorf("selected after clear with shorter source = %d, want 0", ml.selected)
		}
	})

	t.Run("re-activating search after clear starts fresh save", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("project", SearchModeName)
		ml.ClearFilter()
		ml.MoveDown()
		ml.SetFilter("weekend", SearchModeName)
		if ml.preSearchCursor != 1 {
			t.Errorf("preSearchCursor on re-activate = %d, want 1", ml.preSearchCursor)
		}
	})
}

func TestMessageListFilterFoldShadow(t *testing.T) {
	styles := NewStyles(theme.Nord)

	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", Subject: "Server migration", From: "Eve", Date: "Apr 05"},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", Subject: "Re: Server migration", From: "Grace", Date: "Apr 06"},
		{UID: "12", ThreadID: "T1", InReplyTo: "11", Subject: "Re: Server migration", From: "Frank", Date: "Apr 07"},
		{UID: "20", ThreadID: "T2", InReplyTo: "", Subject: "Lunch", From: "Carol", Date: "Apr 08"},
	}

	t.Run("filter expands folded thread when any message matches", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.ToggleFoldAll()
		visibleBefore := 0
		for _, r := range ml.rows {
			if !r.hidden {
				visibleBefore++
			}
		}
		if visibleBefore != 2 {
			t.Fatalf("setup: visible rows = %d, want 2", visibleBefore)
		}

		ml.SetFilter("server", SearchModeName)
		visibleAfter := 0
		for _, r := range ml.rows {
			if !r.hidden {
				visibleAfter++
			}
		}
		if visibleAfter != 3 {
			t.Errorf("filtered visible rows = %d, want 3 (full T1)", visibleAfter)
		}
	})

	t.Run("clear filter restores saved fold state", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.ToggleFoldAll()
		ml.SetFilter("server", SearchModeName)
		ml.ClearFilter()

		var rootRow displayRow
		var childCount int
		for _, r := range ml.rows {
			if r.isThreadRoot && r.msg.UID == "10" {
				rootRow = r
			}
			if !r.isThreadRoot && r.msg.ThreadID == "T1" && r.hidden {
				childCount++
			}
		}
		if !strings.HasPrefix(rootRow.prefix, "[") {
			t.Errorf("T1 root prefix after clear = %q, want folded badge", rootRow.prefix)
		}
		if childCount != 2 {
			t.Errorf("hidden children after clear = %d, want 2", childCount)
		}
	})
}

func TestMessageListFilterResultCount(t *testing.T) {
	styles := NewStyles(theme.Nord)

	msgs := []mail.MessageInfo{
		{UID: "1", ThreadID: "1", Subject: "Project alpha", From: "Alice", Date: "Apr 10"},
		{UID: "2", ThreadID: "2", Subject: "Project beta", From: "Bob", Date: "Apr 09"},
		{UID: "3", ThreadID: "3", Subject: "Weekend", From: "Carol", Date: "Apr 08"},
		{UID: "10", ThreadID: "T4", InReplyTo: "", Subject: "Project gamma", From: "Dave", Date: "Apr 05"},
		{UID: "11", ThreadID: "T4", InReplyTo: "10", Subject: "Re: Project gamma", From: "Eve", Date: "Apr 06"},
		{UID: "12", ThreadID: "T4", InReplyTo: "11", Subject: "Re: Project gamma", From: "Frank", Date: "Apr 07"},
	}

	t.Run("count is thread count, not message count", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("project", SearchModeName)
		if got := ml.FilterResultCount(); got != 3 {
			t.Errorf("FilterResultCount = %d, want 3 (2 singletons + 1 thread)", got)
		}
	})

	t.Run("zero when no matches", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("zzz-nothing", SearchModeName)
		if got := ml.FilterResultCount(); got != 0 {
			t.Errorf("FilterResultCount = %d, want 0", got)
		}
	})

	t.Run("zero when no filter active", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		if got := ml.FilterResultCount(); got != 0 {
			t.Errorf("FilterResultCount with no filter = %d, want 0", got)
		}
	})
}

func TestMessageList_AppendMessages_PreservesCursor(t *testing.T) {
	styles := NewStyles(theme.Nord)

	initial := []mail.MessageInfo{
		{UID: "1", ThreadID: "1", Subject: "First", From: "Alice", Date: "Apr 10", Flags: mail.FlagSeen},
		{UID: "2", ThreadID: "2", Subject: "Second", From: "Bob", Date: "Apr 09", Flags: mail.FlagSeen},
		{UID: "3", ThreadID: "3", Subject: "Third", From: "Carol", Date: "Apr 08", Flags: mail.FlagSeen},
		{UID: "4", ThreadID: "4", Subject: "Fourth", From: "Dave", Date: "Apr 07", Flags: mail.FlagSeen},
		{UID: "5", ThreadID: "5", Subject: "Fifth", From: "Eve", Date: "Apr 06", Flags: mail.FlagSeen},
	}
	extra := []mail.MessageInfo{
		{UID: "6", ThreadID: "6", Subject: "Sixth", From: "Frank", Date: "Apr 05", Flags: mail.FlagSeen},
		{UID: "7", ThreadID: "7", Subject: "Seventh", From: "Grace", Date: "Apr 04", Flags: mail.FlagSeen},
	}

	ml := NewMessageList(styles, initial, 90, 20, FancyIcons)
	// Advance to the third visible row (index 2).
	ml.MoveDown()
	ml.MoveDown()
	anchorUID := ml.rows[ml.selected].msg.UID

	ml.AppendMessages(extra)

	// Source count must grow.
	if got, want := ml.Count(), len(initial)+len(extra); got != want {
		t.Errorf("Count() = %d, want %d", got, want)
	}

	// Cursor must still point at the same UID.
	if got := ml.rows[ml.selected].msg.UID; got != anchorUID {
		t.Errorf("cursor UID after AppendMessages = %q, want %q", got, anchorUID)
	}
}

func TestMessageList_IsNearBottom(t *testing.T) {
	styles := NewStyles(theme.Nord)

	make100 := func() []mail.MessageInfo {
		msgs := make([]mail.MessageInfo, 100)
		for i := range msgs {
			msgs[i] = mail.MessageInfo{
				UID:      mail.UID(fmt.Sprintf("%d", i+1)),
				ThreadID: mail.UID(fmt.Sprintf("%d", i+1)),
				Subject:  fmt.Sprintf("Message %d", i+1),
				From:     "Sender",
				Date:     fmt.Sprintf("2026-04-%02d", (i%28)+1),
				Flags:    mail.FlagSeen,
			}
		}
		return msgs
	}

	t.Run("cursor near bottom returns true", func(t *testing.T) {
		ml := NewMessageList(styles, make100(), 90, 20, FancyIcons)
		ml.MoveToBottom() // selected = 99
		if !ml.IsNearBottom(5) {
			t.Error("IsNearBottom(5) = false, want true when cursor at last row")
		}
	})

	t.Run("cursor within k of bottom returns true", func(t *testing.T) {
		ml := NewMessageList(styles, make100(), 90, 20, FancyIcons)
		ml.MoveToBottom()
		ml.MoveUp()
		ml.MoveUp()
		ml.MoveUp() // selected = 96, 4 from end
		if !ml.IsNearBottom(5) {
			t.Error("IsNearBottom(5) = false, want true when cursor is 4 from end")
		}
	})

	t.Run("cursor far from bottom returns false", func(t *testing.T) {
		ml := NewMessageList(styles, make100(), 90, 20, FancyIcons)
		// Move to row 50 (selected=50, 50 from end of 100).
		for range 50 {
			ml.MoveDown()
		}
		if ml.IsNearBottom(5) {
			t.Error("IsNearBottom(5) = true, want false when cursor is row 50 of 100")
		}
	})

	t.Run("empty list returns false", func(t *testing.T) {
		ml := NewMessageList(styles, nil, 90, 20, FancyIcons)
		if ml.IsNearBottom(5) {
			t.Error("IsNearBottom(5) = true, want false for empty list")
		}
	})
}

func TestMessageListPlaceholder(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("empty source shows No messages", func(t *testing.T) {
		ml := NewMessageList(styles, nil, 90, 20, FancyIcons)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "No messages") {
			t.Error("empty source should render 'No messages'")
		}
		if strings.Contains(plain, "No matches") {
			t.Error("empty source should not render 'No matches'")
		}
	})

	t.Run("filter with no matches shows No matches", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "1", ThreadID: "1", Subject: "Hello", From: "Alice", Date: "Apr 10"},
		}
		ml := NewMessageList(styles, msgs, 90, 20, FancyIcons)
		ml.SetFilter("nothing-here-zzz", SearchModeName)
		plain := stripANSI(ml.View())
		if !strings.Contains(plain, "No matches") {
			t.Error("filter with no matches should render 'No matches'")
		}
		if strings.Contains(plain, "No messages") {
			t.Error("filter with no matches should not render 'No messages'")
		}
	})
}

func TestMessageList_ColumnGaps(t *testing.T) {
	// Regression lock for F5: exactly 2 spaces between sender column and
	// subject, and between subject and date. The layout for a read row
	// with no flag:
	//
	//   cursor(1) + sp×2(2) + flag(2) + sp×2(2) + sender(22) + sp×2(2)
	//   + subject(w-48) + sp×2(2) + date(14) + sp(1)
	//
	// Total fixed cells: 1+2+2+2+22+2+2+14+1 = 48.
	//
	// A read message with no flag glyph means the flag cell is two ASCII
	// spaces. The cursor glyph ▐ is 1 display cell but 3 bytes, so we
	// index by rune position (which equals display-cell position for the
	// ASCII characters that fill the rest of this row).
	styles := NewStyles(theme.Nord)
	const w = 100

	msg := mail.MessageInfo{
		UID:      "1",
		ThreadID: "1",
		From:     "Alice Johnson",  // 13 chars; padded to 22
		Subject:  "Hello world",   // short; padded to fill subject budget
		Date:     "Mon 2026-04-27", // 14 chars; fits date column exactly
		Flags:    mail.FlagSeen,   // read, no flag glyph
	}
	ml := NewMessageList(styles, []mail.MessageInfo{msg}, w, 3, FancyIcons)
	line := stripANSI(strings.Split(ml.View(), "\n")[0])
	runes := []rune(line)

	if got := displayCells(strings.Split(ml.View(), "\n")[0]); got != w {
		t.Fatalf("row displayCells = %d, want %d: %q", got, w, line)
	}

	// Sender block ends at rune index 28 (0-indexed):
	//   cursor(1) + sp×2(2) + flag(2) + sp×2(2) + sender(22) - 1 = 28
	// The cursor ▐ is one display cell, so rune index == cell index here.
	const senderEnd = 1 + 2 + mlFlagWidth + 2 + mlSenderWidth - 1
	gap1 := string(runes[senderEnd+1 : senderEnd+3])
	if gap1 != "  " {
		t.Errorf("sender→subject gap = %q (rune pos %d-%d), want 2 spaces",
			gap1, senderEnd+1, senderEnd+2)
	}

	// Date block: trailing sp(1) + date(14) cells from the right edge.
	// Two-space gap is the 2 runes immediately before the date start.
	const dateStart = w - 1 - mlDateWidth // = 100 - 1 - 14 = 85
	gap2 := string(runes[dateStart-2 : dateStart])
	if gap2 != "  " {
		t.Errorf("subject→date gap = %q (rune pos %d-%d), want 2 spaces",
			gap2, dateStart-2, dateStart-1)
	}
}

func TestMessageListMoveCursor(t *testing.T) {
	styles := NewStyles(theme.Nord)
	msgs := []mail.MessageInfo{
		{UID: "uid-1", ThreadID: "uid-1", From: "Alice", Subject: "First", Date: "2026-04-26", Flags: mail.FlagSeen},
		{UID: "uid-2", ThreadID: "uid-2", From: "Bob", Subject: "Second", Date: "2026-04-25", Flags: mail.FlagSeen},
		{UID: "uid-3", ThreadID: "uid-3", From: "Carol", Subject: "Third", Date: "2026-04-24", Flags: mail.FlagSeen},
	}
	m := NewMessageList(styles, msgs, 90, 20, FancyIcons)

	uid, moved := m.MoveCursor(1)
	if !moved {
		t.Fatal("MoveCursor(+1) from row 0 should move")
	}
	wantUID := mail.UID("uid-2")
	if uid != wantUID {
		t.Fatalf("MoveCursor(+1) = %q, want %q", uid, wantUID)
	}

	_, moved = m.MoveCursor(1)
	if !moved {
		t.Fatal("MoveCursor(+1) from row 1 should move")
	}
	_, moved = m.MoveCursor(1)
	if moved {
		t.Fatal("MoveCursor(+1) from last row should be inert")
	}

	_, moved = m.MoveCursor(-1)
	if !moved {
		t.Fatal("MoveCursor(-1) from last row should move")
	}
}

func TestMessageList_VisualModeAndTargets(t *testing.T) {
	styles := NewStyles(theme.Nord)

	// Thread fixture: root "10" + two children "11", "12" in thread T1.
	// Plus a standalone "20" in T2.
	threadMsgs := func() []mail.MessageInfo {
		return []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Subject: "root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "ReplyA", Subject: "re: root", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
			{UID: "12", ThreadID: "T1", InReplyTo: "10", From: "ReplyB", Subject: "re: root 2", Date: "2026-04-05 12:00", Flags: mail.FlagSeen},
			{UID: "20", ThreadID: "T2", InReplyTo: "", From: "Solo", Subject: "solo", Date: "2026-04-06 10:00", Flags: mail.FlagSeen},
		}
	}

	t.Run("default: VisualMode false, Marked empty", func(t *testing.T) {
		ml := NewMessageList(styles, threadMsgs(), 90, 20, FancyIcons)
		if ml.VisualMode() {
			t.Error("VisualMode() = true, want false by default")
		}
		if got := ml.Marked(); len(got) != 0 {
			t.Errorf("Marked() = %v, want empty", got)
		}
	})

	t.Run("EnterVisual sets VisualMode true, Marked still empty", func(t *testing.T) {
		ml := NewMessageList(styles, threadMsgs(), 90, 20, FancyIcons)
		ml.EnterVisual()
		if !ml.VisualMode() {
			t.Error("VisualMode() = false after EnterVisual, want true")
		}
		if got := ml.Marked(); len(got) != 0 {
			t.Errorf("Marked() = %v after EnterVisual, want empty", got)
		}
	})

	t.Run("ToggleMark adds uid to marked set", func(t *testing.T) {
		ml := NewMessageList(styles, threadMsgs(), 90, 20, FancyIcons)
		ml.EnterVisual()
		ml.ToggleMark("10")
		got := ml.Marked()
		if len(got) != 1 || got[0] != "10" {
			t.Errorf("Marked() = %v after ToggleMark(10), want [10]", got)
		}
	})

	t.Run("ToggleMark twice removes uid", func(t *testing.T) {
		ml := NewMessageList(styles, threadMsgs(), 90, 20, FancyIcons)
		ml.EnterVisual()
		ml.ToggleMark("10")
		ml.ToggleMark("10")
		if got := ml.Marked(); len(got) != 0 {
			t.Errorf("Marked() = %v after double ToggleMark, want empty", got)
		}
	})

	t.Run("ExitVisual clears marked and sets VisualMode false", func(t *testing.T) {
		ml := NewMessageList(styles, threadMsgs(), 90, 20, FancyIcons)
		ml.EnterVisual()
		ml.ToggleMark("10")
		ml.ToggleMark("20")
		ml.ExitVisual()
		if ml.VisualMode() {
			t.Error("VisualMode() = true after ExitVisual, want false")
		}
		if got := ml.Marked(); len(got) != 0 {
			t.Errorf("Marked() = %v after ExitVisual, want empty", got)
		}
	})

	t.Run("ActionTargets with no marks returns cursor UID", func(t *testing.T) {
		ml := NewMessageList(styles, threadMsgs(), 90, 20, FancyIcons)
		// Default sort date-desc: T2 (solo, Apr 6) first, T1 (root, Apr 5) second.
		// Cursor at row 0 = UID "20".
		got := ml.ActionTargets()
		if len(got) != 1 {
			t.Fatalf("ActionTargets() len = %d, want 1", len(got))
		}
		if got[0] != "20" {
			t.Errorf("ActionTargets()[0] = %q, want %q", got[0], "20")
		}
	})

	t.Run("ActionTargets with two marks returns them in source order", func(t *testing.T) {
		ml := NewMessageList(styles, threadMsgs(), 90, 20, FancyIcons)
		ml.EnterVisual()
		ml.ToggleMark("20")
		ml.ToggleMark("10")
		got := ml.ActionTargets()
		// Source order: 10, 11, 12, 20 → marks {10, 20} → [10, 20]
		if len(got) != 2 {
			t.Fatalf("ActionTargets() len = %d, want 2", len(got))
		}
		if got[0] != "10" || got[1] != "20" {
			t.Errorf("ActionTargets() = %v, want [10 20]", got)
		}
	})

	t.Run("ActionTargets from folded thread root expands to root+children", func(t *testing.T) {
		ml := NewMessageList(styles, threadMsgs(), 90, 20, FancyIcons)
		// Date-desc: T2 solo first (row 0), T1 root at row 1.
		// Move cursor to T1 root (row 1) then fold.
		ml.MoveDown()
		ml.ToggleFold()
		// Cursor should be on the folded T1 root.
		msg, ok := ml.SelectedMessage()
		if !ok || msg.UID != "10" {
			t.Fatalf("expected cursor on T1 root (10), got %q ok=%v", msg.UID, ok)
		}
		got := ml.ActionTargets()
		// WYSIWYG: folded root → root + all children = [10, 11, 12]
		if len(got) != 3 {
			t.Fatalf("ActionTargets() from folded root len = %d, want 3 (root+2 children): %v", len(got), got)
		}
		if got[0] != "10" {
			t.Errorf("ActionTargets()[0] = %q, want root 10", got[0])
		}
		// Children 11, 12 should both appear (order: source order)
		found11, found12 := false, false
		for _, u := range got[1:] {
			if u == "11" {
				found11 = true
			}
			if u == "12" {
				found12 = true
			}
		}
		if !found11 || !found12 {
			t.Errorf("ActionTargets() = %v, expected both 11 and 12 in result", got)
		}
	})

	t.Run("ActionTargets from non-folded thread root returns only root UID", func(t *testing.T) {
		ml := NewMessageList(styles, threadMsgs(), 90, 20, FancyIcons)
		// Move cursor to T1 root (row 1, unfolded).
		ml.MoveDown()
		msg, ok := ml.SelectedMessage()
		if !ok || msg.UID != "10" {
			t.Fatalf("expected cursor on T1 root (10), got %q ok=%v", msg.UID, ok)
		}
		got := ml.ActionTargets()
		if len(got) != 1 {
			t.Fatalf("ActionTargets() from unfolded root len = %d, want 1: %v", len(got), got)
		}
		if got[0] != "10" {
			t.Errorf("ActionTargets()[0] = %q, want 10", got[0])
		}
	})
}
