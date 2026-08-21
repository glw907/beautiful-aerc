package main

import (
	"context"
	"log/slog"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/store"
	syncengine "github.com/glw907/poplar/internal/sync"
	"github.com/glw907/poplar/internal/ui"
)

// bridgeSyncHealth installs an Observer on worker translating every
// sync.Health transition into the ui message the status line renders,
// through send: task 11's own whole answer to "cmd/poplar grows a
// bridge goroutine" for the sync half (the engine's own goroutine
// already drives the transitions; nothing here polls). A Synced
// transition also sends a StoreChangedMsg, since a flush cycle
// finishing is the store's own signal that a placeholder's stale
// count is worth rereading. That send fires on every cycle that
// reaches Synced, whether or not it changed anything a placeholder
// reads; narrowing it to real changes is carried to pass 3
// (task-6-findings-r1.md's deferred list).
//
// Sync's own State carries no progress counts yet (SyncKind pages
// Changes without tracking a running total), so the SyncStateMsg this
// sends always reports Done and Total zero; a later pass that adds
// that tracking to sync.Health needs no change here to reach the
// status line.
func bridgeSyncHealth(worker *syncengine.Worker, send func(tea.Msg)) {
	worker.SetObserver(func(h syncengine.Health) {
		send(ui.SyncStateMsg{State: bridgeSyncState(h.State), Retry: bridgeRetrySeconds(h.Retry)})
		if h.State == syncengine.StateSynced {
			send(ui.StoreChangedMsg{})
		}
	})
}

// bridgeSyncState maps sync.State to its ui.SyncState counterpart.
// sync.StateOffline does not exist: Worker's own loop always retries
// through backoff rather than giving up, so it never reaches a state
// "offline" would describe; that state belongs to run's own
// connect-retry path instead (ST-2).
func bridgeSyncState(s syncengine.State) ui.SyncState {
	switch s {
	case syncengine.StateSyncing:
		return ui.SyncStateSyncing
	case syncengine.StateBackingOff:
		return ui.SyncStateBackingOff
	default:
		return ui.SyncStateSynced
	}
}

// bridgeRetrySeconds rounds d up to whole seconds: a sub-second retry
// still reports "1s" rather than the misleadingly-immediate "0s"
// (SY-5's backing-off state carries a visible countdown).
func bridgeRetrySeconds(d time.Duration) int {
	return int(math.Ceil(d.Seconds()))
}

// bridgeOutboxCount reads reads' current queued-outbox count and, if
// it differs from last, sends the change through send: the outbox
// half of the engine-state bridge. It is meant to be called once per
// tick of the existing dispatch loop (task 11's own wiring), riding
// that loop's cadence rather than a poll of its own, and returns the
// count observed so the caller's next call has a last to diff
// against. A read failure logs and reports last unchanged, so one bad
// read never sends a wrong count.
func bridgeOutboxCount(ctx context.Context, reads *store.ReadPool, last int, send func(tea.Msg)) int {
	n, err := reads.OutboxQueuedCount(ctx)
	if err != nil {
		slog.Warn("bridge: read outbox queued count", "error", err)
		return last
	}
	if n != last {
		send(ui.OutboxCountMsg{Queued: n})
	}
	return n
}
