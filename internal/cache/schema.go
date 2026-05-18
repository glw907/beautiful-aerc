package cache

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"

	"github.com/glw907/poplar/internal/content"
)

const schemaVersion = 13

// migration applies one schema step inside a transaction. Index 0 is v0→v1.
type migration func(*sql.Tx) error

var migrations = []migration{
	migrateV1,  // v0 → v1: full Cache I schema (spec §A.3)
	migrateV2,  // v1 → v2: next_eligible_at on outbox
	migrateV3,  // v2 → v3: backend-reported exists/unseen on folders
	migrateV4,  // v3 → v4: drop last_accessed + bodies_lru
	migrateV5,  // v4 → v5: attachments table (metadata + lazy bytes)
	migrateV6,  // v5 → v6: outbox.payload BLOB for Send/Append MIME bytes
	migrateV7,  // v6 → v7: drafts table (local-buffer + server_uid pointer)
	migrateV8,  // v7 → v8: contacts cache + recipient projection
	migrateV9,  // v8 → v9: outbox.folder nullable (contact ops have no folder scope)
	migrateV10, // v9 → v10: outbox.scheduled_for + outbox.draft_id FK
	migrateV11, // v10 → v11: messages_fts FTS5 virtual table for search
	migrateV12, // v11 → v12: drop messages.date_str (sent_at is authoritative)
	migrateV13, // v12 → v13: message_recipients FK to messages(id) ON DELETE CASCADE
}

// migrateV1 installs the full Cache I schema (spec §A.3).
func migrateV1(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE folders (
            id            INTEGER PRIMARY KEY AUTOINCREMENT,
            name          TEXT    NOT NULL UNIQUE,
            protocol_name TEXT    NOT NULL,
            role          TEXT,
            uidvalidity   INTEGER,
            sync_token    BLOB,
            last_synced   INTEGER
        )`,
		`CREATE TABLE messages (
            id           INTEGER PRIMARY KEY AUTOINCREMENT,
            protocol_id  TEXT    NOT NULL UNIQUE,
            thread_id    TEXT,
            in_reply_to  TEXT,
            subject      TEXT,
            from_addr    TEXT,
            to_addr      TEXT,
            cc_addr      TEXT,
            bcc_addr     TEXT,
            date_str     TEXT,
            sent_at      INTEGER,
            flags        INTEGER NOT NULL DEFAULT 0,
            size         INTEGER,
            ui_flags     INTEGER NOT NULL DEFAULT 0,
            ui_hide      INTEGER NOT NULL DEFAULT 0
        )`,
		`CREATE TABLE message_mailboxes (
            message INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
            folder  INTEGER NOT NULL REFERENCES folders(id)  ON DELETE CASCADE,
            PRIMARY KEY (message, folder)
        )`,
		`CREATE INDEX message_mailboxes_folder ON message_mailboxes(folder)`,
		`CREATE INDEX messages_sent   ON messages(sent_at DESC)`,
		`CREATE INDEX messages_thread ON messages(thread_id)`,
		`CREATE TABLE bodies (
            message       INTEGER PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
            bytes         BLOB    NOT NULL,
            fetched_at    INTEGER NOT NULL,
            last_accessed INTEGER NOT NULL
        )`,
		`CREATE INDEX bodies_lru ON bodies(last_accessed)`,
		`CREATE TABLE outbox (
            id           INTEGER PRIMARY KEY AUTOINCREMENT,
            folder       INTEGER NOT NULL REFERENCES folders(id)  ON DELETE CASCADE,
            message      INTEGER          REFERENCES messages(id) ON DELETE CASCADE,
            kind         TEXT    NOT NULL,
            args         TEXT    NOT NULL,
            enqueued_at  INTEGER NOT NULL,
            status       TEXT    NOT NULL DEFAULT 'pending',
            attempts     INTEGER NOT NULL DEFAULT 0,
            last_attempt INTEGER,
            error        TEXT
        )`,
		`CREATE INDEX outbox_pending ON outbox(id) WHERE status IN ('pending', 'failed')`,
		`CREATE INDEX outbox_message ON outbox(message) WHERE status IN ('pending', 'executing')`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("install cache schema: %v", err)
		}
	}
	return nil
}

// migrateV2 adds outbox.next_eligible_at so the drainer filters the
// backoff window in SQL. NULL means eligible immediately.
func migrateV2(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE outbox ADD COLUMN next_eligible_at INTEGER`,
		`DROP INDEX outbox_pending`,
		`CREATE INDEX outbox_pickup ON outbox(next_eligible_at, id) WHERE status IN ('pending', 'failed')`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("add outbox.next_eligible_at: %v", err)
		}
	}
	return nil
}

