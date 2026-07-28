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
// 2 assign it: an HTTP 401/403 is a rejected credential, 404 is a
// missing entity, 429 is throttling, 5xx is a server-side failure,
// and a dead TCP connection is unreachable. A JMAP MethodError (a
// per-call failure embedded in an otherwise-200 response) is not this
// function's concern; isCannotCalculateChanges and isStateMismatch
// classify those against the sync engine's own specific signals.
func classify(op string, err error) error {
	if err == nil {
		return nil
	}
	if re, ok := errors.AsType[*jmap.RequestError](err); ok {
		switch {
		case re.Status == http.StatusUnauthorized || re.Status == http.StatusForbidden:
			return uerr.New(op, nil, uerr.ClassAuth, err)
		case re.Status == http.StatusNotFound:
			return uerr.New(op, nil, uerr.ClassNotFound, err)
		case re.Status == http.StatusTooManyRequests:
			return uerr.New(op, nil, uerr.ClassThrottled, err)
		case re.Status >= http.StatusInternalServerError:
			return uerr.New(op, nil, uerr.ClassServer, err)
		}
	}
	if isConnectionDead(err) {
		return uerr.New(op, nil, uerr.ClassConnection, err)
	}
	return err
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
