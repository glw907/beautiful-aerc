package jmapsource

import (
	"errors"
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
// own failures as DialError instead (below) so its caller's retry
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
func classify(op string, err error, auth *authState) error {
	if err == nil {
		return nil
	}
	if status, ok := statusOf(err); ok {
		if classified := classifyStatus(op, status, err, auth); classified != nil {
			return classified
		}
	}
	if isConnectionDead(err) {
		auth.clear()
		return uerr.New(op, nil, uerr.ClassConnection, err)
	}
	return err
}

// authState dedups a repeated ClassAuth failure across successive
// do() calls on one Session, so a rejected credential logs once per
// failure episode rather than once per call. A backend with no push
// transport (jmapBackend.Capabilities always reports
// PushTransportNone until 6b's Listen lands) polls Changes on every
// kind at sync.Config's PollInterval with no backoff of its own, so a
// token the server keeps rejecting would otherwise construct a fresh
// uerr.Error, and write a fresh log line, on every poll: thousands a
// day for one standing failure, the flood fixing the 401
// classification defect reintroduced. Scope is ClassAuth alone: it is
// the class a standing failure persists under across polls
// unchanged, unlike ClassThrottled, ClassNotFound, ClassServer, or
// ClassConnection, none of which showed a comparable repeat pattern
// under the same experiment.
//
// uerr.Error can only be constructed through uerr.New (the
// error-construction analyzer's own rule), which always logs as a
// side effect of construction, so there is no "classify without
// logging" call for a repeat the way DialError gives Dial's own retry
// loop. report instead reuses the single uerr.Error the first
// occurrence's uerr.New call produced, returning that same value
// (Class still ClassAuth, Cause still the original failure) on every
// later call while the episode continues, rather than constructing
// and logging a new one. This is not silencing (ADR-0013 revision
// 2's own distinction): a caller checking errors.As(err,
// &uerr.Error{}) still sees ClassAuth on every call, only the log
// write itself is deduped, and clear resets the state so the next
// distinct failure (a class change, or a fresh episode after a
// recovery) logs again.
type authState struct {
	mu     sync.Mutex
	active bool
	logged uerr.Error
}

// report returns the uerr.Error for a ClassAuth failure, constructing
// and logging a new one via uerr.New only when a is not already
// mid-episode.
func (a *authState) report(op string, cause error) uerr.Error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.active {
		a.logged = uerr.New(op, nil, uerr.ClassAuth, cause)
		a.active = true
	}
	return a.logged
}

// clear ends a's current episode, so the next ClassAuth failure logs
// again: do() calls it on every successful call, and classify calls
// it on every failure classified to something other than ClassAuth,
// both of which are state transitions worth their own line either
// way (a recovery, or a different problem replacing the old one).
func (a *authState) clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.active = false
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
// rejected credential, 404 a missing entity, 429 throttling, 5xx a
// server-side failure), with ok false for a status none of those
// classes cover. classifyStatus and classifyDial both call this, so
// the mapping lives in exactly one place despite classifyDial needing
// it without classifyStatus's uerr.New construction.
func classifyStatusClass(status int) (class uerr.Class, ok bool) {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return uerr.ClassAuth, true
	case status == http.StatusNotFound:
		return uerr.ClassNotFound, true
	case status == http.StatusTooManyRequests:
		return uerr.ClassThrottled, true
	case status >= http.StatusInternalServerError:
		return uerr.ClassServer, true
	default:
		return 0, false
	}
}

// classifyStatus wraps cause in a uerr.Error under classifyStatusClass's
// mapping, or reports nil for a status none of those classes cover.
// classify is its only caller: do()'s callers run once per outbox
// dispatch attempt or sync flush, each already its own surfacing
// event, unlike Dial's own retry loop (DialError, below), except for
// ClassAuth, which auth.report dedups instead of logging on every
// call (authState's own doc comment).
func classifyStatus(op string, status int, cause error, auth *authState) error {
	class, ok := classifyStatusClass(status)
	if !ok {
		return nil
	}
	if class == uerr.ClassAuth {
		return auth.report(op, cause)
	}
	auth.clear()
	return uerr.New(op, nil, class, cause)
}

// classifyDial classifies a session dial's failure as a DialError,
// without constructing a uerr.Error: Dial is retried by its own
// caller's backoff loop (cmd/poplar's retryConnect), and constructing
// a uerr.Error here would write a log line on every attempt rather
// than only on a state transition (ADR-0013 revision 2). The caller
// that owns that retry loop is the one that decides when to surface
// it.
func classifyDial(err error) error {
	if status, ok := statusOf(err); ok {
		if class, ok := classifyStatusClass(status); ok {
			return DialError{Class: class, Cause: err}
		}
	}
	if isConnectionDead(err) {
		return DialError{Class: uerr.ClassConnection, Cause: err}
	}
	return err
}

// DialError is a session dial's classified failure, carrying the
// uerr.Class SY-4 and ADR-0004 revision 2 assign it and the
// underlying cause, without having constructed a uerr.Error for it.
// classifyDial returns this instead of calling uerr.New directly, the
// same reasoning classifyMutationFailure documents below: Dial's
// caller retries the dial itself in its own backoff loop. The caller
// that owns that retry loop is the one that decides when to surface
// it.
type DialError struct {
	Class uerr.Class
	Cause error
}

// Error returns e's cause's message, or a fixed string when e carries
// no cause: DialError is exported, so a zero value can reach Error
// from outside this package.
func (e DialError) Error() string {
	if e.Cause == nil {
		return "jmap: dial rejected"
	}
	return e.Cause.Error()
}

// Unwrap returns e's cause.
func (e DialError) Unwrap() error { return e.Cause }

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
// parsing a protocol string. It returns a backend.MutationFailure
// rather than calling uerr.New: ApplyBatch runs once per outbox
// dispatch attempt, and constructing a uerr.Error here would write a
// log line on every retry rather than only on a state transition
// (ADR-0013 revision 2). setErrorType survives as Cause, so
// outbox.failure_detail can still record exactly what the server
// said.
func classifyMutationFailure(setErrorType string) backend.MutationFailure {
	class, ok := jmapSetErrorClass[setErrorType]
	if !ok {
		class = uerr.ClassServer
	}
	return backend.MutationFailure{Class: class, Cause: errors.New(setErrorType)}
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
