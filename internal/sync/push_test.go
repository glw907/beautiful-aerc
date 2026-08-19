package sync

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
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
			cfg := testConfig()
			cfg.CoalesceWindow = window
			consumePush(ctx, ch, cfg, flush, func() {})
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

// TestConsumePushHasNoStallDetector asserts silence on the
// notification channel is never read as a drop, however long it lasts.
// The transport owns the liveness check now, against the cadence the
// server actually granted (JT-26); a second detector here, reading a
// figure from local config, tears down connections the transport
// considers healthy, which defeats JT-26 in production while its own
// test still passes.
func TestConsumePushHasNoStallDetector(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ch := make(chan backend.Notification)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result := make(chan bool, 1)
		go func() {
			result <- consumePush(ctx, ch, testConfig(), func() {}, func() {})
		}()

		// Past any window a poll- or ping-derived detector could have
		// been set from: DefaultConfig's old figure was 30s, doubled.
		time.Sleep(10 * time.Minute)
		synctest.Wait()

		select {
		case <-result:
			t.Fatal("consumePush reported a drop on a silent but open channel")
		default:
		}

		// The one thing that still counts: the channel closing.
		close(ch)
		synctest.Wait()
		select {
		case dropped := <-result:
			if !dropped {
				t.Fatal("consumePush() = false on a closed channel, want true")
			}
		default:
			t.Fatal("consumePush did not return after the channel closed")
		}
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
			if _, err := reconnect(context.Background(), push, cfg, &pushState{}, nil, nil); err != nil {
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

// TestRunPushBackoffsAfterAnImmediateStop asserts a transport that
// opens successfully and then stops at once still costs a backoff step
// before RunPush opens it again: without one, such a transport is
// reopened at zero delay and spins the loop into an unbounded request
// storm.
func TestRunPushBackoffsAfterAnImmediateStop(t *testing.T) {
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
			close(ch) // opens, then stops at once
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

// TestRunPushPollsWhileTheStreamStaysRefused is SY-2's MUST against
// the two failures that make it matter, and against the case it must
// not fire on. Both failures leave the worker holding no stream while
// the JMAP API answers normally, and nothing but this pulls Changes
// while push is down, so without it the account receives no mail for
// the life of the process.
//
// A server refusing every connection is the 401-on-EventSource case
// ADR-0005 revision 2 named as its own risk. A server opening a stream
// and dropping it at once is the same outage with no error in it: every
// Listen succeeds, so no failure is ever reported, and the run is
// silent as well as dry unless the stops themselves are counted.
//
// The control is the other half of the rule: a stream that opens and
// stays open must never poll, however quiet it is, because the
// transport owns liveness there.
func TestRunPushPollsWhileTheStreamStaysRefused(t *testing.T) {
	// Long enough that a fallback pulling on PollInterval's cadence and
	// one that never pulls at all cannot be confused.
	const window = 30 * time.Minute

	runPush := func(t *testing.T, listen func(context.Context) (<-chan backend.Notification, error)) (int64, *uerrtest.Buffer) {
		t.Helper()

		buf := uerrtest.Capture(t)
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var calls atomic.Int64
		var be backendtest.Fake
		be.PushSource = &backendtest.FakePush{ListenFunc: listen}
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			calls.Add(1)
			return backend.ChangeSet{}, nil
		}

		worker := NewWorker(accountID, &be, w, testConfig())
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()

		time.Sleep(window)
		synctest.Wait()
		cancel()
		<-done
		return calls.Load(), buf
	}

	// One pull per tick, less whatever the last, partly elapsed interval
	// owed. A pull is only ever late, never skipped: a tick that lands
	// while a Listen call is in flight waits out that call rather than
	// being dropped.
	ticks := int64(window / DefaultConfig().PollInterval)
	wantPolls := func(t *testing.T, calls int64) {
		t.Helper()
		if calls < ticks-2 || calls > ticks {
			t.Errorf("Changes calls = %d over %v with no stream in force, want about %d (one per %v)",
				calls, window, ticks, DefaultConfig().PollInterval)
		}
	}

	t.Run("a refused stream falls back to polling", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls, buf := runPush(t, func(context.Context) (<-chan backend.Notification, error) {
				return nil, backend.Failure{Class: uerr.ClassAuth, Cause: errors.New("event source: 401")}
			})
			wantPolls(t, calls)

			// Thousands of refused Listen calls over the window, and the
			// user is owed one account of the outage rather than one per
			// attempt (episode dedup, ADR-0013 revision 2).
			lines := uerrtest.Lines(t, buf)
			if len(lines) != 1 {
				t.Fatalf("uerr lines = %d over %v of a refused stream, want exactly 1", len(lines), window)
			}
			if got := lines[0]["class"]; got != "auth" {
				t.Errorf("class = %v, want auth", got)
			}
		})
	})

	t.Run("a stream that opens and stops at once falls back to polling", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls, buf := runPush(t, func(context.Context) (<-chan backend.Notification, error) {
				ch := make(chan backend.Notification)
				close(ch) // opens, then stops at once, saying nothing about why
				return ch, nil
			})
			wantPolls(t, calls)

			// Thousands of streams over the window, and the user is owed
			// one account of the outage rather than one per stream.
			lines := uerrtest.Lines(t, buf)
			if len(lines) != 1 {
				t.Fatalf("uerr lines = %d over %v of streams that never delivered, want exactly 1", len(lines), window)
			}
			if got := lines[0]["class"]; got != "connection" {
				t.Errorf("class = %v, want connection", got)
			}
		})
	})

	t.Run("an open stream never polls", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls, buf := runPush(t, func(context.Context) (<-chan backend.Notification, error) {
				return make(chan backend.Notification), nil
			})
			if calls != 0 {
				t.Errorf("Changes calls = %d over %v with the stream open and quiet, want none: the transport owns liveness while it holds one", calls, window)
			}
			if lines := uerrtest.Lines(t, buf); len(lines) != 0 {
				t.Errorf("uerr lines = %v over a stream that was up the whole time, want none", lines)
			}
		})
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

