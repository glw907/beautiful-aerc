// Package backendtest is the scriptable second implementation of
// internal/backend's seam (ADR-0014), the net/http/httptest precedent
// applied to the backend interface: a test double that ships beside
// its interface rather than inside it, so it never compiles into the
// shipped binary and no production package can reference it.
package backendtest

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"sync"

	"github.com/glw907/poplar/internal/backend"
)

// FakeSource scripts one collection's Changes and ApplyBatch, the
// operations every source shares. A nil func returns a zero-value
// success; Changes and ApplyBatch are exercised directly in every
// scripted test, so an unscripted call succeeding with an empty
// result is the useful default here (unlike Mail's and Calendar's
// action methods below).
type FakeSource struct {
	mu sync.Mutex

	ChangesFunc    func(ctx context.Context, kind backend.ObjectKind, token string, limit int) (backend.ChangeSet, error)
	ApplyBatchFunc func(ctx context.Context, mutations []backend.Mutation) (backend.BatchResult, error)

	calls []string
}

// Changes implements backend.Source, recording the call and
// delegating to ChangesFunc.
func (s *FakeSource) Changes(ctx context.Context, kind backend.ObjectKind, token string, limit int) (backend.ChangeSet, error) {
	s.record("Changes")
	if s.ChangesFunc != nil {
		return s.ChangesFunc(ctx, kind, token, limit)
	}
	return backend.ChangeSet{NewToken: token}, nil
}

// ApplyBatch implements backend.Source, recording the call and
// delegating to ApplyBatchFunc.
func (s *FakeSource) ApplyBatch(ctx context.Context, mutations []backend.Mutation) (backend.BatchResult, error) {
	s.record("ApplyBatch")
	if s.ApplyBatchFunc != nil {
		return s.ApplyBatchFunc(ctx, mutations)
	}
	return backend.BatchResult{}, nil
}

func (s *FakeSource) record(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, name)
}

// Calls returns the method names invoked on s, in call order.
func (s *FakeSource) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

// unscripted reports a call to a Fake method whose script field is
// nil, naming the field a test needs to set. An engine test asserting
// against a Fake wants a call nobody scripted to fail loudly, not to
// return a quiet zero-value success that could pass against an engine
// that never made the call at all.
func unscripted(receiver, method, field string) error {
	return fmt.Errorf("backendtest: %s.%s called with no %s set", receiver, method, field)
}

// FakeMail is the scriptable Mail source.
type FakeMail struct {
	FakeSource

	FetchBodiesFunc   func(ctx context.Context, ids []string) (iter.Seq[backend.BodyChunk], error)
	SubmitFunc        func(ctx context.Context, raw []byte) (backend.SubmitResult, error)
	CreateMailboxFunc func(ctx context.Context, name, parentID string) (string, error)
	RenameMailboxFunc func(ctx context.Context, id, name string) error
	DeleteMailboxFunc func(ctx context.Context, id string) error
	FindMailboxesFunc func(ctx context.Context, name, parentID string) ([]string, error)
	SearchFunc        func(ctx context.Context, query string) ([]string, error)
}

// FetchBodies delegates to FetchBodiesFunc.
func (m *FakeMail) FetchBodies(ctx context.Context, ids []string) (iter.Seq[backend.BodyChunk], error) {
	if m.FetchBodiesFunc == nil {
		return nil, unscripted("FakeMail", "FetchBodies", "FetchBodiesFunc")
	}
	return m.FetchBodiesFunc(ctx, ids)
}

// Submit delegates to SubmitFunc.
func (m *FakeMail) Submit(ctx context.Context, raw []byte) (backend.SubmitResult, error) {
	if m.SubmitFunc == nil {
		return backend.SubmitResult{}, unscripted("FakeMail", "Submit", "SubmitFunc")
	}
	return m.SubmitFunc(ctx, raw)
}

// CreateMailbox delegates to CreateMailboxFunc.
func (m *FakeMail) CreateMailbox(ctx context.Context, name, parentID string) (string, error) {
	if m.CreateMailboxFunc == nil {
		return "", unscripted("FakeMail", "CreateMailbox", "CreateMailboxFunc")
	}
	return m.CreateMailboxFunc(ctx, name, parentID)
}

// RenameMailbox delegates to RenameMailboxFunc.
func (m *FakeMail) RenameMailbox(ctx context.Context, id, name string) error {
	if m.RenameMailboxFunc == nil {
		return unscripted("FakeMail", "RenameMailbox", "RenameMailboxFunc")
	}
	return m.RenameMailboxFunc(ctx, id, name)
}

