package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/uerr"
)

// newRecoverableTestWriter is newTestWriter without the automatic
// Close on cleanup: every SY-8 recovery test below closes its writer
// explicitly, mid-test, to release the file before Recover rebuilds
// it, and a second, cleanup-driven Close would double-close the
// writer's stop channel.
func newRecoverableTestWriter(t *testing.T, cfg WriterConfig) (*Writer, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	w, err := NewWriter(db, cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	return w, path
}

// TestIntegrityCheckSkipped is the QA-1 gate: a clean shutdown must
// skip the next startup's integrity check entirely, since the spike
// measured quick_check at 14.5s on a 924MB store.
func TestIntegrityCheckSkipped(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	if !NeedsIntegrityCheck(dbPath, false) {
		t.Fatal("NeedsIntegrityCheck with no marker and no migration = false, want true: a first launch owes a check")
	}

	if err := MarkCleanShutdown(dbPath); err != nil {
		t.Fatalf("MarkCleanShutdown: %v", err)
	}
	if NeedsIntegrityCheck(dbPath, false) {
		t.Fatal("NeedsIntegrityCheck after a clean shutdown = true, want false: quick_check must not run")
	}

	// The marker is consumed by the read above: a second read without
	// an intervening clean shutdown owes another check.
	if !NeedsIntegrityCheck(dbPath, false) {
		t.Fatal("NeedsIntegrityCheck after consuming the marker = false, want true")
	}
}

// TestIntegrityCheckRunsAfterMigration proves a migration forces the
// check even over a clean-shutdown marker: a schema change is exactly
// the case a stale index or a bad upgrade could leave undetected.
func TestIntegrityCheckRunsAfterMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	if err := MarkCleanShutdown(dbPath); err != nil {
		t.Fatalf("MarkCleanShutdown: %v", err)
	}
	if !NeedsIntegrityCheck(dbPath, true) {
		t.Fatal("NeedsIntegrityCheck after a migration = false, want true even with a clean marker present")
	}
}

// TestCheckIntegrityPassesCleanStore proves a freshly migrated,
// untouched store passes both stages of the check.
func TestCheckIntegrityPassesCleanStore(t *testing.T) {
	db := openMigratedTestDB(t)

	if err := CheckIntegrity(context.Background(), db, nil); err != nil {
		t.Fatalf("CheckIntegrity on a fresh store: %v", err)
	}
}

// TestCheckIntegrityReportsProgress proves CheckIntegrity names each
// stage to its progress callback before running it, the visible
// progress state a long quick_check owes the operator (QA-1).
func TestCheckIntegrityReportsProgress(t *testing.T) {
	db := openMigratedTestDB(t)

	var stages []string
	if err := CheckIntegrity(context.Background(), db, func(stage string) { stages = append(stages, stage) }); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}

	want := []string{"checking store integrity", "checking full-text index"}
	if len(stages) != len(want) {
		t.Fatalf("stages = %v, want %v", stages, want)
	}
	for i, s := range want {
		if stages[i] != s {
			t.Errorf("stage %d = %q, want %q", i, stages[i], s)
		}
	}
}

// corruptFTSShadowTable forces a real, detectable corruption by
// deleting message_fts's own shadow-table rows out from under it,
// leaving the index unable to read back its own segments. Probed
// against modernc.org/sqlite v1.54.0, this trips both quick_check and
// FTS5's own integrity-check with a "corruption found" SQLite error.
func corruptFTSShadowTable(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec(`DELETE FROM message_fts_data WHERE rowid > 1`); err != nil {
		t.Fatalf("corrupt message_fts_data: %v", err)
	}
}

// TestCheckIntegrityDetectsCorruption forces a real, detectable
// corruption in the FTS index and proves CheckIntegrity surfaces it as
// a uerr.Error under ClassStoreLocal.
func TestCheckIntegrityDetectsCorruption(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())
	seedAccountAndMailbox(t, w)
	insertMessage(t, w, 1, 100)
	corruptFTSShadowTable(t, w.db)

	err := CheckIntegrity(context.Background(), w.db, nil)
	if err == nil {
		t.Fatal("CheckIntegrity over a corrupted index succeeded, want a failure")
	}
	var uerrErr uerr.Error
	if !errors.As(err, &uerrErr) {
		t.Fatalf("error is not a uerr.Error: %v", err)
	}
	if uerrErr.Class != uerr.ClassStoreLocal {
		t.Errorf("Class = %v, want %v", uerrErr.Class, uerr.ClassStoreLocal)
	}
}

