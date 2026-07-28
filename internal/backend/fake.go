package backend

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"sync"
)

// FakeSource scripts one collection's Changes and ApplyBatch, the
// operations every source shares. A nil func returns a zero-value
// success; Changes and ApplyBatch are exercised directly in every
// scripted test, so an unscripted call succeeding with an empty
// result is the useful default here (unlike Mail's and Calendar's
// action methods below).
type FakeSource struct {
	mu sync.Mutex

	ChangesFunc    func(ctx context.Context, kind ObjectKind, token string, limit int) (ChangeSet, error)
	ApplyBatchFunc func(ctx context.Context, mutations []Mutation) (BatchResult, error)

	calls []string
}

// Changes implements Source, recording the call and delegating to
// ChangesFunc.
func (s *FakeSource) Changes(ctx context.Context, kind ObjectKind, token string, limit int) (ChangeSet, error) {
	s.record("Changes")
	if s.ChangesFunc != nil {
		return s.ChangesFunc(ctx, kind, token, limit)
	}
	return ChangeSet{NewToken: token}, nil
}

// ApplyBatch implements Source, recording the call and delegating to
// ApplyBatchFunc.
func (s *FakeSource) ApplyBatch(ctx context.Context, mutations []Mutation) (BatchResult, error) {
	s.record("ApplyBatch")
	if s.ApplyBatchFunc != nil {
		return s.ApplyBatchFunc(ctx, mutations)
	}
	return BatchResult{}, nil
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
	return fmt.Errorf("backend: %s.%s called with no %s set", receiver, method, field)
}

// FakeMail is the scriptable Mail source.
type FakeMail struct {
	FakeSource

	FetchBodiesFunc   func(ctx context.Context, ids []string) (iter.Seq[BodyChunk], error)
	SubmitFunc        func(ctx context.Context, raw []byte) (SubmitResult, error)
	CreateMailboxFunc func(ctx context.Context, name, parentID string) (string, error)
	RenameMailboxFunc func(ctx context.Context, id, name string) error
	DeleteMailboxFunc func(ctx context.Context, id string) error
	SearchFunc        func(ctx context.Context, query string) ([]string, error)
}

// FetchBodies delegates to FetchBodiesFunc.
func (m *FakeMail) FetchBodies(ctx context.Context, ids []string) (iter.Seq[BodyChunk], error) {
	if m.FetchBodiesFunc == nil {
		return nil, unscripted("FakeMail", "FetchBodies", "FetchBodiesFunc")
	}
	return m.FetchBodiesFunc(ctx, ids)
}

// Submit delegates to SubmitFunc.
func (m *FakeMail) Submit(ctx context.Context, raw []byte) (SubmitResult, error) {
	if m.SubmitFunc == nil {
		return SubmitResult{}, unscripted("FakeMail", "Submit", "SubmitFunc")
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

// FakePush is the scriptable Push source. ListenFunc drives
// TestFakeScripting's push-drop condition: return a channel the
// script closes to simulate the transport dropping.
type FakePush struct {
	ListenFunc func(ctx context.Context) (<-chan Notification, error)
}

// Listen delegates to ListenFunc. With ListenFunc unset, it returns a
// channel that stays open until ctx is done, matching a live
// transport that has not dropped, rather than one that already has.
func (p *FakePush) Listen(ctx context.Context) (<-chan Notification, error) {
	if p.ListenFunc != nil {
		return p.ListenFunc(ctx)
	}
	ch := make(chan Notification)
	context.AfterFunc(ctx, func() { close(ch) })
	return ch, nil
}

// Fake is a scriptable Backend, ADR-0014's second implementation of
// the seam: the sync engine and outbox run against it under
// testing/synctest, driven through the same interface the real
// backends serve. MailSource and CredentialsSource are always
// present; CalendarSource, ContactsSource, and PushSource are nil
// until a test sets them, matching a backend that declares no such
// source.
type Fake struct {
	Caps Capabilities

	MailSource        FakeMail
	CredentialsSource FakeCredentials
	CalendarSource    *FakeCalendar
	ContactsSource    *FakeContacts
	PushSource        *FakePush
}

// Mail returns f's mail source.
func (f *Fake) Mail() Mail { return &f.MailSource }

// Calendar returns f's calendar source, or nil if CalendarSource is
// unset.
func (f *Fake) Calendar() Calendar {
	if f.CalendarSource == nil {
		return nil
	}
	return f.CalendarSource
}

// Contacts returns f's contacts source, or nil if ContactsSource is
// unset.
func (f *Fake) Contacts() Contacts {
	if f.ContactsSource == nil {
		return nil
	}
	return f.ContactsSource
}

// Push returns f's push source, or nil if PushSource is unset.
func (f *Fake) Push() Push {
	if f.PushSource == nil {
		return nil
	}
	return f.PushSource
}

// Capabilities returns f.Caps.
func (f *Fake) Capabilities() Capabilities { return f.Caps }

// Credentials returns f's credential source.
func (f *Fake) Credentials() Credentials { return &f.CredentialsSource }

var _ Backend = (*Fake)(nil)
