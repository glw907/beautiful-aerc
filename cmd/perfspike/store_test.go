package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSchemaInit(t *testing.T) {
	dir := t.TempDir()
	w, r, err := openDBAt(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("openDBAt: %v", err)
	}
	defer w.Close()
	defer r.Close()

	if err := initSchema(w); err != nil {
		t.Fatalf("initSchema: %v", err)
	}
}

func TestSchemaIdempotent(t *testing.T) {
	dir := t.TempDir()
	w, r, err := openDBAt(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("openDBAt: %v", err)
	}
	defer w.Close()
	defer r.Close()

	for range 3 {
		if err := initSchema(w); err != nil {
			t.Fatalf("initSchema run: %v", err)
		}
	}
}

func TestInsertMessage_FTSTrigger(t *testing.T) {
	dir := t.TempDir()
	w, r, err := openDBAt(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("openDBAt: %v", err)
	}
	defer w.Close()
	defer r.Close()

	if err := initSchema(w); err != nil {
		t.Fatalf("initSchema: %v", err)
	}

	msg := message{
		serverID:      "msg-001",
		threadKey:     "thread-001",
		mailbox:       "Inbox",
		receivedAt:    time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC).Unix(),
		subject:       "quarterly invoice",
		fromAddr:      "sender@example.com",
		body:          "please review the attached quarterly invoice",
		hasAttachment: true,
		size:          4096,
		data:          `{"server_id":"msg-001"}`,
	}

	if err := insertMessages(w, []message{msg}); err != nil {
		t.Fatalf("insertMessages: %v", err)
	}

	var count int
	if err := r.QueryRow("SELECT count(*) FROM message").Scan(&count); err != nil {
		t.Fatalf("count message: %v", err)
	}
	if count != 1 {
		t.Errorf("message count = %d, want 1", count)
	}

	var ftsCount int
	if err := r.QueryRow(
		"SELECT count(*) FROM message_fts WHERE message_fts MATCH ?", "invoice",
	).Scan(&ftsCount); err != nil {
		t.Fatalf("count FTS: %v", err)
	}
	if ftsCount != 1 {
		t.Errorf("FTS count = %d, want 1", ftsCount)
	}
}

func TestInsertMessages_DuplicateIgnored(t *testing.T) {
	dir := t.TempDir()
	w, r, err := openDBAt(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("openDBAt: %v", err)
	}
	defer w.Close()
	defer r.Close()

	if err := initSchema(w); err != nil {
		t.Fatalf("initSchema: %v", err)
	}

	msg := message{
		serverID:   "dup-001",
		threadKey:  "t1",
		mailbox:    "Inbox",
		receivedAt: time.Now().Unix(),
		subject:    "duplicate test",
		fromAddr:   "a@b.com",
		body:       "body text here",
		data:       "{}",
	}

	if err := insertMessages(w, []message{msg, msg}); err != nil {
		t.Fatalf("insertMessages: %v", err)
	}

	var count int
	if err := r.QueryRow("SELECT count(*) FROM message").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("message count = %d, want 1 (duplicate must be ignored)", count)
	}
}

func TestHarvestState(t *testing.T) {
	dir := t.TempDir()
	w, r, err := openDBAt(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("openDBAt: %v", err)
	}
	defer w.Close()
	defer r.Close()

	if err := initSchema(w); err != nil {
		t.Fatalf("initSchema: %v", err)
	}

	if err := setState(w, "pos", "42"); err != nil {
		t.Fatalf("setState: %v", err)
	}

	got, ok, err := getState(r, "pos")
	if err != nil {
		t.Fatalf("getState: %v", err)
	}
	if !ok {
		t.Fatal("getState: key not found")
	}
	if got != "42" {
		t.Errorf("getState = %q, want %q", got, "42")
	}

	_, ok2, err := getState(r, "missing-key")
	if err != nil {
		t.Fatalf("getState missing: %v", err)
	}
	if ok2 {
		t.Error("getState: expected not-found for absent key")
	}
}

func TestMessageFTSMailboxIndex(t *testing.T) {
	dir := t.TempDir()
	w, r, err := openDBAt(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("openDBAt: %v", err)
	}
	defer w.Close()
	defer r.Close()

	if err := initSchema(w); err != nil {
		t.Fatalf("initSchema: %v", err)
	}

	msgs := []message{
		{serverID: "a1", threadKey: "t1", mailbox: "Inbox", receivedAt: 100, subject: "alpha", fromAddr: "x@y.com", body: "alpha content", data: "{}"},
		{serverID: "a2", threadKey: "t2", mailbox: "Sent", receivedAt: 200, subject: "beta", fromAddr: "x@y.com", body: "beta content", data: "{}"},
		{serverID: "a3", threadKey: "t3", mailbox: "Inbox", receivedAt: 300, subject: "gamma", fromAddr: "x@y.com", body: "gamma content", data: "{}"},
	}
	if err := insertMessages(w, msgs); err != nil {
		t.Fatalf("insertMessages: %v", err)
	}

	rows, err := r.Query(
		"SELECT id FROM message WHERE mailbox = ? ORDER BY received_at DESC LIMIT 50",
		"Inbox",
	)
	if err != nil {
		t.Fatalf("query mailbox: %v", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ids = append(ids, id)
	}
	if len(ids) != 2 {
		t.Errorf("Inbox count = %d, want 2", len(ids))
	}
	// Most recent first
	if ids[0] <= ids[1] {
		t.Errorf("expected descending received_at, got ids %v", ids)
	}
}
