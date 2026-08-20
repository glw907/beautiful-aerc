package sync

import (
	"context"
	"time"

	"github.com/glw907/poplar/internal/backend"
)

// State is Worker's own high-level condition (SY-5): the value the
// UI's status line renders. Worker reports only what its own push/poll
// loop can observe; Offline, the fourth SY-5 state, names a connection
// that was never established at all and is a cmd/poplar concern (its
// own connect-retry path), never Worker's.
type State int

// The three states Worker's own loop can report.
const (
	StateSynced State = iota
	StateSyncing
	StateBackingOff
)

// Health is one State transition Worker reports to its Observer.
// Retry is meaningful only when State is StateBackingOff: the delay
// before RunPush's next reconnect attempt.
type Health struct {
	State State
	Retry time.Duration
}

// Observer receives one Health value per state transition: a flush
// cycle starting or ending, or a reopen wait about to begin, driven
// entirely by RunPush's and pollKinds' own existing control flow.
// Nothing calls it on a timer, so a caller that never sees a Health
// value has nothing to poll for either.
type Observer func(Health)

// SetObserver installs o as w's own Observer, replacing whatever was
// set before. A nil Observer, the zero value every Worker starts
// with, disables reporting: w.emit and w.emitBackoff no-op rather
// than requiring every call site to check first. SetObserver is not
// safe to call concurrently with RunPush or pollKinds: a caller sets
// it once, before starting either, the same one-time wiring
// NewWorker's own caller already does for every other field.
func (w *Worker) SetObserver(o Observer) {
	w.observer = o
}

// emit reports h to w's Observer, if one is set.
func (w *Worker) emit(h Health) {
	if w.observer != nil {
		w.observer(h)
	}
}

// emitBackoff reports a StateBackingOff transition carrying d, the
// delay RunPush or reconnect is about to sleep: sleepBackoff's own
// onWait callback, so the reported duration is the exact one about to
// run rather than a second, independently jittered computation of it.
func (w *Worker) emitBackoff(d time.Duration) {
	w.emit(Health{State: StateBackingOff, Retry: d})
}

// runFlush wraps flush (SyncKind for each of kinds) with a Syncing
// transition before it runs, and a Synced transition after only when
// state.failing is empty once it returns: the one place both
// RunPush's push-triggered cycles and pollKinds' fallback cycles
// report progress through. A cycle that leaves any kind still failing
// emits nothing further this pass rather than a dishonest Synced
// (task-6-findings-r1.md F3); state.report has already surfaced the
// failure through uerr, and task 8's banner is the fuller channel a
// later pass gives it in the status line itself.
func (w *Worker) runFlush(ctx context.Context, kinds []backend.ObjectKind, state *syncFlushState) {
	w.emit(Health{State: StateSyncing})
	w.flush(ctx, kinds, state)
	if len(state.failing) == 0 {
		w.emit(Health{State: StateSynced})
	}
}
