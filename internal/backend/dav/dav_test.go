package dav

import (
	"context"
	"errors"
	"testing"

	"github.com/glw907/poplar/internal/backend"
)

func TestClientConformance(t *testing.T) {
	c := &Client{}
	ctx := context.Background()

	if _, err := c.Changes(ctx, backend.ObjectKindEvent, "", 0); !errors.Is(err, errNotImplemented) {
		t.Errorf("Changes() error = %v, want errNotImplemented", err)
	}
	if _, err := c.ApplyBatch(ctx, nil); !errors.Is(err, errNotImplemented) {
		t.Errorf("ApplyBatch() error = %v, want errNotImplemented", err)
	}
	if err := c.Respond(ctx, "evt", "ACCEPTED"); !errors.Is(err, errNotImplemented) {
		t.Errorf("Respond() error = %v, want errNotImplemented", err)
	}
}
