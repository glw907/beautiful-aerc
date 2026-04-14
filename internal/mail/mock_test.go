package mail

import (
	"context"
	"testing"
)

func TestMockBackendThreading(t *testing.T) {
	b := NewMockBackend()
	msgs, err := b.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}

	t.Run("total message count", func(t *testing.T) {
		if got, want := len(msgs), 14; got != want {
			t.Errorf("len(msgs) = %d, want %d", got, want)
		}
	})

	t.Run("flat messages have ThreadID == UID and empty InReplyTo", func(t *testing.T) {
		flatUIDs := map[UID]bool{
			"1": true, "2": true, "3": true, "4": true, "5": true,
			"6": true, "7": true, "8": true, "9": true, "10": true,
		}
		for _, m := range msgs {
			if !flatUIDs[m.UID] {
				continue
			}
			if m.ThreadID != m.UID {
				t.Errorf("flat message %s: ThreadID = %q, want %q", m.UID, m.ThreadID, m.UID)
			}
			if m.InReplyTo != "" {
				t.Errorf("flat message %s: InReplyTo = %q, want empty", m.UID, m.InReplyTo)
			}
		}
	})

	t.Run("threaded conversation has 4 messages with ThreadID T1", func(t *testing.T) {
		threaded := map[UID]MessageInfo{}
		for _, m := range msgs {
			if m.ThreadID == "T1" {
				threaded[m.UID] = m
			}
		}
		if len(threaded) != 4 {
			t.Fatalf("threaded conversation has %d messages, want 4", len(threaded))
		}

		root, ok := threaded["20"]
		if !ok {
			t.Fatal("missing root message UID 20")
		}
		if root.InReplyTo != "" {
			t.Errorf("root InReplyTo = %q, want empty", root.InReplyTo)
		}

		grace, ok := threaded["21"]
		if !ok {
			t.Fatal("missing reply UID 21 (Grace Kim)")
		}
		if grace.InReplyTo != "20" {
			t.Errorf("Grace InReplyTo = %q, want 20", grace.InReplyTo)
		}
		if grace.Flags&FlagSeen != 0 {
			t.Error("Grace should be unread (FlagSeen not set)")
		}

		franky, ok := threaded["22"]
		if !ok {
			t.Fatal("missing reply UID 22 (Frank deep)")
		}
		if franky.InReplyTo != "21" {
			t.Errorf("Frank-22 InReplyTo = %q, want 21", franky.InReplyTo)
		}

		henry, ok := threaded["23"]
		if !ok {
			t.Fatal("missing reply UID 23 (Henry)")
		}
		if henry.InReplyTo != "20" {
			t.Errorf("Henry InReplyTo = %q, want 20", henry.InReplyTo)
		}
	})
}

func TestMockBackend(t *testing.T) {
	b := NewMockBackend()

	t.Run("connect succeeds", func(t *testing.T) {
		if err := b.Connect(context.Background()); err != nil {
			t.Fatalf("Connect: %v", err)
		}
	})

	t.Run("list folders returns expected data", func(t *testing.T) {
		folders, err := b.ListFolders()
		if err != nil {
			t.Fatalf("ListFolders: %v", err)
		}
		if len(folders) == 0 {
			t.Fatal("expected at least one folder")
		}
		if folders[0].Name != "Inbox" {
			t.Errorf("first folder = %q, want Inbox", folders[0].Name)
		}
		if folders[0].Role != "inbox" {
			t.Errorf("Inbox role = %q, want inbox", folders[0].Role)
		}
	})

	t.Run("inbox has unread messages", func(t *testing.T) {
		folders, _ := b.ListFolders()
		inbox := folders[0]
		if inbox.Unseen == 0 {
			t.Error("expected Inbox to have unread messages")
		}
		if inbox.Exists == 0 {
			t.Error("expected Inbox to have messages")
		}
	})

	t.Run("fetch headers returns messages", func(t *testing.T) {
		msgs, err := b.FetchHeaders(nil)
		if err != nil {
			t.Fatalf("FetchHeaders: %v", err)
		}
		if len(msgs) == 0 {
			t.Fatal("expected at least one message")
		}
		for i, m := range msgs {
			if m.Subject == "" {
				t.Errorf("message %d has empty subject", i)
			}
			if m.From == "" {
				t.Errorf("message %d has empty from", i)
			}
		}
	})

	t.Run("updates channel is non-nil", func(t *testing.T) {
		ch := b.Updates()
		if ch == nil {
			t.Fatal("Updates() returned nil channel")
		}
	})

	t.Run("disconnect succeeds", func(t *testing.T) {
		if err := b.Disconnect(); err != nil {
			t.Fatalf("Disconnect: %v", err)
		}
	})
}
