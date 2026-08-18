package jmapsource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/jmap"
)

// searchResultLimit bounds Search's Email/query so a broad term
// against a large mailbox returns one page rather than the whole
// match set; SR-7's ranking and further paging are the search
// engine's job (pass 3), not this transport's.
const searchResultLimit = 256

// rawMessageType is the media type an uploaded raw message blob
// carries, both for Submit's import and for downloading a message's
// source back.
const rawMessageType = "message/rfc822"

// mailSource implements backend.Mail against one Session.
type mailSource struct {
	session *Session
}

var _ backend.Mail = (*mailSource)(nil)

// ApplyBatch implements backend.Source, translating mutations into a
// Mailbox/set call for the mailboxes they create and an Email/set call
// for the messages they change, invoked in that order within one
// request so a message update can name a mailbox this same batch
// creates by its "#"+CreationID back-reference. A message
// MutationCreate fails per-mutation rather than the whole batch:
// message creation from structured fields is compose assembly (pass
// 4), so it is not supported here.
func (m *mailSource) ApplyBatch(ctx context.Context, mutations []backend.Mutation) (backend.BatchResult, error) {
	result := backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}
	mailboxes := &jmap.MailboxSet{Account: m.session.accountID, Create: map[jmap.ID]*jmap.Mailbox{}}
	messages := &jmap.EmailSet{Account: m.session.accountID, Update: map[jmap.ID]jmap.Patch{}}
	for _, mut := range mutations {
		switch mut.Kind {
		case backend.ObjectKindMailbox:
			if mut.Op != backend.MutationCreate {
				return backend.BatchResult{}, fmt.Errorf("jmapsource: apply batch: mailbox %v is RenameMailbox or DeleteMailbox", mut.Op)
			}
			create, ok := mut.Fields.(backend.MailboxCreate)
			if !ok {
				return backend.BatchResult{}, fmt.Errorf("jmapsource: apply batch: mailbox create carries %T, want backend.MailboxCreate", mut.Fields)
			}
			mailboxes.Create[jmap.ID(mut.CreationID)] = newMailbox(create)
		case backend.ObjectKindMessage:
			switch mut.Op {
			case backend.MutationUpdate:
				patch, ok := mut.Fields.(backend.MessagePatch)
				if !ok {
					return backend.BatchResult{}, fmt.Errorf("jmapsource: apply batch: message update carries %T, want backend.MessagePatch", mut.Fields)
				}
				messages.Update[jmap.ID(mut.ID)] = messagePatch(patch)
			case backend.MutationDestroy:
				messages.Destroy = append(messages.Destroy, jmap.ID(mut.ID))
			case backend.MutationCreate:
				result.Failed[mut.CreationID] = backend.Failure{
					Class: uerr.ClassServer,
					Cause: errors.New("jmapsource: message create needs compose assembly (pass 4)"),
				}
			default:
				return backend.BatchResult{}, fmt.Errorf("jmapsource: apply batch: unsupported op %v", mut.Op)
			}
		default:
			return backend.BatchResult{}, fmt.Errorf("jmapsource: apply batch: unsupported kind %v", mut.Kind)
		}
	}

	req := &jmap.Request{}
	var mailboxCall, messageCall string
	if len(mailboxes.Create) > 0 {
		mailboxCall = req.Invoke(mailboxes)
	}
	if len(messages.Update) > 0 || len(messages.Destroy) > 0 {
		messageCall = req.Invoke(messages)
	}
	if mailboxCall == "" && messageCall == "" {
		return result, nil
	}
	resp, err := m.session.do(ctx, req)
	if err != nil {
		return backend.BatchResult{}, fmt.Errorf("jmapsource: apply batch: %w", err)
	}

	if mailboxCall != "" {
		sr, err := findResponse[*jmap.MailboxSetResponse](resp, mailboxCall)
		if err != nil {
			return backend.BatchResult{}, batchError("mailbox/set", err)
		}
		for creationID, box := range sr.Created {
			result.Created[string(creationID)] = string(box.ID)
		}
		for creationID, se := range sr.NotCreated {
			result.Failed[string(creationID)] = classifyMailboxCreateFailure(se.Type)
		}
	}
	if messageCall == "" {
		return result, nil
	}
	sr, err := findResponse[*jmap.EmailSetResponse](resp, messageCall)
	if err != nil {
		return backend.BatchResult{}, batchError("email/set", err)
	}
	for id, se := range sr.NotUpdated {
		result.Failed[string(id)] = classifyMutationFailure(se.Type)
	}
	for id, se := range sr.NotDestroyed {
		if se.Type == "notFound" {
			continue
		}
		result.Failed[string(id)] = classifyMutationFailure(se.Type)
	}
	return result, nil
}

