package store

import (
	"context"
	"database/sql"
	"math"
	"slices"

	"github.com/glw907/poplar/internal/uerr"
)

// DefaultReadPoolSize is the read pool's connection count. A small,
// fixed pool holds the QA-2 latency budget under concurrent load
// without growing dsn.go's per-connection cache_size past QA-5's RSS
// ceiling.
const DefaultReadPoolSize = 4

// ReadPool is poplar's pool of read-only connections. Every
// connection carries dsn's query_only pragma, and ReadPool has no
// Exec method, so a write attempted through it fails to compile
// (TestReadHandleHasNoExec) rather than merely failing at runtime
// against the pragma. Reads never share the writer's single
// connection (ADR-0001), so a query here never queues behind an
// in-flight write.
type ReadPool struct {
	db  *sql.DB
	rev *RevisionCounter
}

// NewReadPool opens size read-only connections onto the store file at
// path. rev is the RevisionCounter the store's Writer advances after
// every commit; sharing the same counter is what lets a read result
// carry the revision it saw (SY-1's read path).
func NewReadPool(path string, size int, rev *RevisionCounter) (*ReadPool, error) {
	db, err := sql.Open("sqlite", dsn(path, connReadOnly))
	if err != nil {
		return nil, uerr.New("store.read", nil, uerr.ClassStoreLocal, err)
	}
	db.SetMaxOpenConns(size)
	db.SetMaxIdleConns(size)
	return &ReadPool{db: db, rev: rev}, nil
}

// Close closes every connection in the pool.
func (p *ReadPool) Close() error {
	return p.db.Close()
}

// MailboxCursor is a keyset pagination position in a mailbox's
// received_at DESC, message_id ASC list order: the edge row of the
// page a caller already has. The zero value is the position before
// the newest message, the only sane starting point for
// ListMailboxForward's first page.
type MailboxCursor struct {
	ReceivedAt int64
	MessageID  int64
}

// MailboxRow is one row of a mailbox-list page: the scalar columns
// LT-1's list paints, never message.data.
type MailboxRow struct {
	MessageID  int64
	ReceivedAt int64
}

// MailboxPage is one keyset-paginated window over a mailbox, plus the
// store revision the read saw.
type MailboxPage struct {
	Rows     []MailboxRow
	Revision Revision
}

// ListMailboxForward returns up to limit rows older than cursor, in
// received_at DESC, message_id ASC order. The zero MailboxCursor
// starts at the newest message in the mailbox.
func (p *ReadPool) ListMailboxForward(ctx context.Context, mailboxID int64, cursor MailboxCursor, limit int) (MailboxPage, error) {
	receivedAt := cursor.ReceivedAt
	if cursor == (MailboxCursor{}) {
		receivedAt = math.MaxInt64
	}
	return p.listMailbox(ctx, queryMailboxListForward, mailboxID, receivedAt, cursor.MessageID, limit)
}

// ListMailboxBackward returns up to limit rows newer than cursor,
// already reversed into received_at DESC, message_id ASC order so the
// result continues directly where the caller's current window
// begins. cursor must be an edge row from a page ListMailboxForward
// or ListMailboxBackward already returned; there is no sane
// zero-cursor start for paging backward.
func (p *ReadPool) ListMailboxBackward(ctx context.Context, mailboxID int64, cursor MailboxCursor, limit int) (MailboxPage, error) {
	page, err := p.listMailbox(ctx, queryMailboxListBackward, mailboxID, cursor.ReceivedAt, cursor.MessageID, limit)
	if err != nil {
		return MailboxPage{}, err
	}
	slices.Reverse(page.Rows)
	return page, nil
}

// listMailbox runs query, one of the forward/backward mailbox-list
// constants, both of which share this same argument shape: mailbox
// id, the cursor's received_at (bound twice, once as the plain
// index-seek conjunct and once inside the tie-break clause), the
// cursor's message id, and limit.
//
// rev is read before the query runs, not after: if a commit lands
// while the query is in flight, stamping the result with the
// counter's pre-query value means the result can only understate its
// own freshness, never claim to be fresher than the data it actually
// returned.
func (p *ReadPool) listMailbox(ctx context.Context, query string, mailboxID, receivedAt, messageID int64, limit int) (MailboxPage, error) {
	rev := p.rev.Current()

	rows, err := p.db.QueryContext(ctx, query, mailboxID, receivedAt, receivedAt, messageID, limit)
	if err != nil {
		return MailboxPage{}, uerr.New("store.read", nil, uerr.ClassStoreLocal, err)
	}
	defer func() { _ = rows.Close() }()

	page := MailboxPage{Revision: rev}
	for rows.Next() {
		var row MailboxRow
		if err := rows.Scan(&row.MessageID, &row.ReceivedAt); err != nil {
			return MailboxPage{}, uerr.New("store.read", nil, uerr.ClassStoreLocal, err)
		}
		page.Rows = append(page.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return MailboxPage{}, uerr.New("store.read", nil, uerr.ClassStoreLocal, err)
	}
	return page, nil
}
