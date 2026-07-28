package jmap

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"

	"git.sr.ht/~rockorager/go-jmap"

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

// classify wraps a transport-level failure from do() or a session
// dial in a uerr.Error carrying the class SY-4 and ADR-0004 revision
// 2 assign it. A JMAP MethodError (a per-call failure embedded in an
// otherwise-200 response) is not this function's concern;
// isCannotCalculateChanges and isStateMismatch classify those against
// the sync engine's own specific signals.
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

// classifyStatus maps the HTTP status of a rejected JMAP request to
// the uerr.Class SY-4 and ADR-0004 revision 2 assign it (401/403 a
// rejected credential, 404 a missing entity, 429 throttling, 5xx a
// server-side failure), wrapping cause in a uerr.Error. It reports nil
// for a status none of those classes cover, so both classify (a
// *jmap.RequestError from a completed round trip) and fetchSession (a
// raw HTTP status before go-jmap ever builds one) share the same
// mapping.
func classifyStatus(op string, status int, cause error) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return uerr.New(op, nil, uerr.ClassAuth, cause)
	case status == http.StatusNotFound:
		return uerr.New(op, nil, uerr.ClassNotFound, cause)
	case status == http.StatusTooManyRequests:
		return uerr.New(op, nil, uerr.ClassThrottled, cause)
	case status >= http.StatusInternalServerError:
		return uerr.New(op, nil, uerr.ClassServer, cause)
	default:
		return nil
	}
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

// classifyMutationFailure wraps one Email/set mutation's raw
// SetError type in a uerr.Error under jmapSetErrorClass's mapping, so
// the outbox dispatcher (task 10) can branch on a closed class
// instead of parsing a protocol string. setErrorType survives as
// Cause, so outbox.failure_detail can still record exactly what the
// server said.
func classifyMutationFailure(op, setErrorType string) error {
	class, ok := jmapSetErrorClass[setErrorType]
	if !ok {
		class = uerr.ClassServer
	}
	return uerr.New(op, nil, class, errors.New(setErrorType))
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
