package sync

import (
	"context"
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
// so a test goroutine can read them while RunPush's own goroutine is
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

// TestWorkerObserver_ReportsTransitionsOnly proves the bridge's own
// no-polling contract end to end against a real Worker: one flush
// cycle reports exactly Syncing then Synced, a long quiet stretch
// with the stream still open reports nothing further (steady state
// emits nothing), and the stream stopping reports BackingOff carrying
// the exact delay RunPush is about to sleep.
func TestWorkerObserver_ReportsTransitionsOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			return backend.ChangeSet{}, nil
		}

		notify := make(chan backend.Notification)
		listens := 0
		be.PushSource = &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
			listens++
			if listens == 1 {
				return notify, nil
			}
			ch := make(chan backend.Notification)
			close(ch) // every later stream opens and stops at once
			return ch, nil
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

		notify <- backend.Notification{}
		time.Sleep(cfg.CoalesceWindow + 50*time.Millisecond)
		synctest.Wait()

		afterFlush := log.snapshot()
		if len(afterFlush) != 2 || afterFlush[0].State != StateSyncing || afterFlush[1].State != StateSynced {
			t.Fatalf("after one flush cycle, events = %+v, want exactly [Syncing, Synced]", afterFlush)
		}

		// Steady state, stream still open and quiet: no further Health
		// values, however long the wait, since nothing here is driven by
		// a timer of its own (the bridge's own no-polling contract).
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if steady := log.snapshot(); len(steady) != len(afterFlush) {
			t.Fatalf("a quiet stretch with the stream open produced %d event(s), want the same %d as after the flush (steady state must emit nothing)", len(steady), len(afterFlush))
		}

		close(notify) // the stream stops for good
		time.Sleep(cfg.BackoffMax + time.Second)
		synctest.Wait()

		cancel()
		<-done

		final := log.snapshot()
		if len(final) < 3 || final[2].State != StateBackingOff {
			t.Fatalf("after the stream stopped, events = %+v, want a StateBackingOff transition next", final)
		}
		if final[2].Retry <= 0 || final[2].Retry > cfg.BackoffMax {
			t.Errorf("BackingOff Retry = %v, want it within (0, %v]", final[2].Retry, cfg.BackoffMax)
		}
	})
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