// TestClassifyErr asserts classifyErr's three paths: an error never
// classified upstream defaults to ClassConnection, and a uerr.Error or
// a backend.Failure each yield their own class and their original
// root cause (not the fixed per-class sentence Error() returns). The
// backend.Failure row is what keeps a credential the server rejects on the
// push transport from reaching the user as a connectivity problem,
// which is the one push failure waiting cannot fix.
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

	refused := backend.Failure{Class: uerr.ClassAuth, Cause: root}
	if class, cause := classifyErr(refused); class != uerr.ClassAuth || cause != root {
		t.Fatalf("classifyErr(Failure) = (%v, %v), want (ClassAuth, root)", class, cause)
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

	ch, err := reconnect(ctx, push, DefaultConfig(), &pushState{}, nil, nil)
	if ch != nil {
		t.Fatal("reconnect returned a non-nil channel alongside an error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("reconnect() error = %v, want context.Canceled", err)
	}
}

// TestPushStateIgnoresContextCanceled is the listen half of
// TestSyncFlushStateIgnoresContextCanceled, over the guard that keeps
// a clean shutdown from reading as a connectivity failure. The
// transport reports ctx ending as a Listen error like any other, and
// reconnect hands every Listen error to pushState.fail, whose
// classification defaults to ClassConnection: without the guard every
// Ctrl-C writes an error-level line telling the user poplar could not
// reach the server.
func TestPushStateIgnoresContextCanceled(t *testing.T) {
	buf := uerrtest.Capture(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	push := &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
		// The shape jmapsource reports a shutdown-time stop in: wrapped,
		// and left unclassified because a cancellation is nobody's
		// failure.
		return nil, fmt.Errorf("jmapsource: push: %w", context.Canceled)
	}}

	state := &pushState{}
	if _, err := reconnect(ctx, push, DefaultConfig(), state, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("reconnect() error = %v, want context.Canceled", err)
	}
	if state.failing {
		t.Error("the worker recorded a failure episode for its own shutdown")
	}
	if lines := uerrtest.Lines(t, buf); len(lines) != 0 {
		t.Errorf("uerr lines = %v, want none: shutdown is not a server problem", lines)
	}
}

// TestReconnectSurfacesARefusalOnceUnderItsOwnClass asserts the two
// halves of what a backend-classified Listen refusal owes the user: it
// reaches the log under the class the backend assigned it, not under
// reconnect's ClassConnection default, and it reaches it once across a
// standing run of refusals rather than once per attempt (ADR-0013
// revision 2). A rejected credential is the case both halves are for.
// Nothing else pulls Changes while push is refused, so the class this
// line carries is the only account the user gets of why mail stopped,
// and one line per attempt is thousands a day for one standing
// failure.
func TestReconnectSurfacesARefusalOnceUnderItsOwnClass(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		buf := uerrtest.Capture(t)

		rejected := errors.New("event source: 401")
		var calls atomic.Int64
		push := &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
			calls.Add(1)
			return nil, backend.Failure{Class: uerr.ClassAuth, Cause: rejected}
		}}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			_, _ = reconnect(ctx, push, testConfig(), &pushState{}, nil, nil)
			close(done)
		}()

		time.Sleep(5 * time.Minute)
		synctest.Wait()
		cancel()
		<-done

		if got := calls.Load(); got < 5 {
			t.Fatalf("Listen called %d times, want at least 5: the dedup claim needs a run of refusals behind it", got)
		}
		lines := uerrtest.Lines(t, buf)
		if len(lines) != 1 {
			t.Fatalf("uerr lines = %d over %d refused attempts, want exactly 1", len(lines), calls.Load())
		}
		if got := lines[0]["class"]; got != "auth" {
			t.Errorf("class = %v, want auth: the backend's own classification was dropped", got)
		}
		if got := lines[0]["cause"]; got != rejected.Error() {
			t.Errorf("cause = %v, want %q", got, rejected)
		}
	})
}

