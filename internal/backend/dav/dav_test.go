package dav

import (
	"context"
	"errors"
	"testing"
)

func TestClientConformance(t *testing.T) {
	c := &Client{}
	ctx := context.Background()

	if _, err := c.Changes(ctx, ""); !errors.Is(err, errNotImplemented) {
		t.Errorf("Changes() error = %v, want errNotImplemented", err)
	}
	if _, err := c.ApplyBatch(ctx, nil); !errors.Is(err, errNotImplemented) {
		t.Errorf("ApplyBatch() error = %v, want errNotImplemented", err)
	}
	if err := c.Respond(ctx, "evt", "ACCEPTED"); !errors.Is(err, errNotImplemented) {
		t.Errorf("Respond() error = %v, want errNotImplemented", err)
	}
}
