package jmapsource

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/jmap"
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

// changesRoundTrip issues req, already carrying a Foo/changes call
// plus two Foo/get calls back-referencing its created and updated
// ids (ADR-0004 revision 2's single-request batching), and decodes
// all three responses. It turns a cannotCalculateChanges method error
// on the changes call into backend.ErrStateReset; messageChanges and
// mailboxChanges share this decode path and translate the hydrated
// lists into poplar's field vocabulary themselves, since Email and
// Mailbox each need their own translator.
func changesRoundTrip[C, G any](ctx context.Context, s *Session, req *jmap.Request, kind, changesCall, createdCall, updatedCall string) (changes C, created, updated G, err error) {
	resp, err := s.do(ctx, req)
	if err != nil {
		return changes, created, updated, fmt.Errorf("jmap: %s/changes: %w", kind, err)
	}
	changes, err = findResponse[C](resp, changesCall)
	if err != nil {
		if isCannotCalculateChanges(err) {
			return changes, created, updated, backend.ErrStateReset
		}
		return changes, created, updated, fmt.Errorf("jmap: %s/changes: %w", kind, err)
	}
	created, err = findResponse[G](resp, createdCall)
	if err != nil {
		return changes, created, updated, fmt.Errorf("jmap: %s/get created: %w", kind, err)
	}
	updated, err = findResponse[G](resp, updatedCall)
	if err != nil {
		return changes, created, updated, fmt.Errorf("jmap: %s/get updated: %w", kind, err)
	}
	return changes, created, updated, nil
}

// messageChanges runs Email/changes plus two Email/get calls
// back-referencing its created and updated ids, all in one request.
func (s *Session) messageChanges(ctx context.Context, token string, limit int) (backend.ChangeSet, error) {
	req := &jmap.Request{}
	changesCall := req.Invoke(&jmap.EmailChanges{
		Account:    s.accountID,
		SinceState: token,
		MaxChanges: jmapLimit(limit),
	})
	createdCall := req.Invoke(&jmap.EmailGet{
		Account:      s.accountID,
		Properties:   messageProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: changesCall, Name: "Email/changes", Path: "/created"},
	})
	updatedCall := req.Invoke(&jmap.EmailGet{
		Account:      s.accountID,
		Properties:   messageProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: changesCall, Name: "Email/changes", Path: "/updated"},
	})

	changes, created, updated, err := changesRoundTrip[*jmap.EmailChangesResponse, *jmap.EmailGetResponse](
		ctx, s, req, "email", changesCall, createdCall, updatedCall)
	if err != nil {
		return backend.ChangeSet{}, err
	}

	out := backend.ChangeSet{
		NewToken: changes.NewState,
		HasMore:  changes.HasMoreChanges,
		Created:  hydrateMessages(created.List),
		Updated:  hydrateMessages(updated.List),
	}
	for _, id := range changes.Destroyed {
		out.Destroyed = append(out.Destroyed, string(id))
	}
	return out, nil
}

// baselineToken is a baseline pull's resume marker: the query
// position to page from next, and the queryState the position was
// valid against. A mismatch between a resumed token's queryState and
// the server's current one means a destroy shifted positions since
// the last page, so baselineMessages restarts rather than risk
// silently skipping whatever moved past the resume point.
type baselineToken struct {
	position   int64
	queryState string
}

// parseBaselineToken decodes token, or the zero baselineToken for an
// empty token (the start of a fresh baseline pull).
func parseBaselineToken(token string) (baselineToken, error) {
	if token == "" {
		return baselineToken{}, nil
	}
	rest := strings.TrimPrefix(token, baselineTokenPrefix)
	position, queryState, _ := strings.Cut(rest, ":")
	pos, err := strconv.ParseInt(position, 10, 64)
	if err != nil {
		return baselineToken{}, fmt.Errorf("jmap: baseline token %q: %w", token, err)
	}
	return baselineToken{position: pos, queryState: queryState}, nil
}

func (t baselineToken) String() string {
	return baselineTokenPrefix + strconv.FormatInt(t.position, 10) + ":" + t.queryState
}