// batchError names call in err, translating the server's
// stateMismatch into the seam's own sentinel so a caller re-fetches
// state rather than reading a wire error.
func batchError(call string, err error) error {
	if isStateMismatch(err) {
		return backend.ErrStateMismatch
	}
	return fmt.Errorf("jmapsource: %s: %w", call, err)
}

// newMailbox translates a mailbox create's poplar-vocabulary fields
// into the wire object.
func newMailbox(create backend.MailboxCreate) *jmap.Mailbox {
	box := &jmap.Mailbox{Name: create.Name}
	if create.ParentID != "" {
		box.ParentID = jmap.ID(create.ParentID)
	}
	return box
}

// messagePatch translates a message update's poplar-vocabulary fields
// into a JMAP Patch. A keyword the patch says nothing about stays out
// of it, so a flag poplar did not change keeps whatever the server
// holds.
func messagePatch(p backend.MessagePatch) jmap.Patch {
	patch := jmap.Patch{}
	for flag, keyword := range backend.MessageFlagKeywords {
		switch {
		case p.SetFlags&flag != 0:
			patch[jmap.Pointer("keywords", keyword)] = true
		case p.ClearFlags&flag != 0:
			patch[jmap.Pointer("keywords", keyword)] = nil
		}
	}
	if p.MailboxIDs != nil {
		mailboxIDs := make(map[string]bool, len(p.MailboxIDs))
		for _, id := range p.MailboxIDs {
			mailboxIDs[id] = true
		}
		patch["mailboxIds"] = mailboxIDs
	}
	return patch
}

// FetchBodies implements backend.Mail, streaming each id's raw
// message source as its download completes.
func (m *mailSource) FetchBodies(ctx context.Context, ids []string) (iter.Seq[backend.BodyChunk], error) {
	blobIDs, err := m.session.resolveBlobIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return func(yield func(backend.BodyChunk) bool) {
		for _, id := range ids {
			blobID, ok := blobIDs[id]
			if !ok {
				if !yield(backend.BodyChunk{ID: id, Err: fmt.Errorf("jmapsource: fetch bodies: %s: no blob", id)}) {
					return
				}
				continue
			}
			raw, err := m.session.downloadBlob(ctx, blobID)
			if !yield(backend.BodyChunk{ID: id, Raw: raw, Err: err}) {
				return
			}
		}
	}, nil
}

// resolveBlobIDs returns each id's blobId via one Email/get. It holds
// no cache across calls: Changes already carries blobId into
// messageFields for the store to keep, so a per-process cache here
// would only duplicate that and grow without bound over a long
// session's lifetime.
func (s *Session) resolveBlobIDs(ctx context.Context, ids []string) (map[string]jmap.ID, error) {
	wireIDs := make([]jmap.ID, len(ids))
	for i, id := range ids {
		wireIDs[i] = jmap.ID(id)
	}

	req := &jmap.Request{}
	callID := req.Invoke(&jmap.EmailGet{Account: s.accountID, IDs: wireIDs, Properties: []string{"id", "blobId"}})
	resp, err := s.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("jmapsource: email/get blobid: %w", err)
	}
	get, err := findResponse[*jmap.EmailGetResponse](resp, callID)
	if err != nil {
		return nil, fmt.Errorf("jmapsource: email/get blobid: %w", err)
	}

	out := make(map[string]jmap.ID, len(get.List))
	for _, e := range get.List {
		out[string(e.ID)] = e.BlobID
	}
	return out, nil
}

