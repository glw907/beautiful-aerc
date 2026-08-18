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
// carries, in either of the two uerr.Classified shapes a backend
// hands one over: a uerr.Error it built and logged itself, or a
// backend.Failure it classified and left to this package to surface.
// An err that was never classified reports ClassConnection and
// itself: a Listen or push-triggered SyncKind failure with no finer
// classification is, by construction, a connectivity problem.
func classifyErr(err error) (uerr.Class, error) {
	return uerr.ClassifyErr(err, uerr.ClassConnection)
}

// RunPush drives kinds' push loop against backend: push notifications
// coalesce into SyncKind calls, and a transport that stops for good
// (the channel closing, which under backend.Push's contract is a
// server refusal rather than a drop) is opened again through jittered
// backoff. RunPush blocks until ctx is done.
//
// Whenever push is unavailable, SyncKind is called on PollInterval's
// fixed cadence instead, which is SY-2's MUST and ADR-0005's decision:
// a backend with no push transport at all
// (backend.PushTransportNone) polls for its whole run, and a transport
// whose stream the server keeps refusing polls until a stream attaches
// again. The event source answering 401 while the JMAP API works is
// the case that makes the second one matter, since nothing else pulls
// Changes and the account would otherwise receive no mail for the life
// of the process.
//
// The ticker is RunPush's and its ticks are read only while the stream
// is down, so a working stream never polls. It is not restarted per
// reconnect: a server that opens a stream and refuses the next would
// otherwise keep resetting the cadence and never reach a tick.
func (w *Worker) RunPush(ctx context.Context, kinds []backend.ObjectKind) {
	kinds = orderKindsForSync(kinds)

	transport := w.backend.Push()
	if transport == nil {
		slog.Info("sync: backend reports no push transport, polling instead",
			"account_id", w.accountID, "interval", w.cfg.PollInterval)
		w.pollKinds(ctx, kinds)
		return
	}

	flushState := newSyncFlushState()
	poll := time.NewTicker(w.cfg.PollInterval)
	defer poll.Stop()

	var push pushState
	for {
		ch, err := reconnect(ctx, transport, w.cfg, &push, poll.C, func() { w.flush(ctx, kinds, flushState) })
		if err != nil {
			return
		}
		if !consumePush(ctx, ch, w.cfg, func() { w.flush(ctx, kinds, flushState) }, push.proved) {
			return
		}
		// A stop the transport did not explain still costs a step, or
		// one that opens and stops at once is reopened at zero delay and
		// spins into a request storm. It is a floor, not a schedule: a
		// stop with a refusal behind it comes back as the next Listen's
		// error, and escalating on that is reconnect's job, so
		// escalating here as well would put two delays on one failure.
		if !SleepBackoff(ctx, 0, w.cfg.BackoffMin, w.cfg.BackoffMax) {
			return
		}
	}
}

// pushState is RunPush's view of the transport across every stream it
// opens: the class of failure currently surfaced, so a standing
// failure logs once rather than once per attempt (ADR-0013 revision
// 2), and how far the reopen schedule has escalated.
//
// Both outlive a single reconnect call because the failure they track
// does. A refusal that ends a stream already open reaches the caller
// as the next Listen's error (backend.Push), so a server refusing
// every other connection produces one failure per stream: a schedule
// that started over per call would not slow that down, and a dedup
// that did would write a line per stream. Measured against such a
// server, the per-call form drew about a hundred requests in twenty
// seconds.
//
// Only a refusal reaches this schedule. A drop never does, because the
// transport reconnects through it without closing the channel, so this
// and the transport's own schedule govern disjoint failures and never
// compose on one. What ends a run of refusals is a stream that has
// stayed open past BackoffMax, which is the only evidence available
// that the server is serving again, and it is read while the stream is
// still up: waiting for it to end would leave a failure as the last
// word on a transport that has been working for days, and would keep
// the next failure of the same class from surfacing at all.
type pushState struct {
	failing bool
	class   uerr.Class
	attempt int
}

