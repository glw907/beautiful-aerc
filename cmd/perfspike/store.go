package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// message holds the normalized columns for one email plus its body text.
type message struct {
	serverID      string
	threadKey     string
	mailbox       string
	receivedAt    int64
	subject       string
	fromAddr      string
	flags         int
	hasAttachment bool
	size          int64
	body          string
	cloneOf       int64 // 0 means original
	data          string
}

func defaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "poplar", "perfspike", "spike.db"), nil
}

// openDB opens the production DB, returning a single-connection writer and an
// unbounded reader pool. WAL mode and a 5-second busy timeout are set before
// returning.
func openDB() (*sql.DB, *sql.DB, error) {
	path, err := defaultDBPath()
	if err != nil {
		return nil, nil, err
	}
	return openDBAt(path)
}

// openDBAt is the testable form of openDB. It creates parent directories as
// needed.
func openDBAt(path string) (*sql.DB, *sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create state dir: %w", err)
	}

	w, err := openSQLite(path, 1)
	if err != nil {
		return nil, nil, err
	}
	r, err := openSQLite(path, 0)
	if err != nil {
		w.Close()
		return nil, nil, err
	}
	return w, r, nil
}

func openSQLite(path string, maxConns int) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if maxConns > 0 {
		db.SetMaxOpenConns(maxConns)
	}
	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-32000",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=268435456",
	} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return db, nil
}

// initSchema creates all tables, indexes, triggers, and the FTS5 virtual
// table. Safe to call on an existing database; all statements use IF NOT EXISTS.
func initSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS message (
			id            INTEGER PRIMARY KEY,
			server_id     TEXT    NOT NULL UNIQUE,
			thread_key    TEXT    NOT NULL,
			mailbox       TEXT    NOT NULL,
			received_at   INTEGER NOT NULL,
			subject       TEXT    NOT NULL DEFAULT '',
			from_addr     TEXT    NOT NULL DEFAULT '',
			flags         INTEGER NOT NULL DEFAULT 0,
			has_attachment INTEGER NOT NULL DEFAULT 0,
			size          INTEGER NOT NULL DEFAULT 0,
			body          TEXT    NOT NULL DEFAULT '',
			clone_of      INTEGER,
			data          TEXT    NOT NULL DEFAULT '{}'
		)`,

		`CREATE INDEX IF NOT EXISTS idx_msg_mailbox_date
			ON message (mailbox, received_at DESC)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS message_fts USING fts5(
			subject,
			body,
			content='message',
			content_rowid='id'
		)`,

		// Keep FTS index in sync with the message table. Both the FTS insert
		// and the row insert happen in the same transaction via the trigger.
		`CREATE TRIGGER IF NOT EXISTS message_fts_ai
			AFTER INSERT ON message BEGIN
				INSERT INTO message_fts(rowid, subject, body)
				VALUES (new.id, new.subject, new.body);
			END`,

		`CREATE TRIGGER IF NOT EXISTS message_fts_bd
			BEFORE DELETE ON message BEGIN
				INSERT INTO message_fts(message_fts, rowid, subject, body)
				VALUES ('delete', old.id, old.subject, old.body);
			END`,

		`CREATE TRIGGER IF NOT EXISTS message_fts_au
			AFTER UPDATE OF subject, body ON message BEGIN
				INSERT INTO message_fts(message_fts, rowid, subject, body)
				VALUES ('delete', old.id, old.subject, old.body);
				INSERT INTO message_fts(rowid, subject, body)
				VALUES (new.id, new.subject, new.body);
			END`,

		`CREATE TABLE IF NOT EXISTS event (
			id           INTEGER PRIMARY KEY,
			title        TEXT    NOT NULL,
			location     TEXT    NOT NULL DEFAULT '',
			description  TEXT    NOT NULL DEFAULT '',
			start_at     INTEGER NOT NULL,
			end_at       INTEGER NOT NULL,
			is_recurring INTEGER NOT NULL DEFAULT 0,
			raw_ics      TEXT    NOT NULL DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS event_occurrence (
			id       INTEGER PRIMARY KEY,
			event_id INTEGER NOT NULL REFERENCES event(id),
			start_at INTEGER NOT NULL
		)`,

		`CREATE INDEX IF NOT EXISTS idx_ev_occ_event
			ON event_occurrence (event_id)`,

		`CREATE TABLE IF NOT EXISTS harvest_state (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin schema tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("schema stmt: %w", err)
		}
	}
	return tx.Commit()
}

// insertMessages inserts a batch of messages in a single transaction. Rows
// with duplicate server_id are silently ignored, enabling idempotent harvest
// re-runs.
func insertMessages(db *sql.DB, msgs []message) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO message
			(server_id, thread_key, mailbox, received_at, subject, from_addr,
			 flags, has_attachment, size, body, clone_of, data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, m := range msgs {
		ha := 0
		if m.hasAttachment {
			ha = 1
		}
		if _, err := stmt.Exec(
			m.serverID, m.threadKey, m.mailbox, m.receivedAt,
			m.subject, m.fromAddr, m.flags, ha, m.size, m.body,
			m.cloneOf, m.data,
		); err != nil {
			return fmt.Errorf("insert %s: %w", m.serverID, err)
		}
	}
	return tx.Commit()
}

// setState writes a key/value pair to the harvest_state table, upserting.
func setState(db *sql.DB, key, value string) error {
	_, err := db.Exec(
		`INSERT INTO harvest_state (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set state %s: %w", key, err)
	}
	return nil
}

// getState reads a key from harvest_state. Returns ok=false when absent.
func getState(db *sql.DB, key string) (value string, ok bool, err error) {
	e := db.QueryRow("SELECT value FROM harvest_state WHERE key = ?", key).Scan(&value)
	if e == sql.ErrNoRows {
		return "", false, nil
	}
	if e != nil {
		return "", false, fmt.Errorf("get state %s: %w", key, e)
	}
	return value, true, nil
}
