package store

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"
)

// TestMessageFTSSurvivesCascadeDelete proves message_fts stays
// consistent with message across a cascading delete, not only a
// direct one. Deleting an account cascades straight to message
// (message.account_id's own foreign key; message has none to
// mailbox) inside SQLite itself, below trg_message_fts_insert and
// trg_message_fts_update, which are scoped to writes on message
// itself. Without trg_message_fts_delete, that cascade would orphan
// the deleted row's terms in the index. An orphaned term still
// matches a search and, once a caller reads message_fts's content
// columns for it, raises SQLite's own "missing row from content
// table" corruption error, which is the concrete failure this
// trigger prevents.
func TestMessageFTSSurvivesCascadeDelete(t *testing.T) {
	db := openMigratedTestDB(t)
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text) VALUES (1, 1, 0, 'hello', 'world')`); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if got := matchingRowCount(t, db, "hello"); got != 1 {
		t.Fatalf("matching rows before delete = %d, want 1", got)
	}

	if _, err := db.Exec(`DELETE FROM account WHERE id = 1`); err != nil {
		t.Fatalf("delete account: %v", err)
	}

	var messageCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message`).Scan(&messageCount); err != nil {
		t.Fatalf("count message: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("message row survived the account cascade, want 0, got %d", messageCount)
	}

	if got := matchingRowCount(t, db, "hello"); got != 0 {
		t.Fatalf("matching rows after cascade delete = %d, want 0: message_fts still holds the deleted row's term", got)
	}
}

// TestUnindexedMessageRowMustBeIndexed pins trg_message_fts_delete's
// reciprocal invariant, the one documented on the trigger itself:
// every message row carries a message_fts entry. The test reaches the
// one state that invariant forbids by stripping a row's own fts entry
// by hand, with the same 'delete' command the trigger uses. Deleting
// the row after that strip runs the trigger's own delete against an
// entry that is already gone. Probed against modernc.org/sqlite
// v1.54.0, the result is SQLite's disk-image-malformed error, not a
// clean no-op.
func TestUnindexedMessageRowMustBeIndexed(t *testing.T) {
	db := openMigratedTestDB(t)
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text) VALUES (1, 1, 0, 'hello', 'world')`); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO message_fts(message_fts, rowid, subject, search_text) VALUES ('delete', 1, 'hello', 'world')`); err != nil {
		t.Fatalf("strip fts entry: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM message WHERE id = 1`); err == nil {
		t.Fatal("delete of a stripped message row succeeded, want the trigger's disk-image error the reciprocal invariant predicts")
	}
}

// TestIndexTransactional asserts trg_message_fts_insert's write lands
// or vanishes with the message insert it shares a transaction with. A
// rolled-back write leaves no message_fts row; a committed one leaves
// exactly one.
func TestIndexTransactional(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())
	seedAccountAndMailbox(t, w)

	forcedFailure := errors.New("forced rollback")
	err := w.submit(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text) VALUES (1, 1, 0, 'hello', 'world')`); err != nil {
			return err
		}
		return forcedFailure
	})
	if err == nil {
		t.Fatal("submit succeeded, want the forced failure to roll back")
	}
	if got := matchingRowCount(t, w.db, "hello"); got != 0 {
		t.Fatalf("matching rows after rolled-back write = %d, want 0", got)
	}

	err = w.submit(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text) VALUES (1, 1, 0, 'hello', 'world')`)
		return err
	})
	if err != nil {
		t.Fatalf("commit message write: %v", err)
	}
	if got := matchingRowCount(t, w.db, "hello"); got != 1 {
		t.Fatalf("matching rows after committed write = %d, want 1", got)
	}
}

// TestBackfillReindexes covers a message indexed before its body
// arrives. The initial insert carries an empty search_text, so a body
// term finds nothing. A later update lands search_text, and
// trg_message_fts_update reindexes the row from it.
func TestBackfillReindexes(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())
	seedAccountAndMailbox(t, w)

	err := w.submit(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text) VALUES (1, 1, 0, 'hello', '')`)
		return err
	})
	if err != nil {
		t.Fatalf("index message before body: %v", err)
	}
	if got := matchingRowCount(t, w.db, "hello"); got != 1 {
		t.Fatalf("subject search before backfill = %d, want 1", got)
	}
	if got := matchingRowCount(t, w.db, "world"); got != 0 {
		t.Fatalf("body search before backfill = %d, want 0 (the body has not arrived yet)", got)
	}

	err = w.submit(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE message SET search_text = 'world arrives' WHERE id = 1`)
		return err
	})
	if err != nil {
		t.Fatalf("backfill body: %v", err)
	}
	if got := matchingRowCount(t, w.db, "world"); got != 1 {
		t.Fatalf("body search after backfill = %d, want 1", got)
	}
	if got := matchingRowCount(t, w.db, "hello"); got != 1 {
		t.Fatalf("subject search after backfill = %d, want 1 (subject reindex must not drop the row)", got)
	}
}

