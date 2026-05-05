// SPDX-License-Identifier: MIT

package mailimap

import (
	"errors"

	"github.com/emersion/go-imap/v2"

	"github.com/glw907/poplar/internal/mail"
)

// classifyErr wraps an IMAP transport error with the poplar-internal
// sentinels (mail.ErrAuth, mail.ErrNotFound) so the cache drainer
// can route on errors.Is. Pass-through for unrecognized shapes.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	var ie *imap.Error
	if errors.As(err, &ie) {
		switch ie.Code {
		case imap.ResponseCodeAuthenticationFailed,
			imap.ResponseCodeAuthorizationFailed,
			imap.ResponseCodePrivacyRequired:
			return wrapAuth(err)
		case imap.ResponseCodeNonExistent:
			return wrapNotFound(err)
		}
	}
	return err
}

func wrapAuth(err error) error     { return joined{orig: err, sentinel: mail.ErrAuth} }
func wrapNotFound(err error) error { return joined{orig: err, sentinel: mail.ErrNotFound} }

type joined struct {
	orig     error
	sentinel error
}

func (j joined) Error() string   { return j.orig.Error() }
func (j joined) Unwrap() []error { return []error{j.orig, j.sentinel} }
