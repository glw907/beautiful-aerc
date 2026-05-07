package mailjmap

import (
	"errors"

	"git.sr.ht/~rockorager/go-jmap"

	"github.com/glw907/poplar/internal/mail"
)

// do calls the underlying JMAP client and routes the error through
// classifyErr so callers see typed sentinels for auth/notfound.
func (b *Backend) do(req *jmap.Request) (*jmap.Response, error) {
	resp, err := b.client.Do(req)
	return resp, classifyErr(err)
}

// classifyErr wraps a JMAP transport/method error with the
// poplar-internal sentinels (mail.ErrAuth, mail.ErrNotFound) so the
// cache drainer can route on errors.Is. Pass-through for unrecognized
// shapes.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	var re *jmap.RequestError
	if errors.As(err, &re) {
		switch re.Status {
		case 401, 403:
			return errJoin(err, mail.ErrAuth)
		case 404:
			return errJoin(err, mail.ErrNotFound)
		}
	}
	return err
}

// errJoin wraps two errors so errors.Is matches the sentinel and
// the original message stays in Error().
func errJoin(orig, sentinel error) error { return joined{orig: orig, sentinel: sentinel} }

type joined struct {
	orig     error
	sentinel error
}

func (j joined) Error() string { return j.orig.Error() }
func (j joined) Unwrap() []error {
	return []error{j.orig, j.sentinel}
}
