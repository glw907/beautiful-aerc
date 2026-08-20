package main

import (
	"context"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	syncengine "github.com/glw907/poplar/internal/sync"
	"github.com/glw907/poplar/internal/ui"
)

// msgLog collects tea.Msg values sent through a bridge under a mutex,
// the headless runner's own stand-in for a running *tea.Program's
// Send: the bridge is exercised against a real engine, with nothing
// terminal-shaped anywhere in the loop.
type msgLog struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (l *msgLog) send(msg tea.Msg) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, msg)
}

func (l *msgLog) snapshot() []tea.Msg {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.msgs)
}

// TestBridgeSyncHealth_ProducesExactlyTheTransitionSequence is task
// 6's own bridge test, scripted through a full stop -> recover ->
// quiet sequence (task-6-findings-r1.md F1): a scripted engine-state
// transition sequence against a real sync.Worker produces exactly the
// ui.Msg sequence the status line expects, a steady state between
// transitions produces nothing at all (no polling), and a stream that
// recovers and proves itself clears backing-off with a further
// SyncStateMsg{State: SyncStateSynced}.
func TestBridgeSyncHealth_ProducesExactlyTheTransitionSequence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := storetest.Insert(t, w,
			`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`, "a", "jmap", "a@example.com")

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

		cfg := syncengine.DefaultConfig()
		cfg.CoalesceWindow = 50 * time.Millisecond
		cfg.BackoffMin = 100 * time.Millisecond
		cfg.BackoffMax = time.Second
		// InteractiveQuiet subordinates the bulk lane behind the
		// account row's own interactive-lane insert above
		// (ADR-0003 revision 2); shrunk so this test's own flush
		// cycle does not wait out the full production window.
		cfg.InteractiveQuiet = time.Millisecond
		worker := syncengine.NewWorker(accountID, &be, w, cfg)

		var log msgLog
		bridgeSyncHealth(worker, log.send)

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

		want := []tea.Msg{
			ui.SyncStateMsg{State: ui.SyncStateSyncing},
			ui.SyncStateMsg{State: ui.SyncStateSynced},
			ui.StoreChangedMsg{},
		}
		if got := log.snapshot(); !slices.Equal(got, want) {
			t.Fatalf("after one flush cycle, messages = %#v, want %#v", got, want)
		}

		// A quiet stretch under BackoffMax, stream still open: no
		// further message (the stream has not yet proved itself, and
		// nothing here polls).
		time.Sleep(cfg.BackoffMax / 2)
		synctest.Wait()
		if got := log.snapshot(); len(got) != len(want) {
			t.Fatalf("a quiet stretch under BackoffMax produced %d message(s), want the same %d as after the flush cycle", len(got), len(want))
		}

		close(first) // the stream stops for good
		synctest.Wait()

		afterStop := log.snapshot()
		if len(afterStop) != len(want)+1 {
			t.Fatalf("after the stream stopped, messages = %#v, want exactly one more than %#v", afterStop, want)
		}
		state, ok := afterStop[len(want)].(ui.SyncStateMsg)
		if !ok || state.State != ui.SyncStateBackingOff || state.Retry <= 0 {
			t.Fatalf("first message after the stop = %#v, want a ui.SyncStateMsg{State: SyncStateBackingOff} with a positive Retry", afterStop[len(want)])
		}

		// The backoff wait ends, second's own stream opens and stays
		// open past BackoffMax: proved fires, clearing backing-off.
		time.Sleep(2*cfg.BackoffMax + cfg.BackoffMin)
		synctest.Wait()

		afterRecover := log.snapshot()
		wantRecovered := []tea.Msg{ui.SyncStateMsg{State: ui.SyncStateSynced}, ui.StoreChangedMsg{}}
		if got := afterRecover[len(afterStop):]; !slices.Equal(got, wantRecovered) {
			t.Fatalf("after the second stream proved itself, the appended messages = %#v, want %#v", got, wantRecovered)
		}

		// Quiet again: no further message.
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if got := log.snapshot(); len(got) != len(afterRecover) {
			t.Fatalf("a quiet stretch after recovery produced %d message(s), want the same %d", len(got), len(afterRecover))
		}

		cancel()
		<-done
	})
}

// TestBridgeOutboxCount_SendsOnlyOnChange proves the outbox half of
// the bridge: an unchanged count sends nothing, and a changed count
// sends exactly one OutboxCountMsg carrying the new value.
func TestBridgeOutboxCount_SendsOnlyOnChange(t *testing.T) {
	w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
	storetest.Insert(t, w, `INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`, "a", "jmap", "a@example.com")

	ctx := context.Background()
	var log msgLog

	last := bridgeOutboxCount(ctx, reads, 0, log.send)
	if last != 0 || len(log.snapshot()) != 0 {
		t.Fatalf("bridgeOutboxCount on an empty outbox sent %#v, want nothing", log.snapshot())
	}

	storetest.Insert(t, w, `INSERT INTO outbox (account_id, kind, payload, state, created_at) VALUES (1, 'send', '{}', 'queued', 1000)`)

	last = bridgeOutboxCount(ctx, reads, last, log.send)
	if last != 1 {
		t.Fatalf("bridgeOutboxCount() = %d, want 1", last)
	}
	want := []tea.Msg{ui.OutboxCountMsg{Queued: 1}}
	if got := log.snapshot(); !slices.Equal(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}

	// An unchanged count on the next call sends nothing further.
	last = bridgeOutboxCount(ctx, reads, last, log.send)
	if last != 1 || !slices.Equal(log.snapshot(), want) {
		t.Fatalf("an unchanged count sent an extra message: %#v", log.snapshot())
	}
}
