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

func TestStoreBody_Insert(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	uid := mail.UID("store-me")
	seedMessage(t, a, uid, time.Now())

	body := []byte("RFC822 body bytes")
	if err := a.storeBody(context.Background(), uid, body); err != nil {
		t.Fatalf("storeBody: %v", err)
	}

	got, ok, err := a.lookupBody(context.Background(), uid)
	if err != nil || !ok {
		t.Fatalf("lookup after store: ok=%v err=%v", ok, err)
	}
	if string(got) != string(body) {
		t.Errorf("got %q, want %q", got, body)
	}
}

func TestStoreBody_Replace(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	uid := mail.UID("replace-me")
	seedMessage(t, a, uid, time.Now())

	if err := a.storeBody(context.Background(), uid, []byte("first")); err != nil {
		t.Fatalf("first store: %v", err)
	}
	if err := a.storeBody(context.Background(), uid, []byte("second")); err != nil {
		t.Fatalf("second store: %v", err)
	}

	got, _, err := a.lookupBody(context.Background(), uid)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("got %q, want %q", got, "second")
	}
}

func TestStoreBody_UnknownUID(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	err := a.storeBody(context.Background(), mail.UID("never-seeded"), []byte("body"))
	if err == nil {
		t.Errorf("storeBody on unknown uid: nil error, want error")
	}
}
