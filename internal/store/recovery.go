package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// cleanShutdownMarker returns the sentinel path beside the store file
// at dbPath: present when the store's last shutdown was clean.
func cleanShutdownMarker(dbPath string) string {
	return dbPath + ".clean-shutdown"
}

// MarkCleanShutdown records that the store at dbPath shut down
// cleanly, so the next startup can skip its integrity check (SY-8,
// QA-1). Call it only after the writer and every read connection over
// dbPath have closed.
//
// The write is a bare os.WriteFile rather than a temp-file-and-rename
// swap: the marker is empty, so a write a crash tears mid-flight
// leaves either no file (read as an unclean shutdown, the safe
// default) or a whole one, never a partial file ShouldRunIntegrityCheck
// could mistake for present.
func MarkCleanShutdown(dbPath string) error {
	if err := os.WriteFile(cleanShutdownMarker(dbPath), nil, 0o600); err != nil {
		return localErr("store.shutdown", err)
	}
	return nil
}

// ShouldRunIntegrityCheck reports whether the store at dbPath owes an
// integrity check before startup continues: migrated is true right
// after a Migrate call advanced the schema version, and a missing
// marker means the prior run never reached a clean shutdown, whether
// this is the store's first launch or it crashed. A found marker is
// consumed, since it attests to the run that just ended, not the one
// about to start; a marker that cannot be consumed is not trusted,
// because a marker still on disk reads as a clean shutdown on every
// later start, including the one after a crash.
func ShouldRunIntegrityCheck(dbPath string, migrated bool) bool {
	marker := cleanShutdownMarker(dbPath)
	if _, err := os.Stat(marker); err != nil {
		if !os.IsNotExist(err) {
			slog.Error("store: the clean-shutdown marker could not be read, so nothing attests to a clean shutdown; running the integrity check",
				"marker", marker, "error", err)
		}
		return true
	}
	if err := os.Remove(marker); err != nil {
		slog.Error("store: the clean-shutdown marker could not be consumed, so it no longer attests to anything; running the integrity check",
			"marker", marker, "error", err)
		return true
	}
	return migrated
}

// CheckIntegrity runs SQLite's quick_check followed by FTS5's
// integrity-check against db, reporting each stage to progress as it
// starts. The measurement spike timed quick_check at 14.5s on a 924MB
// store, so a caller renders progress rather than a blocking spinner
// (QA-1); progress may be nil.
func CheckIntegrity(ctx context.Context, db *sql.DB, progress func(stage string)) error {
	report := func(stage string) {
		if progress != nil {
			progress(stage)
		}
	}

	report("checking store integrity")
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&result); err != nil {
		return localErr("store.integrity", err)
	}
	if result != "ok" {
		return localErr("store.integrity", fmt.Errorf("quick_check: %s", result))
	}

	report("checking full-text index")
	if _, err := db.ExecContext(ctx, `INSERT INTO message_fts(message_fts) VALUES ('integrity-check')`); err != nil {
		return localErr("store.integrity", err)
	}
	return nil
}

// RecoveredCounts reports what Recover carried over from the store it
// rebuilt, for a caller to report to the operator. DamagedTables
// names every table whose read stopped early, so a caller reports a
// partial recovery as one.
type RecoveredCounts struct {
	Outbox        int
	Mailboxes     int
	Messages      int
	DamagedTables []string
}

