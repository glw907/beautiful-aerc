package sync

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
)

// TestPushCoalescing asserts consumePush's fixed 200ms window: a
// single notification flushes 200ms after it arrives, and a sustained
// stream of notifications (one every 50ms, well inside the window)
// still flushes on the same fixed schedule rather than the window
// resetting on every arrival.
func TestPushCoalescing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const window = 200 * time.Millisecond
		ch := make(chan backend.Notification)

		var flushes atomic.Int64
		var flushedAt []time.Duration
		start := time.Now()
		flush := func() {
			flushedAt = append(flushedAt, time.Since(start))
			flushes.Add(1)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() {
			consumePush(ctx, ch, window, time.Hour, flush)
			close(done)
		}()

		// A sustained burst: an event every 50ms for 1s, well inside
		// the window each time it starts a fresh one.
		go func() {
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for range 20 {
				<-ticker.C
				select {
				case ch <- backend.Notification{}:
				case <-ctx.Done():
					return
				}
			}
		}()

		time.Sleep(1100 * time.Millisecond)
		synctest.Wait()

		got := flushes.Load()
		// A fixed 200ms window, from the first event, restarted only
		// after each flush: over ~1.1s of sustained traffic that is 5
		// or 6 flushes depending on exact alignment, never one
		// (proving the window is not a resetting debounce) and never
		// as many as 20 (proving it batches instead of flushing per
		// event).
		if got < 4 || got > 7 {
			t.Fatalf("flushes = %d over ~1.1s of sustained traffic, want a fixed ~200ms cadence (4-7 flushes), flush times = %v", got, flushedAt)
		}
		for i, at := range flushedAt {
			if i == 0 {
				continue
			}
			gap := at - flushedAt[i-1]
			if gap < window-10*time.Millisecond {
				t.Fatalf("flush %d landed %v after the previous one, want at least the %v window", i, gap, window)
			}
		}

		cancel()
		<-done
	})
}

// TestStallDetection asserts that silence on the notification channel
// past twice pingInterval counts as a dropped stream, even though the
// channel itself never closes.
func TestStallDetection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const pingInterval = 100 * time.Millisecond
		ch := make(chan backend.Notification)

		result := make(chan bool, 1)
		go func() {
			result <- consumePush(context.Background(), ch, 200*time.Millisecond, pingInterval, func() {})
		}()

		synctest.Wait()
		select {
		case <-result:
			t.Fatal("consumePush returned before any silence elapsed")
		default:
		}

		time.Sleep(2*pingInterval + 10*time.Millisecond)
		synctest.Wait()

		select {
		case dropped := <-result:
			if !dropped {
				t.Fatal("consumePush() = false, want true: silence past twice pingInterval counts as a drop")
			}
		default:
			t.Fatal("consumePush did not return after silence past twice pingInterval")
		}
	})
}

// TestStallDetectionSurvivesLivePings asserts that traffic on the
// channel resets the stall detector, so a live stream that keeps
// sending (even with no state changes to coalesce, a bare ping) never
// trips the stall timer.
func TestStallDetectionSurvivesLivePings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const pingInterval = 100 * time.Millisecond
		ch := make(chan backend.Notification)

		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan bool, 1)
		go func() {
			result <- consumePush(ctx, ch, 200*time.Millisecond, pingInterval, func() {})
		}()

		go func() {
			ticker := time.NewTicker(pingInterval / 2)
			defer ticker.Stop()
			for range 10 {
				<-ticker.C
				select {
				case ch <- backend.Notification{}:
				case <-ctx.Done():
					return
				}
			}
		}()

		time.Sleep(10 * (pingInterval / 2))
		synctest.Wait()

		select {
		case <-result:
			t.Fatal("consumePush returned while pings kept arriving inside the stall window")
		default:
		}

		cancel()
		<-result
	})
}

var errConnectionRefused = errors.New("push: connection refused")

// TestBackoffRecovery asserts SY-2's 30s p95 push-recovery bound over
// 20 synthetic reconnect trials, each simulating a small, randomized
// run of failed Listen attempts before the transport comes back.
func TestBackoffRecovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := DefaultConfig()
		const trials = 20

		elapsed := make([]time.Duration, trials)
		for i := range trials {
			// A realistic transient-drop scenario: 0-3 failed
			// reconnect attempts before the transport answers again.
			failures := i % 4

			attempts := 0
			push := &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
				attempts++
				if attempts <= failures {
					return nil, errConnectionRefused
				}
				return make(chan backend.Notification), nil
			}}

			start := time.Now()
			attempt := 0
			if _, err := reconnect(context.Background(), push, cfg, &attempt); err != nil {
				t.Fatalf("trial %d: reconnect: %v", i, err)
			}
			elapsed[i] = time.Since(start)
		}

		if p95 := percentile95(elapsed); p95 > 30*time.Second {
			t.Fatalf("p95 reconnect time = %v over %d trials, want <= 30s", p95, trials)
		}
	})
}

