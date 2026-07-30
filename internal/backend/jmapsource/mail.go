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
				return backend.BatchResult{}, fmt.Errorf("jmap: apply batch: mailbox %v is RenameMailbox or DeleteMailbox", mut.Op)
			}
			mailboxes.Create[jmap.ID(mut.CreationID)] = newMailbox(mut.Fields)
		case backend.ObjectKindMessage:
			switch mut.Op {
			case backend.MutationUpdate:
				messages.Update[jmap.ID(mut.ID)] = messagePatch(mut.Fields)
			case backend.MutationDestroy:
				messages.Destroy = append(messages.Destroy, jmap.ID(mut.ID))
			case backend.MutationCreate:
				result.Failed[mut.CreationID] = backend.Failure{
					Class: uerr.ClassServer,
					Cause: errors.New("jmap: message create needs compose assembly (pass 4)"),
				}
			default:
				return backend.BatchResult{}, fmt.Errorf("jmap: apply batch: unsupported op %v", mut.Op)
			}
		default:
			return backend.BatchResult{}, fmt.Errorf("jmap: apply batch: unsupported kind %v", mut.Kind)
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
		return backend.BatchResult{}, fmt.Errorf("jmap: apply batch: %w", err)
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
			result.Failed[string(creationID)] = classifyMutationFailure(se.Type)
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
	return fmt.Errorf("jmap: %s: %w", call, err)
}

// newMailbox translates a mailbox create mutation's poplar-vocabulary
// fields into the wire object, the inverse of mailboxFields.
func newMailbox(fields map[string]any) *jmap.Mailbox {
	box := &jmap.Mailbox{}
	box.Name, _ = fields["name"].(string)
	if parent, _ := fields["parent_id"].(string); parent != "" {
		box.ParentID = jmap.ID(parent)
	}
	return box
}

// messagePatch translates mut's poplar-vocabulary fields into a JMAP
// Patch, the inverse of messageFields.
func messagePatch(fields map[string]any) jmap.Patch {
	patch := jmap.Patch{}
	for name, keyword := range backend.MessageFlagKeywords {
		v, ok := fields[name]
		if !ok {
			continue
		}
		if set, _ := v.(bool); set {
			patch[jmap.Pointer("keywords", keyword)] = true
		} else {
			patch[jmap.Pointer("keywords", keyword)] = nil
		}
	}
	if ids, ok := fields["mailbox_ids"].([]string); ok {
		mailboxIDs := make(map[string]bool, len(ids))
		for _, id := range ids {
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
				if !yield(backend.BodyChunk{ID: id, Err: fmt.Errorf("jmap: fetch bodies: %s: no blob", id)}) {
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
		return nil, fmt.Errorf("jmap: email/get blobid: %w", err)
	}
	get, err := findResponse[*jmap.EmailGetResponse](resp, callID)
	if err != nil {
		return nil, fmt.Errorf("jmap: email/get blobid: %w", err)
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
		return nil, fmt.Errorf("jmap: download: %w", err)
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
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: upload: %w", err)
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
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: %w", err)
	}
	if _, err := findResponse[*jmap.EmailImportResponse](resp, importCall); err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: import: %w", err)
	}
	sr, err := findResponse[*jmap.EmailSubmissionSetResponse](resp, submitCall)
	if err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: %w", err)
	}
	if se, bad := sr.NotCreated["s1"]; bad {
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: rejected: %s", se.Type)
	}
	created, ok := sr.Created["s1"]
	if !ok {
		return backend.SubmitResult{}, errors.New("jmap: submit: no submission created")
	}
	return backend.SubmitResult{ID: string(created.ID), Sent: true}, nil
}

func (s *Session) mailboxIDByRole(ctx context.Context, role jmap.Role) (jmap.ID, error) {
	req := &jmap.Request{}
	callID := req.Invoke(&jmap.MailboxQuery{Account: s.accountID, Filter: &jmap.MailboxFilterCondition{Role: role}})
	resp, err := s.do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("jmap: mailbox/query role %s: %w", role, err)
	}
	qr, err := findResponse[*jmap.MailboxQueryResponse](resp, callID)
	if err != nil {
		return "", fmt.Errorf("jmap: mailbox/query role %s: %w", role, err)
	}
	if len(qr.IDs) == 0 {
		return "", fmt.Errorf("jmap: no mailbox with role %s", role)
	}
	return qr.IDs[0], nil
}

func (s *Session) defaultIdentityID(ctx context.Context) (jmap.ID, error) {
	req := &jmap.Request{}
	callID := req.Invoke(&jmap.IdentityGet{Account: s.accountID})
	resp, err := s.do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("jmap: identity/get: %w", err)
	}
	gr, err := findResponse[*jmap.IdentityGetResponse](resp, callID)
	if err != nil {
		return "", fmt.Errorf("jmap: identity/get: %w", err)
	}
	if len(gr.List) == 0 {
		return "", errors.New("jmap: account has no identities")
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
		return "", fmt.Errorf("jmap: create mailbox: rejected: %s", se.Type)
	}
	created, ok := sr.Created["m1"]
	if !ok {
		return "", errors.New("jmap: create mailbox: no created entry")
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
		return fmt.Errorf("jmap: rename mailbox: rejected: %s", se.Type)
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
		return fmt.Errorf("jmap: delete mailbox: rejected: %s", se.Type)
	}
	return nil
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
		return nil, fmt.Errorf("jmap: %s: %w", op, err)
	}
	sr, err := findResponse[*jmap.MailboxSetResponse](resp, callID)
	if err != nil {
		return nil, fmt.Errorf("jmap: %s: %w", op, err)
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
		return nil, fmt.Errorf("jmap: search: %w", err)
	}
	qr, err := findResponse[*jmap.EmailQueryResponse](resp, callID)
	if err != nil {
		return nil, fmt.Errorf("jmap: search: %w", err)
	}
	ids := make([]string, len(qr.IDs))
	for i, id := range qr.IDs {
		ids[i] = string(id)
	}
	return ids, nil
}
