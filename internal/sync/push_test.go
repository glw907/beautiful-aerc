package sync

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
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
			if _, err := reconnect(context.Background(), push, cfg); err != nil {
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
	sorted := append([]time.Duration(nil), d...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	idx := (len(sorted)*95 + 99) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
