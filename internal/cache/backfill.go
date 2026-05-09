package cache

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// nextUnfetchedUID returns the newest message UID without a stored
// body, or ok=false when every cached message has bytes. The query
// is the implicit work queue for the backfill worker: sent_at DESC
// puts new mail at the top, eviction restores rows naturally.
func (a *Account) nextUnfetchedUID(ctx context.Context) (mail.UID, bool, error) {
	var pid string
	err := a.db.QueryRowContext(ctx, `
		SELECT m.protocol_id
		FROM messages m
		LEFT JOIN bodies b ON b.message = m.id
		WHERE b.bytes IS NULL
		ORDER BY m.sent_at DESC
		LIMIT 1
	`).Scan(&pid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return mail.UID(pid), true, nil
}

// Backfiller is the per-account body-cache filler. Lifecycle: built
// in Account.Open, started by Run, stopped by canceling Run's ctx.
type Backfiller struct {
	acct          *Account
	rate          time.Duration
	idleThreshold time.Duration
	maxBatchBytes int64

	lastActivity atomic.Int64 // unix nanos
	connOnline   atomic.Bool
}

func newBackfiller(a *Account) *Backfiller {
	bf := &Backfiller{
		acct:          a,
		rate:          500 * time.Millisecond,
		idleThreshold: 5 * time.Second,
		maxBatchBytes: 2 * 1024 * 1024,
	}
	bf.connOnline.Store(true)
	return bf
}

// fetchOne fetches the next eligible body, or returns nil when the
// cache is caught up. Errors propagate; the run loop classifies them.
func (b *Backfiller) fetchOne(ctx context.Context) error {
	uid, ok, err := b.acct.nextUnfetchedUID(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	body, err := b.acct.Backend.FetchBody(uid)
	if err != nil {
		return err
	}
	return b.acct.storeBody(ctx, uid, body)
}

// Run drives the backfill loop until ctx is canceled.
// TODO(backfill): Task 4 replaces this stub with the rate-limited fetch loop.
func (b *Backfiller) Run(ctx context.Context) {
	<-ctx.Done()
}
