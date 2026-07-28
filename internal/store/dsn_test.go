package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestDSNPragmaSet proves the five pragmas the DSN builder is
// supposed to be the only place spelling actually took effect on the
// opened connection, rather than merely appearing in the string.
func TestDSNPragmaSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	var foreignKeys, synchronous, cacheSize, busyTimeout int
	var journalMode string
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if err := db.QueryRow(`PRAGMA synchronous`).Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous: %v", err)
	}
	if err := db.QueryRow(`PRAGMA cache_size`).Scan(&cacheSize); err != nil {
		t.Fatalf("read cache_size: %v", err)
	}
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}

	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}
	if synchronous != 1 {
		t.Errorf("synchronous = %d, want 1 (NORMAL)", synchronous)
	}
	if cacheSize != cacheSizeKiB {
		t.Errorf("cache_size = %d, want %d", cacheSize, cacheSizeKiB)
	}
	if busyTimeout != busyTimeoutMS {
		t.Errorf("busy_timeout = %d, want %d", busyTimeout, busyTimeoutMS)
	}
}

// TestDSNReadOnlyRejectsWrites proves the read-only DSN this builder
// produces actually enforces read-only at the connection, the
// contract task 3's read pool needs from it.
func TestDSNReadOnlyRejectsWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")

	rw, err := sql.Open("sqlite", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("open read-write: %v", err)
	}
	if _, err := rw.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := rw.Close(); err != nil {
		t.Fatalf("close read-write: %v", err)
	}

	ro, err := sql.Open("sqlite", dsn(path, connReadOnly))
	if err != nil {
		t.Fatalf("open read-only: %v", err)
	}
	defer func() { _ = ro.Close() }()

	if _, err := ro.Exec(`INSERT INTO t (id) VALUES (1)`); err == nil {
		t.Fatal("insert on a read-only connection succeeded, want an error")
	} else if !strings.Contains(strings.ToLower(err.Error()), "readonly") &&
		!strings.Contains(strings.ToLower(err.Error()), "read-only") {
		t.Errorf("insert error %v does not report a read-only database", err)
	}

	var rows int
	if err := ro.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&rows); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rows != 0 {
		t.Errorf("rows = %d, want 0 (the rejected insert must not have landed)", rows)
	}
}
