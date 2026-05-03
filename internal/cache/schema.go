// SPDX-License-Identifier: MIT

package cache

import (
	"database/sql"
	"fmt"
)

// schemaVersion is the current schema version. Migrations from any
// older version up to this one run in order at Open.
const schemaVersion = 2

// migration applies one schema version step inside a single
// transaction. Index 0 holds the v0→v1 step.
type migration func(*sql.Tx) error

// migrations is the ordered chain. migrations[i] upgrades from
// version i to version i+1.
var migrations = []migration{
	migrateV1, // v0 → v1: full Cache I schema (spec §A.3)
	migrateV2, // v1 → v2: next_eligible_at on outbox
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
			return fmt.Errorf("migrate v1: %w", err)
		}
	}
	return nil
}

// migrateV2 adds outbox.next_eligible_at so the drainer's pickup
// query can filter the backoff window in SQL instead of scanning
// every failed row in Go. NULL = "eligible immediately" (matches
// the v1 default for fresh inserts).
func migrateV2(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE outbox ADD COLUMN next_eligible_at INTEGER`,
		`DROP INDEX outbox_pending`,
		`CREATE INDEX outbox_pickup ON outbox(next_eligible_at, id) WHERE status IN ('pending', 'failed')`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrate v2: %w", err)
		}
	}
	return nil
}

// applyMigrations brings db up to schemaVersion. Runs each step in
// its own transaction so a partial failure leaves a known version on
// disk.
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
