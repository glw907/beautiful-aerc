package store

import (
	"database/sql"
	"fmt"
	"net/url"

	"github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/driver" // registers the "sqlite3" database/sql driver
	"github.com/ncruces/go-sqlite3/ext/fts5"
)

// This file is the one every consumer of package store compiles,
// production and test alike, so it carries both the driver's blank
// import and FTS5's registration: neither belongs at each caller's
// own import list. FTS5 is not compiled into the driver by default;
// the extension must be registered once before the first Open, or the
// first connection any binary in this module opens has no full-text
// support at all.
func init() {
	sqlite3.AutoExtension(fts5.Register)
}

// connKind selects the query_only pragma that tells dsn to build a
// read-only connection string. It exists so the write connection and
// the read pool (task 3) share one DSN builder instead of two DSN
// strings that could drift apart.
type connKind int

const (
	connReadWrite connKind = iota
	connReadOnly
)

const (
	// busyTimeoutMS bounds how long the write connection retries
	// against a lock before returning SQLITE_BUSY. The writer never
	// contends with itself on an interactive or bulk transaction; the
	// one lock wait this connection can hit under normal operation is
	// a TRUNCATE checkpoint blocked on a reader's snapshot, and
	// checkpointTruncate (checkpoint.go) lowers busy_timeout around
	// that call specifically, so this 5s bound never gates a queued
	// interactive job. Five seconds here only covers the unlikely
	// case of a reader outliving that shorter window, while still
	// surfacing SQLITE_BUSY instead of hanging the process forever.
	busyTimeoutMS = 5000

	// cacheSizeKiB is negative, so SQLite interprets it as KiB
	// regardless of page_size (cache_size pragma). QA-5 bounds
	// poplar's whole-process steady-state RSS at 250MB. At a
	// single-digit connection count (one writer, a small read pool
	// from task 3), 2MiB per connection keeps the store's page cache
	// in the low single-digit percent of that ceiling, leaving the
	// rest for message bodies, the FTS index, and everything above
	// the store, while still holding the hot mailbox-list and unread
	// indexes resident across a chunked bulk batch.
	cacheSizeKiB = -2000

	// pageSize is fixed at file creation; changing it afterward needs
	// a full blocking VACUUM. 8192 halves the page count of SQLite's
	// 4096 default against poplar's mostly-larger rows (message,
	// event, contact_card) without paying WAL write-amplification on
	// the smaller join tables.
	pageSize = 8192

	// autoVacuumMode is SQLite's numeric code for INCREMENTAL,
	// likewise fixed at creation. FULL reclaims eagerly on every
	// delete and rewrites the file inline, defeating the writer's own
	// bounded, scheduled reclaim (checkpoint.go's incrementalVacuum);
	// NONE requires a bare VACUUM to shrink the file at all, the
	// unbounded blocking stall that wedged Geary's mailbox (issue
	// #1017).
	autoVacuumMode = 2
)

// dsn builds the go-sqlite3 connection string for the database at
// path. It is the only place poplar spells its pragma set, so every
// connection, write or read, carries the same foreign-key
// enforcement, busy timeout, WAL journaling, and cache budget.
//
// Every pragma rides in a single _pragma value, run as one
// multi-statement script, rather than as separate _pragma parameters:
// page_size and auto_vacuum are fixed the moment a database enters
// WAL mode, so journal_mode must run after them, and the driver's own
// documentation warns that pragma order matters. One script pins the
// order written here explicitly; the driver audit's T1 probe found
// ncruces honors query-string order correctly under the separate-
// parameter form too, so this is belt and suspenders, not a
// workaround for a defect.
func dsn(path string, kind connKind) string {
	pragmas := fmt.Sprintf(
		"busy_timeout(%d); PRAGMA page_size(%d); PRAGMA auto_vacuum(%d); PRAGMA journal_mode(wal); PRAGMA synchronous(normal); PRAGMA cache_size(%d); PRAGMA foreign_keys(1)",
		busyTimeoutMS, pageSize, autoVacuumMode, cacheSizeKiB)
	if kind == connReadOnly {
		pragmas += "; PRAGMA query_only(1)"
	}
	return "file:" + path + "?_pragma=" + url.QueryEscape(pragmas)
}

// OpenWriteConn opens path as poplar's write connection, carrying
// dsn's full pragma set. It is the one place outside this package
// allowed to open a write connection directly, for storetest's
// stand-alone Writer (ADR-0014): storetest once kept its own copy of
// this pragma string, which had already drifted from dsn's. NewWriter
// still owns pinning the result to one physical connection and
// pairing it with a migrated schema.
func OpenWriteConn(path string) (*sql.DB, error) {
	return sql.Open("sqlite3", dsn(path, connReadWrite))
}
