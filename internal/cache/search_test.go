package cache

import (
	"context"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/search"
)

// seedSearchAccount upserts three messages across Inbox + Archive,
// each with a body that storeBody indexes into FTS.
func seedSearchAccount(t *testing.T) (*Account, context.Context) {
	t.Helper()
	a := openTestAccount(t)
	ctx := context.Background()

	// Test backend only knows INBOX; add Archive folder via direct insert.
	if _, err := a.db.Exec(`INSERT INTO folders (name, protocol_name) VALUES ('Archive', 'Archive')`); err != nil {
		t.Fatalf("seed Archive folder: %v", err)
	}

	now := time.Now()
	rows := []struct {
		uid     mail.UID
		folder  string
		subject string
		from    string
		to      string
		body    string
		when    time.Time
	}{
		{"u1", "Inbox", "Quarterly review", "alice@example.com", "team@example.com",
			"project pelican kicks off Monday", now.Add(-3 * time.Hour)},
		{"u2", "Inbox", "Lunch?", "bob@example.com", "alice@example.com",
			"are you free thursday", now.Add(-2 * time.Hour)},
		{"u3", "Archive", "Re: pelican plans", "alice@example.com", "carol@example.com",
			"timeline updated for pelican rollout", now.Add(-1 * time.Hour)},
	}
	for _, r := range rows {
		if err := a.upsertMessages(ctx, r.folder, []mail.MessageInfo{
			{UID: r.uid, Subject: r.subject, From: r.from, To: r.to, SentAt: r.when},
		}); err != nil {
			t.Fatalf("upsert %s: %v", r.uid, err)
		}
		mime := "From: " + r.from + "\r\nTo: " + r.to + "\r\nSubject: " + r.subject +
			"\r\nContent-Type: text/plain\r\n\r\n" + r.body
		if err := a.storeBody(ctx, r.uid, []byte(mime)); err != nil {
			t.Fatalf("storeBody %s: %v", r.uid, err)
		}
	}
	return a, ctx
}

func TestSearch_BareTermCrossFolder(t *testing.T) {
	a, ctx := seedSearchAccount(t)
	hits, err := a.Search(ctx, search.Parse("pelican"), SearchScope{}, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2", len(hits))
	}
	// Sorted sent_at DESC: u3 (Archive) before u1 (Inbox).
	if hits[0].UID != "u3" || hits[0].Folder != "Archive" {
		t.Errorf("first hit: %+v", hits[0])
	}
	if hits[1].UID != "u1" || hits[1].Folder != "Inbox" {
		t.Errorf("second hit: %+v", hits[1])
	}
}

func TestSearch_FromOperator(t *testing.T) {
	a, ctx := seedSearchAccount(t)
	hits, err := a.Search(ctx, search.Parse("from:alice"), SearchScope{}, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2 (u1+u3)", len(hits))
	}
}

func TestSearch_FolderScope(t *testing.T) {
	a, ctx := seedSearchAccount(t)
	hits, err := a.Search(ctx, search.Parse("pelican"), SearchScope{Folder: "Inbox"}, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].UID != "u1" {
		t.Errorf("folder-scoped hits: %+v", hits)
	}
}

func TestSearch_SubjectOperator(t *testing.T) {
	a, ctx := seedSearchAccount(t)
	hits, err := a.Search(ctx, search.Parse("subject:lunch"), SearchScope{}, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].UID != "u2" {
		t.Errorf("subject hits: %+v", hits)
	}
}

func TestSearch_InOperator(t *testing.T) {
	a, ctx := seedSearchAccount(t)
	hits, err := a.Search(ctx, search.Parse("pelican in:Archive"), SearchScope{}, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].UID != "u3" {
		t.Errorf("in:Archive hits: %+v", hits)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	a, ctx := seedSearchAccount(t)
	hits, err := a.Search(ctx, search.Query{}, SearchScope{}, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if hits != nil {
		t.Errorf("zero query should return nil, got %d hits", len(hits))
	}
}

func TestSearch_NoMatches(t *testing.T) {
	a, ctx := seedSearchAccount(t)
	hits, err := a.Search(ctx, search.Parse("zzznonsensezzz"), SearchScope{}, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits, got %d", len(hits))
	}
}