// TestPushStateStoppedAdvancesUntilProved asserts pushState.stopped's
// escalation curve directly: a stream that stops before ever proving
// itself advances attempt the same way reconnect advances it after a
// Listen failure, and proved is the only reset. The stop that ends the
// very stream proved just fired for must not advance attempt again,
// since otherwise a healthy stream's own routine close would still
// nudge the schedule up, and the next reopen would draw from a higher
// band than a stream that just proved the server is serving should
// leave behind.
func TestPushStateStoppedAdvancesUntilProved(t *testing.T) {
	_ = uerrtest.Capture(t)
	_ = uerrtest.CaptureDefault(t)

	state := pushState{}
	state.stopped()
	if state.attempt != 1 {
		t.Fatalf("attempt after one unproved stop = %d, want 1", state.attempt)
	}
	state.stopped()
	if state.attempt != 2 {
		t.Fatalf("attempt after two unproved stops = %d, want 2", state.attempt)
	}

	state.proved()
	if state.attempt != 0 {
		t.Fatalf("attempt after proved = %d, want 0", state.attempt)
	}

	// The stop that ends the stream proved just fired for consumes that
	// mark rather than advancing the schedule again.
	state.stopped()
	if state.attempt != 0 {
		t.Fatalf("attempt after the proved stream's own stop = %d, want 0: a stop right after proving inherited escalation", state.attempt)
	}
	if state.unproved != 0 {
		t.Fatalf("unproved after the proved stream's own stop = %d, want 0", state.unproved)
	}

	// The next stream, which never proved, resumes counting from zero.
	state.stopped()
	if state.attempt != 1 {
		t.Fatalf("attempt after the following unproved stop = %d, want 1", state.attempt)
	}
}

// TestRunPushEscalatesOnAnUnprovedStop asserts RunPush's own wait
// against a stream that stops before ever proving itself rides
// pushState's escalating schedule, the same curve reconnect runs
// against a Listen failure, rather than reopening at the BackoffMin
// floor forever.
func TestRunPushEscalatesOnAnUnprovedStop(t *testing.T) {
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
			close(ch) // opens, then stops at once, saying nothing about why
			return ch, nil
		}}

		cfg := testConfig()
		cfg.BackoffMin = 100 * time.Millisecond
		cfg.BackoffMax = 10 * time.Second
		worker := NewWorker(accountID, &be, w, cfg)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()

		time.Sleep(20 * time.Second)
		synctest.Wait()
		cancel()
		<-done

		// A floor pinned at BackoffMin draws on the order of 200 calls
		// over 20s; an escalating schedule reaches BackoffMax within a
		// handful of stops and settles near one call every ~BackoffMax,
		// well under 40 over the same window.
		if got := len(listenTimes); got > 40 {
			t.Fatalf("Listen called %d times over 20s, want well under 40: the wait did not escalate", got)
		}
		if got := len(listenTimes); got < 4 {
			t.Fatalf("Listen called %d times, want several: the test needs a run of unproved stops behind it", got)
		}

		// The maximum gap must clear BackoffMin. Pinning the *last* gap
		// instead flakes: full jitter draws each gap independently and
		// uniformly, so once the schedule saturates near BackoffMax, about
		// 1% of runs still draw under 100ms for that one gap even though
		// the schedule escalated correctly throughout.
		maxGap := time.Duration(0)
		for i := 1; i < len(listenTimes); i++ {
			if gap := listenTimes[i] - listenTimes[i-1]; gap > maxGap {
				maxGap = gap
			}
		}
		if maxGap < cfg.BackoffMin {
			t.Fatalf("max gap between Listen calls = %v, want at least BackoffMin (%v): the schedule stayed at the floor", maxGap, cfg.BackoffMin)
		}

		// Each gap is also bounded above by the schedule's own curve:
		// full jitter draws it uniformly under min(BackoffMin<<i,
		// BackoffMax) for the i-th gap, a hard ceiling a schedule that
		// jumped straight to BackoffMax would blow on the very first one.
		bound := cfg.BackoffMin
		for i := 1; i < len(listenTimes); i++ {
			bound = min(bound*2, cfg.BackoffMax)
			if gap := listenTimes[i] - listenTimes[i-1]; gap >= bound {
				t.Fatalf("gap %d between Listen calls = %v, want under %v (the schedule's own bound at that attempt)", i, gap, bound)
			}
		}
	})
}

