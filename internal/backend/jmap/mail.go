package jmap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"

	"github.com/glw907/poplar/internal/backend"
)

// searchResultLimit bounds Search's Email/query so a broad term
// against a large mailbox returns one page rather than the whole
// match set; SR-7's ranking and further paging are the search
// engine's job (pass 3), not this transport's.
const searchResultLimit = 256

// mailSource implements backend.Mail against one Session.
type mailSource struct {
	session *Session
}

var _ backend.Mail = (*mailSource)(nil)

// ApplyBatch implements backend.Source, translating mutations into
// one Email/set call. A MutationCreate fails per-mutation rather than
// the whole batch: message creation from structured fields is compose
// assembly (pass 4), so it is not supported here.
func (m *mailSource) ApplyBatch(ctx context.Context, mutations []backend.Mutation) (backend.BatchResult, error) {
	result := backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}
	set := &email.Set{Account: m.session.accountID, Update: map[jmap.ID]jmap.Patch{}}
	for _, mut := range mutations {
		switch mut.Op {
		case backend.MutationUpdate:
			set.Update[jmap.ID(mut.ID)] = messagePatch(mut.Fields)
		case backend.MutationDestroy:
			set.Destroy = append(set.Destroy, jmap.ID(mut.ID))
		case backend.MutationCreate:
			result.Failed[mut.CreationID] = errors.New("jmap: message create needs compose assembly (pass 4)")
		default:
			return backend.BatchResult{}, fmt.Errorf("jmap: apply batch: unsupported op %v", mut.Op)
		}
	}
	if len(set.Update) == 0 && len(set.Destroy) == 0 {
		return result, nil
	}

	req := &jmap.Request{Context: ctx}
	callID := req.Invoke(set)
	resp, err := m.session.do(req)
	if err != nil {
		return backend.BatchResult{}, fmt.Errorf("jmap: email/set: %w", err)
	}
	sr, err := findResponse[*email.SetResponse](resp, callID)
	if err != nil {
		if isStateMismatch(err) {
			return backend.BatchResult{}, backend.ErrStateMismatch
		}
		return backend.BatchResult{}, fmt.Errorf("jmap: email/set: %w", err)
	}

	for id, se := range sr.NotUpdated {
		result.Failed[string(id)] = errors.New(se.Type)
	}
	for id, se := range sr.NotDestroyed {
		if se.Type == "notFound" {
			continue
		}
		result.Failed[string(id)] = errors.New(se.Type)
	}
	return result, nil
}

// messagePatch translates mut's poplar-vocabulary fields into a JMAP
// Patch, the inverse of messageFields.
func messagePatch(fields map[string]any) jmap.Patch {
	patch := jmap.Patch{}
	for name, keyword := range messageFlagKeywords {
		v, ok := fields[name]
		if !ok {
			continue
		}
		if set, _ := v.(bool); set {
			patch["keywords/"+keyword] = true
		} else {
			patch["keywords/"+keyword] = nil
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
		return nil, fmt.Errorf("jmap: fetch bodies: %w", err)
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

// resolveBlobIDs returns each id's blobId, consulting the cache
// Changes populates first and issuing one Email/get for whatever is
// missing.
func (s *Session) resolveBlobIDs(ctx context.Context, ids []string) (map[string]jmap.ID, error) {
	out := make(map[string]jmap.ID, len(ids))
	var missing []jmap.ID
	s.mu.Lock()
	for _, id := range ids {
		if blobID, ok := s.blobIDs[id]; ok {
			out[id] = blobID
		} else {
			missing = append(missing, jmap.ID(id))
		}
	}
	s.mu.Unlock()
	if len(missing) == 0 {
		return out, nil
	}

	req := &jmap.Request{Context: ctx}
	callID := req.Invoke(&email.Get{Account: s.accountID, IDs: missing, Properties: []string{"id", "blobId"}})
	resp, err := s.do(req)
	if err != nil {
		return nil, fmt.Errorf("jmap: email/get blobid: %w", err)
	}
	get, err := findResponse[*email.GetResponse](resp, callID)
	if err != nil {
		return nil, fmt.Errorf("jmap: email/get blobid: %w", err)
	}

	s.mu.Lock()
	for _, e := range get.List {
		s.blobIDs[string(e.ID)] = e.BlobID
	}
	s.mu.Unlock()
	for _, e := range get.List {
		out[string(e.ID)] = e.BlobID
	}
	return out, nil
}

func (s *Session) downloadBlob(ctx context.Context, blobID jmap.ID) ([]byte, error) {
	rc, err := s.client.DownloadWithContext(ctx, s.accountID, blobID)
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
	sentID, err := m.session.mailboxIDByRole(ctx, mailbox.RoleSent)
	if err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: %w", err)
	}
	identityID, err := m.session.defaultIdentityID(ctx)
	if err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: %w", err)
	}
	upload, err := m.session.client.UploadWithContext(ctx, m.session.accountID, bytes.NewReader(raw))
	if err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: upload: %w", err)
	}

	req := &jmap.Request{Context: ctx}
	importCall := req.Invoke(&email.Import{
		Account: m.session.accountID,
		Emails: map[string]*email.EmailImport{
			"m1": {
				BlobID:     upload.ID,
				MailboxIDs: map[jmap.ID]bool{sentID: true},
				Keywords:   map[string]bool{"$seen": true},
			},
		},
	})
	submitCall := req.Invoke(&emailsubmission.Set{
		Account: m.session.accountID,
		Create: map[jmap.ID]*emailsubmission.EmailSubmission{
			"s1": {IdentityID: identityID, EmailID: jmap.ID("#m1")},
		},
	})
	resp, err := m.session.do(req)
	if err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: %w", err)
	}
	if _, err := findResponse[*email.ImportResponse](resp, importCall); err != nil {
		return backend.SubmitResult{}, fmt.Errorf("jmap: submit: import: %w", err)
	}
	sr, err := findResponse[*emailsubmission.SetResponse](resp, submitCall)
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

