package ui

// SyncState is one of SY-5's four sync states, the status line's sync
// segment renders distinctly (design decision 7).
type SyncState int

// The four SY-5 states.
const (
	SyncStateSynced SyncState = iota
	SyncStateSyncing
	SyncStateOffline
	SyncStateBackingOff
)

// SyncStateMsg reports the sync engine's current condition to the
// status line. Done and Total carry the syncing state's own progress;
// Total zero means no total is known yet, and the segment shows Done
// alone rather than a stalled-looking blank (decision 7). Retry
// carries the backing-off state's countdown, in whole seconds
// ("retry 12s"); it is meaningless in every other state.
type SyncStateMsg struct {
	State       SyncState
	Done, Total int64
	Retry       int
}

// BackfillProgressMsg reports body-backfill progress, riding the sync
// segment alongside SyncStateMsg (decision 7: "bodies 18,204/36,102"
// takes over the segment while Active, so a rate-limited backfill
// still reads through the same backing-off warn state rather than
// stalling silently). Active false returns the segment to
// SyncStateMsg's own state.
type BackfillProgressMsg struct {
	Active      bool
	Done, Total int64
}

// OutboxCountMsg reports the outbox's current queued-intent count,
// shown beside the sync state whenever nonzero (decision 7).
type OutboxCountMsg struct {
	Queued int
}

// StoreChangedMsg tells every screen that a store fact it read at
// Init may now be stale, so it can re-issue its own load: the
// engines' one route back to a screen that already returned from
// Init, carrying no payload since every screen already knows what to
// reread.
type StoreChangedMsg struct{}