// fail surfaces err once per failure episode, on the first failure or
// a class change (ADR-0013 revision 2). ctx ending mid-retry (RunPush
// shutting the worker down) is never classified or surfaced: it is not
// a server problem.
func (s *pushState) fail(err error) {
	if !surfaceable(err) {
		return
	}
	class, cause := classifyErr(err)
	if !s.failing || class != s.class {
		_ = uerr.New("sync.push.listen", nil, class, cause)
		s.failing = true
		s.class = class
	}
}

// proved records a stream that has stayed open long enough to say the
// server is serving again: the failure run is over, and the reopen
// schedule starts from zero.
func (s *pushState) proved() {
	if s.failing {
		slog.Info("sync: push recovered")
		s.failing = false
	}
	s.attempt = 0
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
// backoff until it succeeds or ctx ends. state carries the schedule
// and the surfacing decision across calls, for the reasons its own doc
// comment gives; reconnect only ever advances them.
//
// poll runs on every tick of pollC that lands while the stream is
// still refused, which is SY-2's fallback and the only thing pulling
// Changes during a refusal. It rides the waits the schedule is already
// taking rather than a schedule of its own, so one failure still backs
// off in exactly one place. A nil pollC never fires, which is the
// no-fallback case a caller drives when it only wants the retry.
func reconnect(ctx context.Context, push backend.Push, cfg Config, state *pushState, pollC <-chan time.Time, poll func()) (<-chan backend.Notification, error) {
	for {
		ch, err := push.Listen(ctx)
		if err == nil {
			return ch, nil
		}
		state.fail(err)
		if !sleepBackoff(ctx, state.attempt, cfg.BackoffMin, cfg.BackoffMax, pollC, poll) {
			return nil, ctx.Err()
		}
		state.attempt++
	}
}

// consumePush reads notifications from ch, invoking flush once per
// coalescing window, measured from that window's first notification
// and never extended by a later one (ADR-0005 revision 2: a steady
// remote burst must not defer sync indefinitely). It calls proved once
// if the stream is still open past cfg.BackoffMax, which is the only
// place that can tell: the loop is inside this call for a whole
// stream's life. It reports true if ch closed, which under
// backend.Push's contract is the transport stopping for good, and
// false if ctx ended first.
//
// Silence is not a drop here. The transport owns the liveness check,
// since it is the only layer that knows the cadence the server granted
// (RFC 8620 section 7.3 lets the server choose it, and JT-26 pins that
// the granted one is what governs), and a second detector reading a
// figure from local config tears down connections the transport
// considers healthy.
func consumePush(ctx context.Context, ch <-chan backend.Notification, cfg Config, flush, proved func()) bool {
	proving := time.NewTimer(cfg.BackoffMax)
	defer proving.Stop()
	provingC := proving.C

	var flushC <-chan time.Time
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				// A window still open holds a notification already taken
				// off the channel. The connect notification ADR-0018's
				// flush-on-connect rests on is exactly the one that
				// arrives just before a stream stops, so leaving it here
				// loses the pull that connection owed.
				if flushC != nil {
					flush()
				}
				return true
			}
			if flushC == nil {
				flushC = time.NewTimer(cfg.CoalesceWindow).C
			}
		case <-flushC:
			flush()
			flushC = nil
		case <-provingC:
			proved()
			provingC = nil
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
	return sleepBackoff(ctx, attempt, minDelay, maxDelay, nil, nil)
}

// sleepBackoff is SleepBackoff with reconnect's poll fallback folded
// in: it runs poll on every tick of pollC that lands inside the delay,
// so a refused stream's own wait is what the fallback rides. A nil
// pollC never fires, which is SleepBackoff's plain sleep.
func sleepBackoff(ctx context.Context, attempt int, minDelay, maxDelay time.Duration, pollC <-chan time.Time, poll func()) bool {
	d := backoffDelay(attempt, minDelay, maxDelay)
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			return true
		case <-pollC:
			poll()
		case <-ctx.Done():
			return false
		}
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
