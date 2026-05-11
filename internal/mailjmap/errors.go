package mailjmap

import (
	"errors"

	"git.sr.ht/~rockorager/go-jmap"

	"github.com/glw907/poplar/internal/mail"
)

// do calls the JMAP client and routes errors through classifyErr so
// callers see the typed mail.Err* sentinels.
func (b *Backend) do(req *jmap.Request) (*jmap.Response, error) {
	resp, err := b.client.Do(req)
	return resp, classifyErr(err)
}

// classifyErr wraps a JMAP transport or method error with the
// mail.Err* sentinels so the cache drainer can route on errors.Is.
// Unrecognized shapes pass through.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	var re *jmap.RequestError
	if errors.As(err, &re) {
		switch re.Status {
		case 401, 403:
			return mail.WrapSentinel(err, mail.ErrAuth)
		case 404:
			return mail.WrapSentinel(err, mail.ErrNotFound)
		}
	}
	if mail.IsConnectionDead(err) {
		return mail.WrapSentinel(err, mail.ErrConnection)
	}
	return err
}
