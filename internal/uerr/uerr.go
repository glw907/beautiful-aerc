// Package uerr is poplar's error seam. Every user-visible failure --
// a banner, a toast, a modal -- is a uerr.Error, built through New,
// the package's only exported constructor. New classifies the
// failure, writes its log line, and returns the view value, so no
// user-visible failure goes unlogged (ER-1).
package uerr

// Class classifies a user-visible failure (SY-4, ER-1). The outbox
// and sync engine branch retry and surfacing behavior on it.
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
}

// Error is poplar's user-visible error. Op, IDs, Class, Message, and
// Cause are exported so a caller can read them back -- a banner
// renders Message, the outbox branches on Class -- but every Error is
// built through New; a composite literal of Error outside this
// package fails the error-construction analyzer.
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

// New builds the Error for op, classifies it under class, writes the
// log line, and returns the view value. ids names the entities op
// acted on (message IDs, mailbox IDs); New logs them for correlation
// and never surfaces them to the user. cause is the underlying error;
// a caller recovers it through errors.As or errors.Is.
func New(op string, ids []string, class Class, cause error) Error {
	e := Error{Op: op, IDs: ids, Class: class, Message: sentence[class], Cause: cause}
	logError(e)
	return e
}
