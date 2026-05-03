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

func TestStoreBody_EvictsBySizeWhenOverCap(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.maxSize = 100 // tiny cap for the test

	now := time.Now()
	// Three messages with descending sent_at — older messages are
	// "older sent" and should evict first.
	old := mail.UID("old-msg") // sent 2 days ago
	mid := mail.UID("mid-msg") // sent 1 day ago
	new := mail.UID("new-msg") // sent now
	seedMessage(t, a, old, now.Add(-48*time.Hour))
	seedMessage(t, a, mid, now.Add(-24*time.Hour))
	seedMessage(t, a, new, now)

	// Each body is 50 bytes. Total cap is 100, so storing the third
	// body should evict the oldest-sent body to fit.
	body50 := make([]byte, 50)
	if err := a.storeBody(context.Background(), old, body50); err != nil {
		t.Fatalf("store old: %v", err)
	}
	if err := a.storeBody(context.Background(), mid, body50); err != nil {
		t.Fatalf("store mid: %v", err)
	}
	if err := a.storeBody(context.Background(), new, body50); err != nil {
		t.Fatalf("store new: %v", err)
	}

	// old should be gone; mid and new survive.
	for _, c := range []struct {
		uid  mail.UID
		want bool
	}{
		{old, false},
		{mid, true},
		{new, true},
	} {
		_, ok, err := a.lookupBody(context.Background(), c.uid)
		if err != nil {
			t.Fatalf("lookup %s: %v", c.uid, err)
		}
		if ok != c.want {
			t.Errorf("uid %s: hit=%v, want %v", c.uid, ok, c.want)
		}
	}
}

func TestStoreBody_MaxSizeZeroDisablesCap(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.maxSize = 0 // disabled

	now := time.Now()
	for i, uid := range []mail.UID{"a", "b", "c"} {
		seedMessage(t, a, uid, now.Add(time.Duration(-i)*time.Hour))
		if err := a.storeBody(context.Background(), uid, make([]byte, 1000)); err != nil {
			t.Fatalf("store %s: %v", uid, err)
		}
	}

	for _, uid := range []mail.UID{"a", "b", "c"} {
		_, ok, err := a.lookupBody(context.Background(), uid)
		if err != nil {
			t.Fatalf("lookup %s: %v", uid, err)
		}
		if !ok {
			t.Errorf("uid %s: evicted with maxSize=0", uid)
		}
	}
}

// fakeBackendWithBody is a minimal mail.Backend that returns a fixed
// body and counts FetchBody calls. Used to verify the write-through
// path doesn't re-call the backend on cache hit.
type fakeBackendWithBody struct {
	fakeBackend
	calls int
	body  []byte
}

func (f *fakeBackendWithBody) FetchBody(_ mail.UID) ([]byte, error) {
	f.calls++
	return f.body, nil
}

func TestFetchBody_PopulatesCacheOnMiss(t *testing.T) {
	be := &fakeBackendWithBody{body: []byte("from-backend")}
	a, err := Open("test", be, &fakeChangeTracker{}, t.TempDir(), Config{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()

	uid := mail.UID("write-through")
	seedMessage(t, a, uid, time.Now())

	body1, err := a.FetchBody(uid)
	if err != nil {
		t.Fatalf("FetchBody first: %v", err)
	}
	if string(body1) != "from-backend" {
		t.Errorf("first body=%q, want %q", body1, "from-backend")
	}
	if be.calls != 1 {
		t.Errorf("backend calls=%d after miss, want 1", be.calls)
	}

	body2, err := a.FetchBody(uid)
	if err != nil {
		t.Fatalf("FetchBody second: %v", err)
	}
	if string(body2) != "from-backend" {
		t.Errorf("second body=%q, want %q", body2, "from-backend")
	}
	if be.calls != 1 {
		t.Errorf("backend calls=%d after hit, want 1 (no re-fetch)", be.calls)
	}
}

func TestEvictByAge(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	now := time.Now()
	old := mail.UID("old")
	new := mail.UID("new")
	seedMessage(t, a, old, now.Add(-90*24*time.Hour))
	seedMessage(t, a, new, now)

	// Insert with explicit fetched_at so we can control the boundary.
	insert := func(uid mail.UID, fetchedAt time.Time) {
		t.Helper()
		var id int64
		if err := a.db.QueryRow(`SELECT id FROM messages WHERE protocol_id = ?`, string(uid)).Scan(&id); err != nil {
			t.Fatalf("lookup msg: %v", err)
		}
		if _, err := a.db.Exec(`INSERT INTO bodies (message, bytes, fetched_at) VALUES (?, ?, ?)`,
			id, []byte("body"), fetchedAt.UnixNano()); err != nil {
			t.Fatalf("insert body: %v", err)
		}
	}
	insert(old, now.Add(-30*24*time.Hour))
	insert(new, now)

	cutoff := now.Add(-7 * 24 * time.Hour)
	evicted, freed, err := a.EvictByAge(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("EvictByAge: %v", err)
	}
	if evicted != 1 {
		t.Errorf("evicted=%d, want 1", evicted)
	}
	if freed != int64(len("body")) {
		t.Errorf("freed=%d, want %d", freed, len("body"))
	}

	// old gone, new survives
	if _, ok, _ := a.lookupBody(context.Background(), old); ok {
		t.Errorf("old body should have been evicted")
	}
	if _, ok, _ := a.lookupBody(context.Background(), new); !ok {
		t.Errorf("new body should still be cached")
	}
}
