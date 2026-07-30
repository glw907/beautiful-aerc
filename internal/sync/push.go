package sync

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
)

// echoTracker holds the record ids a dispatch has already produced,
// keyed by the server state token that dispatch's ApplyBatch call
// resolved to, so the sync worker that later sees a Changes page
// resolving to that same token can skip re-applying just those
// records rather than the whole page (ADR-0005 revision 2). A page
// commonly carries someone else's change alongside the dispatcher's
// own, and that other change must still land.
type echoTracker struct {
	mu      sync.Mutex
	byToken map[backend.ObjectKind]map[string]map[string]bool
}

func newEchoTracker() *echoTracker {
	return &echoTracker{byToken: make(map[backend.ObjectKind]map[string]map[string]bool)}
}

func (e *echoTracker) note(kind backend.ObjectKind, token string, ids []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.byToken[kind] == nil {
		e.byToken[kind] = make(map[string]map[string]bool)
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	e.byToken[kind][token] = set
}

// consume returns the record ids noted for token under kind, if any,
// removing the entry so a later, distinct token is never treated as
// an echo.
func (e *echoTracker) consume(kind backend.ObjectKind, token string) map[string]bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	ids := e.byToken[kind][token]
	delete(e.byToken[kind], token)
	return ids
}

// orderKindsForSync returns kinds with Mailbox moved ahead of every
// other kind. syncMessageMailboxes needs a message's destination
// mailbox row to already exist locally, so ordering removes the
// common case of that gap within one push- or poll-triggered cycle:
// a mailbox created and populated in the same batch as a message move
// into it.
func orderKindsForSync(kinds []backend.ObjectKind) []backend.ObjectKind {
	ordered := make([]backend.ObjectKind, 0, len(kinds))
	for _, k := range kinds {
		if k == backend.ObjectKindMailbox {
			ordered = append(ordered, k)
		}
	}
	for _, k := range kinds {
		if k != backend.ObjectKindMailbox {
			ordered = append(ordered, k)
		}
	}
	return ordered
}

// syncFlushState tracks, per kind, whether a push- or poll-triggered
// SyncKind call is currently failing and under what class, so a
// caller surfaces a uerr.Error only on the first failure or a class
// change and a recovery line once, rather than once per flush
// (ADR-0013 revision 2: construction is the surfacing event, not the
// retry).
type syncFlushState struct {
	failing map[backend.ObjectKind]uerr.Class
}

func newSyncFlushState() *syncFlushState {
	return &syncFlushState{failing: make(map[backend.ObjectKind]uerr.Class)}
}

// report records the outcome of one SyncKind(kind) call. A context
// cancellation (RunPush's caller shutting the worker down mid-flush)
// is never surfaced: it is not a server problem the user needs to
// see, and reporting it would classify shutdown itself as a dropped
// connection.
func (s *syncFlushState) report(kind backend.ObjectKind, err error) {
	prevClass, wasFailing := s.failing[kind]
	switch {
	case err == nil:
		if wasFailing {
			slog.Info("sync: push-triggered sync recovered", "kind", kindName(kind))
			delete(s.failing, kind)
		}
		return
	case !surfaceable(err):
		return
	}
	class, cause := classifyErr(err)
	if !wasFailing || prevClass != class {
		_ = uerr.New("sync.push.flush", nil, class, cause)
	}
	s.failing[kind] = class
}

// surfaceable reports whether err represents a real failure worth
// classifying and surfacing through uerr.New, as opposed to
// context.Canceled: the caller (RunPush, through reconnect or a
// push-triggered flush) shutting down mid-flight is not a server
// problem, so it must never render as one.
func surfaceable(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled)
}

// classifyErr reports the uerr.Class and root cause err already
// carries, in either of the two shapes a backend hands one over: a
// uerr.Error it built and logged itself, or a backend.PushFailure it
// classified and left to this package to surface. Both unwrap to their
// own Cause, since Error() only ever returns the class's fixed
// sentence. An err that was never classified reports ClassConnection
// and itself: a Listen or push-triggered SyncKind failure with no
// finer classification is, by construction, a connectivity problem.
func classifyErr(err error) (uerr.Class, error) {
	if ue, ok := errors.AsType[uerr.Error](err); ok {
		return ue.Class, ue.Cause
	}
	if pf, ok := errors.AsType[backend.PushFailure](err); ok {
		return pf.Class, pf.Cause
	}
	return uerr.ClassConnection, err
}

