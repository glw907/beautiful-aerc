// Package backend declares the seam every mail, calendar, and
// contacts source implements. ADR-0004 revision 2 expresses the seam
// as shapes rather than wire vocabulary: a hydrated changes-since-
// token feed, a batched mutation entry point, and a push transport.
// A backend composes the sources one account exposes; Calendar,
// Contacts, and Push return nil for a source the account lacks.
// internal/backend/jmap and internal/backend/dav implement it
// against a live server; Fake, in this package, is the seam's second
// implementation (ADR-0014), scripted for engine tests.
package backend

import (
	"context"
	"errors"
)

// ErrStateReset reports that a Changes token no longer names any
// state the server knows: the account's history was reset (or is
// unknown to this session). The caller resyncs from an empty token.
var ErrStateReset = errors.New("backend: state reset")

// ErrStateMismatch reports that ApplyBatch's mutations assumed a
// server state that has since moved on: nothing in the batch was
// applied. The caller re-fetches state through Changes and retries.
var ErrStateMismatch = errors.New("backend: state mismatch")

// Record is one hydrated item a Changes call returns. Fields carries
// whatever the collection's own package (mail, calendar, contacts)
// decodes into its model, keyed to the server's id for the record.
type Record struct {
	ID     string
	Fields map[string]any
}

// ChangeSet is what Changes returns for one collection since a
// watermark: records created or updated since Token, hydrated in the
// same round trip, and the ids of anything destroyed.
type ChangeSet struct {
	Created   []Record
	Updated   []Record
	Destroyed []string
	Token     string
}

// MutationOp names the kind of change a Mutation describes.
type MutationOp int

const (
	// MutationCreate creates a record. ID is empty; CreationID
	// names the record for a later Mutation in the same batch.
	MutationCreate MutationOp = iota
	// MutationUpdate changes fields on an existing record, named by
	// ID.
	MutationUpdate
	// MutationDestroy removes an existing record, named by ID.
	MutationDestroy
)

// Mutation is one change ApplyBatch applies within a batch.
type Mutation struct {
	Op MutationOp
	// ID names the record Op acts on: a server id for Update or
	// Destroy, empty for Create.
	ID string
	// CreationID names a Create mutation so a later mutation in the
	// same batch can reference the record before the server assigns
	// it a permanent id, letting an offline create-folder-then-move
	// dispatch as one request.
	CreationID string
	Fields     map[string]any
}

// BatchResult is what ApplyBatch returns: the server id assigned to
// each Mutation.CreationID, and any per-mutation failure, keyed by
// whichever id the mutation carried (ID for an update or destroy,
// CreationID for a create).
type BatchResult struct {
	Created map[string]string
	Failed  map[string]error
}

// SubmitResult is what Submit returns once the backend accepts an
// outgoing message: the backend's submission id, and whether the
// message is already Sent, for a backend whose accept and deliver
// are the same round trip.
type SubmitResult struct {
	ID   string
	Sent bool
}

// Notification is one push signal: something changed, at the
// granularity Capabilities.DeltaGranularity promises. It carries no
// hydrated data; the caller responds by calling Changes again.
type Notification struct {
	Scope string
}

// Source is the delta-and-mutate shape every collection a backend
// composes shares.
type Source interface {
	// Changes returns everything created or updated since token,
	// hydrated in the same round trip, and the ids of anything
	// destroyed. An empty token asks for a full initial sync.
	Changes(ctx context.Context, token string) (ChangeSet, error)

	// ApplyBatch applies mutations as one request.
	ApplyBatch(ctx context.Context, mutations []Mutation) (BatchResult, error)
}

// Mail is the mail source: message and mailbox changes, message
// bodies, outgoing submission, and mailbox lifecycle.
type Mail interface {
	Source

	// FetchBodies returns the raw message source for each id in
	// ids.
	FetchBodies(ctx context.Context, ids []string) (map[string][]byte, error)

	// Submit hands raw outgoing message source to the backend's
	// submission lifecycle.
	Submit(ctx context.Context, raw []byte) (SubmitResult, error)

	CreateMailbox(ctx context.Context, name, parentID string) (id string, err error)
	RenameMailbox(ctx context.Context, id, name string) error
	DeleteMailbox(ctx context.Context, id string) error

	// Search runs a server-side search (SR-7). A caller checks
	// Capabilities().ServerSearch before calling; a backend that
	// did not declare it returns an error.
	Search(ctx context.Context, query string) ([]string, error)
}

// Calendar is the calendar source: event changes, and an attendee's
// RSVP submitted through whichever mechanism Capabilities().RSVP
// names.
type Calendar interface {
	Source
	Respond(ctx context.Context, id, partstat string) error
}

// Contacts is the contacts source.
type Contacts interface {
	Source
}

// Push is the backend's push transport. Listen opens it and returns
// a channel of notifications; the channel closes when the transport
// drops, and the caller decides whether and how to reconnect.
type Push interface {
	Listen(ctx context.Context) (<-chan Notification, error)
}

// Backend composes the sources one account exposes. Calendar,
// Contacts, and Push return nil when the account has no such
// source; a caller checks before use.
type Backend interface {
	Mail() Mail
	Calendar() Calendar
	Contacts() Contacts
	Push() Push
	Capabilities() Capabilities
}