// seedRecoveryFixture writes one account, one undispatched outbox row,
// a draft (message 200) with its local revision, a plain local message
// (message 300) with its body, and a disposable server-origin message
// (message 100): the mix every SY-8 recovery test rebuilds from.
func seedRecoveryFixture(t *testing.T, w *Writer) {
	t.Helper()

	err := w.submit(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO message (id, account_id, server_id, received_at, subject, search_text, origin)
			VALUES (100, 1, 'srv-100', 0, 'server subject', 'server body', 'server')`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO outbox (id, account_id, kind, payload, created_at) VALUES (1, 1, 'move-messages', '{}', 0)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text, origin)
			VALUES (200, 1, 0, 'draft subject', 'draft body', 'local')`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO body (message_id, content, fetched_at) VALUES (200, ?, 0)`, []byte("draft body content")); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO draft_meta (message_id, local_rev, pushed_rev, anchor_msgid) VALUES (200, 3, 1, 'anchor@x')`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text, origin)
			VALUES (300, 1, 0, 'local subject', 'local body', 'local')`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO body (message_id, content, fetched_at) VALUES (300, ?, 0)`, []byte("local content"))
		return err
	})
	if err != nil {
		t.Fatalf("seed recovery fixture: %v", err)
	}
}

// assertRecoveryPreservedFixture reads path's rebuilt store and
// asserts seedRecoveryFixture's preserved rows survived while its
// disposable server-origin message did not.
func assertRecoveryPreservedFixture(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite", dsn(path, connReadOnly))
	if err != nil {
		t.Fatalf("reopen rebuilt store: %v", err)
	}
	defer func() { _ = db.Close() }()

	var outboxCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id = 1`).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox row: %v", err)
	}
	if outboxCount != 1 {
		t.Error("outbox row missing after recovery, want it preserved")
	}

	var localRev int
	if err := db.QueryRow(`SELECT local_rev FROM draft_meta WHERE message_id = 200`).Scan(&localRev); err != nil {
		t.Errorf("draft_meta missing after recovery: %v", err)
	} else if localRev != 3 {
		t.Errorf("draft local_rev = %d, want 3", localRev)
	}

	var draftBody, localBody []byte
	if err := db.QueryRow(`SELECT content FROM body WHERE message_id = 200`).Scan(&draftBody); err != nil {
		t.Errorf("draft body missing after recovery: %v", err)
	}
	if err := db.QueryRow(`SELECT content FROM body WHERE message_id = 300`).Scan(&localBody); err != nil {
		t.Errorf("local message body missing after recovery: %v", err)
	}

	var disposableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message WHERE id = 100`).Scan(&disposableCount); err != nil {
		t.Fatalf("count disposable message: %v", err)
	}
	if disposableCount != 0 {
		t.Error("the disposable server-origin message survived recovery, want it dropped")
	}
}

// TestRecoverAfterCorruption is SY-8's forced-corruption test: a
// corrupted FTS index fails CheckIntegrity as a typed error, and
// Recover rebuilds the store, preserving the outbox row and both local
// messages while dropping the disposable server-origin one.
func TestRecoverAfterCorruption(t *testing.T) {
	w, path := newRecoverableTestWriter(t, DefaultWriterConfig())
	seedRecoveryFixture(t, w)
	corruptFTSShadowTable(t, w.db)

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	db, err := sql.Open("sqlite", dsn(path, connReadOnly))
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	checkErr := CheckIntegrity(context.Background(), db, nil)
	_ = db.Close()

	if checkErr == nil {
		t.Fatal("CheckIntegrity over the corrupted index succeeded, want a failure")
	}
	var uerrErr uerr.Error
	if !errors.As(checkErr, &uerrErr) {
		t.Fatalf("error is not a uerr.Error: %v", checkErr)
	}
	if uerrErr.Class != uerr.ClassStoreLocal {
		t.Errorf("Class = %v, want %v", uerrErr.Class, uerr.ClassStoreLocal)
	}

	counts, err := Recover(context.Background(), path)
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if counts.Outbox != 1 || counts.Messages != 2 {
		t.Errorf("RecoveredCounts = %+v, want {Outbox:1 Messages:2}", counts)
	}
	assertRecoveryPreservedFixture(t, path)
}

