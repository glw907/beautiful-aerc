package cache

import (
	"database/sql"
	"fmt"
)

const schemaVersion = 7

// migration applies one schema step inside a transaction. Index 0 is v0→v1.
type migration func(*sql.Tx) error

var migrations = []migration{
	migrateV1, // v0 → v1: full Cache I schema (spec §A.3)
	migrateV2, // v1 → v2: next_eligible_at on outbox
	migrateV3, // v2 → v3: backend-reported exists/unseen on folders
	migrateV4, // v3 → v4: drop last_accessed + bodies_lru
	migrateV5, // v4 → v5: attachments table (metadata + lazy bytes)
	migrateV6, // v5 → v6: outbox.payload BLOB for Send/Append MIME bytes
	migrateV7, // v6 → v7: drafts table (local-buffer + server_uid pointer)
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

// applyMigrations brings db up to schemaVersion. Each step runs in
// its own transaction so a partial failure leaves a known version
// on disk.
func applyMigrations(db *sql.DB) error {
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
	for current < schemaVersion {
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
		current++
	}
	return nil
}
