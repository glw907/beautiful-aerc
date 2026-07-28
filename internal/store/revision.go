package store

import "sync/atomic"

// Revision is a monotonically increasing count of every write the
// store has committed. Every read result carries the revision it
// saw, so a caller holding two results can order them with < or >
// instead of re-querying: the mechanism pass 2's 100ms UI coalescing
// keys on, and ADR-0003's optimistic-paint discipline uses to keep a
// stale re-query from reverting a paint already applied.
type Revision int64

// RevisionCounter is the store's single revision source, shared
// between the writer, which advances it once for every transaction
// it commits, and every ReadPool opened over the same store file,
// which stamps each result with the value it saw.
type RevisionCounter struct {
	n atomic.Int64
}

// Current returns the revision as of the moment of the call.
func (c *RevisionCounter) Current() Revision {
	return Revision(c.n.Load())
}

// advance records one committed write and returns the new revision.
// Only the writer calls it, and only after a transaction commits.
// Calling it before the commit lands would let a result claim a
// revision the data does not yet reflect.
func (c *RevisionCounter) advance() Revision {
	return Revision(c.n.Add(1))
}