// baselineMessages pages the account's whole mailbox via Email/query
// plus a back-referenced Email/get, since Email/changes has no
// from-genesis mode (RFC 8621 requires a valid sinceState). NewToken
// carries the next query position and the query's state while
// paging, then the real JMAP state once the last page lands, so the
// next call switches over to messageChanges automatically.
func (s *Session) baselineMessages(ctx context.Context, token string, limit int) (backend.ChangeSet, error) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	prev, err := parseBaselineToken(token)
	if err != nil {
		return backend.ChangeSet{}, err
	}

	req := &jmap.Request{}
	queryCall := req.Invoke(&jmap.EmailQuery{
		Account:        s.accountID,
		Sort:           []*jmap.Comparator{{Property: "receivedAt", IsAscending: new(false)}},
		Position:       prev.position,
		Limit:          jmapLimit(limit),
		CalculateTotal: true,
	})
	getCall := req.Invoke(&jmap.EmailGet{
		Account:      s.accountID,
		Properties:   messageProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: queryCall, Name: "Email/query", Path: "/ids"},
	})

	resp, err := s.do(ctx, req)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline email/query: %w", err)
	}
	query, err := findResponse[*jmap.EmailQueryResponse](resp, queryCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline email/query: %w", err)
	}
	get, err := findResponse[*jmap.EmailGetResponse](resp, getCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline email/get: %w", err)
	}

	if prev.queryState != "" && query.QueryState != prev.queryState {
		return s.baselineMessages(ctx, "", limit)
	}

	out := backend.ChangeSet{Created: hydrateMessages(get.List)}
	next := prev.position + int64(len(query.IDs))
	if len(query.IDs) == 0 || next >= toInt64(query.Total) {
		out.NewToken = get.State
	} else {
		out.NewToken = baselineToken{position: next, queryState: query.QueryState}.String()
		out.HasMore = true
	}
	return out, nil
}

// mailboxChanges runs Mailbox/changes plus two Mailbox/get calls
// back-referencing its created and updated ids, all in one request.
func (s *Session) mailboxChanges(ctx context.Context, token string, limit int) (backend.ChangeSet, error) {
	req := &jmap.Request{}
	changesCall := req.Invoke(&jmap.MailboxChanges{
		Account:    s.accountID,
		SinceState: token,
		MaxChanges: jmapLimit(limit),
	})
	createdCall := req.Invoke(&jmap.MailboxGet{
		Account:      s.accountID,
		Properties:   mailboxProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: changesCall, Name: "Mailbox/changes", Path: "/created"},
	})
	updatedCall := req.Invoke(&jmap.MailboxGet{
		Account:      s.accountID,
		Properties:   mailboxProperties,
		ReferenceIDs: &jmap.ResultReference{ResultOf: changesCall, Name: "Mailbox/changes", Path: "/updated"},
	})

	changes, created, updated, err := changesRoundTrip[*jmap.MailboxChangesResponse, *jmap.MailboxGetResponse](
		ctx, s, req, "mailbox", changesCall, createdCall, updatedCall)
	if err != nil {
		return backend.ChangeSet{}, err
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
	req := &jmap.Request{}
	getCall := req.Invoke(&jmap.MailboxGet{Account: s.accountID, Properties: mailboxProperties})
	resp, err := s.do(ctx, req)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline mailbox/get: %w", err)
	}
	get, err := findResponse[*jmap.MailboxGetResponse](resp, getCall)
	if err != nil {
		return backend.ChangeSet{}, fmt.Errorf("jmap: baseline mailbox/get: %w", err)
	}
	out := backend.ChangeSet{NewToken: get.State}
	for _, b := range get.List {
		out.Created = append(out.Created, backend.Record{ID: string(b.ID), Fields: mailboxFields(b)})
	}
	return out, nil
}

// hydrateMessages translates list into Records.
func hydrateMessages(list []*jmap.Email) []backend.Record {
	if len(list) == 0 {
		return nil
	}
	records := make([]backend.Record, 0, len(list))
	for _, e := range list {
		records = append(records, backend.Record{ID: string(e.ID), Fields: messageFields(e)})
	}
	return records
}

// messageFields translates e into poplar's message field vocabulary,
// never exposing a JMAP property name through the seam (ADR-0004
// revision 2).
func messageFields(e *jmap.Email) map[string]any {
	fields := map[string]any{
		"blob_id":        string(e.BlobID),
		"thread_id":      string(e.ThreadID),
		"subject":        e.Subject,
		"size":           toInt64(e.Size),
		"has_attachment": e.HasAttachment,
		"preview":        e.Preview,
	}
	if e.ReceivedAt != nil {
		fields["received_at"] = e.ReceivedAt.Time()
	}
	if e.SentAt != nil {
		fields["sent_at"] = e.SentAt.Time()
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
	for name, keyword := range backend.MessageFlagKeywords {
		// A keyword absent from e.Keywords is a real "false", not
		// "unknown": messagePatch (the inverse translation) reads a
		// missing key as no change, so a server-side clear must
		// still hydrate as an explicit false rather than vanish.
		fields[name] = e.Keywords[keyword]
	}
	return fields
}

func addressList(addrs []*jmap.Address) []map[string]string {
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
func mailboxFields(b *jmap.Mailbox) map[string]any {
	fields := map[string]any{
		"name":          b.Name,
		"role":          string(b.Role),
		"sort_order":    toInt64(b.SortOrder),
		"total_emails":  toInt64(b.TotalEmails),
		"unread_emails": toInt64(b.UnreadEmails),
		"is_subscribed": boolValue(b.IsSubscribed),
	}
	if b.ParentID != "" {
		fields["parent_id"] = string(b.ParentID)
	}
	return fields
}
