package jmap

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"

	"github.com/glw907/poplar/internal/backend"
)

// defaultPageSize bounds a baseline pull's page when the caller asks
// for the backend's own default (limit 0).
const defaultPageSize = 500

// baselineTokenPrefix marks a NewToken as a baseline pull's resume
// position rather than a genuine JMAP state string, so Changes knows
// to keep paging the Foo/query fallback instead of switching to
// Foo/changes.
const baselineTokenPrefix = "baseline:"

// messageProperties is the Email/get property set Changes hydrates
// into a Record's Fields.
var messageProperties = []string{
	"id", "blobId", "threadId", "mailboxIds", "keywords", "size",
	"receivedAt", "sentAt", "subject", "from", "to", "cc", "bcc",
	"replyTo", "sender", "messageId", "inReplyTo", "references",
	"preview", "hasAttachment",
}

// mailboxProperties is the Mailbox/get property set Changes hydrates
// into a Record's Fields.
var mailboxProperties = []string{
	"id", "name", "parentId", "role", "sortOrder", "totalEmails",
	"unreadEmails", "isSubscribed",
}

// messageFlagKeywords maps poplar's boolean flag vocabulary to the
// JMAP keyword each one wire-encodes as.
var messageFlagKeywords = map[string]string{
	"seen":      "$seen",
	"flagged":   "$flagged",
	"answered":  "$answered",
	"draft":     "$draft",
	"forwarded": "$forwarded",
}

// Changes implements backend.Source for both Message and Mailbox,
// the two collections a Mail source composes (ADR-0004 revision 2).
// An empty token, or one left over from a baseline pull's own
// pagination, asks for the initial sync; anything else is a JMAP
// state string handed to the account-wide Foo/changes method.
func (m *mailSource) Changes(ctx context.Context, kind backend.ObjectKind, token string, limit int) (backend.ChangeSet, error) {
	switch kind {
	case backend.ObjectKindMessage:
		if token == "" || strings.HasPrefix(token, baselineTokenPrefix) {
			return m.session.baselineMessages(ctx, token, limit)
		}
		return m.session.messageChanges(ctx, token, limit)
	case backend.ObjectKindMailbox:
		if token == "" {
			return m.session.baselineMailboxes(ctx)
		}
		return m.session.mailboxChanges(ctx, token, limit)
	default:
		return backend.ChangeSet{}, fmt.Errorf("jmap: changes: unsupported kind %v", kind)
	}
}

// messageChanges runs Email/changes plus two Email/get calls
// back-referencing its created and updated ids, all in one request
// (ADR-0004 revision 2's single-request batching).
func (s *Session) messageChanges(ctx context.Context, token string, limit int) (backend.ChangeSet, error) {
	req := &jmap.Request{Context: ctx}
	changesCall := req.Invoke(&email.Changes{
		Account:    s.accountID,
		SinceState: token,
		MaxChanges: jmapLimit(limit),
	})
	createdCall := req.Invoke(&email.Get{
		Account:      s.accountID,
		Properties:   messageProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: changesCall, Name: "Email/changes", Path: "/created"},
	})
	updatedCall := req.Invoke(&email.Get{
		Account:      s.accountID,
		Properties:   messageProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: changesCall, Name: "Email/changes", Path: "/updated"},
	})

	resp, err := s.do(req)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: email/changes: %w", err)
	}
	changes, err := findResponse[*email.ChangesResponse](resp, changesCall)
	if err != nil {
		if isCannotCalculateChanges(err) {
			return backend.ChangeSet{}, backend.ErrStateReset
		}
		return backend.ChangeSet{}, fmt.Errorf("jmap: email/changes: %w", err)
	}
	created, err := findResponse[*email.GetResponse](resp, createdCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: email/get created: %w", err)
	}
	updated, err := findResponse[*email.GetResponse](resp, updatedCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: email/get updated: %w", err)
	}

	out := backend.ChangeSet{
		NewToken: changes.NewState,
		HasMore:  changes.HasMoreChanges,
		Created:  s.hydrateMessages(created.List),
		Updated:  s.hydrateMessages(updated.List),
	}
	for _, id := range changes.Destroyed {
		out.Destroyed = append(out.Destroyed, string(id))
	}
	return out, nil
}

