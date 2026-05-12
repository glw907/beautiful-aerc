package mailimap

import (
	"errors"

	"github.com/emersion/go-imap/v2"
	gosmtp "github.com/emersion/go-smtp"

	"github.com/glw907/poplar/internal/mail"
)

// classifyErr wraps a transport error with the poplar-internal
// sentinels (mail.ErrAuth, mail.ErrNotFound, mail.ErrConnection) so
// the cache drainer can route on errors.Is. Recognizes both IMAP
// (*imap.Error) and SMTP (*gosmtp.SMTPError) shapes; pass-through
// for unrecognized errors.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	if mail.IsConnectionDead(err) {
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
	var se *gosmtp.SMTPError
	if errors.As(err, &se) {
		// 530 not authenticated, 535 bad credentials, 538 encryption
		// required for requested auth mechanism.
		switch se.Code {
		case 530, 535, 538:
			return mail.WrapSentinel(err, mail.ErrAuth)
		}
	}
	return err
}
