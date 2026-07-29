package jmap

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"

	"git.sr.ht/~rockorager/go-jmap"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
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
// surfacing event, unlike fetchSession's dial retry loop, which
// classifies its own failures as DialError instead (below) so its
// caller's retry loop owns the surfacing. A JMAP MethodError (a
// per-call failure embedded in an otherwise-200 response) is not this
// function's concern; isCannotCalculateChanges and isStateMismatch
// classify those against the sync engine's own specific signals.
func classify(op string, err error) error {
	if err == nil {
		return nil
	}
	if re, ok := errors.AsType[*jmap.RequestError](err); ok {
		if classified := classifyStatus(op, re.Status, err); classified != nil {
			return classified
		}
	}
	if isConnectionDead(err) {
		return uerr.New(op, nil, uerr.ClassConnection, err)
	}
	return err
}

// classifyStatusClass maps the HTTP status of a rejected JMAP request
// to the uerr.Class SY-4 and ADR-0004 revision 2 assign it (401/403 a
// rejected credential, 404 a missing entity, 429 throttling, 5xx a
// server-side failure), with ok false for a status none of those
// classes cover. classifyStatus and fetchSession's own dial-path
// classification both call this, so the mapping lives in exactly one
// place despite fetchSession needing it without classifyStatus's
// uerr.New construction.
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
// classify (a *jmap.RequestError from a completed round trip) is its
// only caller: do()'s callers run once per outbox dispatch attempt or
// sync flush, each already its own surfacing event, unlike
// fetchSession's dial retry loop (DialError, below).
func classifyStatus(op string, status int, cause error) error {
	class, ok := classifyStatusClass(status)
	if !ok {
		return nil
	}
	return uerr.New(op, nil, class, cause)
}

// DialError is a session dial's classified failure, carrying the
// uerr.Class SY-4 and ADR-0004 revision 2 assign it and the
// underlying cause, without having constructed a uerr.Error for it.
// fetchSession returns this instead of calling uerr.New directly, the
// same reasoning classifyMutationFailure documents below: Dial's
// caller retries the dial itself in its own backoff loop, and
// constructing a uerr.Error here would write a log line on every
// attempt rather than only on a state transition (ADR-0013 revision
// 2). The caller that owns that retry loop is the one that decides
// when to surface it.
type DialError struct {
	Class uerr.Class
	Cause error
}

// Error returns e's cause's message.
func (e DialError) Error() string { return e.Cause.Error() }

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
