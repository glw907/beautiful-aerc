package backend

import (
	"context"
	"slices"
	"sync"
)

// FakeSource scripts one collection's Changes and ApplyBatch, the
// operations every source shares. A nil func returns a zero-value
// success.
type FakeSource struct {
	mu sync.Mutex

	ChangesFunc    func(ctx context.Context, token string) (ChangeSet, error)
	ApplyBatchFunc func(ctx context.Context, mutations []Mutation) (BatchResult, error)

	calls []string
}

// Changes implements Source, recording the call and delegating to
// ChangesFunc.
func (s *FakeSource) Changes(ctx context.Context, token string) (ChangeSet, error) {
	s.record("Changes")
	if s.ChangesFunc != nil {
		return s.ChangesFunc(ctx, token)
	}
	return ChangeSet{Token: token}, nil
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

// FakeMail is the scriptable Mail source.
type FakeMail struct {
	FakeSource

	FetchBodiesFunc   func(ctx context.Context, ids []string) (map[string][]byte, error)
	SubmitFunc        func(ctx context.Context, raw []byte) (SubmitResult, error)
	CreateMailboxFunc func(ctx context.Context, name, parentID string) (string, error)
	RenameMailboxFunc func(ctx context.Context, id, name string) error
	DeleteMailboxFunc func(ctx context.Context, id string) error
	SearchFunc        func(ctx context.Context, query string) ([]string, error)
}

func (m *FakeMail) FetchBodies(ctx context.Context, ids []string) (map[string][]byte, error) {
	if m.FetchBodiesFunc != nil {
		return m.FetchBodiesFunc(ctx, ids)
	}
	return nil, nil
}

func (m *FakeMail) Submit(ctx context.Context, raw []byte) (SubmitResult, error) {
	if m.SubmitFunc != nil {
		return m.SubmitFunc(ctx, raw)
	}
	return SubmitResult{}, nil
}

func (m *FakeMail) CreateMailbox(ctx context.Context, name, parentID string) (string, error) {
	if m.CreateMailboxFunc != nil {
		return m.CreateMailboxFunc(ctx, name, parentID)
	}
	return "", nil
}

func (m *FakeMail) RenameMailbox(ctx context.Context, id, name string) error {
	if m.RenameMailboxFunc != nil {
		return m.RenameMailboxFunc(ctx, id, name)
	}
	return nil
}

func (m *FakeMail) DeleteMailbox(ctx context.Context, id string) error {
	if m.DeleteMailboxFunc != nil {
		return m.DeleteMailboxFunc(ctx, id)
	}
	return nil
}

func (m *FakeMail) Search(ctx context.Context, query string) ([]string, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, query)
	}
	return nil, nil
}

// FakeCalendar is the scriptable Calendar source.
type FakeCalendar struct {
	FakeSource

	RespondFunc func(ctx context.Context, id, partstat string) error
}

func (c *FakeCalendar) Respond(ctx context.Context, id, partstat string) error {
	if c.RespondFunc != nil {
		return c.RespondFunc(ctx, id, partstat)
	}
	return nil
}

// FakeContacts is the scriptable Contacts source.
type FakeContacts struct {
	FakeSource
}

// FakePush is the scriptable Push source. ListenFunc drives
// TestFakeScripting's push-drop condition: return a channel the
// script closes to simulate the transport dropping.
type FakePush struct {
	ListenFunc func(ctx context.Context) (<-chan Notification, error)
}

func (p *FakePush) Listen(ctx context.Context) (<-chan Notification, error) {
	if p.ListenFunc != nil {
		return p.ListenFunc(ctx)
	}
	ch := make(chan Notification)
	close(ch)
	return ch, nil
}

// Fake is a scriptable Backend, ADR-0014's second implementation of
// the seam: the sync engine and outbox run against it under
// testing/synctest, driven through the same interface the real
// backends serve. MailSource is always present; CalendarSource,
// ContactsSource, and PushSource are nil until a test sets them,
// matching a backend that declares no such source.
type Fake struct {
	Caps Capabilities

	MailSource     FakeMail
	CalendarSource *FakeCalendar
	ContactsSource *FakeContacts
	PushSource     *FakePush
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

var _ Backend = (*Fake)(nil)