// TestRunPushEscalatesOnAConnectFlushDeath is the shape a real
// transport actually produces, and the one a delivery-keyed schedule
// misses entirely (BACKLOG #65): jmapsource's OnConnect always queues
// one notification before Listen returns (ADR-0018's flush-on-connect),
// so a stream that opens and dies at once still arrives with a
// notification already buffered on the channel. Keying the schedule on
// delivery reads that connect flush as proof of health and resets
// attempt on every single stop, leaving the reopen rate pinned at the
// floor as if pushState.stopped never ran at all. Proving has to be
// the bar instead, and the run still has to surface: no other report
// covers a server that never refuses a connection, only opens and
// drops it.
func TestRunPushEscalatesOnAConnectFlushDeath(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		buf := uerrtest.Capture(t)
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
			// The ADR-0018 shape: OnConnect's flush is already queued by
			// the time Listen returns, and the stream dies right after.
			ch := make(chan backend.Notification, 1)
			ch <- backend.Notification{}
			close(ch)
			return ch, nil
		}}

		cfg := testConfig()
		cfg.BackoffMin = 100 * time.Millisecond
		cfg.BackoffMax = 10 * time.Second
		worker := NewWorker(accountID, &be, w, cfg)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()

		time.Sleep(20 * time.Second)
		synctest.Wait()
		cancel()
		<-done

		if got := len(listenTimes); got > 40 {
			t.Fatalf("Listen called %d times over 20s, want well under 40: the connect flush was read as health and the schedule never escalated", got)
		}
		if got := len(listenTimes); got < 4 {
			t.Fatalf("Listen called %d times, want several: the test needs a run of connect-flush-then-die streams behind it", got)
		}

		lines := uerrtest.Lines(t, buf)
		if len(lines) != 1 {
			t.Fatalf("uerr lines = %d over %d Listen calls, want exactly 1: the run of stops never surfaced", len(lines), len(listenTimes))
		}
		if got := lines[0]["class"]; got != "connection" {
			t.Errorf("class = %v, want connection", got)
		}
	})
}

// TestConsumePushFlushesAWindowTheStopInterrupted covers the
// notification already taken off the channel when the transport stops.
// The coalescing window holds it for 200ms, and ADR-0018's
// flush-on-connect rests on exactly the notification that arrives just
// before a stream ends, so discarding it loses the /changes pull that
// connection owed.
func TestConsumePushFlushesAWindowTheStopInterrupted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ch := make(chan backend.Notification, 1)
		ch <- backend.Notification{}
		close(ch)

		flushes := 0
		if !consumePush(context.Background(), ch, testConfig(), func() { flushes++ }, func() {}) {
			t.Fatal("consumePush() = false on a closed channel, want true")
		}
		if flushes != 1 {
			t.Fatalf("flushes = %d, want 1: the notification the stop interrupted was dropped", flushes)
		}
	})
}

