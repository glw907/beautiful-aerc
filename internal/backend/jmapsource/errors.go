package jmapsource

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/jmap"
)

// isCannotCalculateChanges reports whether err is the JMAP
// cannotCalculateChanges method error (RFC 8621 section 5.2): the
// requested sinceState no longer names any state the server can diff
// from, so the caller resyncs from scratch.
func isCannotCalculateChanges(err error) bool {
	var me *jmap.MethodError
	return errors.As(err, &me) && me.Type == "cannotCalculateChanges"
}

// isStateMismatch reports whether err is the JMAP stateMismatch
// method error: a Set call's ifInState assumed a server state that
// has since moved on.
func isStateMismatch(err error) bool {
	var me *jmap.MethodError
	return errors.As(err, &me) && me.Type == "stateMismatch"
}

// classify wraps a transport-level failure from do() in a uerr.Error
// carrying the class SY-4 and ADR-0004 revision 2 assign it. do()'s
// callers (Changes, ApplyBatch, and the rest of jmap.Mail) each run
// once per outbox dispatch attempt or sync flush, already their own
// surfacing event, unlike Dial's own retry loop, which classifies its
// own failures as a backend.Failure instead (below) so its caller's
// loop owns the surfacing. A JMAP MethodError (a per-call failure
// embedded in an otherwise-200 response) is not this function's
// concern; isCannotCalculateChanges and isStateMismatch classify
// those against the sync engine's own specific signals.
//
// A 401 reaching here now actually classifies, where it silently did
// not before this package's cutover. Fastmail sends a 401 as bare
// text/plain, not a problem-details body (RFC 8620 section 3.6.1 only
// has the server SHOULD send one, per RFC 7807, which is the RFC that
// fixes application/problem+json as its media type). Package jmap's
// refusal() covers both cases: a problem-details body decodes to a
// *jmap.RequestError carrying the real status, and anything else,
// Fastmail's 401 included, degrades to a *jmap.HTTPError that still
// carries the status. go-jmap decoded neither shape reliably: its
// decodeHttpError matched only the literal application/json content
// type, so a real problem-details body sent with a charset parameter,
// or any non-JSON body such as Fastmail's, reached the caller as an
// unrecognized error instead.
func classify(op string, err error, episode *episodeState) error {
	if err == nil {
		return nil
	}
	if status, ok := statusOf(err); ok {
		if classified := classifyStatus(op, status, err, episode); classified != nil {
			return classified
		}
	}
	if isConnectionDead(err) {
		return episode.report(op, uerr.ClassConnection, err)
	}
	return err
}

// episodeState dedups a standing failure across successive do() calls
// on one Session, so a server answering the same way every time logs
// once per episode rather than once per call. do()'s callers run on
// whatever cadence their engine sets, and none of them backs off for
// a failure the server repeats deterministically: the sync worker
// polls Changes on every kind at sync.Config's PollInterval, and the
// outbox dispatcher retries on its own. The episode is Session-wide
// rather than per engine, so the two share one: a call from either
// engine can be the one that opens or extends an episode, and the
// deduped uerr.Error a later call of the other engine's gets back may
// carry a cause its own call never produced. A rejected credential, a
// standing 400 (RFC 8620 section 3.6.1's unknownCapability and limit
// are the deterministic ones), or an unreachable host would otherwise
// construct a fresh uerr.Error, and write a fresh log line, on every
// one of those: thousands a day for one standing failure.
//
// The episode is keyed on the class rather than on one chosen class,
// because any of them can stand. What ends an episode is a state
// transition, which is ADR-0013 revision 2's own rule for when a
// failure is worth a line: the class changing, or clear, which do()
// calls on every successful call.
//
// uerr.Error can only be constructed through uerr.New (the
// error-construction analyzer's own rule), which always logs as a
// side effect of construction, so there is no "classify without
// logging" call for a repeat the way classifyRetried gives Dial's own
// loop. report instead reuses the single uerr.Error the first
// occurrence's uerr.New call produced, returning that same value on
// every later call while the episode continues, rather than
// constructing and logging a new one. This is not silencing (ADR-0013
// revision 2's own distinction): a caller checking errors.As(err,
// &uerr.Error{}) still sees the class on every call, and only the log
// write is deduped. A repeat under one class keeps the first
// occurrence's cause, since a class is what the episode is, and the
// alternative is a line per call.
type episodeState struct {
	mu     sync.Mutex
	active bool
	class  uerr.Class
	logged uerr.Error
}

// report returns the uerr.Error for a failure of class, constructing
// and logging a new one via uerr.New only when e is not already
// mid-episode under that same class.
func (e *episodeState) report(op string, class uerr.Class, cause error) uerr.Error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.active || e.class != class {
		e.logged = uerr.New(op, nil, class, cause)
		e.active, e.class = true, class
	}
	return e.logged
}

// clear ends e's current episode, so the next failure logs again:
// do() calls it on every successful call, which is the recovery a
// later failure of the same class has to be told apart from.
func (e *episodeState) clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.active = false
}

// statusOf reports the HTTP status a rejected JMAP request carried,
// whichever of package jmap's two refusal shapes err holds: a
// *jmap.RequestError for a problem-details body, or a *jmap.HTTPError
// for anything else.
func statusOf(err error) (status int, ok bool) {
	if re, ok := errors.AsType[*jmap.RequestError](err); ok {
		return re.Status, true
	}
	if he, ok := errors.AsType[*jmap.HTTPError](err); ok {
		return he.Status, true
	}
	return 0, false
}

