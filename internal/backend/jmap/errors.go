package jmap

import (
	"errors"

	"git.sr.ht/~rockorager/go-jmap"
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
