package store

import (
	"database/sql"
	"fmt"
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
)

// dsn builds the modernc.org/sqlite connection string for the
// database at path. It is the only place poplar spells its pragma
// set, so every connection, write or read, carries the same
// foreign-key enforcement, busy timeout, WAL journaling, and cache
// budget.
func dsn(path string, kind connKind) string {
	q := fmt.Sprintf(
		"_pragma=foreign_keys(1)&_pragma=busy_timeout(%d)&_pragma=journal_mode(wal)&_pragma=synchronous(normal)&_pragma=cache_size(%d)",
		busyTimeoutMS, cacheSizeKiB)
	if kind == connReadOnly {
		q += "&_pragma=query_only(1)"
	}
	return "file:" + path + "?" + q
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