// classifyStatusClass maps the HTTP status of a rejected JMAP request
// to the uerr.Class SY-4 and ADR-0004 revision 2 assign it (401/403 a
// rejected credential, 404 a missing entity, 429 throttling, and every
// other rejection a server-side failure), with ok false for a status
// outside the rejection range entirely. classifyStatus and classifyRetried
// both call this, so the mapping lives in exactly one place despite
// classifyRetried needing it without classifyStatus's uerr.New
// construction.
//
// The whole 4xx band classifies, rather than only the three statuses
// named above, because the seam is where the class is decided. RFC
// 8620 section 3.6.1's request-level errors (notRequest, notJSON,
// limit, unknownCapability) all arrive as 400, and leaving those
// unclassified left each engine to invent its own default: the same
// rejection read as "Couldn't reach the server" from a sync flush and
// "The server reported a problem" from an outbox dispatch, under two
// log keys and two retry treatments. A request the server answered at
// all is not a connectivity problem, whatever it answered.
func classifyStatusClass(status int) (class uerr.Class, ok bool) {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return uerr.ClassAuth, true
	case status == http.StatusNotFound:
		return uerr.ClassNotFound, true
	case status == http.StatusTooManyRequests:
		return uerr.ClassThrottled, true
	case status >= http.StatusBadRequest:
		return uerr.ClassServer, true
	default:
		return 0, false
	}
}

// classifyStatus wraps cause in a uerr.Error under
// classifyStatusClass's mapping, or reports nil for a status none of
// those classes cover. classify is its only caller, and the line is
// written once per failure episode rather than once per call
// (episodeState's own doc comment): a status the server keeps
// answering with is exactly the shape that repeats.
func classifyStatus(op string, status int, cause error, episode *episodeState) error {
	class, ok := classifyStatusClass(status)
	if !ok {
		return nil
	}
	return episode.report(op, class, cause)
}

// classifyRetried classifies the two failures this package leaves
// unlogged because their caller retries them on its own schedule: a
// session dial (cmd/poplar's retryConnect) and a push Listen
// (RunPush's reconnect). It returns the backend.Failure that carries
// the class across the seam, or err untouched when neither an HTTP
// status nor a dead connection classifies it.
//
// Constructing a uerr.Error here would write a log line on every
// attempt rather than only on a state transition (ADR-0013 revision
// 2), so the layer that owns the retry loop is the layer that decides
// when to surface it. Without the class crossing the seam at all, a
// credential the server rejects on the event source reaches the user
// as a connectivity problem, which is the one push failure no amount
// of waiting fixes.
func classifyRetried(err error) error {
	if status, ok := statusOf(err); ok {
		if class, ok := classifyStatusClass(status); ok {
			return backend.Failure{Class: class, Cause: err}
		}
	}
	if isConnectionDead(err) {
		return backend.Failure{Class: uerr.ClassConnection, Cause: err}
	}
	return err
}

// jmapSetErrorClass maps a JMAP SetError's type (RFC 8620 section
// 5.3) to the uerr.Class SY-4 assigns it. A type with no entry here
// classifies as ClassServer, a rejection with no finer class to name
// it.
var jmapSetErrorClass = map[string]uerr.Class{
	"notFound":  uerr.ClassNotFound,
	"forbidden": uerr.ClassAuth,
	"rateLimit": uerr.ClassThrottled,
}

// classifyMutationFailure maps one Email/set mutation's raw SetError
// type to the uerr.Class jmapSetErrorClass names for it, so the
// outbox dispatcher (task 10) can branch on a closed class instead of
// parsing a protocol string. It returns a backend.Failure
// rather than calling uerr.New: ApplyBatch runs once per outbox
// dispatch attempt, and constructing a uerr.Error here would write a
// log line on every retry rather than only on a state transition
// (ADR-0013 revision 2). setErrorType survives as Cause, so
// outbox.failure_detail can still record exactly what the server
// said.
func classifyMutationFailure(setErrorType string) backend.Failure {
	class, ok := jmapSetErrorClass[setErrorType]
	if !ok {
		class = uerr.ClassServer
	}
	return backend.Failure{Class: class, Cause: errors.New(setErrorType)}
}

// mailboxNameConflict is the SetError type a server answers a
// duplicate sibling mailbox name with. RFC 8621 defines none for it:
// section 2.5's extra types are mailboxHasChild and mailboxHasEmail,
// both for destroy, so a server refusing a create under section 2's
// sibling-uniqueness rule reaches for a type from elsewhere. Fastmail
// and Stalwart both answer alreadyExists, which RFC 8620 section 5.4
// defines for /copy, and that is the one type poplar reads as a name
// conflict. A server answering anything else, invalidProperties among
// them, leaves the caller an ordinary rejection: reading a generic
// refusal as a name conflict would send a create looking for a mailbox
// that was never the reason it failed.
const mailboxNameConflict = "alreadyExists"

// classifyMailboxCreateFailure is classifyMutationFailure for a
// Mailbox/set create's refusal, wrapping backend.ErrMailboxNameExists
// into the Cause of the one type that says the mailbox the caller
// asked for is already there. The wire type survives as the Cause's
// own text, so outbox.failure_detail still records what the server
// said.
func classifyMailboxCreateFailure(setErrorType string) backend.Failure {
	f := classifyMutationFailure(setErrorType)
	if setErrorType == mailboxNameConflict {
		f.Cause = fmt.Errorf("%s: %w", setErrorType, backend.ErrMailboxNameExists)
	}
	return f
}

// isConnectionDead reports whether err comes from the underlying TCP
// connection closing or timing out mid-call.
func isConnectionDead(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return isConnectionDead(ue.Err)
	}
	return false
}