// migrateV3 adds backend-reported exists/unseen counts so unopened
// folders can show unread badges before any sync.
func migrateV3(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE folders ADD COLUMN exists_total INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE folders ADD COLUMN unseen_total INTEGER NOT NULL DEFAULT 0`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("add folders exists/unseen counts: %v", err)
		}
	}
	return nil
}

// migrateV4 drops bodies.last_accessed and its index. The lazy-
// population, no-LRU policy made them dead weight.
func migrateV4(tx *sql.Tx) error {
	stmts := []string{
		`DROP INDEX IF EXISTS bodies_lru`,
		`ALTER TABLE bodies DROP COLUMN last_accessed`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("narrow bodies table: %v", err)
		}
	}
	return nil
}

// migrateV5 adds the attachments table for lazy metadata + bytes.
func migrateV5(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE attachments (
            id           INTEGER PRIMARY KEY AUTOINCREMENT,
            message      INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
            part_id      TEXT    NOT NULL,
            filename     TEXT    NOT NULL DEFAULT '',
            mime_type    TEXT    NOT NULL,
            size         INTEGER NOT NULL,
            content_id   TEXT    NOT NULL DEFAULT '',
            disposition  TEXT    NOT NULL,
            bytes        BLOB,
            fetched_at   INTEGER,
            UNIQUE (message, part_id)
        )`,
		`CREATE INDEX attachments_message ON attachments(message)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("add attachments table: %v", err)
		}
	}
	return nil
}

// migrateV6 adds outbox.payload to carry assembled MIME bytes for
// Send/Append ops. NULL for Move/Flag/Destroy.
func migrateV6(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE outbox ADD COLUMN payload BLOB`); err != nil {
		return fmt.Errorf("add outbox.payload: %v", err)
	}
	return nil
}

