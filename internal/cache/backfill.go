package cache

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/glw907/poplar/internal/backoff"
	"github.com/glw907/poplar/internal/mail"
)

// nextUnfetchedUID returns the newest message UID without a stored
// body. ok is false when every cached message has bytes. The query
// is the implicit work queue for the backfill worker.
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

// Backfiller fills the body cache in the background. Open creates
// it, Run drives the loop, and canceling Run's context stops it.
type Backfiller struct {
	acct          *Account
	rate          time.Duration
	idleThreshold time.Duration
	maxBatchBytes int64

	lastActivity     atomic.Int64 // unix nanos
	connOnline       atomic.Bool
	throttleAttempts int
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

// fetchOne fetches the next eligible body. Returns 0 when caught up.
// Errors propagate; the run loop classifies them.
func (b *Backfiller) fetchOne(ctx context.Context) (int, error) {
	uid, ok, err := b.acct.nextUnfetchedUID(ctx)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	body, err := b.acct.Backend.FetchBody(uid)
	if err != nil {
		return 0, err
	}
	if err := b.acct.storeBody(ctx, uid, body); err != nil {
		return 0, err
	}
	return len(body), nil
}

// NotifyActivity suspends backfill until idleThreshold elapses since
// the last call.
func (b *Backfiller) NotifyActivity() {
	b.lastActivity.Store(time.Now().UnixNano())
}

// NotifyConnState suspends backfill while online is false.
func (b *Backfiller) NotifyConnState(online bool) {
	b.connOnline.Store(online)
}

// atCap returns true once the body cache has crossed 90% of the
// configured cap. Zero-or-negative maxSize disables the cap.
func (b *Backfiller) atCap(ctx context.Context) bool {
	if b.acct.maxSize <= 0 {
		return false
	}
	var total int64
	if err := b.acct.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(LENGTH(bytes)), 0) FROM bodies`).Scan(&total); err != nil {
		return false
	}
	return total >= (b.acct.maxSize*9)/10
}

func (b *Backfiller) idle() bool {
	if b.idleThreshold == 0 {
		return true
	}
	last := b.lastActivity.Load()
	if last == 0 {
		return true
	}
	return time.Since(time.Unix(0, last)) >= b.idleThreshold
}

// Run drives the backfill loop until ctx is canceled.
func (b *Backfiller) Run(ctx context.Context) {
	t := time.NewTicker(b.rate)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if !b.connOnline.Load() || !b.idle() {
			continue
		}
		b.runBatch(ctx)
	}
}

func (b *Backfiller) runBatch(ctx context.Context) {
	var bytesFetched int64
	for bytesFetched < b.maxBatchBytes {
		if !b.idle() || !b.connOnline.Load() || b.atCap(ctx) {
			return
		}
		n, err := b.fetchOne(ctx)
		if err != nil {
			if isThrottleErr(err) {
				b.throttleAttempts++
				select {
				case <-ctx.Done():
				case <-time.After(backoff.Exponential(b.throttleAttempts, time.Second, 60*time.Second)):
				}
				return
			}
			b.throttleAttempts = 0
			return
		}
		if n == 0 {
			return // caught up
		}
		b.throttleAttempts = 0
		bytesFetched += int64(n)
	}
}

// isThrottleErr reports whether err is a backend rate-limit signal.
// Upstream libraries surface these as opaque strings, so substring
// matching is the only option at this layer.
func isThrottleErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "THROTTLED") ||
		strings.Contains(s, "rate limited") ||
		strings.Contains(s, "rateLimit") ||
		strings.Contains(s, "429")
}
