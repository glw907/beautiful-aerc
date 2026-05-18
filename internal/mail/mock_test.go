package mail

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestMockBackendImplementsBackend(t *testing.T) {
	var _ Backend = (*MockBackend)(nil)
}

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

	t.Run("flat: ThreadID == UID, empty InReplyTo", func(t *testing.T) {
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

	t.Run("threaded: shared ThreadID", func(t *testing.T) {
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

func TestMockBackend_QueryFolder(t *testing.T) {
	b := NewMockBackend()
	total := len(b.msgs)
	cases := []struct {
		name          string
		offset, limit int
		wantLen       int
		wantNil       bool // result slice must be nil (early-return path)
		wantCap       int  // -1 = don't check
	}{
		{"first window", 0, 5, 5, false, 5},
		{"past end", total + 10, 5, 0, true, -1},
		{"at end", total, 5, 0, true, -1},
		{"clamps end", total - 2, 10, 2, false, 2},
		{"zero limit", 0, 0, 0, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uids, gotTotal, err := b.QueryFolder("Inbox", tc.offset, tc.limit)
			if err != nil {
				t.Fatalf("QueryFolder: %v", err)
			}
			if gotTotal != total {
				t.Errorf("total = %d, want %d", gotTotal, total)
			}
			if len(uids) != tc.wantLen {
				t.Errorf("len(uids) = %d, want %d", len(uids), tc.wantLen)
			}
			if tc.wantNil && uids != nil {
				t.Errorf("uids = %v, want nil (early-return path)", uids)
			}
			if !tc.wantNil && uids == nil {
				t.Errorf("uids = nil, want non-nil slice")
			}
			if tc.wantCap >= 0 && cap(uids) != tc.wantCap {
				t.Errorf("cap(uids) = %d, want %d", cap(uids), tc.wantCap)
			}
		})
	}
}

func TestMockBackend_FetchBody_SeededUIDs(t *testing.T) {
	b := NewMockBackend()
	// Each UID in the seeded mockBodies table has a distinctive substring
	// near every ARITHMETIC_BASE mutation site so a corruption shifts
	// the substring out of the body.
	cases := map[UID]string{
		"1": "Q2 launch",
		"2": "/v2/messages",
		"3": "lunch tomorrow",
		"4": "planning meeting",
		"5": "invoice_id",
		"6": "Cougar Mountain",
		"7": "Acme Cloud Pro",
		"8": "SessionGuard",
	}
	for uid, marker := range cases {
		t.Run(string(uid), func(t *testing.T) {
			body, err := b.FetchBody(uid)
			if err != nil {
				t.Fatalf("FetchBody(%s): %v", uid, err)
			}
			if len(body) == 0 {
				t.Fatalf("FetchBody(%s) returned empty body", uid)
			}
			if !strings.Contains(string(body), marker) {
				t.Errorf("FetchBody(%s) missing marker %q", uid, marker)
			}
		})
	}

	t.Run("unknown uid returns synthetic body", func(t *testing.T) {
		body, err := b.FetchBody("nope")
		if err != nil {
			t.Fatalf("FetchBody(nope): %v", err)
		}
		if !strings.Contains(string(body), "Mock body for message nope") {
			t.Errorf("synthetic fallback missing UID echo: %q", body)
		}
	})
}

func TestMockBackend_Destroy_RecordsAndRemoves(t *testing.T) {
	m := NewMockBackend()
	headers, err := m.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}
	startCount := len(headers)
	if startCount < 2 {
		t.Fatalf("mock seed too small: %d", startCount)
	}
	target := []UID{headers[0].UID, headers[1].UID}

	if err := m.Destroy(context.Background(), target); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	if len(m.DestroyCalls) != 1 {
		t.Fatalf("DestroyCalls len = %d, want 1", len(m.DestroyCalls))
	}
	if !reflect.DeepEqual(m.DestroyCalls[0], target) {
		t.Errorf("DestroyCalls[0] = %v, want %v", m.DestroyCalls[0], target)
	}

	after, err := m.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders after: %v", err)
	}
	if len(after) != startCount-2 {
		t.Errorf("after-Destroy count = %d, want %d", len(after), startCount-2)
	}
	for _, msg := range after {
		if msg.UID == target[0] || msg.UID == target[1] {
			t.Errorf("destroyed UID %q still present", msg.UID)
		}
	}
}

func TestMockBackend_Destroy_EmptyIsNoop(t *testing.T) {
	m := NewMockBackend()
	if err := m.Destroy(context.Background(), nil); err != nil {
		t.Fatalf("Destroy(nil): %v", err)
	}
	if len(m.DestroyCalls) != 0 {
		t.Errorf("DestroyCalls len = %d, want 0", len(m.DestroyCalls))
	}
}

func TestMockBackend_SendRecordsAndInjectsError(t *testing.T) {
	m := NewMockBackend()
	env := Envelope{From: "a@x", Rcpts: []string{"b@y"}}
	if err := m.Send(context.Background(), env, []byte("body")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(m.SendCalls) != 1 {
		t.Fatalf("SendCalls = %d, want 1", len(m.SendCalls))
	}
	if !reflect.DeepEqual(m.SendCalls[0].Env, env) {
		t.Errorf("SendCalls[0].Env = %+v, want %+v", m.SendCalls[0].Env, env)
	}
	if string(m.SendCalls[0].MIME) != "body" {
		t.Errorf("SendCalls[0].MIME = %q, want %q", m.SendCalls[0].MIME, "body")
	}

	want := errors.New("smtp denied")
	m.SendErr = want
	if got := m.Send(context.Background(), env, nil); !errors.Is(got, want) {
		t.Errorf("Send err = %v, want %v", got, want)
	}
	if len(m.SendCalls) != 2 {
		t.Errorf("SendCalls = %d, want 2 (call still recorded on error)", len(m.SendCalls))
	}
}

func TestMockBackend_AppendRecordsAndInjectsError(t *testing.T) {
	m := NewMockBackend()
	if err := m.Append(context.Background(), "Sent", []byte("mime"), FlagSeen); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if len(m.AppendCalls) != 1 {
		t.Fatalf("AppendCalls = %d, want 1", len(m.AppendCalls))
	}
	if m.AppendCalls[0].Folder != "Sent" || m.AppendCalls[0].Flag != FlagSeen {
		t.Errorf("AppendCalls[0] = %+v", m.AppendCalls[0])
	}

	want := errors.New("append denied")
	m.AppendErr = want
	if got := m.Append(context.Background(), "Sent", nil, 0); !errors.Is(got, want) {
		t.Errorf("Append err = %v, want %v", got, want)
	}
}

func TestMockBackend(t *testing.T) {
	b := NewMockBackend()

	t.Run("connect", func(t *testing.T) {
		if err := b.Connect(context.Background()); err != nil {
			t.Fatalf("Connect: %v", err)
		}
	})

	t.Run("folder list", func(t *testing.T) {
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

	t.Run("inbox unread", func(t *testing.T) {
		folders, _ := b.ListFolders()
		inbox := folders[0]
		if inbox.Unseen == 0 {
			t.Error("expected Inbox to have unread messages")
		}
		if inbox.Exists == 0 {
			t.Error("expected Inbox to have messages")
		}
	})

	t.Run("header fetch", func(t *testing.T) {
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

	t.Run("updates channel", func(t *testing.T) {
		ch := b.Updates()
		if ch == nil {
			t.Fatal("Updates() returned nil channel")
		}
	})

	t.Run("disconnect", func(t *testing.T) {
		if err := b.Disconnect(); err != nil {
			t.Fatalf("Disconnect: %v", err)
		}
	})
}
