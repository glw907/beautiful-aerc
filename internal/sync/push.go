package sync

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/glw907/poplar/internal/backend"
)

// echoTracker holds the server state tokens a dispatch has already
// produced, so the sync worker that later sees the same token as a
// push-triggered Changes result can skip re-applying it (ADR-0005
// revision 2's self-echo suppression). A token is consumed once: the
// next distinct state change reaching that kind is never suppressed.
type echoTracker struct {
	mu     sync.Mutex
	tokens map[backend.ObjectKind]map[string]bool
}

func newEchoTracker() *echoTracker {
	return &echoTracker{tokens: make(map[backend.ObjectKind]map[string]bool)}
}

func (e *echoTracker) note(kind backend.ObjectKind, token string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tokens[kind] == nil {
		e.tokens[kind] = make(map[string]bool)
	}
	e.tokens[kind][token] = true
}

// consume reports whether token was previously noted for kind,
// removing it if so.
func (e *echoTracker) consume(kind backend.ObjectKind, token string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.tokens[kind][token] {
		return false
	}
	delete(e.tokens[kind], token)
	return true
}

// RunPush drives kinds' push loop against backend: EventSource
// notifications coalesce into SyncKind calls, and a stream drop
// (the channel closing, or silence past twice PingInterval, counting
// as a missing ping) reconnects through jittered backoff. RunPush
// blocks until ctx is done.
func (w *Worker) RunPush(ctx context.Context, kinds []backend.ObjectKind) {
	push := w.backend.Push()
	if push == nil {
		return
	}

	for {
		ch, err := reconnect(ctx, push, w.cfg)
		if err != nil {
			return
		}
		dropped := consumePush(ctx, ch, w.cfg.CoalesceWindow, w.cfg.PingInterval, func() {
			for _, kind := range kinds {
				_ = w.SyncKind(ctx, kind)
			}
		})
		if !dropped {
			return
		}
	}
}

// reconnect calls push.Listen, retrying with jittered exponential
// backoff on failure, until it succeeds or ctx ends.
func reconnect(ctx context.Context, push backend.Push, cfg Config) (<-chan backend.Notification, error) {
	for attempt := 0; ; attempt++ {
		ch, err := push.Listen(ctx)
		if err == nil {
			return ch, nil
		}
		if !sleepBackoff(ctx, attempt, cfg.BackoffMin, cfg.BackoffMax) {
			return nil, ctx.Err()
		}
	}
}

// consumePush reads notifications from ch, invoking flush once per
// coalescing window, measured from that window's first notification
// and never extended by a later one (ADR-0005 revision 2: a steady
// remote burst must not defer sync indefinitely). It reports true if
// ch closed or fell silent past twice pingInterval (a missing ping,
// treated the same as a dropped transport), false if ctx ended first.
func consumePush(ctx context.Context, ch <-chan backend.Notification, window, pingInterval time.Duration, flush func()) bool {
	stall := time.NewTimer(2 * pingInterval)
	defer stall.Stop()

	var flushTimer *time.Timer
	var flushC <-chan time.Time
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return true
			}
			if !stall.Stop() {
				<-stall.C
			}
			stall.Reset(2 * pingInterval)
			if flushTimer == nil {
				flushTimer = time.NewTimer(window)
				flushC = flushTimer.C
			}
		case <-flushC:
			flush()
			flushTimer = nil
			flushC = nil
		case <-stall.C:
			return true
		case <-ctx.Done():
			return false
		}
	}
}

// sleepBackoff sleeps a jittered exponential delay for attempt (0 for
// the first retry) and reports whether it finished; false means ctx
// ended first.
func sleepBackoff(ctx context.Context, attempt int, minDelay, maxDelay time.Duration) bool {
	d := backoffDelay(attempt, minDelay, maxDelay)
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// backoffDelay returns a full-jitter delay for attempt: uniformly
// random between zero and minDelay doubled attempt times, capped at
// maxDelay.
func backoffDelay(attempt int, minDelay, maxDelay time.Duration) time.Duration {
	bound := minDelay
	for range attempt {
		if bound >= maxDelay {
			bound = maxDelay
			break
		}
		bound *= 2
	}
	if bound > maxDelay {
		bound = maxDelay
	}
	if bound <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(bound))) //nolint:gosec // G404: jitter timing, not a security decision
}