// TestRecoverAfterFailedMigration is SY-8's failed-migration test:
// resetting schema_version to 0 while the tables it would recreate
// already exist fails Migrate as a typed error (the same collision
// TestMigrateFailureReachesUerrSeam uses, now against a store already
// holding real preserved data), and Recover rebuilds from it.
func TestRecoverAfterFailedMigration(t *testing.T) {
	w, path := newRecoverableTestWriter(t, DefaultWriterConfig())
	seedRecoveryFixture(t, w)
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	db, err := sql.Open("sqlite", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if _, err := db.Exec(`UPDATE schema_version SET version = 0`); err != nil {
		t.Fatalf("reset schema_version: %v", err)
	}
	migrateErr := Migrate(db)
	_ = db.Close()

	if migrateErr == nil {
		t.Fatal("Migrate against a colliding schema succeeded, want a failure")
	}
	var uerrErr uerr.Error
	if !errors.As(migrateErr, &uerrErr) {
		t.Fatalf("error is not a uerr.Error: %v", migrateErr)
	}
	if uerrErr.Class != uerr.ClassStoreLocal {
		t.Errorf("Class = %v, want %v", uerrErr.Class, uerr.ClassStoreLocal)
	}

	counts, err := Recover(context.Background(), path)
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if counts.Outbox != 1 || counts.Messages != 2 {
		t.Errorf("RecoveredCounts = %+v, want {Outbox:1 Messages:2}", counts)
	}
	assertRecoveryPreservedFixture(t, path)
}

// TestUnquarantineRestoresOriginalOnFailedRebuild proves Recover's
// rollback half: when rebuildAt fails after the original store is
// already quarantined, unquarantine puts it back at path rather than
// leaving whatever rebuildAt left behind (an empty or partial fresh
// file) in its place.
func TestUnquarantineRestoresOriginalOnFailedRebuild(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	quarantined := path + ".corrupt-test"

	if err := os.WriteFile(quarantined, []byte("original store bytes"), 0o600); err != nil {
		t.Fatalf("seed quarantined file: %v", err)
	}

	// A path with a nonexistent parent directory makes OpenWriteConn
	// fail inside rebuildAt, the same shape of failure a full disk or
	// a cancelled context produces mid-rebuild.
	badPath := filepath.Join(t.TempDir(), "missing-dir", "store.db")
	preserved := preservedData{outbox: []preservedOutboxRow{{id: 1}}}
	if err := rebuildAt(context.Background(), badPath, preserved); err == nil {
		t.Fatal("rebuildAt over an impossible path succeeded, want a failure")
	}

	unquarantine(quarantined, path, preserved)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored path: %v", err)
	}
	if string(got) != "original store bytes" {
		t.Errorf("restored content = %q, want the quarantined original", got)
	}
	if _, err := os.Stat(quarantined); !os.IsNotExist(err) {
		t.Error("quarantined file still present after a successful restore")
	}
}

// TestRecoverRestoresDispatchingRowAsQueued proves a preserved outbox
// row a crashed run had claimed comes back as 'queued', not
// 'dispatching': the outbox's own claim query only ever selects
// 'queued' rows, so a row restored under 'dispatching' would sit
// forever undispatchable.
func TestRecoverRestoresDispatchingRowAsQueued(t *testing.T) {
	w, path := newRecoverableTestWriter(t, DefaultWriterConfig())
	err := w.submit(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO outbox (id, account_id, kind, payload, state, created_at) VALUES (1, 1, 'move-messages', '{}', 'dispatching', 0)`)
		return err
	})
	if err != nil {
		t.Fatalf("seed a dispatching outbox row: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	if _, err := Recover(context.Background(), path); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	db, err := sql.Open("sqlite", dsn(path, connReadOnly))
	if err != nil {
		t.Fatalf("reopen rebuilt store: %v", err)
	}
	defer func() { _ = db.Close() }()

	var state string
	if err := db.QueryRow(`SELECT state FROM outbox WHERE id = 1`).Scan(&state); err != nil {
		t.Fatalf("read restored outbox row: %v", err)
	}
	if state != "queued" {
		t.Errorf("restored state = %q, want %q", state, "queued")
	}
}

// TestRecoverAfterDiskFull is SY-8's full-disk test: max_page_count
// capped at the store's current size is the standard portable
// stand-in for an actual full disk (TestDiskFullInjection uses the
// same technique). The failing write neither corrupts nor loses
// anything on its own; Recover still runs cleanly against the capped
// file, since the fresh file it rebuilds carries no such cap.
func TestRecoverAfterDiskFull(t *testing.T) {
	w, path := newRecoverableTestWriter(t, DefaultWriterConfig())
	seedRecoveryFixture(t, w)

	var pageCount int
	if err := w.db.QueryRow(`PRAGMA page_count`).Scan(&pageCount); err != nil {
		t.Fatalf("read page_count: %v", err)
	}
	if _, err := w.db.Exec(fmt.Sprintf(`PRAGMA max_page_count = %d`, pageCount)); err != nil {
		t.Fatalf("cap max_page_count: %v", err)
	}

	big := strings.Repeat("x", 8192)
	writeErr := w.submit(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO message (id, account_id, received_at, subject, search_text, origin, data) VALUES (999, 1, 0, 'disposable', '', 'server', ?)`,
			big)
		return err
	})
	if writeErr == nil {
		t.Fatal("submit succeeded past max_page_count, want a disk-full failure")
	}
	var uerrErr uerr.Error
	if !errors.As(writeErr, &uerrErr) {
		t.Fatalf("error is not a uerr.Error: %v", writeErr)
	}
	if uerrErr.Class != uerr.ClassStoreLocal {
		t.Errorf("Class = %v, want %v", uerrErr.Class, uerr.ClassStoreLocal)
	}

	var count int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM message WHERE id = 999`).Scan(&count); err != nil {
		t.Fatalf("count disposable message: %v", err)
	}
	if count != 0 {
		t.Fatal("the failed transaction committed partially, want nothing (SY-8 never corrupts)")
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	counts, err := Recover(context.Background(), path)
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if counts.Outbox != 1 || counts.Messages != 2 {
		t.Errorf("RecoveredCounts = %+v, want {Outbox:1 Messages:2}", counts)
	}
	assertRecoveryPreservedFixture(t, path)
}