// Recover rebuilds the store at path from nothing but its
// non-rebuildable state: undispatched outbox rows, and origin =
// 'local' messages, drafts among them, with their bodies and
// draft_meta rows (SY-8). Every account and mailbox row travels too.
// Pass 1 has no onboarding flow to recreate an account, and a mailbox
// row carries the internal key an undispatched intent's payload names
// (ADR-0006): drop it and the intent resolves against whatever mailbox
// the next sync happens to mint under that id, so a queued rename
// lands on a different folder with no error. Server-origin messages
// and the FTS index are disposable by construction and return from the
// next sync. Recover runs before the writer starts, when nothing else
// holds path open, so it manages its own short-lived connections
// rather than routing through a Writer.
//
// If the rebuild fails after path is quarantined, Recover renames the
// quarantined file back to path rather than leaving a fresh, empty
// store in its place: the operator that ran with --recover keeps the
// exact input they started with, to retry or hand to a human.
func Recover(ctx context.Context, path string) (RecoveredCounts, error) {
	preserved, err := extractPreserved(ctx, path)
	if err != nil {
		return RecoveredCounts{}, localErr("store.recover.extract",
			fmt.Errorf("extract preserved rows: %w", err))
	}

	quarantined := fmt.Sprintf("%s.corrupt-%d", path, time.Now().UnixNano())
	if err := os.Rename(path, quarantined); err != nil {
		return RecoveredCounts{}, localErr("store.recover.quarantine",
			fmt.Errorf("quarantine %s: %w", path, err))
	}
	if err := moveSidecars(path, quarantined); err != nil {
		slog.Error("store: a sidecar of the quarantined store stayed behind, where the next open will read it against a different database file",
			"error", err)
	}

	counts, err := rebuildAt(ctx, path, preserved)
	if err != nil {
		rebuildErr := fmt.Errorf("rebuild %s: %w", path, err)
		restoreErr := unquarantine(quarantined, path, preserved)
		// The damaged tables travel even here: the store the operator
		// gets back is the one they started with, and what it no longer
		// holds is the next thing they have to weigh.
		return RecoveredCounts{DamagedTables: preserved.damagedTables},
			localErr("store.recover.rebuild", errors.Join(rebuildErr, restoreErr))
	}

	counts.DamagedTables = preserved.damagedTables
	return counts, nil
}

// moveSidecars renames the -wal and -shm files beside a store file
// that has just moved from one path to the other. A missing sidecar is
// normal, since a checkpointed store has neither and SQLite recreates
// both on the next open. Any other failure strands a write-ahead log
// holding committed transactions away from the database file they
// belong to, so it is reported rather than assumed harmless.
func moveSidecars(from, to string) error {
	var errs []error
	for _, suffix := range [...]string{"-wal", "-shm"} {
		if err := os.Rename(from+suffix, to+suffix); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("rename %s to %s: %w", from+suffix, to+suffix, err))
		}
	}
	return errors.Join(errs...)
}

