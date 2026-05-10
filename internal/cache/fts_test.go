package cache

import (
	"context"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestFTSWriteHooks(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()

	if err := a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "u1", Subject: "Quarterly review", From: "alice@example.com", To: "bob@example.com", SentAt: time.Now()},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	mustMatch := func(q string) {
		t.Helper()
		var rowid int64
		if err := a.db.QueryRow(`SELECT rowid FROM messages_fts WHERE messages_fts MATCH ?`, q).Scan(&rowid); err != nil {
			t.Fatalf("MATCH %q: %v", q, err)
		}
		if rowid == 0 {
			t.Errorf("MATCH %q: rowid 0", q)
		}
	}

	mustMatch("subject:quarterly")
	mustMatch("from_addr:alice")

	// storeBody should populate the body column without disturbing headers.
	body := "From: a@x\r\nTo: b@y\r\nSubject: Quarterly review\r\n" +
		"Content-Type: text/plain\r\n\r\nproject pelican kicks off Monday"
	if err := a.storeBody(ctx, mail.UID("u1"), []byte(body)); err != nil {
		t.Fatalf("storeBody: %v", err)
	}
	mustMatch("body:pelican")
	mustMatch("subject:quarterly") // headers preserved across body write
}

func TestFTSHeaderUpsertPreservesBody(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()

	if err := a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "u1", Subject: "first", From: "a@x", SentAt: time.Now()},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	body := "From: a@x\r\nSubject: first\r\nContent-Type: text/plain\r\n\r\nbeacon term inside"
	if err := a.storeBody(ctx, mail.UID("u1"), []byte(body)); err != nil {
		t.Fatalf("storeBody: %v", err)
	}
	// Re-upsert with new headers.
	if err := a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "u1", Subject: "second", From: "a@x", SentAt: time.Now()},
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	var rowid int64
	if err := a.db.QueryRow(`SELECT rowid FROM messages_fts WHERE messages_fts MATCH 'body:beacon'`).Scan(&rowid); err != nil {
		t.Fatalf("body MATCH after re-upsert: %v", err)
	}
	if err := a.db.QueryRow(`SELECT rowid FROM messages_fts WHERE messages_fts MATCH 'subject:second'`).Scan(&rowid); err != nil {
		t.Fatalf("subject MATCH after re-upsert: %v", err)
	}
}