// baselineMessages pages the account's whole mailbox via Email/query
// plus a back-referenced Email/get, since Email/changes has no
// from-genesis mode (RFC 8621 requires a valid sinceState). NewToken
// carries the next query position while paging, then the real JMAP
// state once the last page lands, so the next call switches over to
// messageChanges automatically.
func (s *Session) baselineMessages(ctx context.Context, token string, limit int) (backend.ChangeSet, error) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	position, err := baselinePosition(token)
	if err != nil {
		return backend.ChangeSet{}, err
	}

	req := &jmap.Request{Context: ctx}
	queryCall := req.Invoke(&email.Query{
		Account:        s.accountID,
		Sort:           []*email.SortComparator{{Property: "receivedAt", IsAscending: false}},
		Position:       position,
		Limit:          jmapLimit(limit),
		CalculateTotal: true,
	})
	getCall := req.Invoke(&email.Get{
		Account:      s.accountID,
		Properties:   messageProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: queryCall, Name: "Email/query", Path: "/ids"},
	})

	resp, err := s.do(req)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline email/query: %w", err)
	}
	query, err := findResponse[*email.QueryResponse](resp, queryCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline email/query: %w", err)
	}
	get, err := findResponse[*email.GetResponse](resp, getCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline email/get: %w", err)
	}

	out := backend.ChangeSet{Created: s.hydrateMessages(get.List)}
	next := position + int64(len(query.IDs))
	if len(query.IDs) == 0 || next >= toInt64(query.Total) {
		out.NewToken = get.State
	} else {
		out.NewToken = baselineTokenPrefix + strconv.FormatInt(next, 10)
		out.HasMore = true
	}
	return out, nil
}

func baselinePosition(token string) (int64, error) {
	if token == "" {
		return 0, nil
	}
	position, err := strconv.ParseInt(strings.TrimPrefix(token, baselineTokenPrefix), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("jmap: baseline token %q: %w", token, err)
	}
	return position, nil
}

// mailboxChanges runs Mailbox/changes plus two Mailbox/get calls
// back-referencing its created and updated ids, all in one request.
func (s *Session) mailboxChanges(ctx context.Context, token string, limit int) (backend.ChangeSet, error) {
	req := &jmap.Request{Context: ctx}
	changesCall := req.Invoke(&mailbox.Changes{
		Account:    s.accountID,
		SinceState: token,
		MaxChanges: jmapLimit(limit),
	})
	createdCall := req.Invoke(&mailbox.Get{
		Account:      s.accountID,
		Properties:   mailboxProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: changesCall, Name: "Mailbox/changes", Path: "/created"},
	})
	updatedCall := req.Invoke(&mailbox.Get{
		Account:      s.accountID,
		Properties:   mailboxProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: changesCall, Name: "Mailbox/changes", Path: "/updated"},
	})

	resp, err := s.do(req)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: mailbox/changes: %w", err)
	}
	changes, err := findResponse[*mailbox.ChangesResponse](resp, changesCall)
	if err != nil {
		if isCannotCalculateChanges(err) {
			return backend.ChangeSet{}, backend.ErrStateReset
		}
		return backend.ChangeSet{}, fmt.Errorf("jmap: mailbox/changes: %w", err)
	}
	created, err := findResponse[*mailbox.GetResponse](resp, createdCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: mailbox/get created: %w", err)
	}
	updated, err := findResponse[*mailbox.GetResponse](resp, updatedCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: mailbox/get updated: %w", err)
	}

	out := backend.ChangeSet{
		NewToken: changes.NewState,
		HasMore:  changes.HasMoreChanges,
	}
	for _, b := range created.List {
		out.Created = append(out.Created, backend.Record{ID: string(b.ID), Fields: mailboxFields(b)})
	}
	for _, b := range updated.List {
		out.Updated = append(out.Updated, backend.Record{ID: string(b.ID), Fields: mailboxFields(b)})
	}
	for _, id := range changes.Destroyed {
		out.Destroyed = append(out.Destroyed, string(id))
	}
	return out, nil
}