func (s *Session) downloadBlob(ctx context.Context, blobID jmap.ID) ([]byte, error) {
	rc, err := s.client.Download(ctx, s.accountID, blobID, rawMessageType, "")
	if err != nil {
		return nil, fmt.Errorf("jmapsource: download: %w", err)
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

// Submit implements backend.Mail: uploads raw, imports it into the
// account's Sent mailbox, and creates an EmailSubmission referencing
// it, all as raw bytes with no MIME assembly (that is pass 4's
// compose feature, not this transport's).
func (m *mailSource) Submit(ctx context.Context, raw []byte) (backend.SubmitResult, error) {
	sentID, err := m.session.mailboxIDByRole(ctx, jmap.RoleSent)
	if err != nil {
		return backend.SubmitResult{}, err
	}
	identityID, err := m.session.defaultIdentityID(ctx)
	if err != nil {
		return backend.SubmitResult{}, err
	}
	upload, err := m.session.client.Upload(ctx, m.session.accountID, rawMessageType, bytes.NewReader(raw))
	if err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmapsource: submit: upload: %w", err)
	}

	req := &jmap.Request{}
	importCall := req.Invoke(&jmap.EmailImport{
		Account: m.session.accountID,
		Emails: map[jmap.ID]*jmap.EmailImportItem{
			"m1": {
				BlobID:     upload.BlobID,
				MailboxIDs: map[jmap.ID]bool{sentID: true},
				Keywords:   map[string]bool{"$seen": true},
			},
		},
	})
	submitCall := req.Invoke(&jmap.EmailSubmissionSet{
		Account: m.session.accountID,
		Create: map[jmap.ID]*jmap.EmailSubmission{
			"s1": {IdentityID: identityID, EmailID: jmap.ID("#m1")},
		},
	})
	resp, err := m.session.do(ctx, req)
	if err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmapsource: submit: %w", err)
	}
	if _, err := findResponse[*jmap.EmailImportResponse](resp, importCall); err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmapsource: submit: import: %w", err)
	}
	sr, err := findResponse[*jmap.EmailSubmissionSetResponse](resp, submitCall)
	if err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmapsource: submit: %w", err)
	}
	if se, bad := sr.NotCreated["s1"]; bad {
		return backend.SubmitResult{}, fmt.Errorf("jmapsource: submit: rejected: %s", se.Type)
	}
	created, ok := sr.Created["s1"]
	if !ok {
		return backend.SubmitResult{}, errors.New("jmapsource: submit: no submission created")
	}
	return backend.SubmitResult{ID: string(created.ID), Sent: true}, nil
}

func (s *Session) mailboxIDByRole(ctx context.Context, role jmap.Role) (jmap.ID, error) {
	req := &jmap.Request{}
	callID := req.Invoke(&jmap.MailboxQuery{Account: s.accountID, Filter: &jmap.MailboxFilterCondition{Role: role}})
	resp, err := s.do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("jmapsource: mailbox/query role %s: %w", role, err)
	}
	qr, err := findResponse[*jmap.MailboxQueryResponse](resp, callID)
	if err != nil {
		return "", fmt.Errorf("jmapsource: mailbox/query role %s: %w", role, err)
	}
	if len(qr.IDs) == 0 {
		return "", fmt.Errorf("jmapsource: no mailbox with role %s", role)
	}
	return qr.IDs[0], nil
}

func (s *Session) defaultIdentityID(ctx context.Context) (jmap.ID, error) {
	req := &jmap.Request{}
	callID := req.Invoke(&jmap.IdentityGet{Account: s.accountID})
	resp, err := s.do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("jmapsource: identity/get: %w", err)
	}
	gr, err := findResponse[*jmap.IdentityGetResponse](resp, callID)
	if err != nil {
		return "", fmt.Errorf("jmapsource: identity/get: %w", err)
	}
	if len(gr.List) == 0 {
		return "", errors.New("jmapsource: account has no identities")
	}
	return gr.List[0].ID, nil
}

// CreateMailbox implements backend.Mail.
func (m *mailSource) CreateMailbox(ctx context.Context, name, parentID string) (string, error) {
	box := &jmap.Mailbox{Name: name}
	if parentID != "" {
		box.ParentID = jmap.ID(parentID)
	}
	sr, err := m.session.mailboxSet(ctx, "create mailbox", &jmap.MailboxSet{
		Account: m.session.accountID,
		Create:  map[jmap.ID]*jmap.Mailbox{"m1": box},
	})
	if err != nil {
		return "", err
	}
	if se, bad := sr.NotCreated["m1"]; bad {
		// .Cause only: classifyMailboxCreateFailure returns a
		// backend.Failure, and a backend.Failure is uerr.Classified.
		// Returning the Failure itself here would let outbox's
		// classifyFailure read a Class off a create-path error that
		// was never meant to reach the dispatcher pre-classified,
		// which flips its retry/terminal decision. The Cause alone
		// carries the same text and the same errors.Is chain to
		// backend.ErrMailboxNameExists.
		return "", fmt.Errorf("jmapsource: create mailbox: rejected: %w", classifyMailboxCreateFailure(se.Type).Cause)
	}
	created, ok := sr.Created["m1"]
	if !ok {
		return "", errors.New("jmapsource: create mailbox: no created entry")
	}
	return string(created.ID), nil
}

// RenameMailbox implements backend.Mail.
func (m *mailSource) RenameMailbox(ctx context.Context, id, name string) error {
	sr, err := m.session.mailboxSet(ctx, "rename mailbox", &jmap.MailboxSet{
		Account: m.session.accountID,
		Update:  map[jmap.ID]jmap.Patch{jmap.ID(id): {"name": name}},
	})
	if err != nil {
		return err
	}
	if se, bad := sr.NotUpdated[jmap.ID(id)]; bad {
		return fmt.Errorf("jmapsource: rename mailbox: rejected: %s", se.Type)
	}
	return nil
}