// migrateV7 adds the drafts table for compose's local edit buffer.
// server_uid points at the JMAP/IMAP image once a PushDraftOp has
// succeeded. The CHECK pairs server_uid and server_folder so a draft
// is either local-only or fully pushed, never half-and-half.
func migrateV7(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE drafts (
            draft_id       TEXT PRIMARY KEY,
            server_uid     TEXT,
            server_folder  TEXT,
            payload        BLOB    NOT NULL,
            dirty          INTEGER NOT NULL DEFAULT 1,
            created_at     INTEGER NOT NULL,
            updated_at     INTEGER NOT NULL,
            last_pushed_at INTEGER,
            CHECK ((server_uid IS NULL) = (server_folder IS NULL))
        )`,
		`CREATE INDEX drafts_by_server_uid
            ON drafts(server_uid) WHERE server_uid IS NOT NULL`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("add drafts table: %v", err)
		}
	}
	return nil
}

// migrateV8 adds the contacts cache and the message_recipients projection
// used by SuggestAddresses ranking. Backfills message_recipients from
// existing messages rows in the same transaction.
func migrateV8(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE addressbooks (
			href           TEXT PRIMARY KEY,
			display_name   TEXT NOT NULL,
			description    TEXT NOT NULL DEFAULT '',
			sync_token     TEXT NOT NULL DEFAULT '',
			ctag           TEXT NOT NULL DEFAULT '',
			supports_sync  INTEGER NOT NULL DEFAULT 0,
			last_synced_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE contacts (
			uid              TEXT PRIMARY KEY,
			addressbook_href TEXT NOT NULL REFERENCES addressbooks(href) ON DELETE CASCADE,
			href             TEXT NOT NULL UNIQUE,
			etag             TEXT NOT NULL,
			vcard            BLOB NOT NULL,
			rev              TEXT NOT NULL DEFAULT '',
			fn               TEXT NOT NULL,
			family           TEXT NOT NULL DEFAULT '',
			given            TEXT NOT NULL DEFAULT '',
			org              TEXT NOT NULL DEFAULT '',
			title            TEXT NOT NULL DEFAULT '',
			note             TEXT NOT NULL DEFAULT '',
			last_synced_at   INTEGER NOT NULL
		)`,
		`CREATE INDEX contacts_by_book ON contacts(addressbook_href)`,
		`CREATE TABLE contact_emails (
			contact_uid TEXT NOT NULL REFERENCES contacts(uid) ON DELETE CASCADE,
			address     TEXT NOT NULL,
			label       TEXT NOT NULL DEFAULT '',
			pref        INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (contact_uid, address)
		)`,
		`CREATE INDEX contact_emails_by_addr ON contact_emails(address COLLATE NOCASE)`,
		`CREATE TABLE contact_phones (
			contact_uid TEXT NOT NULL REFERENCES contacts(uid) ON DELETE CASCADE,
			number      TEXT NOT NULL,
			label       TEXT NOT NULL DEFAULT '',
			pref        INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (contact_uid, number)
		)`,
		`CREATE TABLE message_recipients (
			message_uid INTEGER NOT NULL,
			role        TEXT NOT NULL CHECK (role IN ('from','to','cc')),
			address     TEXT NOT NULL,
			name        TEXT NOT NULL DEFAULT '',
			sent_at     INTEGER NOT NULL,
			PRIMARY KEY (message_uid, role, address)
		)`,
		`CREATE INDEX message_recipients_by_addr_sent ON message_recipients(address COLLATE NOCASE, sent_at DESC)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrateV8: %w", err)
		}
	}
	return backfillRecipients(tx)
}

// backfillRecipients populates message_recipients from existing messages
// rows. Runs once in the v7→v8 migration; INSERT OR IGNORE makes it
// safe on retry.
func backfillRecipients(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, sent_at, from_addr, to_addr, cc_addr FROM messages`)
	if err != nil {
		return fmt.Errorf("backfill recipients: %w", err)
	}
	defer rows.Close()

	insert, err := tx.Prepare(
		`INSERT OR IGNORE INTO message_recipients(message_uid, role, address, name, sent_at) VALUES (?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("backfill prepare: %w", err)
	}
	defer insert.Close()

	for rows.Next() {
		var id, sentAt int64
		var from, to, cc sql.NullString
		if err := rows.Scan(&id, &sentAt, &from, &to, &cc); err != nil {
			return fmt.Errorf("backfill scan: %w", err)
		}
		if err := writeRoleAddrs(insert, id, sentAt, "from", from.String); err != nil {
			return err
		}
		if err := writeRoleAddrs(insert, id, sentAt, "to", to.String); err != nil {
			return err
		}
		if err := writeRoleAddrs(insert, id, sentAt, "cc", cc.String); err != nil {
			return err
		}
	}
	return rows.Err()
}

func writeRoleAddrs(stmt *sql.Stmt, msgID, sentAt int64, role, raw string) error {
	if raw == "" {
		return nil
	}
	addrs, err := mail.ParseAddressList(raw)
	if err != nil {
		return nil // malformed legacy row; skip
	}
	for _, a := range addrs {
		if _, err := stmt.Exec(msgID, role, strings.ToLower(a.Address), a.Name, sentAt); err != nil {
			return fmt.Errorf("backfill insert: %w", err)
		}
	}
	return nil
}

// migrateV9 makes outbox.folder nullable. Contact ops (contact-put,
// contact-delete) are not scoped to any mail folder; NULL replaces the
// __contacts__ sentinel that previously satisfied the NOT NULL constraint.
// SQLite does not support ALTER COLUMN, so this rebuilds the table.
func migrateV9(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE outbox RENAME TO outbox_old`,
		`CREATE TABLE outbox (
            id               INTEGER PRIMARY KEY AUTOINCREMENT,
            folder           INTEGER          REFERENCES folders(id)  ON DELETE CASCADE,
            message          INTEGER          REFERENCES messages(id) ON DELETE CASCADE,
            kind             TEXT    NOT NULL,
            args             TEXT    NOT NULL,
            enqueued_at      INTEGER NOT NULL,
            status           TEXT    NOT NULL DEFAULT 'pending',
            attempts         INTEGER NOT NULL DEFAULT 0,
            last_attempt     INTEGER,
            error            TEXT,
            next_eligible_at INTEGER,
            payload          BLOB
        )`,
		`INSERT INTO outbox
            SELECT outbox_old.id,
                   CASE WHEN f.name = '__contacts__' THEN NULL ELSE outbox_old.folder END,
                   outbox_old.message, outbox_old.kind, outbox_old.args,
                   outbox_old.enqueued_at, outbox_old.status, outbox_old.attempts,
                   outbox_old.last_attempt, outbox_old.error,
                   outbox_old.next_eligible_at, outbox_old.payload
              FROM outbox_old
              LEFT JOIN folders f ON f.id = outbox_old.folder`,
		`DROP TABLE outbox_old`,
		`CREATE INDEX outbox_pickup ON outbox(next_eligible_at, id) WHERE status IN ('pending', 'failed')`,
		`CREATE INDEX outbox_message ON outbox(message) WHERE status IN ('pending', 'executing')`,
		`DELETE FROM folders WHERE name = '__contacts__'`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("nullable outbox.folder: %v", err)
		}
	}
	return nil
}

