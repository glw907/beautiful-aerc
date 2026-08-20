package store

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestListReadTouchesNoJSON asserts the mailbox-list query's column
// set excludes data: the list path never parses message.data.
func TestListReadTouchesNoJSON(t *testing.T) {
	db := openMigratedTestDB(t)

	rows, err := db.QueryContext(context.Background(), queryMailboxListForward, 1, int64(0), int64(0), int64(0), 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	if slices.Contains(cols, "data") {
		t.Fatalf("mailbox list columns = %v, want no data column", cols)
	}
}

// TestMessageSummaryTouchesNoJSON asserts the detail query's column
// set excludes data, the same guarantee TestListReadTouchesNoJSON
// gives the keyset list query but which the list query alone leaves
// unchecked for the second read a page needs to paint LT-1's row.
func TestMessageSummaryTouchesNoJSON(t *testing.T) {
	db := openMigratedTestDB(t)

	rows, err := db.QueryContext(context.Background(), queryMessageSummaryByID+"?)", int64(1))
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	if slices.Contains(cols, "data") {
		t.Fatalf("message summary columns = %v, want no data column", cols)
	}
}

// TestMailboxRowDetails proves the companion detail read returns
// LT-1's list-painting columns (subject, sender, flags, the
// attachment marker, and the thread key) for the ids a mailbox page
// already returned, and that an id with no matching message is
// simply absent from the result rather than an error.
func TestMailboxRowDetails(t *testing.T) {
	w, path := newTestWriter(t, DefaultWriterConfig())
	pool, err := NewReadPool(path, DefaultReadPoolSize, w.Revision())
	if err != nil {
		t.Fatalf("NewReadPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	ctx := context.Background()
	seedAccountAndMailbox(t, w)

	err = w.submit(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO message (id, account_id, received_at, subject, from_addr, flags, has_attachment, thread_key) VALUES
			(1, 1, 100, 'hello', 'a@example.com', ?, 1, 'thread-1'),
			(2, 1, 200, 'world', 'b@example.com', 0, 0, 'thread-2')`, FlagFlagged); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES (1, 1, 100), (2, 1, 200)`)
		return err
	})
	if err != nil {
		t.Fatalf("seed messages: %v", err)
	}

	got, err := pool.MailboxRowDetails(ctx, []int64{1, 2, 999})
	if err != nil {
		t.Fatalf("MailboxRowDetails: %v", err)
	}

	want := map[int64]MessageSummary{
		1: {MessageID: 1, Subject: "hello", FromAddr: "a@example.com", Flags: FlagFlagged, HasAttachment: true, ThreadKey: "thread-1"},
		2: {MessageID: 2, Subject: "world", FromAddr: "b@example.com", Flags: 0, HasAttachment: false, ThreadKey: "thread-2"},
	}
	if !maps.Equal(got.Summaries, want) {
		t.Fatalf("MailboxRowDetails(1, 2, 999).Summaries = %+v, want %+v (999 absent)", got.Summaries, want)
	}
	if got.Revision == 0 {
		t.Fatalf("MailboxRowDetails(1, 2, 999).Revision = 0, want the revision the seed commits advanced past")
	}
}

// TestMailboxRowDetailsRevisionPairsWithPage proves a page's Revision
// and a details read's Revision are directly comparable: a details
// read taken before a write commits must not claim the freshness of a
// page read after it, and vice versa. This is the caller-facing
// guarantee a bare map result cannot carry.
func TestMailboxRowDetailsRevisionPairsWithPage(t *testing.T) {
	w, path := newTestWriter(t, DefaultWriterConfig())
	pool, err := NewReadPool(path, DefaultReadPoolSize, w.Revision())
	if err != nil {
		t.Fatalf("NewReadPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	ctx := context.Background()
	seedAccountAndMailbox(t, w)

	err = w.submit(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO message (id, account_id, received_at, subject, from_addr, flags, has_attachment, thread_key) VALUES (1, 1, 100, 'hello', 'a@example.com', 0, 0, 'thread-1')`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES (1, 1, 100)`)
		return err
	})
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}

	page, err := pool.ListMailboxForward(ctx, 1, MailboxCursor{}, 10)
	if err != nil {
		t.Fatalf("ListMailboxForward: %v", err)
	}

	err = w.submit(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE message SET subject = 'updated' WHERE id = 1`)
		return err
	})
	if err != nil {
		t.Fatalf("update message: %v", err)
	}

	details, err := pool.MailboxRowDetails(ctx, []int64{1})
	if err != nil {
		t.Fatalf("MailboxRowDetails: %v", err)
	}

	if details.Revision <= page.Revision {
		t.Fatalf("details.Revision = %d, page.Revision = %d, want details taken after a later commit to compare greater", details.Revision, page.Revision)
	}
}

// TestKeysetPagination walks a seeded 100k-row mailbox forward and
// then backward, asserting no duplicate row and no skipped row across
// page boundaries, including rows sharing a received_at.
func TestKeysetPagination(t *testing.T) {
	const n = 100_000
	const pageSize = 500 // not a multiple of the seed's 7-row tie groups, so page boundaries fall inside a tie group

	path, want := seedLargeMailbox(t, n)
	pool, err := NewReadPool(path, DefaultReadPoolSize, &RevisionCounter{})
	if err != nil {
		t.Fatalf("NewReadPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	ctx := context.Background()

	var gotForward []MailboxRow
	cursor := MailboxCursor{}
	for {
		page, err := pool.ListMailboxForward(ctx, 1, cursor, pageSize)
		if err != nil {
			t.Fatalf("ListMailboxForward: %v", err)
		}
		gotForward = append(gotForward, page.Rows...)
		if len(page.Rows) < pageSize {
			break
		}
		last := page.Rows[len(page.Rows)-1]
		cursor = MailboxCursor{ReceivedAt: last.ReceivedAt, MessageID: last.MessageID}
	}
	if !slices.Equal(gotForward, want) {
		t.Fatalf("forward walk returned %d rows, want %d rows in the exact seeded order", len(gotForward), len(want))
	}

	last := want[len(want)-1]
	cursor = MailboxCursor{ReceivedAt: last.ReceivedAt, MessageID: last.MessageID}
	var gotBackward []MailboxRow
	for {
		page, err := pool.ListMailboxBackward(ctx, 1, cursor, pageSize)
		if err != nil {
			t.Fatalf("ListMailboxBackward: %v", err)
		}
		if len(page.Rows) == 0 {
			break
		}
		gotBackward = append(page.Rows, gotBackward...)
		top := page.Rows[0]
		cursor = MailboxCursor{ReceivedAt: top.ReceivedAt, MessageID: top.MessageID}
		if len(page.Rows) < pageSize {
			break
		}
	}
	gotBackward = append(gotBackward, last)
	if !slices.Equal(gotBackward, want) {
		t.Fatalf("backward walk reconstructed %d rows, want %d rows in the exact seeded order", len(gotBackward), len(want))
	}
}

// seedLargeMailbox creates a fresh migrated store file with n messages
// in mailbox 1, grouped into runs of 7 sharing a received_at so the
// keyset walk in TestKeysetPagination crosses tie boundaries. It
// returns the file's path and the rows in received_at DESC,
// message_id ASC order: the exact order a correct keyset walk must
// reproduce.
func seedLargeMailbox(t *testing.T, n int) (path string, want []MailboxRow) {
	t.Helper()

	const tieGroup = 7
	rows := make([]MailboxRow, n)
	for i := range n {
		rows[i] = MailboxRow{
			MessageID:  int64(i + 1),
			ReceivedAt: int64(1_700_000_000 - i/tieGroup),
		}
	}

	path = filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite3", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	seedAccount(t, db)
	if _, err := db.Exec(seedMailboxSQL); err != nil {
		t.Fatalf("insert mailbox: %v", err)
	}

	const batch = 1000
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin seed transaction: %v", err)
	}
	for start := 0; start < n; start += batch {
		end := min(start+batch, n)
		if err := insertMailboxRowBatch(tx, rows[start:end]); err != nil {
			t.Fatalf("insert batch [%d,%d): %v", start, end, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed transaction: %v", err)
	}

	want = slices.Clone(rows)
	slices.SortFunc(want, func(a, b MailboxRow) int {
		return cmp.Or(cmp.Compare(b.ReceivedAt, a.ReceivedAt), cmp.Compare(a.MessageID, b.MessageID))
	})
	return path, want
}

// insertMailboxRowBatch inserts rows into both message and
// message_mailbox in one multi-row statement per table, the shape
// seeding 100k rows one at a time is too slow for.
func insertMailboxRowBatch(tx *sql.Tx, rows []MailboxRow) error {
	var msgSQL, mmSQL strings.Builder
	msgSQL.WriteString(`INSERT INTO message (id, account_id, received_at) VALUES `)
	mmSQL.WriteString(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES `)
	args := make([]any, 0, len(rows)*2)
	for i, r := range rows {
		if i > 0 {
			msgSQL.WriteString(",")
			mmSQL.WriteString(",")
		}
		msgSQL.WriteString("(?, 1, ?)")
		mmSQL.WriteString("(?, 1, ?)")
		args = append(args, r.MessageID, r.ReceivedAt)
	}
	if _, err := tx.Exec(msgSQL.String(), args...); err != nil {
		return fmt.Errorf("insert message batch: %w", err)
	}
	if _, err := tx.Exec(mmSQL.String(), args...); err != nil {
		return fmt.Errorf("insert message_mailbox batch: %w", err)
	}
	return nil
}

