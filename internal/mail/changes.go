package mail

import (
	"context"
	"errors"
)

// SyncToken is an opaque per-folder cursor managed by a backend's
// ChangeTracker. JMAP encodes its state string. IMAP packs
// (uidvalidity, modseq, maxuid). The cache stores it verbatim on
// folders.sync_token and never inspects it.
type SyncToken []byte

// ChangeSet is a complete delta returned by ChangeTracker.Changes.
// Implementations drain all backend pages before returning. Partial
// deltas are an implementation bug.
type ChangeSet struct {
	Added    []UID
	Modified []UID
	Removed  []UID
}

// ErrCannotCalculateChanges signals the backend cannot compute a delta
// from the supplied SyncToken (JMAP cannotCalculateChanges, IMAP
// UIDVALIDITY change). The cache re-anchors the folder.
var ErrCannotCalculateChanges = errors.New("mail: cannot calculate changes")

// ChangeTracker is the protocol-level change-detection sibling of
// Backend. Both v1 backends implement both interfaces.
//
// Changes loops internally until the backend reports no more pages
// (JMAP hasMoreChanges = false, IMAP VANISHED + FETCH drained) and
// returns a single complete delta or an error.
// ErrCannotCalculateChanges means re-anchor. Context errors propagate.
// Other errors are transient and the syncer applies backoff.
type ChangeTracker interface {
	Changes(ctx context.Context, folder string, since SyncToken) (ChangeSet, SyncToken, error)
}
