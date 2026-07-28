package store

import (
	"database/sql"
	"fmt"
	"net/url"
)

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

// dsn builds the modernc.org/sqlite connection string for the
// database at path. It is the only place poplar spells its pragma
// set, so every connection, write or read, carries the same
// foreign-key enforcement, busy timeout, WAL journaling, and cache
// budget.
//
// Every pragma rides in a single _pragma value, run as one
// multi-statement script (modernc.org/sqlite's documented "script"
// form for a value with trailing SQL past the first statement),
// rather than as separate _pragma parameters: applyQueryParams
// re-sorts separate parameters alphabetically before running them
// (busy_timeout first, the rest lexicographic), which would run
// journal_mode ahead of page_size and auto_vacuum. Both are fixed the
// moment a database enters WAL mode, so that reordering would
// silently no-op them on a fresh file. One script preserves the
// order written here.
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
	return sql.Open("sqlite", dsn(path, connReadWrite))
}
