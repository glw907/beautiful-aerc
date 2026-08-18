package jmapsource

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/glw907/poplar/internal/uerr"
)

// errNoStaticToken is Token's cause when a static credential (v1's
// non-expiring Fastmail app token) was never configured.
var errNoStaticToken = errors.New("jmapsource: credentials: no token configured")

// Credentials owns a JMAP backend's token lifecycle (ADR-0004
// revision 2): Token returns a valid credential, running RefreshFunc
// first when the cached one has expired, and collapsing concurrent
// callers onto one in-flight refresh. A nil RefreshFunc marks a
// static credential with nothing to refresh, matching Fastmail's v1
// app token, which never expires; Token is then a lock and a
// comparison, never a network call.
type Credentials struct {
	RefreshFunc func(ctx context.Context) (token string, expiresAt time.Time, err error)

	mu        sync.Mutex
	token     string
	expiresAt time.Time
	inFlight  *tokenRefresh
}

type tokenRefresh struct {
	done  chan struct{}
	token string
	err   error
}

// NewStaticCredentials returns Credentials that always report token
// and never refresh.
func NewStaticCredentials(token string) *Credentials {
	return &Credentials{token: token}
}

// Token implements backend.Credentials.
func (c *Credentials) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	if token, ok := c.validLocked(); ok {
		c.mu.Unlock()
		return token, nil
	}
	if c.inFlight != nil {
		refresh := c.inFlight
		c.mu.Unlock()
		select {
		case <-refresh.done:
			return refresh.token, refresh.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if c.RefreshFunc == nil {
		token := c.token
		c.mu.Unlock()
		if token == "" {
			return "", uerr.New("jmapsource.credentials", nil, uerr.ClassAuth, errNoStaticToken)
		}
		return token, nil
	}
	refresh := &tokenRefresh{done: make(chan struct{})}
	c.inFlight = refresh
	c.mu.Unlock()

	token, expiresAt, err := c.RefreshFunc(ctx)
	if err != nil {
		err = uerr.New("jmapsource.credentials", nil, uerr.ClassAuthRefreshFailed, err)
	}

	c.mu.Lock()
	if err == nil {
		c.token, c.expiresAt = token, expiresAt
	}
	c.inFlight = nil
	c.mu.Unlock()

	refresh.token, refresh.err = token, err
	close(refresh.done)
	return token, err
}

// validLocked reports whether c's cached token is still usable.
// Caller holds c.mu.
func (c *Credentials) validLocked() (string, bool) {
	if c.token == "" {
		return "", false
	}
	if c.expiresAt.IsZero() || time.Now().Before(c.expiresAt) {
		return c.token, true
	}
	return "", false
}