// DeleteMailbox implements backend.Mail.
func (m *mailSource) DeleteMailbox(ctx context.Context, id string) error {
	sr, err := m.session.mailboxSet(ctx, "delete mailbox", &jmap.MailboxSet{
		Account: m.session.accountID,
		Destroy: []jmap.ID{jmap.ID(id)},
	})
	if err != nil {
		return err
	}
	if se, bad := sr.NotDestroyed[jmap.ID(id)]; bad && se.Type != "notFound" {
		return fmt.Errorf("jmapsource: delete mailbox: rejected: %s", se.Type)
	}
	return nil
}

// mailboxLookupProperties is what FindMailboxes needs off each
// candidate: the id it may return, and the two properties it matches
// exactly before returning one.
var mailboxLookupProperties = []string{"id", "name", "parentId"}

// FindMailboxes implements backend.Mail as one round trip: a
// Mailbox/query narrowed by name, and a Mailbox/get over that query's
// ids by back-reference. It reaches neither Mailbox/changes nor the
// account's whole mailbox list, so nothing here touches the state
// token the sync engine's watermark discipline owns.
//
// Narrowing is all the filter is trusted for. RFC 8621 section 2.3
// defines the name condition as "The Mailbox 'name' property contains
// the given string", and both Fastmail and Stalwart answer a query for
// "Work" with "Workshop" as well, so the exact name and parent are
// matched here against what Mailbox/get returned. A mailbox at the
// root carries parentId null, which decodes to the empty id, and both
// servers send it that way.
func (m *mailSource) FindMailboxes(ctx context.Context, name, parentID string) ([]string, error) {
	req := &jmap.Request{}
	queryCall := req.Invoke(&jmap.MailboxQuery{
		Account: m.session.accountID,
		Filter:  &jmap.MailboxFilterCondition{Name: name},
	})
	getCall := req.Invoke(&jmap.MailboxGet{
		Account:      m.session.accountID,
		Properties:   mailboxLookupProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: queryCall, Name: "Mailbox/query", Path: "/ids"},
	})

	resp, err := m.session.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("jmapsource: find mailboxes: %w", err)
	}
	// The query is read back on its own so a failure there reports as
	// itself. The get names it by back-reference, so a failed query
	// reaches the get as an unresolvable reference instead.
	if _, err := findResponse[*jmap.MailboxQueryResponse](resp, queryCall); err != nil {
		return nil, fmt.Errorf("jmapsource: find mailboxes: query: %w", err)
	}
	get, err := findResponse[*jmap.MailboxGetResponse](resp, getCall)
	if err != nil {
		return nil, fmt.Errorf("jmapsource: find mailboxes: %w", err)
	}

	var ids []string
	for _, box := range get.List {
		if box.Name == name && string(box.ParentID) == parentID {
			ids = append(ids, string(box.ID))
		}
	}
	return ids, nil
}

// mailboxSet runs one Mailbox/set call and returns its response,
// naming op ("create mailbox", "rename mailbox", "delete mailbox")
// in whatever transport error it wraps. CreateMailbox, RenameMailbox,
// and DeleteMailbox otherwise differ only in which field of set they
// populate and which of the response's three result maps they check.
func (s *Session) mailboxSet(ctx context.Context, op string, set *jmap.MailboxSet) (*jmap.MailboxSetResponse, error) {
	req := &jmap.Request{}
	callID := req.Invoke(set)
	resp, err := s.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("jmapsource: %s: %w", op, err)
	}
	sr, err := findResponse[*jmap.MailboxSetResponse](resp, callID)
	if err != nil {
		return nil, fmt.Errorf("jmapsource: %s: %w", op, err)
	}
	return sr, nil
}

// Search implements backend.Mail (SR-7): a caller checks
// Capabilities().ServerSearch before calling.
func (m *mailSource) Search(ctx context.Context, query string) ([]string, error) {
	req := &jmap.Request{}
	callID := req.Invoke(&jmap.EmailQuery{
		Account: m.session.accountID,
		Filter:  &jmap.EmailFilterCondition{Text: query},
		Sort:    []*jmap.Comparator{{Property: "receivedAt", IsAscending: new(false)}},
		Limit:   searchResultLimit,
	})
	resp, err := m.session.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("jmapsource: search: %w", err)
	}
	qr, err := findResponse[*jmap.EmailQueryResponse](resp, callID)
	if err != nil {
		return nil, fmt.Errorf("jmapsource: search: %w", err)
	}
	ids := make([]string, len(qr.IDs))
	for i, id := range qr.IDs {
		ids[i] = string(id)
	}
	return ids, nil
}