// TestRunPushSlowsAndSurfacesAServerRefusingEveryOtherStream is the
// shape that gets past a per-call schedule and a per-call dedup: a
// server that opens a stream, drops it, and refuses the reconnection.
// The transport reports that refusal as the next Listen's error, so
// every cycle is one failure, and a schedule or a surfacing decision
// that started over each time would neither slow the loop down nor say
// anything. Measured against the real adapter before this held, twenty
// seconds of it drew about a hundred requests and wrote no log line at
// all.
func TestRunPushSlowsAndSurfacesAServerRefusingEveryOtherStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		buf := uerrtest.Capture(t)
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			return backend.ChangeSet{}, nil
		}

		refused := errors.New("event source: 401")
		calls := 0
		be.PushSource = &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
			calls++
			if calls%2 == 0 {
				// The refusal that ended the stream the previous call
				// opened, which is how the transport reports one.
				return nil, backend.Failure{Class: uerr.ClassAuth, Cause: refused}
			}
			ch := make(chan backend.Notification)
			close(ch)
			return ch, nil
		}}

		cfg := testConfig()
		cfg.BackoffMin = 100 * time.Millisecond
		cfg.BackoffMax = 10 * time.Second
		worker := NewWorker(accountID, &be, w, cfg)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()

		time.Sleep(20 * time.Second)
		synctest.Wait()
		cancel()
		<-done

		// A schedule that never escalated would run a cycle per
		// BackoffMin, which is 400 calls over this window; an escalating
		// one reaches BackoffMax within about nine.
		if calls > 40 {
			t.Errorf("Listen called %d times over 20s, want well under 40: the schedule did not escalate", calls)
		}
		if calls < 4 {
			t.Fatalf("Listen called %d times, want several: the test needs a run of refusals behind it", calls)
		}
		lines := uerrtest.Lines(t, buf)
		if len(lines) != 1 {
			t.Fatalf("uerr lines = %d over %d Listen calls, want exactly 1", len(lines), calls)
		}
		if got := lines[0]["class"]; got != "auth" {
			t.Errorf("class = %v, want auth", got)
		}
	})
}

// TestPushStateProvedEndsTheFailureRun covers what says a run of
// refusals is over, and what that has to unblock. Nothing else can say
// it: a stream that opens proves nothing against a server that opens
// one and refuses the next, so only a stream that stays open long
// enough to be worth having does. And until something says it, the
// next failure of that same class is silent, which is the worse half.
func TestPushStateProvedEndsTheFailureRun(t *testing.T) {
	logs := uerrtest.CaptureDefault(t)
	buf := uerrtest.Capture(t)

	state := pushState{}
	state.attempt = 7
	state.fail(backend.Failure{Class: uerr.ClassAuth, Cause: errConnectionRefused})
	state.fail(backend.Failure{Class: uerr.ClassAuth, Cause: errConnectionRefused})
	if got := len(uerrtest.Lines(t, buf)); got != 1 {
		t.Fatalf("uerr lines across two same-class failures = %d, want 1", got)
	}

	state.proved()

	if state.attempt != 0 {
		t.Errorf("attempt = %d, want 0: the reopen schedule did not start over", state.attempt)
	}
	if !strings.Contains(logs.String(), "push recovered") {
		t.Errorf("no recovery line for a failure run that ended; log was:\n%s", logs.String())
	}

	// The half that matters more: a failure of the same class after the
	// recovery is news again, not a repeat of an episode long over.
	state.fail(backend.Failure{Class: uerr.ClassAuth, Cause: errConnectionRefused})
	if got := len(uerrtest.Lines(t, buf)); got != 2 {
		t.Errorf("uerr lines after a failure following a recovery = %d, want 2: the new failure was swallowed as a repeat", got)
	}

	// And a proving with nothing left to recover from says nothing: the
	// timer behind it fires once per stream, so a healthy transport
	// must not narrate one.
	state.proved()
	logs2 := uerrtest.CaptureDefault(t)
	state.proved()
	if strings.Contains(logs2.String(), "push recovered") {
		t.Error("a healthy stream logged a recovery with no failure run behind it")
	}
}

// TestRunPushRecoversWhileTheStreamIsStillUp is the same claim on the
// production path, and it is about when: the stream that proves the
// server is serving again is normally the one still running, so
// waiting for it to end would leave a failure as the last word on a
// transport that has been working for days.
func TestRunPushRecoversWhileTheStreamIsStillUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_ = uerrtest.Capture(t) // the refusals surface; this test is about what follows
		logs := uerrtest.CaptureDefault(t)
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			return backend.ChangeSet{}, nil
		}

		cfg := testConfig()
		cfg.BackoffMin = 100 * time.Millisecond
		cfg.BackoffMax = time.Second

		calls := 0
		be.PushSource = &backendtest.FakePush{ListenFunc: func(ctx context.Context) (<-chan backend.Notification, error) {
			calls++
			if calls <= 3 {
				return nil, backend.Failure{Class: uerr.ClassServer, Cause: errConnectionRefused}
			}
			// The stream that proves the server is serving again, and it
			// never ends on its own.
			ch := make(chan backend.Notification)
			context.AfterFunc(ctx, func() { close(ch) })
			return ch, nil
		}}

		worker := NewWorker(accountID, &be, w, cfg)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()

		time.Sleep(5 * time.Second)
		synctest.Wait()

		// Read the log with the stream still open.
		if !strings.Contains(logs.String(), "push recovered") {
			t.Errorf("no recovery line while a stream had been open past BackoffMax; log was:\n%s", logs.String())
		}

		cancel()
		<-done
	})
}
