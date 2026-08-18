// Package uerr is poplar's error seam. Every user-visible failure is
// a uerr.Error: a banner, a toast, a modal. It is built through New,
// the package's only exported constructor. New classifies the
// failure, writes its log line, and returns the view value, so no
// user-visible failure goes unlogged (ER-1).
//
// Redaction is structural, per ADR-0013, not a scrub step at write
// time. Error's field set is exactly Op, IDs, Class, Message, and
// Cause. TestErrorFieldsAreExactlyRedactionSafe pins it. None of
// those fields is a message body or an address, so a type that
// cannot represent a secret cannot leak one into the log. A field
// added later must hold the same discipline: no body, no address or
// subject outside a debug-level type, and never a credential.
package uerr

import "errors"

// Class classifies a user-visible failure (SY-4, ER-1). The outbox
// and sync engine branch retry and surfacing behavior on it.
//
// Class enumerates SY-4's remote and outbox reasons only: a rejected
// or expired credential, a missing entity, an unreachable server, a
// server-side failure, and throttling. A local failure, such as SY-8's
// store corruption, a failed migration, or a full disk, is not a
// server problem. It must not reuse ClassServer, or any other class
// here, to say so. It gets its own class from whichever task first
// needs to surface one.
type Class int

const (
	// ClassAuth is a rejected credential.
	ClassAuth Class = iota
	// ClassAuthRefreshFailed is auth's refresh-failed sub-reason: a
	// token refresh attempt itself failed, distinct from a bare
	// credential rejection.
	ClassAuthRefreshFailed
	// ClassNotFound is a reference to an entity the server no
	// longer has.
	ClassNotFound
	// ClassConnection is a failure to reach the server at all.
	ClassConnection
	// ClassServer is a server-side failure with no finer
	// classification.
	ClassServer
	// ClassThrottled is a rate-limited request the server asked to
	// retry later.
	ClassThrottled
	// ClassSchemaVersion is a store whose on-disk schema_version
	// exceeds this build's known maximum: a newer poplar binary
	// migrated it forward, and this build cannot read it (SY-1).
	ClassSchemaVersion
	// ClassStoreLocal is a local store failure with no server
	// involved: a failed migration, corruption, or a full disk
	// (SY-8). It must not reuse ClassServer, which names a remote
	// failure.
	ClassStoreLocal
	// ClassInstanceLocked is a second poplar process refused startup
	// against a store another instance already holds (SY-7, ADR-0015).
	ClassInstanceLocked
)

// String returns class's log key.
func (c Class) String() string {
	switch c {
	case ClassAuth:
		return "auth"
	case ClassAuthRefreshFailed:
		return "auth-refresh-failed"
	case ClassNotFound:
		return "not-found"
	case ClassConnection:
		return "connection"
	case ClassServer:
		return "server"
	case ClassThrottled:
		return "throttled"
	case ClassSchemaVersion:
		return "schema-version"
	case ClassStoreLocal:
		return "store-local"
	case ClassInstanceLocked:
		return "instance-locked"
	default:
		return "unknown"
	}
}

// sentence is the fixed user-facing text for each class. Fixing it
// per class, rather than letting a call site supply free text, keeps
// what the user sees and what the log records the same string, which
// is the correlation ER-1's acceptance test checks for.
var sentence = map[Class]string{
	ClassAuth:              "Sign-in was rejected",
	ClassAuthRefreshFailed: "Sign-in expired and could not refresh",
	ClassNotFound:          "That item is no longer there",
	ClassConnection:        "Couldn't reach the server",
	ClassServer:            "The server reported a problem",
	ClassThrottled:         "The server asked us to slow down",
	ClassSchemaVersion:     "This store needs a newer version of poplar",
	ClassStoreLocal:        "Poplar could not open its store",
	ClassInstanceLocked:    "Poplar is already running",
}

// Error is poplar's user-visible error. Op, IDs, Class, Message, and
// Cause are exported so a caller can read them back. A banner renders
// Message; the outbox branches on Class. Every Error is still built
// through New: a composite literal of Error outside this package
// fails the error-construction analyzer.
type Error struct {
	Op      string
	IDs     []string
	Class   Class
	Message string
	Cause   error
}

// Error returns e's user-facing sentence.
func (e Error) Error() string { return e.Message }

// Unwrap returns e's cause.
func (e Error) Unwrap() error { return e.Cause }

// ClassCause returns e's Class and Cause, satisfying Classified.
func (e Error) ClassCause() (Class, error) { return e.Class, e.Cause }

// Classified is a typed failure that carries the Class/Cause pair a
// caller peels off it to decide whether to retry, wrap, or surface
// it: the shape Error, backend.Failure, and jmapsource.DialError all
// share. ClassifyErr is the one place that checks for it, instead of
// each caller repeating its own per-type errors.AsType chain.
type Classified interface {
	error
	ClassCause() (Class, error)
}

// ClassifyErr walks err's tree for the first Classified error and
// returns its Class/Cause pair, or fallback and err itself when
// nothing in the tree implements Classified.
func ClassifyErr(err error, fallback Class) (Class, error) {
	if c, ok := errors.AsType[Classified](err); ok {
		return c.ClassCause()
	}
	return fallback, err
}

// New builds the Error for op, classifies it under class, writes the
// log line, and returns the view value. ids names the entities op
// acted on (message IDs, mailbox IDs). New logs ids for correlation
// and never surfaces them to the user. cause is the underlying error.
// A caller recovers cause through errors.As or errors.Is.
//
// New logs cause verbatim at error level. cause must never carry a
// credential and should avoid message-body content. The caller that
// wraps a secret into cause is the leak, not this seam.
func New(op string, ids []string, class Class, cause error) Error {
	e := Error{Op: op, IDs: ids, Class: class, Message: sentence[class], Cause: cause}
	logError(e)
	return e
}
