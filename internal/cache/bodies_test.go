// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// seedMessage inserts a minimal message row and returns its id. Used
// by body-cache tests that need a parent message before inserting a
// body. SentAt is now()-offset so size-eviction tests can order rows.
func seedMessage(t *testing.T, a *Account, uid mail.UID, sentAt time.Time) int64 {
	t.Helper()
	res, err := a.db.Exec(`
        INSERT INTO messages (protocol_id, sent_at, flags) VALUES (?, ?, 0)`,
		string(uid), sentAt.UnixNano())
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestLookupBody_Miss(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	_, ok, err := a.lookupBody(context.Background(), "no-such-uid")
	if err != nil {
		t.Fatalf("lookupBody: %v", err)
	}
	if ok {
		t.Errorf("lookupBody on missing uid: ok=true, want false")
	}
}

func TestLookupBody_Hit(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	uid := mail.UID("hit-me")
	msgID := seedMessage(t, a, uid, time.Now())
	want := []byte("hello rfc822")
	if _, err := a.db.Exec(`INSERT INTO bodies (message, bytes, fetched_at) VALUES (?, ?, ?)`,
		msgID, want, time.Now().UnixNano()); err != nil {
		t.Fatalf("seed body: %v", err)
	}

	got, ok, err := a.lookupBody(context.Background(), uid)
	if err != nil {
		t.Fatalf("lookupBody: %v", err)
	}
	if !ok {
		t.Fatalf("lookupBody on present uid: ok=false")
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
