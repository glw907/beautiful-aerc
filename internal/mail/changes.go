// SPDX-License-Identifier: MIT

package mail

import (
	"context"
	"errors"
)

// SyncToken is an opaque per-folder cursor managed by a backend's
// ChangeTracker. JMAP encodes its state string; IMAP packs
// (uidvalidity, modseq, maxuid) into bytes. The cache stores it on
// folders.sync_token verbatim and never inspects it.
type SyncToken []byte

// ChangeSet is a complete delta returned by ChangeTracker.Changes.
// Implementations must drain all backend pages internally before
// returning — partial deltas are an implementation bug.
type ChangeSet struct {
	Added    []UID
	Modified []UID
	Removed  []UID
}

// ErrCannotCalculateChanges signals the backend cannot compute a
// delta from the supplied SyncToken (JMAP cannotCalculateChanges,
// IMAP UIDVALIDITY change). The cache responds by re-anchoring the
// folder per spec §D.4.
var ErrCannotCalculateChanges = errors.New("mail: cannot calculate changes")

// ErrAuth is the sentinel for backend authentication failure (4xx
// from JMAP/HTTP, BAD/NO with auth context from IMAP, or expired
// OAuth token). The cache drainer matches this with errors.Is and
// promotes the outbox row to conflict with kind "auth-failure"
// instead of cycling through the backoff loop. Backends MUST wrap
// auth failures with %w so the chain unwraps to ErrAuth.
var ErrAuth = errors.New("mail: authentication failed")

// ErrNotFound signals the message no longer exists on the server.
// The cache drainer treats it as idempotent success per spec §D.4.
var ErrNotFound = errors.New("mail: not found")

// ChangeTracker is the protocol-level change-detection sibling of
// Backend. Both v1 backends (mailjmap, mailimap) implement both
// interfaces.
//
// Implementations MUST loop internally until the backend reports no
// more pages (JMAP: hasMoreChanges = false; IMAP: VANISHED + FETCH
// fully drained). Callers receive a single complete delta or an
// error.
//
// Errors:
//   - ErrCannotCalculateChanges (sentinel) — re-anchor required.
//   - context errors — propagated.
//   - other errors — transient; the syncer applies backoff.
type ChangeTracker interface {
	Changes(ctx context.Context, folder string, since SyncToken) (ChangeSet, SyncToken, error)
}
