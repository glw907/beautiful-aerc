package sync

import (
	"context"
	"errors"
	"fmt"
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

// TestSurfaceable asserts surfaceable's two exceptions to "any
// non-nil error is worth surfacing": nil itself, and a context
// cancellation (bare or wrapped), which reports a caller shutting
// down, not a server problem. A different context error
// (DeadlineExceeded, a real timeout) still surfaces.
func TestSurfaceable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("boom"), true},
		{"context canceled", context.Canceled, false},
		{"wrapped context canceled", fmt.Errorf("dial: %w", context.Canceled), false},
		{"context deadline exceeded", context.DeadlineExceeded, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := surfaceable(tt.err); got != tt.want {
				t.Errorf("surfaceable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestSyncFlushStateIgnoresContextCanceled asserts report treats a
// context cancellation as neither a failure nor a recovery: it never
// marks kind failing (which would otherwise surface a uerr.Error
// through classifyErr's ClassConnection default), and a real failure
// reported afterward still surfaces normally.
func TestSyncFlushStateIgnoresContextCanceled(t *testing.T) {
	state := newSyncFlushState()
	kind := backend.ObjectKindMessage

	state.report(kind, context.Canceled)
	if _, failing := state.failing[kind]; failing {
		t.Fatal("report marked kind failing on context.Canceled, want shutdown treated as no outcome at all")
	}

	state.report(kind, errConnectionRefused)
	if class, failing := state.failing[kind]; !failing || class != uerr.ClassConnection {
		t.Fatalf("a real failure after a canceled report did not surface: (class, failing) = (%v, %v)", class, failing)
	}
}

// TestReconnectReturnsPromptlyOnContextCanceled asserts reconnect
// returns ctx.Err() as soon as ctx is already done, rather than
// classifying the cancellation as a connectivity failure and looping
// through a backoff wait first.
func TestReconnectReturnsPromptlyOnContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	push := &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
		return nil, context.Canceled
	}}

	attempt := 0
	ch, err := reconnect(ctx, push, DefaultConfig(), &attempt)
	if ch != nil {
		t.Fatal("reconnect returned a non-nil channel alongside an error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("reconnect() error = %v, want context.Canceled", err)
	}
}

// TestRunPushResetsBackoffAfterHealthyConnection asserts a drop
// following a connection that stayed up at least BackoffMax resets
// the escalated backoff attempt counter, so the next Listen call
// lands within BackoffMin of the drop rather than the near-BackoffMax
// range a long run of quick prior drops would otherwise leave it at.
// Without this, SY-2's 30s p95 recovery bound erodes over a
// long-running process: attempt only ever grows, so a single drop
// after an hours-long healthy stream would wait the same escalated
// range as the sixth drop in a row.
func TestRunPushResetsBackoffAfterHealthyConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			return backend.ChangeSet{}, nil
		}

		const quickDrops = 6
		const healthyHold = 1500 * time.Millisecond
		start := time.Now()
		var listenTimes []time.Duration
		calls := 0
		be.PushSource = &backendtest.FakePush{ListenFunc: func(ctx context.Context) (<-chan backend.Notification, error) {
			calls++
			listenTimes = append(listenTimes, time.Since(start))
			ch := make(chan backend.Notification)
			switch {
			case calls <= quickDrops:
				close(ch) // an immediate drop, escalating attempt each time
			case calls == quickDrops+1:
				// The connection right after the run of quick drops stays
				// open long enough to prove itself healthy, then drops.
				go func() {
					time.Sleep(healthyHold)
					close(ch)
				}()
			default:
				// The reconnect under test: stays open until the test
				// cancels ctx, so its own Listen time is all this
				// assertion needs.
				context.AfterFunc(ctx, func() { close(ch) })
			}
			return ch, nil
		}}

		cfg := testConfig()
		cfg.BackoffMin = 100 * time.Millisecond
		cfg.BackoffMax = time.Second
		cfg.PingInterval = time.Hour // never trips the stall detector during the healthy hold
		worker := NewWorker(accountID, &be, w, cfg)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()

		time.Sleep(8 * time.Second)
		synctest.Wait()
		cancel()
		<-done
		synctest.Wait()

		if len(listenTimes) <= quickDrops+1 {
			t.Fatalf("Listen calls = %d, want more than %d (the healthy connection plus at least one reconnect after it dropped)", len(listenTimes), quickDrops+1)
		}
		// The gap between the healthy connection's own Listen call and
		// the next one covers both how long it stayed open and the
		// backoff delay after it dropped; only the latter is under test.
		gap := listenTimes[quickDrops+1] - listenTimes[quickDrops] - healthyHold
		if gap >= cfg.BackoffMin {
			t.Fatalf("reconnect after a %v healthy connection waited %v, want under BackoffMin (%v): the escalated attempt counter was not reset", healthyHold, gap, cfg.BackoffMin)
		}
	})
}
