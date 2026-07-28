package store

import "fmt"

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
	// busyTimeoutMS bounds how long a connection retries against a
	// lock before returning SQLITE_BUSY. The write connection never
	// contends with itself, so the only lock wait it can hit is a
	// TRUNCATE checkpoint blocked on a reader's snapshot. Five
	// seconds is generous next to that (QA-2's interactive budget is
	// 25ms p95, one transaction is bounded at roughly 50ms of work),
	// long enough for a slow reader to finish, short enough that a
	// genuinely stuck connection still surfaces SQLITE_BUSY instead
	// of hanging the process.
	busyTimeoutMS = 5000

	// cacheSizeKiB is negative, so SQLite interprets it as KiB
	// regardless of page_size (cache_size pragma). At the
	// 924MB/100k-message envelope, this many KiB times a single-digit
	// connection count (one writer, a small read pool) stays a small
	// fraction of QA-5's RSS ceiling, while still holding the hot
	// mailbox-list and unread indexes resident across a chunked bulk
	// batch.
	cacheSizeKiB = -8000
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
