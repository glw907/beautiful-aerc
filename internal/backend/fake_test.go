package backend

import (
	"context"
	"errors"
	"testing"

	"github.com/glw907/poplar/internal/uerr"
)

func TestFakeScripting(t *testing.T) {
	t.Run("state reset", func(t *testing.T) {
		f := &Fake{}
		f.MailSource.ChangesFunc = func(context.Context, string) (ChangeSet, error) {
			return ChangeSet{}, ErrStateReset
		}

		_, err := f.Mail().Changes(context.Background(), "stale-token")
		if !errors.Is(err, ErrStateReset) {
			t.Fatalf("Changes() error = %v, want ErrStateReset", err)
		}
		if got := f.MailSource.Calls(); len(got) != 1 || got[0] != "Changes" {
			t.Errorf("Calls() = %v, want [Changes]", got)
		}
	})

	t.Run("412 state mismatch", func(t *testing.T) {
		f := &Fake{}
		f.MailSource.ApplyBatchFunc = func(context.Context, []Mutation) (BatchResult, error) {
			return BatchResult{}, ErrStateMismatch
		}

		mutations := []Mutation{{Op: MutationCreate, CreationID: "c1"}}
		_, err := f.Mail().ApplyBatch(context.Background(), mutations)
		if !errors.Is(err, ErrStateMismatch) {
			t.Fatalf("ApplyBatch() error = %v, want ErrStateMismatch", err)
		}
	})

	t.Run("push drop", func(t *testing.T) {
		dropped := make(chan Notification)
		close(dropped)
		f := &Fake{PushSource: &FakePush{
			ListenFunc: func(context.Context) (<-chan Notification, error) {
				return dropped, nil
			},
		}}

		ch, err := f.Push().Listen(context.Background())
		if err != nil {
			t.Fatalf("Listen() error = %v", err)
		}
		if _, ok := <-ch; ok {
			t.Fatal("Listen() channel still open, want it closed to signal a dropped transport")
		}
	})

	t.Run("throttled first sync", func(t *testing.T) {
		f := &Fake{}
		f.MailSource.ChangesFunc = func(_ context.Context, token string) (ChangeSet, error) {
			if token != "" {
				t.Fatalf("Changes(token = %q), want an empty token for a first sync", token)
			}
			return ChangeSet{}, uerr.New("mail changes", nil, uerr.ClassThrottled, errors.New("rate limited"))
		}

		_, err := f.Mail().Changes(context.Background(), "")
		var uerrErr uerr.Error
		if !errors.As(err, &uerrErr) || uerrErr.Class != uerr.ClassThrottled {
			t.Fatalf("Changes() error = %v, want a uerr.Error classed ClassThrottled", err)
		}
	})
}

// TestCapabilityDefaults asserts a backend that declares no calendar
// (the Fake's zero value) returns a nil Calendar(), and that a
// caller checking before use never reaches a nil-pointer call.
func TestCapabilityDefaults(t *testing.T) {
	var b Backend = &Fake{}

	if cal := b.Calendar(); cal != nil {
		t.Fatalf("Calendar() = %v, want nil for a backend with no calendar source", cal)
	}

	// A caller guards on the nil, the same check the calendar engine
	// makes before it ever dispatches to Respond.
	if cal := b.Calendar(); cal != nil {
		_ = cal.Respond(context.Background(), "evt", "ACCEPTED")
	}
}