func (s *Session) mailboxIDByRole(ctx context.Context, role mailbox.Role) (jmap.ID, error) {
	req := &jmap.Request{Context: ctx}
	callID := req.Invoke(&mailbox.Query{Account: s.accountID, Filter: &mailbox.FilterCondition{Role: role}})
	resp, err := s.do(req)
	if err != nil {
		return "", fmt.Errorf("jmap: mailbox/query role %s: %w", role, err)
	}
	qr, err := findResponse[*mailbox.QueryResponse](resp, callID)
	if err != nil {
		return "", fmt.Errorf("jmap: mailbox/query role %s: %w", role, err)
	}
	if len(qr.IDs) == 0 {
		return "", fmt.Errorf("jmap: no mailbox with role %s", role)
	}
	return qr.IDs[0], nil
}

func (s *Session) defaultIdentityID(ctx context.Context) (jmap.ID, error) {
	req := &jmap.Request{Context: ctx}
	callID := req.Invoke(&identity.Get{Account: s.accountID})
	resp, err := s.do(req)
	if err != nil {
		return "", fmt.Errorf("jmap: identity/get: %w", err)
	}
	gr, err := findResponse[*identity.GetResponse](resp, callID)
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
	box := &mailbox.Mailbox{Name: name}
	if parentID != "" {
		box.ParentID = jmap.ID(parentID)
	}
	req := &jmap.Request{Context: ctx}
	callID := req.Invoke(&mailbox.Set{Account: m.session.accountID, Create: map[jmap.ID]*mailbox.Mailbox{"m1": box}})
	resp, err := m.session.do(req)
	if err != nil {
		return "", fmt.Errorf("jmap: create mailbox: %w", err)
	}
	sr, err := findResponse[*mailbox.SetResponse](resp, callID)
	if err != nil {
		return "", fmt.Errorf("jmap: create mailbox: %w", err)
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
	req := &jmap.Request{Context: ctx}
	callID := req.Invoke(&mailbox.Set{
		Account: m.session.accountID,
		Update:  map[jmap.ID]jmap.Patch{jmap.ID(id): {"name": name}},
	})
	resp, err := m.session.do(req)
	if err != nil {
		return fmt.Errorf("jmap: rename mailbox: %w", err)
	}
	sr, err := findResponse[*mailbox.SetResponse](resp, callID)
	if err != nil {
		return fmt.Errorf("jmap: rename mailbox: %w", err)
	}
	if se, bad := sr.NotUpdated[jmap.ID(id)]; bad {
		return fmt.Errorf("jmap: rename mailbox: rejected: %s", se.Type)
	}
	return nil
}

// DeleteMailbox implements backend.Mail.
func (m *mailSource) DeleteMailbox(ctx context.Context, id string) error {
	req := &jmap.Request{Context: ctx}
	callID := req.Invoke(&mailbox.Set{Account: m.session.accountID, Destroy: []jmap.ID{jmap.ID(id)}})
	resp, err := m.session.do(req)
	if err != nil {
		return fmt.Errorf("jmap: delete mailbox: %w", err)
	}
	sr, err := findResponse[*mailbox.SetResponse](resp, callID)
	if err != nil {
		return fmt.Errorf("jmap: delete mailbox: %w", err)
	}
	if se, bad := sr.NotDestroyed[jmap.ID(id)]; bad && se.Type != "notFound" {
		return fmt.Errorf("jmap: delete mailbox: rejected: %s", se.Type)
	}
	return nil
}

// Search implements backend.Mail (SR-7): a caller checks
// Capabilities().ServerSearch before calling.
func (m *mailSource) Search(ctx context.Context, query string) ([]string, error) {
	req := &jmap.Request{Context: ctx}
	callID := req.Invoke(&email.Query{
		Account: m.session.accountID,
		Filter:  &email.FilterCondition{Text: query},
		Sort:    []*email.SortComparator{{Property: "receivedAt", IsAscending: false}},
		Limit:   searchResultLimit,
	})
	resp, err := m.session.do(req)
	if err != nil {
		return nil, fmt.Errorf("jmap: search: %w", err)
	}
	qr, err := findResponse[*email.QueryResponse](resp, callID)
	if err != nil {
		return nil, fmt.Errorf("jmap: search: %w", err)
	}
	ids := make([]string, len(qr.IDs))
	for i, id := range qr.IDs {
		ids[i] = string(id)
	}
	return ids, nil
}