// RunPush drives kinds' push loop against backend: push notifications
// coalesce into SyncKind calls, and a transport that stops for good
// (the channel closing, which under backend.Push's contract is a
// server refusal rather than a drop) is opened again through jittered
// backoff. A backend with no push transport at all
// (backend.PushTransportNone) falls back to polling SyncKind on a
// fixed cadence instead. RunPush blocks until ctx is done.
func (w *Worker) RunPush(ctx context.Context, kinds []backend.ObjectKind) {
	kinds = orderKindsForSync(kinds)

	push := w.backend.Push()
	if push == nil {
		slog.Info("sync: backend reports no push transport, polling instead",
			"account_id", w.accountID, "interval", w.cfg.PollInterval)
		w.pollKinds(ctx, kinds)
		return
	}

	state := newSyncFlushState()
	for {
		ch, err := reconnect(ctx, push, w.cfg)
		if err != nil {
			return
		}
		if !consumePush(ctx, ch, w.cfg.CoalesceWindow, func() { w.flush(ctx, kinds, state) }) {
			return
		}
		// One step, and the same one every time. A transport that
		// opens and stops at once would otherwise be reopened at zero
		// delay, spinning into a request storm. Escalating across
		// connections is the transport's own schedule to run, and
		// running a second one here is what puts SY-2's 30s p95 bound
		// out of reach. A server that goes on refusing still escalates,
		// through reconnect's own loop.
		if !SleepBackoff(ctx, 0, w.cfg.BackoffMin, w.cfg.BackoffMax) {
			return
		}
	}
}

// pollKinds runs kinds' SyncKind on a fixed PollInterval cadence, the
// fallback a backend with no push transport at all uses in place of
// EventSource. It syncs once immediately, then on every tick, and
// blocks until ctx is done.
func (w *Worker) pollKinds(ctx context.Context, kinds []backend.ObjectKind) {
	state := newSyncFlushState()
	w.flush(ctx, kinds, state)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.flush(ctx, kinds, state)
		case <-ctx.Done():
			return
		}
	}
}

// flush runs one SyncKind cycle per kind (already ordered Mailbox
// before Message) and reports each outcome to state.
func (w *Worker) flush(ctx context.Context, kinds []backend.ObjectKind, state *syncFlushState) {
	for _, kind := range kinds {
		state.report(kind, w.SyncKind(ctx, kind))
	}
}

// reconnect calls push.Listen, retrying with jittered exponential
// backoff until it succeeds or ctx ends. Each call starts its own
// schedule: a transport reached at all ends whatever run of refusals
// escalated the last one, and carrying the count forward is how a
// process that has been up for days answers its first refusal with the
// saturated delay. A Listen failure surfaces a uerr.Error once, on the
// first failure or a class change (ADR-0013 revision 2); a later
// success after a run of failures logs recovery. ctx ending mid-retry
// (RunPush shutting the worker down) is never classified or surfaced:
// it is not a server problem.
func reconnect(ctx context.Context, push backend.Push, cfg Config) (<-chan backend.Notification, error) {
	var failClass uerr.Class
	var failing bool
	for attempt := 0; ; attempt++ {
		ch, err := push.Listen(ctx)
		if err == nil {
			if failing {
				slog.Info("sync: push reconnected", "attempts", attempt+1)
			}
			return ch, nil
		}
		if surfaceable(err) {
			class, cause := classifyErr(err)
			if !failing || class != failClass {
				_ = uerr.New("sync.push.listen", nil, class, cause)
				failing = true
				failClass = class
			}
		}
		if !SleepBackoff(ctx, attempt, cfg.BackoffMin, cfg.BackoffMax) {
			return nil, ctx.Err()
		}
	}
}

// consumePush reads notifications from ch, invoking flush once per
// coalescing window, measured from that window's first notification
// and never extended by a later one (ADR-0005 revision 2: a steady
// remote burst must not defer sync indefinitely). It reports true if
// ch closed, which under backend.Push's contract is the transport
// stopping for good, and false if ctx ended first.
//
// Silence is not a drop here. The transport owns the liveness check,
// since it is the only layer that knows the cadence the server granted
// (RFC 8620 section 7.3 lets the server choose it, and JT-26 pins that
// the granted one is what governs), and a second detector reading a
// figure from local config tears down connections the transport
// considers healthy.
func consumePush(ctx context.Context, ch <-chan backend.Notification, window time.Duration, flush func()) bool {
	var flushC <-chan time.Time
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return true
			}
			if flushC == nil {
				flushC = time.NewTimer(window).C
			}
		case <-flushC:
			flush()
			flushC = nil
		case <-ctx.Done():
			return false
		}
	}
}

// SleepBackoff sleeps a jittered exponential delay for attempt (0 for
// the first retry) and reports whether it finished; false means ctx
// ended first. It is exported so a caller outside this package
// (cmd/poplar's own connect retry) can back off on the same schedule
// as RunPush's reconnect, rather than a second, near-identical
// implementation.
func SleepBackoff(ctx context.Context, attempt int, minDelay, maxDelay time.Duration) bool {
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
	bound := min(minDelay, maxDelay)
	for range attempt {
		bound = min(bound*2, maxDelay)
	}
	if bound <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(bound))) //nolint:gosec // G404: jitter timing, not a security decision
}