// TestReadsDoNotBlockOnWriter runs a sustained bulk write on the
// writer's bulk lane while the read pool serves a mailbox list on
// every iteration, asserting read p95 stays inside QA-2's 25ms
// interaction-latency budget: reads on separate connections must
// never queue behind the writer's single connection (ADR-0001).
func TestReadsDoNotBlockOnWriter(t *testing.T) {
	w, path := newTestWriter(t, DefaultWriterConfig())
	pool, err := NewReadPool(path, DefaultReadPoolSize, w.Revision())
	if err != nil {
		t.Fatalf("NewReadPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	ctx := context.Background()
	seedAccountAndMailbox(t, w)

	stop := make(chan struct{})
	writeErr := make(chan error, 1)
	go func() {
		for id := int64(1); ; id++ {
			select {
			case <-stop:
				writeErr <- nil
				return
			default:
			}
			err := w.submitBulk(ctx, func(tx *sql.Tx) error {
				if _, err := tx.Exec(`INSERT INTO message (id, account_id, received_at) VALUES (?, 1, ?)`, id, id); err != nil {
					return err
				}
				_, err := tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES (?, 1, ?)`, id, id)
				return err
			})
			if err != nil {
				writeErr <- err
				return
			}
		}
	}()

	const samples = 300
	const budget = 25 * time.Millisecond // QA-2's interaction-latency gate
	latencies := make([]time.Duration, samples)
	for i := range samples {
		start := time.Now()
		if _, err := pool.ListMailboxForward(ctx, 1, MailboxCursor{}, 50); err != nil {
			close(stop)
			t.Fatalf("ListMailboxForward: %v", err)
		}
		latencies[i] = time.Since(start)
	}
	close(stop)
	if err := <-writeErr; err != nil {
		t.Fatalf("bulk writer: %v", err)
	}

	slices.Sort(latencies)
	p95 := latencies[int(float64(samples)*0.95)]
	if p95 > budget {
		t.Fatalf("read p95 = %v while the writer bulk-writes concurrently, want <= %v", p95, budget)
	}
}

// TestMailStats proves the mail placeholder's live-fact read: message
// and mailbox counts across two mailboxes and three messages.
func TestMailStats(t *testing.T) {
	w, path := newTestWriter(t, DefaultWriterConfig())
	pool, err := NewReadPool(path, DefaultReadPoolSize, w.Revision())
	if err != nil {
		t.Fatalf("NewReadPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	ctx := context.Background()
	seedAccountAndMailbox(t, w)

	err = w.submit(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO mailbox (id, account_id, name) VALUES (2, 1, 'Archive')`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO message (id, account_id, received_at, subject, from_addr, thread_key) VALUES
			(1, 1, 100, 'a', 'a@example.com', 't1'),
			(2, 1, 200, 'b', 'b@example.com', 't2'),
			(3, 1, 300, 'c', 'c@example.com', 't3')`)
		return err
	})
	if err != nil {
		t.Fatalf("seed mailbox and messages: %v", err)
	}

	got, err := pool.MailStats(ctx)
	if err != nil {
		t.Fatalf("MailStats: %v", err)
	}
	if want := (MailStats{Messages: 3, Mailboxes: 2}); got != want {
		t.Errorf("MailStats() = %+v, want %+v", got, want)
	}
}

// TestOutboxQueuedCount proves the status line's live-fact read: only
// 'queued' rows count, a 'dispatching' row does not.
func TestOutboxQueuedCount(t *testing.T) {
	w, path := newTestWriter(t, DefaultWriterConfig())
	pool, err := NewReadPool(path, DefaultReadPoolSize, w.Revision())
	if err != nil {
		t.Fatalf("NewReadPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	ctx := context.Background()
	seedAccountAndMailbox(t, w)

	err = w.submit(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO outbox (account_id, kind, payload, state, created_at) VALUES
			(1, 'send', '{}', 'queued', 1000),
			(1, 'send', '{}', 'queued', 2000),
			(1, 'send', '{}', 'dispatching', 3000)`)
		return err
	})
	if err != nil {
		t.Fatalf("seed outbox rows: %v", err)
	}

	got, err := pool.OutboxQueuedCount(ctx)
	if err != nil {
		t.Fatalf("OutboxQueuedCount: %v", err)
	}
	if got != 2 {
		t.Errorf("OutboxQueuedCount() = %d, want 2", got)
	}
}

// TestEventCount proves the calendar placeholder's live-fact read:
// event count across two events on one calendar.
func TestEventCount(t *testing.T) {
	w, path := newTestWriter(t, DefaultWriterConfig())
	pool, err := NewReadPool(path, DefaultReadPoolSize, w.Revision())
	if err != nil {
		t.Fatalf("NewReadPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	ctx := context.Background()
	seedAccountAndMailbox(t, w)

	err = w.submit(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO calendar (id, account_id, name) VALUES (1, 1, 'Home')`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO event (id, account_id, calendar_id, uid, summary, start_local) VALUES
			(1, 1, 1, 'uid-1', 'first', 1000),
			(2, 1, 1, 'uid-2', 'second', 2000)`)
		return err
	})
	if err != nil {
		t.Fatalf("seed calendar and events: %v", err)
	}

	got, err := pool.EventCount(ctx)
	if err != nil {
		t.Fatalf("EventCount: %v", err)
	}
	if got != 2 {
		t.Errorf("EventCount() = %d, want 2", got)
	}
}