// migrateV10 adds outbox.scheduled_for (undo/schedule window) and
// outbox.draft_id (FK to drafts for compose-restore on undo). The
// pickup index gains scheduled_for as the leading column so the
// drainer's gate is index-resolved.
func migrateV10(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE outbox ADD COLUMN scheduled_for INTEGER`,
		`ALTER TABLE outbox ADD COLUMN draft_id TEXT REFERENCES drafts(draft_id) ON DELETE SET NULL`,
		`DROP INDEX outbox_pickup`,
		`CREATE INDEX outbox_pickup ON outbox(scheduled_for, next_eligible_at, id) WHERE status IN ('pending', 'failed')`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrateV10: %v", err)
		}
	}
	return nil
}

// migrateV11 adds the messages_fts FTS5 virtual table for cross-folder
// search. The cache layer owns all writes from Go; SQLite triggers
// can't extract plain text from MIME bytes, which is what feeds the
// body column. Header columns backfill from existing messages rows;
// body columns backfill from any cached bodies.bytes in the same
// transaction so users upgrading with a warm cache get body-search
// coverage without waiting for evict-and-refetch.
func migrateV11(tx *sql.Tx) error {
	stmts := []string{
		`CREATE VIRTUAL TABLE messages_fts USING fts5(
            subject, from_addr, to_addr, cc_addr, body
        )`,
		`INSERT INTO messages_fts(rowid, subject, from_addr, to_addr, cc_addr, body)
            SELECT id,
                   COALESCE(subject, ''),
                   COALESCE(from_addr, ''),
                   COALESCE(to_addr, ''),
                   COALESCE(cc_addr, ''),
                   ''
              FROM messages`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("install messages_fts: %v", err)
		}
	}
	return backfillFTSBodies(tx)
}

// backfillFTSBodies populates messages_fts.body from cached body rows.
// Parse and write failures skip the row rather than abort the upgrade;
// a corrupt body that was already on disk shouldn't trap users out of
// the new schema.
func backfillFTSBodies(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT message, bytes FROM bodies`)
	if err != nil {
		return fmt.Errorf("backfill fts bodies: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var msgID int64
		var raw []byte
		if err := rows.Scan(&msgID, &raw); err != nil {
			return fmt.Errorf("backfill fts scan: %w", err)
		}
		body, err := content.ExtractPlainText(raw)
		if err != nil || body == "" {
			continue
		}
		if err := writeFTSBodyTx(context.Background(), tx, msgID, body); err != nil {
			slog.Default().Debug("backfill fts write skipped", "msg", msgID, "err", err)
			continue
		}
	}
	return rows.Err()
}