// DeleteMailbox delegates to DeleteMailboxFunc.
func (m *FakeMail) DeleteMailbox(ctx context.Context, id string) error {
	if m.DeleteMailboxFunc == nil {
		return unscripted("FakeMail", "DeleteMailbox", "DeleteMailboxFunc")
	}
	return m.DeleteMailboxFunc(ctx, id)
}

// FindMailboxes delegates to FindMailboxesFunc.
func (m *FakeMail) FindMailboxes(ctx context.Context, name, parentID string) ([]string, error) {
	if m.FindMailboxesFunc == nil {
		return nil, unscripted("FakeMail", "FindMailboxes", "FindMailboxesFunc")
	}
	return m.FindMailboxesFunc(ctx, name, parentID)
}

// Search delegates to SearchFunc.
func (m *FakeMail) Search(ctx context.Context, query string) ([]string, error) {
	if m.SearchFunc == nil {
		return nil, unscripted("FakeMail", "Search", "SearchFunc")
	}
	return m.SearchFunc(ctx, query)
}

// FakeCalendar is the scriptable Calendar source.
type FakeCalendar struct {
	FakeSource

	RespondFunc func(ctx context.Context, id, partstat string) error
}

// Respond delegates to RespondFunc.
func (c *FakeCalendar) Respond(ctx context.Context, id, partstat string) error {
	if c.RespondFunc == nil {
		return unscripted("FakeCalendar", "Respond", "RespondFunc")
	}
	return c.RespondFunc(ctx, id, partstat)
}

// FakeContacts is the scriptable Contacts source.
type FakeContacts struct {
	FakeSource
}

// FakeCredentials scripts Token, the credential seam every backend
// exposes (ADR-0004 revision 2). A nil TokenFunc returns an empty
// token and no error, matching a backend whose v1 credential is a
// static value with nothing to refresh.
type FakeCredentials struct {
	TokenFunc func(ctx context.Context) (string, error)
}

// Token delegates to TokenFunc.
func (c *FakeCredentials) Token(ctx context.Context) (string, error) {
	if c.TokenFunc != nil {
		return c.TokenFunc(ctx)
	}
	return "", nil
}

// FakePush is the scriptable Push source. ListenFunc returns one
// stream: a channel the script closes to end it, or an error for a
// stream the transport would not open. Under backend.Push's contract a
// closed channel is the transport stopping for good rather than a
// drop, and the reason comes back as the next Listen call's error, so
// a script that models a refusal returns the channel and then that
// error.
type FakePush struct {
	ListenFunc func(ctx context.Context) (<-chan backend.Notification, error)
}

// Listen delegates to ListenFunc. With ListenFunc unset, it returns a
// channel that stays open until ctx is done, matching a live transport
// that is connected, rather than one that has already stopped.
func (p *FakePush) Listen(ctx context.Context) (<-chan backend.Notification, error) {
	if p.ListenFunc != nil {
		return p.ListenFunc(ctx)
	}
	ch := make(chan backend.Notification)
	context.AfterFunc(ctx, func() { close(ch) })
	return ch, nil
}

// Fake is a scriptable backend.Backend, ADR-0014's second
// implementation of the seam: the sync engine and outbox run against
// it under testing/synctest, driven through the same interface the
// real backends serve. MailSource and CredentialsSource are always
// present; CalendarSource, ContactsSource, and PushSource are nil
// until a test sets them, matching a backend that declares no such
// source.
type Fake struct {
	Caps backend.Capabilities

	MailSource        FakeMail
	CredentialsSource FakeCredentials
	CalendarSource    *FakeCalendar
	ContactsSource    *FakeContacts
	PushSource        *FakePush
}

// Mail returns f's mail source.
func (f *Fake) Mail() backend.Mail { return &f.MailSource }

// Calendar returns f's calendar source, or nil if CalendarSource is
// unset.
func (f *Fake) Calendar() backend.Calendar {
	if f.CalendarSource == nil {
		return nil
	}
	return f.CalendarSource
}

// Contacts returns f's contacts source, or nil if ContactsSource is
// unset.
func (f *Fake) Contacts() backend.Contacts {
	if f.ContactsSource == nil {
		return nil
	}
	return f.ContactsSource
}

// Push returns f's push source, or nil if PushSource is unset.
func (f *Fake) Push() backend.Push {
	if f.PushSource == nil {
		return nil
	}
	return f.PushSource
}

// Capabilities returns f.Caps.
func (f *Fake) Capabilities() backend.Capabilities { return f.Caps }

// Credentials returns f's credential source.
func (f *Fake) Credentials() backend.Credentials { return &f.CredentialsSource }

var _ backend.Backend = (*Fake)(nil)
