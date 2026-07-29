package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/glw907/poplar/internal/uerr"
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
func MarkCleanShutdown(dbPath string) error {
	if err := os.WriteFile(cleanShutdownMarker(dbPath), nil, 0o600); err != nil {
		return uerr.New("store.shutdown", nil, uerr.ClassStoreLocal, err)
	}
	return nil
}

// NeedsIntegrityCheck reports whether the store at dbPath owes an
// integrity check before startup continues: migrated is true right
// after a Migrate call advanced the schema version, and a missing
// marker means the prior run never reached a clean shutdown, whether
// this is the store's first launch or it crashed. A found marker is
// consumed, since it attests to the run that just ended, not the one
// about to start.
func NeedsIntegrityCheck(dbPath string, migrated bool) bool {
	marker := cleanShutdownMarker(dbPath)
	_, err := os.Stat(marker)
	clean := err == nil
	if clean {
		_ = os.Remove(marker)
	}
	return migrated || !clean
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
		return uerr.New("store.integrity", nil, uerr.ClassStoreLocal, err)
	}
	if result != "ok" {
		return uerr.New("store.integrity", nil, uerr.ClassStoreLocal, fmt.Errorf("quick_check: %s", result))
	}

	report("checking full-text index")
	if _, err := db.ExecContext(ctx, `INSERT INTO message_fts(message_fts) VALUES ('integrity-check')`); err != nil {
		return uerr.New("store.integrity", nil, uerr.ClassStoreLocal, err)
	}
	return nil
}

// RecoveredCounts reports what Recover carried over from the store it
// rebuilt, for a caller to report to the operator.
type RecoveredCounts struct {
	Outbox   int
	Messages int
}

// Recover rebuilds the store at path from nothing but its
// non-rebuildable state: undispatched outbox rows, and origin =
// 'local' messages, drafts among them, with their bodies and
// draft_meta rows (SY-8). Every account row travels too, since pass 1
// has no onboarding flow to recreate one; everything else, mailboxes,
// server-origin messages, the FTS index, is disposable by construction
// and returns from the next sync. Recover runs before the writer
// starts, when nothing else holds path open, so it manages its own
// short-lived connections rather than routing through a Writer.
func Recover(ctx context.Context, path string) (RecoveredCounts, error) {
	preserved, err := extractPreserved(ctx, path)
	if err != nil {
		return RecoveredCounts{}, uerr.New("store.recover", nil, uerr.ClassStoreLocal, err)
	}

	quarantined := fmt.Sprintf("%s.corrupt-%d", path, time.Now().UnixNano())
	if err := os.Rename(path, quarantined); err != nil {
		return RecoveredCounts{}, uerr.New("store.recover", nil, uerr.ClassStoreLocal, err)
	}
	for _, suffix := range [...]string{"-wal", "-shm"} {
		_ = os.Rename(path+suffix, quarantined+suffix)
	}

	db, err := OpenWriteConn(path)
	if err != nil {
		return RecoveredCounts{}, uerr.New("store.recover", nil, uerr.ClassStoreLocal, err)
	}
	defer func() { _ = db.Close() }()

	if err := Migrate(db); err != nil {
		return RecoveredCounts{}, err
	}
	if err := restorePreserved(ctx, db, preserved); err != nil {
		return RecoveredCounts{}, uerr.New("store.recover", nil, uerr.ClassStoreLocal, err)
	}

	return RecoveredCounts{Outbox: len(preserved.outbox), Messages: len(preserved.messages)}, nil
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

// preservedOutboxRow is one outbox row Recover carries into the
// rebuilt store verbatim, id included, so an outbox payload's
// references to a preserved message's id keep resolving after
// rebuild.
type preservedOutboxRow struct {
	id                          int64
	accountID                   int64
	kind, payload, state        string
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

// preservedData is everything Recover extracts from a store before
// discarding it.
type preservedData struct {
	accounts []preservedAccount
	outbox   []preservedOutboxRow
	messages []preservedMessage
}

// extractPreserved reads path's non-rebuildable rows through a
// dedicated read-only connection, closed before Recover's caller
// quarantines the file.
func extractPreserved(ctx context.Context, path string) (preservedData, error) {
	db, err := sql.Open("sqlite", dsn(path, connReadOnly))
	if err != nil {
		return preservedData{}, err
	}
	defer func() { _ = db.Close() }()

	accounts, err := extractAccounts(ctx, db)
	if err != nil {
		return preservedData{}, err
	}
	outboxRows, err := extractOutbox(ctx, db)
	if err != nil {
		return preservedData{}, err
	}
	messages, err := extractLocalMessages(ctx, db)
	if err != nil {
		return preservedData{}, err
	}
	return preservedData{accounts: accounts, outbox: outboxRows, messages: messages}, nil
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
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func extractOutbox(ctx context.Context, db *sql.DB) ([]preservedOutboxRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, account_id, kind, payload, state, undo_group, chunk_seq, attempt_count,
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
			&o.id, &o.accountID, &o.kind, &o.payload, &o.state, &o.undoGroup, &o.chunkSeq, &o.attemptCount,
			&o.nextAttemptAt, &o.createdAt, &o.failureClass, &o.failureDetail,
		); err != nil {
			return nil, err
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
			return nil, err
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

// restorePreserved writes preserved into db, freshly migrated, as one
// transaction: accounts first, since outbox and message rows carry a
// foreign key to them.
func restorePreserved(ctx context.Context, db *sql.DB, preserved preservedData) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, a := range preserved.accounts {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO account (id, slug, backend_kind, address, data) VALUES (?, ?, ?, ?, ?)`,
			a.id, a.slug, a.backendKind, a.address, a.data); err != nil {
			return err
		}
	}
	for _, o := range preserved.outbox {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO outbox (id, account_id, kind, payload, state, undo_group, chunk_seq, attempt_count,
			                      next_attempt_at, created_at, failure_class, failure_detail)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			o.id, o.accountID, o.kind, o.payload, o.state, o.undoGroup, o.chunkSeq, o.attemptCount,
			o.nextAttemptAt, o.createdAt, o.failureClass, o.failureDetail); err != nil {
			return err
		}
	}
	for _, m := range preserved.messages {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message (id, account_id, server_id, blob_id, thread_key, received_at, subject, from_addr,
			                       flags, size, has_attachment, origin, hidden_until, search_text, data)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.id, m.accountID, m.serverID, m.blobID, m.threadKey, m.receivedAt, m.subject, m.fromAddr,
			m.flags, m.size, m.hasAttachment, m.origin, m.hiddenUntil, m.searchText, m.data); err != nil {
			return err
		}
		if m.hasBody {
			if _, err := tx.ExecContext(ctx, `INSERT INTO body (message_id, content, fetched_at) VALUES (?, ?, ?)`,
				m.id, m.bodyContent, m.bodyFetchedAt); err != nil {
				return err
			}
		}
		if m.hasDraftMeta {
			if _, err := tx.ExecContext(ctx, `INSERT INTO draft_meta (message_id, local_rev, pushed_rev, anchor_msgid) VALUES (?, ?, ?, ?)`,
				m.id, m.localRev, m.pushedRev, m.anchorMsgID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