// baselineMailboxes fetches every mailbox in one Mailbox/get call.
// Accounts carry at most a few hundred mailboxes, so unlike messages
// this needs no pagination.
func (s *Session) baselineMailboxes(ctx context.Context) (backend.ChangeSet, error) {
	req := &jmap.Request{Context: ctx}
	getCall := req.Invoke(&mailbox.Get{Account: s.accountID, Properties: mailboxProperties})
	resp, err := s.do(req)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline mailbox/get: %w", err)
	}
	get, err := findResponse[*mailbox.GetResponse](resp, getCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline mailbox/get: %w", err)
	}
	out := backend.ChangeSet{NewToken: get.State}
	for _, b := range get.List {
		out.Created = append(out.Created, backend.Record{ID: string(b.ID), Fields: mailboxFields(b)})
	}
	return out, nil
}

// hydrateMessages translates list into Records and caches each
// message's blobId, so FetchBodies can skip a redundant Email/get for
// anything Changes already hydrated.
func (s *Session) hydrateMessages(list []*email.Email) []backend.Record {
	if len(list) == 0 {
		return nil
	}
	s.mu.Lock()
	for _, e := range list {
		s.blobIDs[string(e.ID)] = e.BlobID
	}
	s.mu.Unlock()

	records := make([]backend.Record, 0, len(list))
	for _, e := range list {
		records = append(records, backend.Record{ID: string(e.ID), Fields: messageFields(e)})
	}
	return records
}

// messageFields translates e into poplar's message field vocabulary,
// never exposing a JMAP property name through the seam (ADR-0004
// revision 2).
func messageFields(e *email.Email) map[string]any {
	fields := map[string]any{
		"blob_id":        string(e.BlobID),
		"thread_id":      string(e.ThreadID),
		"subject":        e.Subject,
		"size":           toInt64(e.Size),
		"has_attachment": e.HasAttachment,
		"preview":        e.Preview,
	}
	if e.ReceivedAt != nil {
		fields["received_at"] = *e.ReceivedAt
	}
	if e.SentAt != nil {
		fields["sent_at"] = *e.SentAt
	}
	if len(e.MessageID) > 0 {
		fields["message_id"] = e.MessageID[0]
	}
	if len(e.InReplyTo) > 0 {
		fields["in_reply_to"] = e.InReplyTo[0]
	}
	if len(e.References) > 0 {
		fields["references"] = e.References
	}
	if addrs := addressList(e.From); addrs != nil {
		fields["from"] = addrs
	}
	if addrs := addressList(e.To); addrs != nil {
		fields["to"] = addrs
	}
	if addrs := addressList(e.CC); addrs != nil {
		fields["cc"] = addrs
	}
	if addrs := addressList(e.BCC); addrs != nil {
		fields["bcc"] = addrs
	}
	if addrs := addressList(e.ReplyTo); addrs != nil {
		fields["reply_to"] = addrs
	}
	if addrs := addressList(e.Sender); addrs != nil {
		fields["sender"] = addrs
	}
	if ids := mailboxIDList(e.MailboxIDs); ids != nil {
		fields["mailbox_ids"] = ids
	}
	for name, keyword := range messageFlagKeywords {
		if e.Keywords[keyword] {
			fields[name] = true
		}
	}
	return fields
}

func addressList(addrs []*jmapmail.Address) []map[string]string {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]map[string]string, 0, len(addrs))
	for _, a := range addrs {
		if a == nil {
			continue
		}
		out = append(out, map[string]string{"name": a.Name, "email": a.Email})
	}
	return out
}

func mailboxIDList(ids map[jmap.ID]bool) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for id, present := range ids {
		if present {
			out = append(out, string(id))
		}
	}
	slices.Sort(out)
	return out
}

// mailboxFields translates b into poplar's mailbox field vocabulary.
func mailboxFields(b *mailbox.Mailbox) map[string]any {
	fields := map[string]any{
		"name":          b.Name,
		"role":          string(b.Role),
		"sort_order":    toInt64(b.SortOrder),
		"total_emails":  toInt64(b.TotalEmails),
		"unread_emails": toInt64(b.UnreadEmails),
		"is_subscribed": b.IsSubscribed,
	}
	if b.ParentID != "" {
		fields["parent_id"] = string(b.ParentID)
	}
	return fields
}
