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
	"github.com/glw907/poplar/internal/uerr/uerrtest"
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

// TestIntegrityCheckRunsOnUnconsumableMarker proves a marker that
// survives its own read forces the check instead of being trusted. A
// marker attests to the run that just ended, so one that cannot be
// consumed reads as a clean shutdown on every later start, including
// the start after a real crash, disabling SY-8's corruption gate for
// the life of the store.
func TestIntegrityCheckRunsOnUnconsumableMarker(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	// A non-empty directory at the marker path stats like a marker and
	// refuses os.Remove, the same shape a read-only store directory
	// gives a marker file.
	marker := cleanShutdownMarker(dbPath)
	if err := os.Mkdir(marker, 0o700); err != nil {
		t.Fatalf("seed an unremovable marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(marker, "child"), nil, 0o600); err != nil {
		t.Fatalf("fill the marker directory: %v", err)
	}

	if !NeedsIntegrityCheck(dbPath, false) {
		t.Fatal("NeedsIntegrityCheck over a marker that could not be consumed = false, want true")
	}
}

// TestIntegrityCheckReportsAnUnreadableMarkerPath proves a marker
// whose path cannot even be read is reported. Every later start reads
// it the same way and runs the full quick_check, so an operator whose
// state directory has gone unreadable is owed the reason rather than a
// silent 14-second check on every launch.
func TestIntegrityCheckReportsAnUnreadableMarkerPath(t *testing.T) {
	log := uerrtest.CaptureDefault(t)

	// A regular file where a directory belongs fails the stat with
	// ENOTDIR, the same shape a permissions failure on the state
	// directory gives it, and unlike a missing marker.
	notADir := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(notADir, nil, 0o600); err != nil {
		t.Fatalf("seed a file where the state directory belongs: %v", err)
	}
	dbPath := filepath.Join(notADir, "store.db")

	if !NeedsIntegrityCheck(dbPath, false) {
		t.Fatal("NeedsIntegrityCheck over an unreadable marker path = false, want true")
	}
	if !strings.Contains(log.String(), "store.db.clean-shutdown") {
		t.Errorf("log = %q, want it naming the marker path it could not read", log.String())
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

// seedRecoveryFixture writes one account, one mailbox (id 7, the
// internal key an intent payload would name), one undispatched outbox
// row, a draft (message 200) with its local revision, a plain local
// message (message 300) with its body, and a disposable server-origin
// message (message 100): the mix every SY-8 recovery test rebuilds
// from.
func seedRecoveryFixture(t *testing.T, w *Writer) {
	t.Helper()

	err := w.submit(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(seedAccountSQL); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO mailbox (id, account_id, server_id, role, name, sort_order, unread_count, total_count)
			VALUES (7, 1, 'srv-mbx-7', 'archive', 'Archive', 4, 2, 9)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO message (id, account_id, server_id, received_at, subject, search_text, origin)
			VALUES (100, 1, 'srv-100', 0, 'server subject', 'server body', 'server')`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at, unread) VALUES (100, 7, 0, 1)`); err != nil {
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
		if _, err := tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at, unread) VALUES (200, 7, 11, 1)`); err != nil {
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

	// Id 7 is the assertion, not the name: a queued intent names its
	// mailbox by that internal key, so a rebuilt store minting a fresh
	// id for Archive would silently rebind the intent.
	var serverID, name, role string
	var sortOrder, unread, total int64
	if err := db.QueryRow(`SELECT server_id, name, role, sort_order, unread_count, total_count FROM mailbox WHERE id = 7`).
		Scan(&serverID, &name, &role, &sortOrder, &unread, &total); err != nil {
		t.Errorf("mailbox 7 missing after recovery: %v", err)
	} else if serverID != "srv-mbx-7" || name != "Archive" || role != "archive" || sortOrder != 4 || unread != 2 || total != 9 {
		t.Errorf("mailbox 7 = (%q, %q, %q, %d, %d, %d), want (%q, %q, %q, 4, 2, 9)",
			serverID, name, role, sortOrder, unread, total, "srv-mbx-7", "Archive", "archive")
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

	// A preserved draft that comes back in no folder is invisible to
	// every list the operator has: the membership row is as
	// non-rebuildable as the message itself.
	var draftMailbox, draftReceivedAt, draftUnread int64
	if err := db.QueryRow(`SELECT mailbox_id, received_at, unread FROM message_mailbox WHERE message_id = 200`).
		Scan(&draftMailbox, &draftReceivedAt, &draftUnread); err != nil {
		t.Errorf("the preserved draft came back in no folder: %v", err)
	} else if draftMailbox != 7 || draftReceivedAt != 11 || draftUnread != 1 {
		t.Errorf("draft membership = (%d, %d, %d), want (7, 11, 1)", draftMailbox, draftReceivedAt, draftUnread)
	}

	var disposableMembership int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message_mailbox WHERE message_id = 100`).Scan(&disposableMembership); err != nil {
		t.Fatalf("count the disposable message's membership: %v", err)
	}
	if disposableMembership != 0 {
		t.Error("the disposable server-origin message kept a folder membership, want it dropped with the message")
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
	if counts.Outbox != 1 || counts.Mailboxes != 1 || counts.Messages != 2 {
		t.Errorf("RecoveredCounts = %+v, want {Outbox:1 Mailboxes:1 Messages:2}", counts)
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
	if counts.Outbox != 1 || counts.Mailboxes != 1 || counts.Messages != 2 {
		t.Errorf("RecoveredCounts = %+v, want {Outbox:1 Mailboxes:1 Messages:2}", counts)
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
	if _, err := rebuildAt(context.Background(), badPath, preserved); err == nil {
		t.Fatal("rebuildAt over an impossible path succeeded, want a failure")
	}

	if err := unquarantine(quarantined, path, preserved); err != nil {
		t.Fatalf("unquarantine: %v", err)
	}

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

// TestUnquarantineReportsAStrandedWAL proves a rollback that leaves
// the quarantined write-ahead log behind is reported rather than
// treated as a full restore: that file holds committed transactions
// the restored database file does not, so a caller printing success
// over it tells the operator their data came back when the newest
// commits did not.
func TestUnquarantineReportsAStrandedWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	quarantined := path + ".corrupt-test"

	if err := os.WriteFile(quarantined, []byte("original store bytes"), 0o600); err != nil {
		t.Fatalf("seed quarantined file: %v", err)
	}
	if err := os.WriteFile(quarantined+"-wal", []byte("uncheckpointed commits"), 0o600); err != nil {
		t.Fatalf("seed quarantined wal: %v", err)
	}
	// A directory at the destination makes the rename fail with EISDIR,
	// the same shape a permissions or cross-device failure gives it.
	if err := os.Mkdir(path+"-wal", 0o700); err != nil {
		t.Fatalf("block the wal destination: %v", err)
	}

	err := unquarantine(quarantined, path, preservedData{})
	if err == nil {
		t.Fatal("unquarantine over a wal that could not move = nil, want the stranded wal reported")
	}
	if !strings.Contains(err.Error(), "-wal") {
		t.Errorf("err = %v, want it naming the -wal sidecar", err)
	}
	if _, statErr := os.Stat(quarantined + "-wal"); statErr != nil {
		t.Errorf("the stranded wal is gone from the quarantine path: %v", statErr)
	}
}

// TestRecoverContinuesPastAnUnreadableTable proves extraction is
// per-table: the largest and most corruption-prone table is the last
// one read, so aborting the whole recovery on it would cost an
// operator every outbox intent and mailbox that read cleanly. The
// damaged table is named back to the caller so the summary it prints
// is not a lie about what survived.
func TestRecoverContinuesPastAnUnreadableTable(t *testing.T) {
	w, path := newRecoverableTestWriter(t, DefaultWriterConfig())
	seedRecoveryFixture(t, w)

	// Renaming the table away leaves every other table intact and gives
	// its read the same error shape an unreadable page does.
	if err := w.submit(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE message RENAME TO message_damaged`)
		return err
	}); err != nil {
		t.Fatalf("damage the message table: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	counts, err := Recover(context.Background(), path)
	if err != nil {
		t.Fatalf("Recover over a damaged message table: %v", err)
	}
	if len(counts.DamagedTables) != 1 || counts.DamagedTables[0] != "message" {
		t.Errorf("DamagedTables = %v, want [message]", counts.DamagedTables)
	}
	if counts.Outbox != 1 || counts.Mailboxes != 1 {
		t.Errorf("RecoveredCounts = %+v, want the outbox and mailbox rows preserved anyway", counts)
	}
	if counts.Messages != 0 {
		t.Errorf("Messages = %d, want 0: the damaged table yielded nothing", counts.Messages)
	}

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
		t.Error("the outbox row was lost to a damaged message table, want it preserved")
	}
}

// TestRestorePreservedSkipsRowsOfALostAccount proves a partially read
// account table costs the operator only the rows hanging off the
// accounts that did not survive. Every preserved mailbox, outbox and
// message row carries a foreign key to account, and foreign keys are
// on in the rebuilt store, so one unguarded insert fails the whole
// restore transaction and takes every table that read cleanly with it.
func TestRestorePreservedSkipsRowsOfALostAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	preserved := preservedData{
		accounts: []preservedAccount{{id: 1, slug: "kept", backendKind: "jmap", address: "kept@example.com", data: "{}"}},
		mailboxes: []preservedMailbox{
			{id: 7, accountID: 1, role: "archive", name: "Archive", visible: 1, data: "{}"},
			{id: 8, accountID: 2, role: "archive", name: "Lost", visible: 1, data: "{}"},
		},
		outbox: []preservedOutboxRow{
			{id: 1, accountID: 1, kind: "move-messages", payload: "{}"},
			{id: 2, accountID: 2, kind: "move-messages", payload: "{}"},
		},
		messages: []preservedMessage{
			{id: 200, accountID: 1, origin: "local", data: "{}"},
			{id: 201, accountID: 2, origin: "local", data: "{}"},
		},
		messageMailboxes: []preservedMessageMailbox{
			{messageID: 200, mailboxID: 7},
			{messageID: 201, mailboxID: 8},
		},
		damagedTables: []string{"account"},
	}

	counts, err := rebuildAt(context.Background(), path, preserved)
	if err != nil {
		t.Fatalf("rebuildAt with one account lost to a damaged table: %v", err)
	}
	if counts.Outbox != 1 || counts.Mailboxes != 1 || counts.Messages != 1 {
		t.Errorf("RecoveredCounts = %+v, want one of each: the dropped rows are not preserved rows", counts)
	}

	db, err := sql.Open("sqlite", dsn(path, connReadOnly))
	if err != nil {
		t.Fatalf("reopen rebuilt store: %v", err)
	}
	defer func() { _ = db.Close() }()

	for _, q := range []struct {
		name  string
		query string
		want  int
	}{
		{"mailboxes of the surviving account", `SELECT COUNT(*) FROM mailbox WHERE account_id = 1`, 1},
		{"mailboxes of the lost account", `SELECT COUNT(*) FROM mailbox WHERE account_id = 2`, 0},
		{"outbox rows of the surviving account", `SELECT COUNT(*) FROM outbox WHERE account_id = 1`, 1},
		{"outbox rows of the lost account", `SELECT COUNT(*) FROM outbox WHERE account_id = 2`, 0},
		{"messages of the surviving account", `SELECT COUNT(*) FROM message WHERE account_id = 1`, 1},
		{"messages of the lost account", `SELECT COUNT(*) FROM message WHERE account_id = 2`, 0},
		{"memberships", `SELECT COUNT(*) FROM message_mailbox`, 1},
	} {
		var got int
		if err := db.QueryRow(q.query).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", q.name, err)
		}
		if got != q.want {
			t.Errorf("%s = %d, want %d", q.name, got, q.want)
		}
	}
}

// TestRecoverNamesDamagedTablesAfterAFailedRebuild proves the damaged
// tables are still named when the rebuild itself fails. That report is
// the operator's only account of what the store no longer holds, and a
// rebuild failure is exactly the moment they need it: the original file
// comes back, and whether it comes back whole is the next thing they
// have to decide.
func TestRecoverNamesDamagedTablesAfterAFailedRebuild(t *testing.T) {
	w, path := newRecoverableTestWriter(t, DefaultWriterConfig())
	seedRecoveryFixture(t, w)

	// A unique index that no longer holds its constraint lets two
	// mailboxes share a server id, which the rebuilt store's own index
	// rejects: a restore failure sourced from the damage itself, with
	// the message table unreadable beside it.
	err := w.submit(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DROP INDEX idx_mailbox_account_server`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO mailbox (id, account_id, server_id, name) VALUES (8, 1, 'srv-mbx-7', 'Archive again')`); err != nil {
			return err
		}
		_, err := tx.Exec(`ALTER TABLE message RENAME TO message_damaged`)
		return err
	})
	if err != nil {
		t.Fatalf("damage the store: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	counts, err := Recover(context.Background(), path)
	if err == nil {
		t.Fatal("Recover over a store whose restore cannot commit = nil, want the rebuild failure")
	}
	if len(counts.DamagedTables) != 1 || counts.DamagedTables[0] != "message" {
		t.Errorf("DamagedTables = %v, want [message] even though the rebuild failed", counts.DamagedTables)
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
		if _, err := tx.Exec(seedAccountSQL); err != nil {
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
	if counts.Outbox != 1 || counts.Mailboxes != 1 || counts.Messages != 2 {
		t.Errorf("RecoveredCounts = %+v, want {Outbox:1 Mailboxes:1 Messages:2}", counts)
	}
	assertRecoveryPreservedFixture(t, path)
}
