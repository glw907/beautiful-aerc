package sync

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// observerLog collects Health values from an Observer under a mutex,
// so a test goroutine can read them while RunPush's goroutine is
// still delivering to it.
type observerLog struct {
	mu   sync.Mutex
	logs []Health
}

func (o *observerLog) observe(h Health) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.logs = append(o.logs, h)
}

func (o *observerLog) snapshot() []Health {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.logs)
}

// TestWorkerObserver_ReportsTransitionsOnly proves the bridge's
// no-polling contract end to end against a real Worker, scripted
// through a full stop -> recover -> quiet sequence
// (task-6-findings-r1.md F1): one flush cycle reports exactly Syncing
// then Synced; a quiet stretch under BackoffMax with the stream still
// open reports nothing further (steady state emits nothing); the
// stream stopping reports BackingOff carrying the exact delay RunPush
// is about to sleep; the next stream staying open long enough to
// prove itself reports Synced again, clearing backing-off; and a
// quiet stretch after that recovery again reports nothing.
func TestWorkerObserver_ReportsTransitionsOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			return backend.ChangeSet{}, nil
		}

		first := make(chan backend.Notification)
		second := make(chan backend.Notification)
		listens := 0
		be.PushSource = &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
			listens++
			if listens == 1 {
				return first, nil
			}
			return second, nil // stays open for the rest of the test, long enough to prove itself
		}}

		cfg := testConfig()
		cfg.BackoffMin = 100 * time.Millisecond
		cfg.BackoffMax = time.Second
		worker := NewWorker(accountID, &be, w, cfg)

		var log observerLog
		worker.SetObserver(log.observe)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()
		synctest.Wait()

		first <- backend.Notification{}
		time.Sleep(cfg.CoalesceWindow + 50*time.Millisecond)
		synctest.Wait()

		afterFlush := log.snapshot()
		if len(afterFlush) != 2 || afterFlush[0].State != StateSyncing || afterFlush[1].State != StateSynced {
			t.Fatalf("after one flush cycle, events = %+v, want exactly [Syncing, Synced]", afterFlush)
		}

		// A quiet stretch under BackoffMax, stream still open: nothing
		// further, since the stream has not yet proved itself and
		// nothing here is driven by a separate timer.
		time.Sleep(cfg.BackoffMax / 2)
		synctest.Wait()
		if steady := log.snapshot(); len(steady) != len(afterFlush) {
			t.Fatalf("a quiet stretch under BackoffMax produced %d event(s), want the same %d as after the flush (steady state must emit nothing)", len(steady), len(afterFlush))
		}

		close(first) // the stream stops for good
		synctest.Wait()

		afterStop := log.snapshot()
		if len(afterStop) != 3 || afterStop[2].State != StateBackingOff {
			t.Fatalf("after the stream stopped, events = %+v, want exactly one StateBackingOff transition appended", afterStop)
		}
		if afterStop[2].Retry <= 0 || afterStop[2].Retry > cfg.BackoffMax {
			t.Errorf("BackingOff Retry = %v, want it within (0, %v]", afterStop[2].Retry, cfg.BackoffMax)
		}

		// The backoff wait ends (bounded by BackoffMax), second's
		// stream opens, and stays open past BackoffMax: proved fires,
		// clearing backing-off.
		time.Sleep(2*cfg.BackoffMax + cfg.BackoffMin)
		synctest.Wait()

		afterRecover := log.snapshot()
		if len(afterRecover) != 4 || afterRecover[3].State != StateSynced {
			t.Fatalf("after the second stream proved itself, events = %+v, want exactly one StateSynced transition appended", afterRecover)
		}

		// Quiet again, second's stream still open: nothing further.
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if final := log.snapshot(); len(final) != len(afterRecover) {
			t.Fatalf("a quiet stretch after recovery produced %d event(s), want the same %d", len(final), len(afterRecover))
		}

		cancel()
		<-done
	})
}

// TestRunFlush_FailedCycleEmitsNoSynced proves F3: a flush cycle that
// leaves a kind still failing emits Syncing but never a dishonest
// Synced after it.
func TestRunFlush_FailedCycleEmitsNoSynced(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{}, errors.New("boom")
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	var log observerLog
	worker.SetObserver(log.observe)

	worker.runFlush(context.Background(), []backend.ObjectKind{backend.ObjectKindMessage}, newSyncFlushState())

	got := log.snapshot()
	if len(got) != 1 || got[0].State != StateSyncing {
		t.Fatalf("a failed cycle logged %+v, want exactly [Syncing] (no Synced, an honest read of the failure)", got)
	}
}

// TestWorkerObserver_NilByDefault proves a Worker with no Observer set
// runs its flush cycle exactly as before (SetObserver is opt-in, never
// a behavior change on its own).
func TestWorkerObserver_NilByDefault(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		calls := 0
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			calls++
			return backend.ChangeSet{}, nil
		}

		worker := NewWorker(accountID, &be, w, testConfig())

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			worker.pollKinds(ctx, []backend.ObjectKind{backend.ObjectKindMessage})
			close(done)
		}()
		synctest.Wait()

		cancel()
		<-done

		if calls == 0 {
			t.Fatal("pollKinds with a nil Observer never called SyncKind, want the ordinary flush cycle to still run")
		}
	})
}
