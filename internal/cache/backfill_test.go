package cache

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestNextUnfetchedUID_NewestFirst(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	now := time.Now()
	older := mail.UID("older-uid")
	newer := mail.UID("newer-uid")
	seedMessage(t, a, older, now.Add(-time.Hour))
	seedMessage(t, a, newer, now)

	uid, ok, err := a.nextUnfetchedUID(context.Background())
	if err != nil {
		t.Fatalf("nextUnfetchedUID: %v", err)
	}
	if !ok {
		t.Fatalf("nextUnfetchedUID: ok=false, want true")
	}
	if uid != newer {
		t.Errorf("nextUnfetchedUID = %s, want %s (newest)", uid, newer)
	}
}

func TestNextUnfetchedUID_SkipsCachedBodies(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	now := time.Now()
	cached := mail.UID("cached")
	uncached := mail.UID("uncached")
	cachedID := seedMessage(t, a, cached, now)
	seedMessage(t, a, uncached, now.Add(-time.Hour))
	if _, err := a.db.Exec(
		`INSERT INTO bodies (message, bytes, fetched_at) VALUES (?, ?, ?)`,
		cachedID, []byte("body"), now.UnixNano(),
	); err != nil {
		t.Fatalf("seed body: %v", err)
	}

	uid, ok, err := a.nextUnfetchedUID(context.Background())
	if err != nil {
		t.Fatalf("nextUnfetchedUID: %v", err)
	}
	if !ok || uid != uncached {
		t.Errorf("nextUnfetchedUID = (%s, %v), want (%s, true)", uid, ok, uncached)
	}
}

func TestNextUnfetchedUID_AllCached(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	uid := mail.UID("all-cached")
	id := seedMessage(t, a, uid, time.Now())
	if _, err := a.db.Exec(
		`INSERT INTO bodies (message, bytes, fetched_at) VALUES (?, ?, ?)`,
		id, []byte("b"), time.Now().UnixNano(),
	); err != nil {
		t.Fatalf("seed body: %v", err)
	}

	_, ok, err := a.nextUnfetchedUID(context.Background())
	if err != nil {
		t.Fatalf("nextUnfetchedUID: %v", err)
	}
	if ok {
		t.Errorf("nextUnfetchedUID with all bodies cached: ok=true, want false")
	}
}

func TestBackfiller_OneShot(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	uid := mail.UID("u-1")
	seedMessage(t, a, uid, time.Now())
	a.Backend = &fakeBackendWithBody{body: []byte("hello")}

	bf := newBackfiller(a)
	if err := bf.fetchOne(context.Background()); err != nil {
		t.Fatalf("fetchOne: %v", err)
	}

	got, ok, err := a.lookupBody(context.Background(), uid)
	if err != nil || !ok {
		t.Fatalf("lookupBody: ok=%v err=%v", ok, err)
	}
	if string(got) != "hello" {
		t.Errorf("body = %q, want %q", got, "hello")
	}
}

func TestBackfiller_RunFillsThenIdles(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.Backend = &fakeBackendWithBody{body: []byte("body")}

	for i := 0; i < 3; i++ {
		seedMessage(t, a, mail.UID(fmt.Sprintf("u-%d", i)), time.Now().Add(-time.Duration(i)*time.Hour))
	}

	bf := newBackfiller(a)
	bf.rate = 5 * time.Millisecond
	bf.idleThreshold = 0

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	bf.Run(ctx)

	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM bodies`).Scan(&n); err != nil {
		t.Fatalf("count bodies: %v", err)
	}
	if n != 3 {
		t.Errorf("bodies stored = %d, want 3", n)
	}
}

func TestBackfiller_RespectsActivityGate(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.Backend = &fakeBackendWithBody{body: []byte("body")}
	seedMessage(t, a, mail.UID("u-1"), time.Now())

	bf := newBackfiller(a)
	bf.rate = 5 * time.Millisecond
	bf.idleThreshold = 100 * time.Millisecond
	bf.NotifyActivity()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	bf.Run(ctx)

	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM bodies`).Scan(&n)
	if n != 0 {
		t.Errorf("active gate failed: bodies = %d, want 0", n)
	}
}

func TestBackfiller_PausesWhenOffline(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.Backend = &fakeBackendWithBody{body: []byte("b")}
	seedMessage(t, a, mail.UID("u-1"), time.Now())

	bf := newBackfiller(a)
	bf.rate = 5 * time.Millisecond
	bf.idleThreshold = 0
	bf.NotifyConnState(false)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	bf.Run(ctx)

	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM bodies`).Scan(&n)
	if n != 0 {
		t.Errorf("offline gate failed: bodies = %d, want 0", n)
	}
}

func TestBackfiller_BailsAtCap(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	a.maxSize = 100
	a.Backend = &fakeBackendWithBody{body: bytes.Repeat([]byte("x"), 95)}

	for i := 0; i < 3; i++ {
		seedMessage(t, a, mail.UID(fmt.Sprintf("u-%d", i)), time.Now().Add(-time.Duration(i)*time.Hour))
	}

	bf := newBackfiller(a)
	bf.rate = 5 * time.Millisecond
	bf.idleThreshold = 0

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	bf.Run(ctx)

	var stored int
	a.db.QueryRow(`SELECT COUNT(*) FROM bodies`).Scan(&stored)
	if stored != 1 {
		t.Errorf("at-cap bail: stored=%d, want 1 (further fetches should bail)", stored)
	}
}
