package backendtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
)

func TestFakeScripting(t *testing.T) {
	t.Run("state reset", func(t *testing.T) {
		f := &Fake{}
		f.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			return backend.ChangeSet{}, backend.ErrStateReset
		}

		_, err := f.Mail().Changes(context.Background(), backend.ObjectKindMessage, "stale-token", 0)
		if !errors.Is(err, backend.ErrStateReset) {
			t.Fatalf("Changes() error = %v, want ErrStateReset", err)
		}
		if got := f.MailSource.Calls(); len(got) != 1 || got[0] != "Changes" {
			t.Errorf("Calls() = %v, want [Changes]", got)
		}
	})

	t.Run("412 state mismatch", func(t *testing.T) {
		f := &Fake{}
		f.MailSource.ApplyBatchFunc = func(context.Context, []backend.Mutation) (backend.BatchResult, error) {
			return backend.BatchResult{}, backend.ErrStateMismatch
		}

		mutations := []backend.Mutation{{Op: backend.MutationCreate, CreationID: "c1"}}
		_, err := f.Mail().ApplyBatch(context.Background(), mutations)
		if !errors.Is(err, backend.ErrStateMismatch) {
			t.Fatalf("ApplyBatch() error = %v, want ErrStateMismatch", err)
		}
	})

	t.Run("push drop", func(t *testing.T) {
		dropped := make(chan backend.Notification)
		close(dropped)
		f := &Fake{PushSource: &FakePush{
			ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
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
		f.MailSource.ChangesFunc = func(_ context.Context, _ backend.ObjectKind, token string, _ int) (backend.ChangeSet, error) {
			if token != "" {
				t.Fatalf("Changes(token = %q), want an empty token for a first sync", token)
			}
			return backend.ChangeSet{}, uerr.New("mail changes", nil, uerr.ClassThrottled, errors.New("rate limited"))
		}

		_, err := f.Mail().Changes(context.Background(), backend.ObjectKindMessage, "", 0)
		var uerrErr uerr.Error
		if !errors.As(err, &uerrErr) || uerrErr.Class != uerr.ClassThrottled {
			t.Fatalf("Changes() error = %v, want a uerr.Error classed ClassThrottled", err)
		}
	})
}

// TestCapabilityDefaults asserts a backend that declares no calendar
// (the Fake's zero value) returns a nil Calendar(), and that a
// backend with a scripted CalendarSource returns a non-nil Calendar()
// whose Respond call dispatches to the script.
func TestCapabilityDefaults(t *testing.T) {
	var b backend.Backend = &Fake{}

	if cal := b.Calendar(); cal != nil {
		t.Fatalf("Calendar() = %v, want nil for a backend with no calendar source", cal)
	}

	var respondCalled bool
	b = &Fake{CalendarSource: &FakeCalendar{
		RespondFunc: func(_ context.Context, id, partstat string) error {
			respondCalled = true
			if id != "evt" || partstat != "ACCEPTED" {
				t.Errorf("Respond(id = %q, partstat = %q), want (\"evt\", \"ACCEPTED\")", id, partstat)
			}
			return nil
		},
	}}

	cal := b.Calendar()
	if cal == nil {
		t.Fatal("Calendar() = nil, want the scripted CalendarSource")
	}
	if err := cal.Respond(context.Background(), "evt", "ACCEPTED"); err != nil {
		t.Fatalf("Respond() error = %v", err)
	}
	if !respondCalled {
		t.Fatal("Respond() did not dispatch to RespondFunc")
	}
}

// TestFakeMailUnscripted asserts every FakeMail action method fails
// loudly, naming its unset script field, rather than returning a
// quiet zero-value success that could mask an engine test never
// making the call at all.
func TestFakeMailUnscripted(t *testing.T) {
	m := &FakeMail{}
	ctx := context.Background()

	if _, err := m.FetchBodies(ctx, []string{"m1"}); err == nil {
		t.Error("FetchBodies() error = nil, want an unscripted-call error")
	}
	if _, err := m.Submit(ctx, []byte("raw")); err == nil {
		t.Error("Submit() error = nil, want an unscripted-call error")
	}
	if _, err := m.CreateMailbox(ctx, "Archive", ""); err == nil {
		t.Error("CreateMailbox() error = nil, want an unscripted-call error")
	}
	if err := m.RenameMailbox(ctx, "mb1", "Archive"); err == nil {
		t.Error("RenameMailbox() error = nil, want an unscripted-call error")
	}
	if err := m.DeleteMailbox(ctx, "mb1"); err == nil {
		t.Error("DeleteMailbox() error = nil, want an unscripted-call error")
	}
	if _, err := m.Search(ctx, "subject:hello"); err == nil {
		t.Error("Search() error = nil, want an unscripted-call error")
	}

	c := &FakeCalendar{}
	if err := c.Respond(ctx, "evt", "ACCEPTED"); err == nil {
		t.Error("Respond() error = nil, want an unscripted-call error")
	}
}

// TestFakePushListenOpenUntilCancel asserts an unscripted FakePush
// models a live transport, not a dropped one: its channel stays open
// until the caller's context is done, then closes.
func TestFakePushListenOpenUntilCancel(t *testing.T) {
	p := &FakePush{}
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := p.Listen(ctx)
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	select {
	case _, ok := <-ch:
		t.Fatalf("Listen() channel ready before cancel (ok = %v), want it to block", ok)
	default:
	}

	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("Listen() channel delivered a value, want it closed")
		}
	case <-time.After(time.Second):
		t.Fatal("Listen() channel did not close after context cancellation")
	}
}

// TestFakeCredentials asserts Fake.Credentials() is never nil and
// that Token dispatches to a scripted TokenFunc.
func TestFakeCredentials(t *testing.T) {
	f := &Fake{}
	if f.Credentials() == nil {
		t.Fatal("Credentials() = nil, want a non-nil credential source")
	}

	tok, err := f.Credentials().Token(context.Background())
	if err != nil || tok != "" {
		t.Fatalf("Token() = (%q, %v), want (\"\", nil) for an unscripted TokenFunc", tok, err)
	}

	f.CredentialsSource.TokenFunc = func(context.Context) (string, error) {
		return "scripted-token", nil
	}
	tok, err = f.Credentials().Token(context.Background())
	if err != nil || tok != "scripted-token" {
		t.Fatalf("Token() = (%q, %v), want (\"scripted-token\", nil)", tok, err)
	}
}

// TestCapabilitiesFields asserts the server-limit and scheduled-send
// facts a live session reports round-trip through Fake.Capabilities().
func TestCapabilitiesFields(t *testing.T) {
	caps := backend.Capabilities{
		ScheduledSend: true,
		Limits: backend.ServerLimits{
			MaxCallsInRequest:     16,
			MaxConcurrentRequests: 4,
		},
	}
	f := &Fake{Caps: caps}

	got := f.Capabilities()
	if !got.ScheduledSend {
		t.Error("Capabilities().ScheduledSend = false, want true")
	}
	if got.Limits.MaxCallsInRequest != 16 {
		t.Errorf("Capabilities().Limits.MaxCallsInRequest = %d, want 16", got.Limits.MaxCallsInRequest)
	}
	if got.Limits.MaxConcurrentRequests != 4 {
		t.Errorf("Capabilities().Limits.MaxConcurrentRequests = %d, want 4", got.Limits.MaxConcurrentRequests)
	}
}
