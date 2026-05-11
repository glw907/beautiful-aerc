package mailimap

import (
	"errors"
	"io"
	"net"

	"github.com/emersion/go-imap/v2"

	"github.com/glw907/poplar/internal/mail"
)

// classifyErr wraps an IMAP transport error with the poplar-internal
// sentinels (mail.ErrAuth, mail.ErrNotFound, mail.ErrConnection) so
// the cache drainer can route on errors.Is. Pass-through for
// unrecognized shapes.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	if isConnectionDead(err) {
		return mail.WrapSentinel(err, mail.ErrConnection)
	}
	var ie *imap.Error
	if errors.As(err, &ie) {
		switch ie.Code {
		case imap.ResponseCodeAuthenticationFailed,
			imap.ResponseCodeAuthorizationFailed,
			imap.ResponseCodePrivacyRequired:
			return mail.WrapSentinel(err, mail.ErrAuth)
		case imap.ResponseCodeNonExistent:
			return mail.WrapSentinel(err, mail.ErrNotFound)
		}
	}
	return err
}

// isConnectionDead reports whether err comes from the underlying TCP
// connection being closed or having timed out. Mirrors the filters in
// imapclient.Client.Close.
func isConnectionDead(err error) bool {
	if errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, net.ErrClosed) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	var oe *net.OpError
	return errors.As(err, &oe)
}