// TestRebuildIndex corrupts message_fts two ways and asserts
// RebuildIndex repairs both: FTS5's own 'delete-all' command, which
// empties the index while leaving message untouched, and a bogus
// entry planted under an existing row's id, which leaves the index
// non-empty but wrong. A rebuild that only appended missing rows
// would pass the first case and miss the second; --rebuild-index
// exists for stale terms, not only a gone index.
func TestRebuildIndex(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())
	db := w.db

	if _, err := db.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	messages := []struct {
		id                  int64
		subject, searchText string
	}{
		{1, "alpha bravo", "charlie delta"},
		{2, "bravo charlie", "echo"},
		{3, "delta echo", "alpha"},
	}
	for _, m := range messages {
		if _, err := db.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text) VALUES (?, 1, 0, ?, ?)`,
			m.id, m.subject, m.searchText); err != nil {
			t.Fatalf("insert message %d: %v", m.id, err)
		}
	}

	terms := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	baseline := make(map[string][]int64, len(terms))
	for _, term := range terms {
		baseline[term] = searchIDs(t, db, term)
	}

	if _, err := db.Exec(`INSERT INTO message_fts(message_fts) VALUES ('delete-all')`); err != nil {
		t.Fatalf("corrupt index with delete-all: %v", err)
	}
	for _, term := range terms {
		if got := searchIDs(t, db, term); len(got) != 0 {
			t.Fatalf("search(%q) after delete-all = %v, want empty", term, got)
		}
	}

	if err := RebuildIndex(context.Background(), w); err != nil {
		t.Fatalf("RebuildIndex after delete-all: %v", err)
	}
	for _, term := range terms {
		got := searchIDs(t, db, term)
		if !slices.Equal(got, baseline[term]) {
			t.Fatalf("search(%q) after rebuild = %v, want the pre-corruption baseline %v", term, got, baseline[term])
		}
	}

	// message 2's real subject is "bravo charlie", which shares no
	// term with "zulu": planting "zulu" under its rowid is a stale
	// term a rebuild must discard, not a row a rebuild would ever add
	// on its own.
	if _, err := db.Exec(`INSERT INTO message_fts(rowid, subject, search_text) VALUES (2, 'zulu', '')`); err != nil {
		t.Fatalf("plant bogus fts entry: %v", err)
	}
	if got := searchIDs(t, db, "zulu"); !slices.Equal(got, []int64{2}) {
		t.Fatalf("search(\"zulu\") after planting = %v, want [2]", got)
	}

	if err := RebuildIndex(context.Background(), w); err != nil {
		t.Fatalf("RebuildIndex after planting a bogus entry: %v", err)
	}
	if got := searchIDs(t, db, "zulu"); len(got) != 0 {
		t.Fatalf("search(\"zulu\") after rebuild = %v, want empty: rebuild must discard the stale term", got)
	}
	for _, term := range terms {
		got := searchIDs(t, db, term)
		if !slices.Equal(got, baseline[term]) {
			t.Fatalf("search(%q) after rebuild = %v, want the pre-corruption baseline %v", term, got, baseline[term])
		}
	}
}

// matchingRowCount returns how many message_fts rows match term.
func matchingRowCount(t *testing.T, db *sql.DB, term string) int {
	t.Helper()

	return len(searchIDs(t, db, term))
}

// searchIDs returns the message ids message_fts matches against term,
// in ascending order.
func searchIDs(t *testing.T, db *sql.DB, term string) []int64 {
	t.Helper()

	rows, err := db.Query(`SELECT rowid FROM message_fts WHERE message_fts MATCH ? ORDER BY rowid`, term)
	if err != nil {
		t.Fatalf("search %q: %v", term, err)
	}
	defer func() { _ = rows.Close() }()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan search row for %q: %v", term, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate search rows for %q: %v", term, err)
	}
	return ids
}