// rebuildAt opens a fresh, migrated store at path and writes preserved
// into it, the half of Recover that runs after path's prior contents
// are already quarantined, reporting what it carried over.
func rebuildAt(ctx context.Context, path string, preserved preservedData) (RecoveredCounts, error) {
	db, err := OpenWriteConn(path)
	if err != nil {
		return RecoveredCounts{}, fmt.Errorf("open rebuilt store: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := Migrate(db); err != nil {
		return RecoveredCounts{}, fmt.Errorf("migrate rebuilt store: %w", err)
	}
	counts, err := restorePreserved(ctx, db, preserved)
	if err != nil {
		return RecoveredCounts{}, fmt.Errorf("restore preserved rows: %w", err)
	}
	return counts, nil
}

// unquarantine restores quarantined back to path after a failed
// rebuild, so a partial rebuild never leaves a fresh, empty store
// where the operator's data used to be. If the rename back itself
// fails, the preserved rows are still sitting in quarantined; that
// path and what it holds is logged, since nothing else names it
// again. What stopped a full restore is returned as well as logged,
// so the caller reports a partial restore as one: a write-ahead log
// left in the quarantine holds committed transactions the restored
// file does not.
func unquarantine(quarantined, path string, preserved preservedData) error {
	if err := os.Rename(quarantined, path); err != nil {
		slog.Error("store: recovery failed and the quarantined store could not be restored; preserved rows survive only in the quarantined file",
			"quarantine_path", quarantined, "outbox_rows", len(preserved.outbox),
			"mailbox_rows", len(preserved.mailboxes), "message_rows", len(preserved.messages), "error", err)
		return fmt.Errorf("restore %s to %s: %w", quarantined, path, err)
	}
	if err := moveSidecars(quarantined, path); err != nil {
		slog.Error("store: a sidecar did not come back with the restored store, so commits it still holds are missing from the file at path",
			"path", path, "quarantine_path", quarantined, "error", err)
		return err
	}
	return nil
}

// preservedAccount is one account row Recover carries into the
// rebuilt store verbatim, id included: it is the foreign-key root
// every preserved outbox and message row hangs off, and pass 1 has no
// onboarding flow to recreate it otherwise.
type preservedAccount struct {
	id                int64
	slug, backendKind string
	address           string
	data              string
}

// preservedMailbox is one mailbox row Recover carries into the
// rebuilt store, id included: an undispatched intent's payload names
// its mailbox by that id, and the next sync matches the row back to
// the same server object through the (account_id, server_id) unique
// index rather than minting a fresh id for it. The whole row travels
// rather than its identity alone, so a reader still sees the folder
// list they had before the rebuild until that sync lands.
type preservedMailbox struct {
	id, accountID           int64
	serverID                sql.NullString
	role, name              string
	sortOrder, visible      int64
	unreadCount, totalCount int64
	data                    string
}

// preservedOutboxRow is one outbox row Recover carries into the
// rebuilt store, id included, so an outbox payload's references to a
// preserved message's id keep resolving after rebuild. Recover always
// restores it as state 'queued' regardless of the state it was
// extracted under: a row a crashed run had claimed sits in
// 'dispatching', which the outbox's claim query never selects, so
// carrying that state across would leave the intent permanently
// undispatchable.
type preservedOutboxRow struct {
	id                          int64
	accountID                   int64
	kind, payload               string
	undoGroup                   sql.NullString
	chunkSeq, attemptCount      int
	nextAttemptAt, createdAt    int64
	failureClass, failureDetail sql.NullString
}

// preservedMessage is one origin = 'local' message row Recover carries
// into the rebuilt store, id included, along with its body and
// draft_meta rows if it has them.
type preservedMessage struct {
	id                                                     int64
	accountID                                              int64
	serverID, blobID                                       sql.NullString
	threadKey, subject, fromAddr, origin, searchText, data string
	receivedAt, flags, size, hasAttachment                 int64
	hiddenUntil                                            sql.NullInt64

	hasBody       bool
	bodyContent   []byte
	bodyFetchedAt int64

	hasDraftMeta        bool
	localRev, pushedRev int64
	anchorMsgID         sql.NullString
}

// preservedMessageMailbox is one message_mailbox row Recover carries
// with the local message it belongs to. Without it a preserved draft
// comes back in no folder, which is every list an operator could
// reach it from.
type preservedMessageMailbox struct {
	messageID, mailboxID int64
	receivedAt, unread   int64
}

// preservedData is everything Recover extracts from a store before
// discarding it. damagedTables names the tables whose read stopped
// early, the rows past the failure being unreadable.
type preservedData struct {
	accounts         []preservedAccount
	mailboxes        []preservedMailbox
	outbox           []preservedOutboxRow
	messages         []preservedMessage
	messageMailboxes []preservedMessageMailbox
	damagedTables    []string
}

// extractPreserved reads path's non-rebuildable rows through a
// dedicated read-only connection, closed before Recover's caller
// quarantines the file.
//
// Every table is read on its own terms, and each read keeps whatever
// it had before it stopped. An operator reaches for recovery because
// the store is already damaged, and the largest, most
// corruption-prone table is read last: one unreadable table must not
// cost them the queued intents and mailboxes that read cleanly.
func extractPreserved(ctx context.Context, path string) (preservedData, error) {
	db, err := sql.Open("sqlite", dsn(path, connReadOnly))
	if err != nil {
		return preservedData{}, err
	}
	defer func() { _ = db.Close() }()

	var damaged []string
	note := func(table string, recovered int, err error) {
		if err == nil {
			return
		}
		slog.Error("store: recovery could not read a table in full, so the rows past the failure are lost",
			"table", table, "rows_recovered", recovered, "error", err)
		damaged = append(damaged, table)
	}

	accounts, err := extractAccounts(ctx, db)
	note("account", len(accounts), err)
	mailboxes, err := extractMailboxes(ctx, db)
	note("mailbox", len(mailboxes), err)
	outboxRows, err := extractOutbox(ctx, db)
	note("outbox", len(outboxRows), err)
	messages, err := extractLocalMessages(ctx, db)
	note("message", len(messages), err)

	// The membership read joins the message table, so it is worth
	// running only once that table has yielded a message to attach a
	// membership to.
	var memberships []preservedMessageMailbox
	if len(messages) > 0 {
		memberships, err = extractLocalMemberships(ctx, db)
		note("message_mailbox", len(memberships), err)
	}

	return preservedData{
		accounts:         accounts,
		mailboxes:        mailboxes,
		outbox:           outboxRows,
		messages:         messages,
		messageMailboxes: memberships,
		damagedTables:    damaged,
	}, nil
}

func extractAccounts(ctx context.Context, db *sql.DB) ([]preservedAccount, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, slug, backend_kind, address, data FROM account`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []preservedAccount
	for rows.Next() {
		var a preservedAccount
		if err := rows.Scan(&a.id, &a.slug, &a.backendKind, &a.address, &a.data); err != nil {
			return out, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func extractMailboxes(ctx context.Context, db *sql.DB) ([]preservedMailbox, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, account_id, server_id, role, name, sort_order, visible, unread_count, total_count, data
		FROM mailbox`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []preservedMailbox
	for rows.Next() {
		var m preservedMailbox
		if err := rows.Scan(
			&m.id, &m.accountID, &m.serverID, &m.role, &m.name,
			&m.sortOrder, &m.visible, &m.unreadCount, &m.totalCount, &m.data,
		); err != nil {
			return out, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func extractOutbox(ctx context.Context, db *sql.DB) ([]preservedOutboxRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, account_id, kind, payload, undo_group, chunk_seq, attempt_count,
		       next_attempt_at, created_at, failure_class, failure_detail
		FROM outbox`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []preservedOutboxRow
	for rows.Next() {
		var o preservedOutboxRow
		if err := rows.Scan(
			&o.id, &o.accountID, &o.kind, &o.payload, &o.undoGroup, &o.chunkSeq, &o.attemptCount,
			&o.nextAttemptAt, &o.createdAt, &o.failureClass, &o.failureDetail,
		); err != nil {
			return out, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func extractLocalMessages(ctx context.Context, db *sql.DB) ([]preservedMessage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.id, m.account_id, m.server_id, m.blob_id, m.thread_key, m.received_at, m.subject,
		       m.from_addr, m.flags, m.size, m.has_attachment, m.origin, m.hidden_until, m.search_text, m.data,
		       b.content, b.fetched_at,
		       d.local_rev, d.pushed_rev, d.anchor_msgid
		FROM message m
		LEFT JOIN body b ON b.message_id = m.id
		LEFT JOIN draft_meta d ON d.message_id = m.id
		WHERE m.origin = 'local'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []preservedMessage
	for rows.Next() {
		var m preservedMessage
		var bodyContent []byte
		var bodyFetchedAt, localRev, pushedRev sql.NullInt64
		if err := rows.Scan(
			&m.id, &m.accountID, &m.serverID, &m.blobID, &m.threadKey, &m.receivedAt, &m.subject,
			&m.fromAddr, &m.flags, &m.size, &m.hasAttachment, &m.origin, &m.hiddenUntil, &m.searchText, &m.data,
			&bodyContent, &bodyFetchedAt,
			&localRev, &pushedRev, &m.anchorMsgID,
		); err != nil {
			return out, err
		}
		if bodyFetchedAt.Valid {
			m.hasBody, m.bodyContent, m.bodyFetchedAt = true, bodyContent, bodyFetchedAt.Int64
		}
		if localRev.Valid {
			m.hasDraftMeta, m.localRev, m.pushedRev = true, localRev.Int64, pushedRev.Int64
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func extractLocalMemberships(ctx context.Context, db *sql.DB) ([]preservedMessageMailbox, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT mm.message_id, mm.mailbox_id, mm.received_at, mm.unread
		FROM message_mailbox mm
		JOIN message m ON m.id = mm.message_id
		WHERE m.origin = 'local'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []preservedMessageMailbox
	for rows.Next() {
		var mm preservedMessageMailbox
		if err := rows.Scan(&mm.messageID, &mm.mailboxID, &mm.receivedAt, &mm.unread); err != nil {
			return out, err
		}
		out = append(out, mm)
	}
	return out, rows.Err()
}

// restorePreserved writes preserved into db, freshly migrated, as one
// transaction, and reports how many rows of each kind it carried over.
// Accounts go first, since every other preserved row carries a foreign
// key to them, and a row whose account did not survive extraction is
// dropped: its foreign key would fail and take the whole transaction
// with it, costing the operator every table that read cleanly.
func restorePreserved(ctx context.Context, db *sql.DB, preserved preservedData) (RecoveredCounts, error) {
	var counts RecoveredCounts
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return counts, err
	}
	defer func() { _ = tx.Rollback() }()

	restoredAccounts := make(map[int64]bool, len(preserved.accounts))
	for _, a := range preserved.accounts {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO account (id, slug, backend_kind, address, data) VALUES (?, ?, ?, ?, ?)`,
			a.id, a.slug, a.backendKind, a.address, a.data); err != nil {
			return counts, fmt.Errorf("insert account %d: %w", a.id, err)
		}
		restoredAccounts[a.id] = true
	}
	restoredMailboxes := make(map[int64]bool, len(preserved.mailboxes))
	for _, m := range preserved.mailboxes {
		if !restoredAccounts[m.accountID] {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mailbox (id, account_id, server_id, role, name, sort_order, visible, unread_count, total_count, data)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.id, m.accountID, m.serverID, m.role, m.name,
			m.sortOrder, m.visible, m.unreadCount, m.totalCount, m.data); err != nil {
			return counts, fmt.Errorf("insert mailbox %d: %w", m.id, err)
		}
		restoredMailboxes[m.id] = true
		counts.Mailboxes++
	}
	for _, o := range preserved.outbox {
		if !restoredAccounts[o.accountID] {
			continue
		}
		// state is always 'queued', regardless of what Recover
		// extracted it as: a crashed run can leave a claimed row in
		// 'dispatching', which the outbox's claim query never selects,
		// so restoring that state verbatim would strand the intent.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO outbox (id, account_id, kind, payload, state, undo_group, chunk_seq, attempt_count,
			                      next_attempt_at, created_at, failure_class, failure_detail)
			 VALUES (?, ?, ?, ?, 'queued', ?, ?, ?, ?, ?, ?, ?)`,
			o.id, o.accountID, o.kind, o.payload, o.undoGroup, o.chunkSeq, o.attemptCount,
			o.nextAttemptAt, o.createdAt, o.failureClass, o.failureDetail); err != nil {
			return counts, fmt.Errorf("insert outbox row %d: %w", o.id, err)
		}
		counts.Outbox++
	}
	restoredMessages := make(map[int64]bool, len(preserved.messages))
	for _, m := range preserved.messages {
		if !restoredAccounts[m.accountID] {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message (id, account_id, server_id, blob_id, thread_key, received_at, subject, from_addr,
			                       flags, size, has_attachment, origin, hidden_until, search_text, data)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.id, m.accountID, m.serverID, m.blobID, m.threadKey, m.receivedAt, m.subject, m.fromAddr,
			m.flags, m.size, m.hasAttachment, m.origin, m.hiddenUntil, m.searchText, m.data); err != nil {
			return counts, fmt.Errorf("insert message %d: %w", m.id, err)
		}
		if m.hasBody {
			if _, err := tx.ExecContext(ctx, `INSERT INTO body (message_id, content, fetched_at) VALUES (?, ?, ?)`,
				m.id, m.bodyContent, m.bodyFetchedAt); err != nil {
				return counts, fmt.Errorf("insert body for message %d: %w", m.id, err)
			}
		}
		if m.hasDraftMeta {
			if _, err := tx.ExecContext(ctx, `INSERT INTO draft_meta (message_id, local_rev, pushed_rev, anchor_msgid) VALUES (?, ?, ?, ?)`,
				m.id, m.localRev, m.pushedRev, m.anchorMsgID); err != nil {
				return counts, fmt.Errorf("insert draft_meta for message %d: %w", m.id, err)
			}
		}
		restoredMessages[m.id] = true
		counts.Messages++
	}
	for _, mm := range preserved.messageMailboxes {
		// A membership whose message or mailbox did not survive
		// extraction is dropped rather than attempted: its foreign key
		// would fail and take the whole restore transaction with it,
		// costing the operator every row that did survive.
		if !restoredMessages[mm.messageID] || !restoredMailboxes[mm.mailboxID] {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_mailbox (message_id, mailbox_id, received_at, unread) VALUES (?, ?, ?, ?)`,
			mm.messageID, mm.mailboxID, mm.receivedAt, mm.unread); err != nil {
			return counts, fmt.Errorf("insert mailbox %d membership for message %d: %w", mm.mailboxID, mm.messageID, err)
		}
	}
	return counts, tx.Commit()
}