func percentile95(d []time.Duration) time.Duration {
	sorted := slices.Clone(d)
	slices.Sort(sorted)
	idx := (len(sorted)*95 + 99) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// TestRunPushBackoffsAfterImmediateDrop asserts a transport that
// connects successfully and then immediately drops the stream still
// costs at least one backoff step before RunPush reconnects: without
// that, a connect-then-drop transport reconnects at zero delay and
// spins the loop into an unbounded request storm.
func TestRunPushBackoffsAfterImmediateDrop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			return backend.ChangeSet{}, nil
		}

		start := time.Now()
		var listenTimes []time.Duration
		be.PushSource = &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
			listenTimes = append(listenTimes, time.Since(start))
			ch := make(chan backend.Notification)
			close(ch) // connects, then the stream immediately drops
			return ch, nil
		}}

		cfg := testConfig()
		cfg.BackoffMin = 100 * time.Millisecond
		cfg.BackoffMax = time.Second
		worker := NewWorker(accountID, &be, w, cfg)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()

		time.Sleep(350 * time.Millisecond)
		synctest.Wait()
		cancel()
		<-done

		if len(listenTimes) < 2 {
			t.Fatalf("Listen called %d times in 350ms, want at least 2 (a bounded reconnect rate, not a spin)", len(listenTimes))
		}
		for i := 1; i < len(listenTimes); i++ {
			if gap := listenTimes[i] - listenTimes[i-1]; gap <= 0 {
				t.Fatalf("Listen call %d landed with zero delay after the previous one, want at least one backoff step", i)
			}
		}
	})
}

// TestRunPushPollsWithNoPushTransport asserts RunPush falls back to
// polling SyncKind on PollInterval's cadence for a backend whose
// Push() is nil (backend.PushTransportNone's documented contract),
// rather than returning immediately and never syncing again.
func TestRunPushPollsWithNoPushTransport(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var calls atomic.Int64
		var be backendtest.Fake // PushSource left nil: no push transport
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			calls.Add(1)
			return backend.ChangeSet{}, nil
		}

		cfg := testConfig()
		cfg.PollInterval = 100 * time.Millisecond
		worker := NewWorker(accountID, &be, w, cfg)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()

		synctest.Wait()
		if got := calls.Load(); got < 1 {
			t.Fatalf("calls = %d immediately after start, want at least 1: RunPush must not sit idle with no push transport", got)
		}

		time.Sleep(250 * time.Millisecond)
		synctest.Wait()
		if got := calls.Load(); got < 3 {
			t.Fatalf("calls = %d after 250ms of a 100ms poll interval, want at least 3", got)
		}

		cancel()
		<-done
	})
}

// TestSyncFlushStateSurfacesTransitions asserts syncFlushState's
// state-transition gate (ADR-0013 revision 2): a first failure and a
// class change both count as a new surfacing event, a repeated
// failure of the same class does not change the tracked state, and a
// success after a failure clears it, so a later failure of the same
// class surfaces again.
func TestSyncFlushStateSurfacesTransitions(t *testing.T) {
	state := newSyncFlushState()
	kind := backend.ObjectKindMessage

	connErr := errors.New("dial refused")
	authErr := uerr.New("test.op", nil, uerr.ClassAuth, errors.New("token rejected"))

	state.report(kind, connErr)
	class, failing := state.failing[kind]
	if !failing || class != uerr.ClassConnection {
		t.Fatalf("after first failure: (class, failing) = (%v, %v), want (ClassConnection, true)", class, failing)
	}

	state.report(kind, connErr)
	if class := state.failing[kind]; class != uerr.ClassConnection {
		t.Fatalf("a repeated same-class failure changed the tracked class to %v", class)
	}

	state.report(kind, authErr)
	if class := state.failing[kind]; class != uerr.ClassAuth {
		t.Fatalf("class change was not recorded: class = %v, want ClassAuth", class)
	}

	state.report(kind, nil)
	if _, failing := state.failing[kind]; failing {
		t.Fatal("a success left the kind marked failing")
	}

	state.report(kind, connErr)
	if class, failing := state.failing[kind]; !failing || class != uerr.ClassConnection {
		t.Fatalf("a failure after recovery did not surface: (class, failing) = (%v, %v)", class, failing)
	}
}

// TestClassifyErr asserts classifyErr's two paths: an error never
// classified upstream defaults to ClassConnection, and a uerr.Error
// yields its own class and its original root cause (not the fixed
// per-class sentence uerr.Error.Error() returns).
func TestClassifyErr(t *testing.T) {
	plain := errors.New("boom")
	if class, cause := classifyErr(plain); class != uerr.ClassConnection || cause != plain {
		t.Fatalf("classifyErr(plain) = (%v, %v), want (ClassConnection, plain)", class, cause)
	}

	root := errors.New("token rejected")
	wrapped := uerr.New("test.op", nil, uerr.ClassAuth, root)
	if class, cause := classifyErr(wrapped); class != uerr.ClassAuth || cause != root {
		t.Fatalf("classifyErr(wrapped) = (%v, %v), want (ClassAuth, root)", class, cause)
	}
}
