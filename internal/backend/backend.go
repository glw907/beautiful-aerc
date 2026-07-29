// Package backend declares the seam every mail, calendar, and
// contacts source implements. ADR-0004 revision 2 expresses the seam
// as shapes rather than wire vocabulary: a hydrated changes-since-
// token feed, a batched mutation entry point, and a push transport.
// A backend composes the sources one account exposes; Calendar,
// Contacts, and Push return nil for a source the account lacks.
// internal/backend/jmap and internal/backend/dav implement it
// against a live server; internal/backend/backendtest's Fake is the
// seam's second implementation (ADR-0014), scripted for engine tests.
package backend

import (
	"context"
	"errors"
	"iter"

	"github.com/glw907/poplar/internal/uerr"
)

// ErrStateReset reports that a Changes token no longer names any
// state the server knows: the account's history was reset (or is
// unknown to this session). The caller resyncs from an empty token.
var ErrStateReset = errors.New("backend: state reset")

// ErrStateMismatch reports that ApplyBatch's mutations assumed a
// server state that has since moved on: nothing in the batch was
// applied. The caller re-fetches state through Changes and retries.
var ErrStateMismatch = errors.New("backend: state mismatch")

// ObjectKind names the collection a Source's Changes call pages
// through. A backend composes several kinds behind one Mail,
// Calendar, or Contacts source (Mail carries both Message and
// Mailbox), so Changes takes the kind as an argument rather than
// mixing collections into one response.
type ObjectKind int

const (
	// ObjectKindMessage is a mail source's messages.
	ObjectKindMessage ObjectKind = iota
	// ObjectKindMailbox is a mail source's mailboxes (folders).
	ObjectKindMailbox
	// ObjectKindEvent is a calendar source's events.
	ObjectKindEvent
	// ObjectKindContact is a contacts source's cards.
	ObjectKindContact
)

// Record is one hydrated item a Changes call returns. Fields is
// keyed to poplar's own field vocabulary for its kind, the same names
// internal/mail, internal/calendar, and internal/contacts decode. A
// backend translates its wire protocol's property names into this
// vocabulary before Changes returns; it never leaks a wire property
// name (JMAP's mailboxIds, keywords, and so on) through the seam.
type Record struct {
	ID     string
	Fields map[string]any
}

// MessageFlagKeywords maps the boolean flag names a message Record's
// Fields carries to the keyword each one stands for on the wire and in
// the store. A backend translates its own keyword set through this
// table on the way out of Changes and back through it on the way into
// ApplyBatch; internal/sync reads the same table to reach
// store.EncodeFlags's keyword vocabulary.
var MessageFlagKeywords = map[string]string{
	"seen":      "$seen",
	"flagged":   "$flagged",
	"answered":  "$answered",
	"draft":     "$draft",
	"forwarded": "$forwarded",
}

// ChangeSet is what Changes returns for one page of one collection:
// records created or updated since the requested token, hydrated in
// the same round trip, and the ids of anything destroyed. HasMore is
// true when the collection has more changes past this page than limit
// admitted; the caller repeats Changes with NewToken to fetch the
// rest.
type ChangeSet struct {
	Created   []Record
	Updated   []Record
	Destroyed []string
	NewToken  string
	HasMore   bool
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

// Mutation is one change ApplyBatch applies within a batch. Fields is
// keyed to the same poplar field vocabulary Record.Fields uses.
type Mutation struct {
	Op MutationOp
	// Kind names the collection the record belongs to, the same
	// discriminator Changes takes, so one batch can mix the kinds a
	// source composes (a mailbox create and the message updates
	// referencing it). The zero value is ObjectKindMessage.
	Kind ObjectKind
	// ID names the record Op acts on: a server id for Update or
	// Destroy, empty for Create.
	ID string
	// CreationID names a Create mutation so a later mutation in the
	// same batch can reference the record before the server assigns
	// it a permanent id, letting an offline create-folder-then-move
	// dispatch as one request. A later mutation names it as
	// "#"+CreationID wherever a field takes an id, and the backend
	// resolves that reference against its own protocol.
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

// MutationFailure is one mutation's classified failure: the uerr.Class
// a backend assigns it, and the wire-level cause preserved as Cause
// for outbox.failure_detail. A backend populates BatchResult.Failed
// with this instead of a uerr.Error: ApplyBatch runs once per outbox
// dispatch attempt, and ADR-0013 revision 2 reserves uerr.New for a
// state transition (first failure, class change, recovery), not every
// attempt. The dispatcher constructs the uerr.Error itself, once, when
// a mutation's failure state changes, using the id BatchResult.Failed
// already keys the result by.
type MutationFailure struct {
	Class uerr.Class
	Cause error
}

// Error returns f's cause's message.
func (f MutationFailure) Error() string { return f.Cause.Error() }

// Unwrap returns f's cause.
func (f MutationFailure) Unwrap() error { return f.Cause }

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

// BodyChunk is one message's raw source, yielded by FetchBodies'
// iterator. Err is the failure fetching this one id; a caller ranges
// over the iterator and checks Err per item rather than losing every
// other body to one message's failure.
type BodyChunk struct {
	ID  string
	Raw []byte
	Err error
}

// Source is the delta-and-mutate shape every collection a backend
// composes shares.
type Source interface {
	// Changes returns one page of everything created or updated
	// since token for kind, hydrated in the same round trip, and the
	// ids of anything destroyed. An empty token asks for a full
	// initial sync. limit bounds the page size; a limit of 0 asks for
	// the backend's own default.
	Changes(ctx context.Context, kind ObjectKind, token string, limit int) (ChangeSet, error)

	// ApplyBatch applies mutations as one request.
	ApplyBatch(ctx context.Context, mutations []Mutation) (BatchResult, error)
}

// Mail is the mail source: message and mailbox changes, message
// bodies, outgoing submission, and mailbox lifecycle.
type Mail interface {
	Source

	// FetchBodies returns the raw message source for each id in ids,
	// yielded lazily by the returned iterator so a multi-megabyte
	// body never has to sit whole in memory alongside every other one
	// requested.
	FetchBodies(ctx context.Context, ids []string) (iter.Seq[BodyChunk], error)

	// Submit hands raw outgoing message source to the backend's
	// submission lifecycle.
	Submit(ctx context.Context, raw []byte) (SubmitResult, error)

	// CreateMailbox creates a mailbox named name under parentID (the
	// root, if empty) and returns its server id.
	CreateMailbox(ctx context.Context, name, parentID string) (id string, err error)
	// RenameMailbox changes the mailbox named id's display name.
	RenameMailbox(ctx context.Context, id, name string) error
	// DeleteMailbox removes the mailbox named id.
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

	// Respond submits id's RSVP as partstat, one of iCalendar's
	// PARTSTAT values ("ACCEPTED", "DECLINED", "TENTATIVE").
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

// Credentials owns a backend's auth token lifecycle. Token returns a
// valid credential for the current request, refreshing it first if
// needed; the seam owns single-flight refresh and persistence through
// internal/keyring, so a caller never sees a 401-refresh-retry
// sequence. For v1's static Fastmail token, Token is a read.
type Credentials interface {
	Token(ctx context.Context) (string, error)
}

// Backend composes the sources one account exposes. Calendar,
// Contacts, and Push return nil when the account has no such source;
// a caller checks before use. Credentials is never nil.
type Backend interface {
	Mail() Mail
	Calendar() Calendar
	Contacts() Contacts
	Push() Push
	Capabilities() Capabilities
	Credentials() Credentials
}
