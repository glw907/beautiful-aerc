package cache

import (
	"context"
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
