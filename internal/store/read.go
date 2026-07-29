package store

import (
	"context"
	"database/sql"
	"math"
	"slices"
	"strings"

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

// MessageSummary is one message's list-painting columns beyond
// MailboxRow's id and date: sender, subject, flags, the attachment
// marker, and the thread key a caller groups rows by for LT-1's
// thread count. It is never joined into the keyset list query, so
// ListMailboxForward/Backward's plan and column set stay exactly
// what TestListReadTouchesNoJSON asserts.
type MessageSummary struct {
	MessageID     int64
	Subject       string
	FromAddr      string
	Flags         Flags
	HasAttachment bool
	ThreadKey     string
}

// MailboxPage is one keyset-paginated window over a mailbox, plus the
// store revision the read saw.
type MailboxPage struct {
	Rows     []MailboxRow
	Revision Revision
}

// MailboxDetails is a MailboxRowDetails result: a MessageSummary per
// requested id, plus the store revision the read saw. A caller pairs
// this revision against a MailboxPage's own Revision to tell whether
// the two reads landed on the same store state.
type MailboxDetails struct {
	Summaries map[int64]MessageSummary
	Revision  Revision
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
// listMailbox captures rev before running the query. A commit that
// lands mid-query would otherwise let the stamped revision claim
// freshness the returned rows never actually saw. Capturing it early
// means a result can undercount its own freshness but can never
// overstate it.
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

// FirstMailboxID returns the lowest-id mailbox in the store. A
// startup trace with no onboarding flow yet to name a mailbox uses it
// to pick one to list.
func (p *ReadPool) FirstMailboxID(ctx context.Context) (int64, error) {
	var id int64
	if err := p.db.QueryRowContext(ctx, queryMailboxFirstID).Scan(&id); err != nil {
		return 0, uerr.New("store.read", nil, uerr.ClassStoreLocal, err)
	}
	return id, nil
}

// MailboxRowDetails returns a MessageSummary for each of ids, keyed
// by message id, so a caller can paint a page ListMailboxForward or
// ListMailboxBackward already returned. It selects only the
// list-painting columns and never message.data. An id with no
// matching message is simply absent from the result.
//
// It captures the store revision before running the query, the same
// timing listMailbox uses, so a caller can compare a page's Revision
// against this result's Revision to tell whether the two reads landed
// on the same store state.
func (p *ReadPool) MailboxRowDetails(ctx context.Context, ids []int64) (MailboxDetails, error) {
	rev := p.rev.Current()
	if len(ids) == 0 {
		return MailboxDetails{Revision: rev}, nil
	}

	var query strings.Builder
	query.WriteString(queryMessageSummaryByID)
	args := make([]any, len(ids))
	for i, id := range ids {
		if i > 0 {
			query.WriteString(",")
		}
		query.WriteString("?")
		args[i] = id
	}
	query.WriteString(")")

	rows, err := p.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return MailboxDetails{}, uerr.New("store.read", nil, uerr.ClassStoreLocal, err)
	}
	defer func() { _ = rows.Close() }()

	summaries := make(map[int64]MessageSummary, len(ids))
	for rows.Next() {
		var s MessageSummary
		var flags int64
		var hasAttachment int
		if err := rows.Scan(&s.MessageID, &s.Subject, &s.FromAddr, &flags, &hasAttachment, &s.ThreadKey); err != nil {
			return MailboxDetails{}, uerr.New("store.read", nil, uerr.ClassStoreLocal, err)
		}
		s.Flags = Flags(flags) //nolint:gosec // G115: message.flags is written only through EncodeFlags's uint32 bitfield, never a value outside its range
		s.HasAttachment = hasAttachment != 0
		summaries[s.MessageID] = s
	}
	if err := rows.Err(); err != nil {
		return MailboxDetails{}, uerr.New("store.read", nil, uerr.ClassStoreLocal, err)
	}
	return MailboxDetails{Summaries: summaries, Revision: rev}, nil
}
