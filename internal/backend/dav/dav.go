// Package dav is CalDAV's calendar transport. This pass ships
// interface conformance only: Client's methods return
// errNotImplemented and the package makes no network call. Live
// CalDAV entry lands in the calendar pass.
package dav

import (
	"context"
	"errors"

	"github.com/glw907/poplar/internal/backend"
)

var errNotImplemented = errors.New("dav: not implemented")

// Client is CalDAV's Calendar source.
type Client struct{}

var _ backend.Calendar = (*Client)(nil)

func (c *Client) Changes(context.Context, string) (backend.ChangeSet, error) {
	return backend.ChangeSet{}, errNotImplemented
}

func (c *Client) ApplyBatch(context.Context, []backend.Mutation) (backend.BatchResult, error) {
	return backend.BatchResult{}, errNotImplemented
}

func (c *Client) Respond(context.Context, string, string) error {
	return errNotImplemented
}