// migrateV12 drops messages.date_str. SentAt has been authoritative
// since Pass 8.5; the legacy string column was a fallback for fixtures
// without parsed dates. Pre-beta cleanup retires the field everywhere.
func migrateV12(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE messages DROP COLUMN date_str`); err != nil {
		return fmt.Errorf("drop messages.date_str: %w", err)
	}
	return nil
}

// migrateV13 narrows message_recipients with a FK to messages(id)
// ON DELETE CASCADE. SQLite has no ALTER COLUMN, so we rebuild;
// the JOIN in the copy step drops any pre-existing orphans rather
// than carry them past the FK boundary. The same migration also
// scrubs orphan messages_fts rows (F2): FTS5 virtual tables don't
// participate in FK cascade, and the scan is free here.
func migrateV13(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE message_recipients_new (
            message_uid INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
            role        TEXT NOT NULL CHECK (role IN ('from','to','cc')),
            address     TEXT NOT NULL,
            name        TEXT NOT NULL DEFAULT '',
            sent_at     INTEGER NOT NULL,
            PRIMARY KEY (message_uid, role, address)
        )`,
		`INSERT INTO message_recipients_new
            SELECT mr.message_uid, mr.role, mr.address, mr.name, mr.sent_at
              FROM message_recipients mr
              JOIN messages m ON m.id = mr.message_uid`,
		`DROP TABLE message_recipients`,
		`ALTER TABLE message_recipients_new RENAME TO message_recipients`,
		`CREATE INDEX message_recipients_by_addr_sent ON message_recipients(address COLLATE NOCASE, sent_at DESC)`,
		`DELETE FROM messages_fts WHERE rowid NOT IN (SELECT id FROM messages)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("rebuild message_recipients: %v", err)
		}
	}
	return nil
}

// applyMigrations brings db up to schemaVersion. Each step runs in
// its own transaction so a partial failure leaves a known version
// on disk.
func applyMigrations(db *sql.DB) error {
	return applyMigrationsTo(db, schemaVersion)
}

// applyMigrationsTo brings db up to target (target ≤ schemaVersion).
// Tests use this to seed a database at an older version.
func applyMigrationsTo(db *sql.DB, target int) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	var current int
	row := db.QueryRow(`SELECT version FROM schema_version LIMIT 1`)
	if err := row.Scan(&current); err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("read schema_version: %w", err)
		}
		if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (0)`); err != nil {
			return fmt.Errorf("seed schema_version: %w", err)
		}
		current = 0
	}
	if current > schemaVersion {
		return fmt.Errorf("on-disk schema version %d is newer than this build (max %d) — newer poplar binary expected", current, schemaVersion)
	}
	for current < target {
		if current >= len(migrations) {
			return fmt.Errorf("missing migration step v%d→v%d", current, current+1)
		}
		step := migrations[current]
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration v%d: %w", current+1, err)
		}
		if err := step(tx); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`UPDATE schema_version SET version = ?`, current+1); err != nil {
			tx.Rollback()
			return fmt.Errorf("bump schema_version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration v%d: %w", current+1, err)
		}
		slog.Default().With("component", "cache").Debug("cache migrate", "from", current, "to", current+1)
		current++
	}
	return nil
}
