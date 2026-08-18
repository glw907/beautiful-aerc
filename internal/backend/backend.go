// Package backend declares the seam every mail, calendar, and
// contacts source implements. ADR-0004 revision 2 expresses the seam
// as shapes rather than wire vocabulary: a hydrated changes-since-
// token feed, a batched mutation entry point, and a push transport.
// A backend composes the sources one account exposes; Calendar,
// Contacts, and Push return nil for a source the account lacks.
// internal/backend/jmapsource and internal/backend/dav implement it
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

// ErrStateMismatch is reserved for the all-or-nothing batch
// guarantee, and nothing requests that guarantee yet. RFC 8620
// section 5.3 lets a server answer stateMismatch only to a Set that
// supplied ifInState, and no backend here supplies one, so a backend
// that translates the refusal (jmapsource does) cannot currently
// produce this sentinel and no caller checks for it.
//
// Pass 3 owns the policy, being the first with a batch whose parts
// must not land separately: which mutations request the guarantee,
// and what a caller does with the refusal (re-fetch state through
// Changes and retry is the shape, but nothing has ruled on the retry
// bound). Until then a batch is applied per mutation, and
// BatchResult.Failed is the whole account of what did not land.
var ErrStateMismatch = errors.New("backend: state mismatch")

// ErrMailboxNameExists reports that a mailbox create was refused
// because the account already holds a mailbox of that name under that
// parent. RFC 8621 section 2 states the rule the refusal enforces:
// "There MUST NOT be two sibling Mailboxes with both the same parent
// and the same name." A CreateMailbox call returns it, and ApplyBatch
// puts it in the Cause of the Failure it records against a mailbox
// create's CreationID.
//
// Retrying earns the same answer every time, so a caller that meant to
// create exactly this mailbox resolves the refusal through
// FindMailboxes instead.
var ErrMailboxNameExists = errors.New("backend: mailbox name exists under that parent")

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

// Record is one hydrated item a Changes call returns: the server's id
// for the object, and the payload for the kind the call named. A
// backend fills Fields from its wire protocol before Changes returns
// and never leaks a wire property name (JMAP's mailboxIds, keywords,
// and so on) through the seam.
type Record struct {
	ID     string
	Fields RecordFields
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

// Mutation is one change ApplyBatch applies within a batch.
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
	// Fields carries what the mutation changes, and is nil for a
	// destroy.
	Fields MutationFields
}

// BatchResult is what ApplyBatch returns: the server id assigned to
// each Mutation.CreationID, and any per-mutation failure, keyed by
// whichever id the mutation carried (ID for an update or destroy,
// CreationID for a create).
type BatchResult struct {
	Created map[string]string
	Failed  map[string]error
}

// Failure is a classified failure a backend hands over without having
// logged it: the uerr.Class SY-4 and ADR-0004 revision 2 assign it, and
// the wire-level cause preserved as Cause. Every call that returns one
// is a call some caller retries on its own schedule, and ADR-0013
// revision 2 reserves uerr.New for a state transition (first failure,
// class change, recovery) rather than every attempt, so the layer that
// owns the retry loop is the layer that constructs the uerr.Error.
//
// Two calls return one. ApplyBatch populates BatchResult.Failed with a
// Failure per mutation, and the dispatcher surfaces it once, when that
// mutation's failure state changes, under the id the map already keys
// it by; Cause is what reaches outbox.failure_detail. A push
// transport's Listen returns one for a refused stream, and RunPush's
// reconnect loop surfaces it once per failure episode. Without the
// class crossing the seam there, a credential the server rejects on
// the event source reads as a connectivity problem, which is the one
// push failure no amount of waiting fixes.
type Failure struct {
	Class uerr.Class
	Cause error
}

// Error returns f's cause's message.
func (f Failure) Error() string { return f.Cause.Error() }

// Unwrap returns f's cause.
func (f Failure) Unwrap() error { return f.Cause }

// ClassCause returns f's Class and Cause, satisfying uerr.Classified.
func (f Failure) ClassCause() (uerr.Class, error) { return f.Class, f.Cause }

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
	// root, if empty) and returns its server id. It returns an error
	// wrapping ErrMailboxNameExists when the server refuses the name
	// for a sibling it already holds.
	CreateMailbox(ctx context.Context, name, parentID string) (id string, err error)
	// RenameMailbox changes the mailbox named id's display name.
	RenameMailbox(ctx context.Context, id, name string) error
	// DeleteMailbox removes the mailbox named id.
	DeleteMailbox(ctx context.Context, id string) error

	// FindMailboxes returns the server id of every mailbox whose name
	// is exactly name and whose parent is exactly parentID (the root,
	// if empty). It is the point query that resolves an
	// ErrMailboxNameExists refusal, and it reaches nothing the delta
	// feed owns.
	//
	// Matching is exact on both, whatever the backend's own query
	// vocabulary offers. RFC 8621 section 2.3 defines JMAP's name
	// filter condition as "The Mailbox 'name' property contains the
	// given string", so a backend over it narrows with that filter and
	// confirms the exact name itself; a lookup of "Work" that answered
	// with "Workshop" would bind a caller to a folder nobody named. A
	// server holding section 2's sibling-uniqueness rule returns at
	// most one id, so a caller reads two as that rule broken rather
	// than as a choice to make.
	FindMailboxes(ctx context.Context, name, parentID string) (ids []string, err error)

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

// Push is the backend's push transport. Listen opens it and returns a
// channel of notifications, reporting an error when it will not open.
//
// The transport owns reconnection across a drop, and with it the
// liveness check that decides a connection is gone: it is the only
// layer that knows the cadence the server granted, and a second
// backoff above it puts ADR-0005's 30s p95 recovery bound out of reach
// by construction. So the channel survives a drop and closes only once
// the transport has stopped for good, which is the server refusing the
// connection or ctx ending. A caller that sees it close reopens by
// calling Listen again, and that call reports what stopped the last
// stream before it opens anything: a refusal arriving after Listen has
// returned its channel has no other way to reach the caller, and it is
// both the reason to wait longer and the failure the user is owed.
// Listen's error carries a Failure when the backend classified it.
//
// Every connection the transport makes, the first and each one after,
// produces a notification of its own. The stream says nothing about
// what happened while it was down, so the caller pulls Changes from its
// persisted token on each one (ADR-0018).
//
// A notification is a signal, not a delivery: the transport drops one
// rather than waiting for a caller that has not read the last. Nothing
// is lost by that, since the Changes call any one of them triggers
// reads everything since the persisted token, and a transport that
// blocked on the caller would stall the stream it is reading behind a
// sync that can take seconds.
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
